package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/luoyjx/crdt-redis/routing"
)

// Registry manages node registration and discovery
type Registry struct {
	mu       sync.RWMutex
	nodes    map[string]*routing.Node
	watchers []chan *RegistryEvent
}

// RegistryEvent represents a change in the registry
type RegistryEvent struct {
	Type EventType
	Node *routing.Node
}

// EventType represents the type of registry event
type EventType int

const (
	// EventNodeAdded indicates a node was added
	EventNodeAdded EventType = iota
	// EventNodeRemoved indicates a node was removed
	EventNodeRemoved
	// EventNodeUpdated indicates a node was updated
	EventNodeUpdated
)

// NewRegistry creates a new Registry instance
func NewRegistry() *Registry {
	return &Registry{
		nodes:    make(map[string]*routing.Node),
		watchers: make([]chan *RegistryEvent, 0),
	}
}

// Register registers a node with the registry
func (r *Registry) Register(ctx context.Context, node *routing.Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.nodes[node.ID]; ok {
		if existing.Addr != node.Addr {
			r.nodes[node.ID] = node
			r.notifyWatchers(&RegistryEvent{
				Type: EventNodeUpdated,
				Node: node,
			})
		}
		return nil
	}

	r.nodes[node.ID] = node
	r.notifyWatchers(&RegistryEvent{
		Type: EventNodeAdded,
		Node: node,
	})
	return nil
}

// Deregister removes a node from the registry
func (r *Registry) Deregister(ctx context.Context, nodeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node, ok := r.nodes[nodeID]; ok {
		delete(r.nodes, nodeID)
		r.notifyWatchers(&RegistryEvent{
			Type: EventNodeRemoved,
			Node: node,
		})
	}
	return nil
}

// GetNode returns a node by its ID
func (r *Registry) GetNode(ctx context.Context, nodeID string) (*routing.Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if node, ok := r.nodes[nodeID]; ok {
		return node, nil
	}
	return nil, fmt.Errorf("node %s not found", nodeID)
}

// GetAllNodes returns all registered nodes
func (r *Registry) GetAllNodes(ctx context.Context) []*routing.Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*routing.Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// Watch returns a channel that receives registry events
func (r *Registry) Watch(ctx context.Context) <-chan *RegistryEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan *RegistryEvent, 100)
	r.watchers = append(r.watchers, ch)

	go func() {
		<-ctx.Done()
		r.mu.Lock()
		defer r.mu.Unlock()
		for i, watcher := range r.watchers {
			if watcher == ch {
				r.watchers = append(r.watchers[:i], r.watchers[i+1:]...)
				close(ch)
				break
			}
		}
	}()

	return ch
}

func (r *Registry) notifyWatchers(event *RegistryEvent) {
	for _, watcher := range r.watchers {
		select {
		case watcher <- event:
		default:
			// Skip if channel is full
		}
	}
}
