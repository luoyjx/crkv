package peer

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
)

// GossipConfig holds configuration for the gossip protocol
type GossipConfig struct {
	// Number of peers to push/pull from in each round
	FanOut int
	// Interval between gossip rounds
	GossipInterval time.Duration
	// Maximum number of operations to send in each message
	MaxOperations int
}

// Gossip implements the gossip protocol for operation dissemination
type Gossip struct {
	config     GossipConfig
	manager    *Manager
	operations *OperationBuffer
	ctx        context.Context
	cancel     context.CancelFunc
}

// OperationBuffer is a fixed-size buffer for recent operations
type OperationBuffer struct {
	mu         sync.RWMutex
	operations []*proto.Operation
	maxSize    int
	current    int
}

// NewOperationBuffer creates a new operation buffer
func NewOperationBuffer(size int) *OperationBuffer {
	return &OperationBuffer{
		operations: make([]*proto.Operation, size),
		maxSize:    size,
	}
}

// Add adds an operation to the buffer
func (b *OperationBuffer) Add(op *proto.Operation) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.operations[b.current] = op
	b.current = (b.current + 1) % b.maxSize
}

// GetRecent returns recent operations up to the specified limit
func (b *OperationBuffer) GetRecent(limit int) []*proto.Operation {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*proto.Operation, 0, limit)
	count := 0
	current := b.current

	// Iterate backwards through the buffer
	for i := 0; i < b.maxSize && count < limit; i++ {
		idx := (current - i - 1 + b.maxSize) % b.maxSize
		if b.operations[idx] != nil {
			result = append(result, b.operations[idx])
			count++
		}
	}

	return result
}

// NewGossip creates a new gossip protocol instance
func NewGossip(manager *Manager, config GossipConfig) *Gossip {
	ctx, cancel := context.WithCancel(context.Background())
	return &Gossip{
		config:     config,
		manager:    manager,
		operations: NewOperationBuffer(1000), // Buffer last 1000 operations
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the gossip protocol
func (g *Gossip) Start() {
	go g.gossipLoop()
}

// Stop stops the gossip protocol
func (g *Gossip) Stop() {
	g.cancel()
}

// AddOperation adds an operation to be gossiped
func (g *Gossip) AddOperation(op *proto.Operation) {
	g.operations.Add(op)
}

func (g *Gossip) gossipLoop() {
	ticker := time.NewTicker(g.config.GossipInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.doGossip()
		}
	}
}

func (g *Gossip) doGossip() {
	peers := g.manager.GetPeers()
	if len(peers) == 0 {
		return
	}

	// Randomly select peers to gossip with
	selectedPeers := g.selectPeers(peers, g.config.FanOut)
	if len(selectedPeers) == 0 {
		return
	}

	// Get recent operations to gossip
	ops := g.operations.GetRecent(g.config.MaxOperations)
	if len(ops) == 0 {
		return
	}

	// Send operations to selected peers
	for _, peer := range selectedPeers {
		for _, op := range ops {
			if err := peer.Send(op); err != nil {
				// Log error but continue with other peers
				continue
			}
		}
	}
}

func (g *Gossip) selectPeers(peers []*Peer, count int) []*Peer {
	if len(peers) <= count {
		return peers
	}

	// Fisher-Yates shuffle
	result := make([]*Peer, len(peers))
	copy(result, peers)
	for i := len(result) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		result[i], result[j] = result[j], result[i]
	}

	return result[:count]
}
