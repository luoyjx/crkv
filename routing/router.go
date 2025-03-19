package routing

import (
	"context"
	"fmt"
	"math"
	"sync"
)

// Config holds router configuration
type Config struct {
	SelfID   string
	Addr     string
	Location *Location
}

// DefaultConfig returns a default router configuration
func DefaultConfig() Config {
	return Config{
		SelfID:   "default-server-id",                  // 默认 Server ID
		Addr:     "localhost:8080",                     // 默认监听地址
		Location: &Location{Latitude: 0, Longitude: 0}, // 默认地理位置
	}
}

// Node represents a proxy server node in the cluster
type Node struct {
	ID       string
	Addr     string
	Location *Location
}

// Location represents geographical coordinates
type Location struct {
	Latitude  float64
	Longitude float64
}

// Router handles request routing between nodes
type Router struct {
	mu    sync.RWMutex
	nodes map[string]*Node // map[nodeID]*Node
	self  *Node
}

// NewRouter creates a new Router instance
func NewRouter(cfg Config) *Router {
	self := &Node{
		ID:       cfg.SelfID,
		Addr:     cfg.Addr,
		Location: cfg.Location,
	}

	return &Router{
		nodes: make(map[string]*Node),
		self:  self,
	}
}

// AddNode adds a new node to the routing table
func (r *Router) AddNode(node *Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.nodes[node.ID]; exists {
		return fmt.Errorf("node %s already exists", node.ID)
	}

	r.nodes[node.ID] = node
	return nil
}

// RemoveNode removes a node from the routing table
func (r *Router) RemoveNode(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.nodes, nodeID)
}

// GetClosestNode returns the closest node to the given location
func (r *Router) GetClosestNode(ctx context.Context, clientLocation *Location) (*Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodes) == 0 {
		return r.self, nil // Return self if no other nodes available
	}

	var closestNode *Node
	minDistance := float64(1<<63 - 1) // Max float64

	// Include self in the search
	if r.self.Location != nil && clientLocation != nil {
		minDistance = calculateDistance(clientLocation, r.self.Location)
		closestNode = r.self
	}

	// Check all other nodes
	for _, node := range r.nodes {
		if node.Location == nil || clientLocation == nil {
			continue
		}

		distance := calculateDistance(clientLocation, node.Location)
		if distance < minDistance {
			minDistance = distance
			closestNode = node
		}
	}

	if closestNode == nil {
		return r.self, nil // Default to self if no suitable node found
	}

	return closestNode, nil
}

// GetAllNodes returns all nodes in the routing table
func (r *Router) GetAllNodes() []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*Node, 0, len(r.nodes)+1)
	nodes = append(nodes, r.self)

	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// calculateDistance calculates the Haversine distance between two locations
func calculateDistance(l1, l2 *Location) float64 {
	const earthRadius = 6371.0 // Earth's radius in kilometers

	lat1 := toRadians(l1.Latitude)
	lon1 := toRadians(l1.Longitude)
	lat2 := toRadians(l2.Latitude)
	lon2 := toRadians(l2.Longitude)

	dLat := lat2 - lat1
	dLon := lon2 - lon1

	a := sin(dLat/2)*sin(dLat/2) +
		cos(lat1)*cos(lat2)*sin(dLon/2)*sin(dLon/2)
	c := 2 * atan2(sqrt(a), sqrt(1-a))

	return earthRadius * c
}

func toRadians(degrees float64) float64 {
	return degrees * (math.Pi / 180.0)
}

func sin(x float64) float64 {
	return math.Sin(x)
}

func cos(x float64) float64 {
	return math.Cos(x)
}

func sqrt(x float64) float64 {
	return math.Sqrt(x)
}

func atan2(y, x float64) float64 {
	return math.Atan2(y, x)
}
