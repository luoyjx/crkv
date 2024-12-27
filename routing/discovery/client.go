package discovery

import (
	"context"
	"sync"

	"github.com/luoyjx/crdt-redis/routing"
)

// Client handles service discovery and maintains a local cache of nodes
type Client struct {
	mu          sync.RWMutex
	registry    *Registry
	nodes       map[string]*routing.Node
	router      *routing.Router
	updatesChan <-chan *RegistryEvent
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewClient creates a new discovery client
func NewClient(registry *Registry, router *routing.Router) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		registry: registry,
		nodes:    make(map[string]*routing.Node),
		router:   router,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins watching for registry updates
func (c *Client) Start() error {
	// Initial load of all nodes
	nodes := c.registry.GetAllNodes(c.ctx)
	c.mu.Lock()
	for _, node := range nodes {
		c.nodes[node.ID] = node
		c.router.AddNode(node)
	}
	c.mu.Unlock()

	// Start watching for updates
	c.updatesChan = c.registry.Watch(c.ctx)
	go c.watchUpdates()

	return nil
}

// Stop stops watching for registry updates
func (c *Client) Stop() {
	c.cancel()
}

func (c *Client) watchUpdates() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case event := <-c.updatesChan:
			if event == nil {
				continue
			}

			c.mu.Lock()
			switch event.Type {
			case EventNodeAdded:
				c.nodes[event.Node.ID] = event.Node
				c.router.AddNode(event.Node)
			case EventNodeRemoved:
				delete(c.nodes, event.Node.ID)
				c.router.RemoveNode(event.Node.ID)
			case EventNodeUpdated:
				c.nodes[event.Node.ID] = event.Node
				// Remove and re-add to update the node in the router
				c.router.RemoveNode(event.Node.ID)
				c.router.AddNode(event.Node)
			}
			c.mu.Unlock()
		}
	}
}

// GetNode returns a node by its ID from the local cache
func (c *Client) GetNode(nodeID string) (*routing.Node, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	node, ok := c.nodes[nodeID]
	return node, ok
}

// GetAllNodes returns all nodes from the local cache
func (c *Client) GetAllNodes() []*routing.Node {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodes := make([]*routing.Node, 0, len(c.nodes))
	for _, node := range c.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetClosestNode returns the closest node to the given location
func (c *Client) GetClosestNode(ctx context.Context, location *routing.Location) (*routing.Node, error) {
	return c.router.GetClosestNode(ctx, location)
}
