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
	// Number of peers to gossip with in each round
	FanOut int
	// Interval between gossip rounds
	GossipInterval time.Duration
	// Maximum number of operations to send in each gossip message
	MaxOperations int
}

// GossipManager manages the gossip protocol
type GossipManager struct {
	config GossipConfig
	peers  map[string]*Peer
	mu     sync.RWMutex

	// Operation buffer for gossip
	opBuffer     []*proto.Operation
	opBufferLock sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewGossipManager creates a new gossip manager
func NewGossipManager(cfg GossipConfig) *GossipManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &GossipManager{
		config:   cfg,
		peers:    make(map[string]*Peer),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// AddPeer adds a peer to the gossip network
func (g *GossipManager) AddPeer(peer *Peer) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.peers[peer.ID] = peer
}

// RemovePeer removes a peer from the gossip network
func (g *GossipManager) RemovePeer(peerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.peers, peerID)
}

// Start starts the gossip protocol
func (g *GossipManager) Start() {
	go g.gossipLoop()
}

// Stop stops the gossip protocol
func (g *GossipManager) Stop() {
	g.cancel()
}

// BroadcastOperation broadcasts an operation to the gossip network
func (g *GossipManager) BroadcastOperation(op *proto.Operation) {
	g.opBufferLock.Lock()
	defer g.opBufferLock.Unlock()
	g.opBuffer = append(g.opBuffer, op)
}

func (g *GossipManager) gossipLoop() {
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

func (g *GossipManager) doGossip() {
	g.mu.RLock()
	peers := make([]*Peer, 0, len(g.peers))
	for _, p := range g.peers {
		if p.IsActive() {
			peers = append(peers, p)
		}
	}
	g.mu.RUnlock()

	if len(peers) == 0 {
		return
	}

	// Select random peers to gossip with
	numPeers := min(g.config.FanOut, len(peers))
	selectedPeers := g.selectRandomPeers(peers, numPeers)

	// Get operations to gossip
	ops := g.getOperationsToGossip()
	if len(ops) == 0 {
		return
	}

	// Create gossip message
	msg := &proto.Message{
		Type:       proto.MessageType_GOSSIP,
		Operations: ops,
		Timestamp:  time.Now().UnixNano(),
	}

	// Send to selected peers
	for _, peer := range selectedPeers {
		if err := peer.Send(msg); err != nil {
			// TODO: Handle error
			continue
		}
	}
}

func (g *GossipManager) selectRandomPeers(peers []*Peer, n int) []*Peer {
	if n >= len(peers) {
		return peers
	}

	// Fisher-Yates shuffle
	result := make([]*Peer, len(peers))
	copy(result, peers)
	for i := len(result) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		result[i], result[j] = result[j], result[i]
	}

	return result[:n]
}

func (g *GossipManager) getOperationsToGossip() []*proto.Operation {
	g.opBufferLock.Lock()
	defer g.opBufferLock.Unlock()

	if len(g.opBuffer) == 0 {
		return nil
	}

	numOps := min(g.config.MaxOperations, len(g.opBuffer))
	ops := make([]*proto.Operation, numOps)
	copy(ops, g.opBuffer[:numOps])
	
	// Remove sent operations from buffer
	g.opBuffer = g.opBuffer[numOps:]
	
	return ops
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
