package peer

import (
	"context"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
	"github.com/luoyjx/crdt-redis/storage"
)

// SyncConfig holds configuration for the sync manager
type SyncConfig struct {
	// Interval between full state sync attempts
	SyncInterval time.Duration
	// Maximum number of operations to sync in one batch
	BatchSize int
	// Timeout for sync operations
	SyncTimeout time.Duration
}

// SyncManager manages CRDT state synchronization between peers
type SyncManager struct {
	config SyncConfig
	store  *storage.Store
	gossip *GossipManager
	mu     sync.RWMutex

	// Track last sync time with each peer
	lastSync map[string]time.Time

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSyncManager creates a new sync manager
func NewSyncManager(cfg SyncConfig, store *storage.Store, gossip *GossipManager) *SyncManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &SyncManager{
		config:    cfg,
		store:     store,
		gossip:    gossip,
		lastSync:  make(map[string]time.Time),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the sync manager
func (s *SyncManager) Start() {
	go s.syncLoop()
}

// Stop stops the sync manager
func (s *SyncManager) Stop() {
	s.cancel()
}

// HandleOperation handles an incoming operation from a peer
func (s *SyncManager) HandleOperation(op *proto.Operation) error {
	// Apply operation to local store
	// TODO: Implement operation application logic
	return nil
}

func (s *SyncManager) syncLoop() {
	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.performSync()
		}
	}
}

func (s *SyncManager) performSync() {
	// Get active peers from gossip manager
	s.gossip.mu.RLock()
	peers := make([]*Peer, 0, len(s.gossip.peers))
	for _, p := range s.gossip.peers {
		if p.IsActive() {
			peers = append(peers, p)
		}
	}
	s.gossip.mu.RUnlock()

	// Sync with each peer
	for _, peer := range peers {
		if err := s.syncWithPeer(peer); err != nil {
			// TODO: Handle error
			continue
		}
	}
}

func (s *SyncManager) syncWithPeer(peer *Peer) error {
	s.mu.RLock()
	lastSync, ok := s.lastSync[peer.ID]
	s.mu.RUnlock()

	// Create sync message
	msg := &proto.Message{
		Type:      proto.MessageType_SYNC_REQUEST,
		Timestamp: time.Now().UnixNano(),
	}

	if ok {
		msg.LastSyncTime = lastSync.UnixNano()
	}

	// Send sync request
	if err := peer.Send(msg); err != nil {
		return err
	}

	// Update last sync time
	s.mu.Lock()
	s.lastSync[peer.ID] = time.Now()
	s.mu.Unlock()

	return nil
}

// HandleSyncRequest handles an incoming sync request from a peer
func (s *SyncManager) HandleSyncRequest(peer *Peer, msg *proto.Message) error {
	// TODO: Implement sync request handling
	// 1. Get operations since LastSyncTime
	// 2. Send operations in batches
	// 3. Update sync state
	return nil
}

// HandleSyncResponse handles an incoming sync response from a peer
func (s *SyncManager) HandleSyncResponse(peer *Peer, msg *proto.Message) error {
	// TODO: Implement sync response handling
	// 1. Apply received operations
	// 2. Update sync state
	return nil
}
