package server

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/operation"
	"github.com/luoyjx/crdt-redis/proto"
	"github.com/luoyjx/crdt-redis/storage"
)

// Server represents the main CRDT Redis server
type Server struct {
	mu           sync.RWMutex
	store        *storage.Store
	opLog        *operation.OperationLog
	lastSync     int64
	syncInterval time.Duration
	stopSync     chan struct{}
	syncDone     chan struct{} // New channel to signal when sync is done
}

// Config holds server configuration
type Config struct {
	DataDir      string
	RedisAddr    string
	RedisDB      int
	OpLogPath    string
	SyncInterval time.Duration
}

// NewServerWithConfig creates a new CRDT Redis server instance with configuration
func NewServerWithConfig(cfg Config) (*Server, error) {
	store, err := storage.NewStore(cfg.DataDir, cfg.RedisAddr, cfg.RedisDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %v", err)
	}

	opLog, err := operation.NewOperationLog(cfg.OpLogPath)
	if err != nil {
		store.Close() // Clean up store if oplog creation fails
		return nil, fmt.Errorf("failed to create operation log: %v", err)
	}

	server := &Server{
		store:        store,
		opLog:        opLog,
		syncInterval: cfg.SyncInterval,
		stopSync:     make(chan struct{}),
		syncDone:     make(chan struct{}),
	}

	go server.syncOperations()
	return server, nil
}

// NewServer creates a new CRDT Redis server instance with default configuration
func NewServer(dataDir string, redisAddr string, oplogPath string) (*Server, error) {
	return NewServerWithConfig(Config{
		DataDir:      dataDir,
		RedisAddr:    redisAddr,
		OpLogPath:    oplogPath,
		SyncInterval: 5 * time.Second, // Default sync interval
	})
}

// syncOperations periodically syncs operations between nodes
func (s *Server) syncOperations() {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()
	defer close(s.syncDone)

	for {
		select {
		case <-s.stopSync:
			return
		case <-ticker.C:
			s.mu.Lock()
			lastSync := s.lastSync
			s.lastSync = time.Now().UnixNano()
			s.mu.Unlock()

			// Get operations since last sync
			ops, err := s.opLog.GetOperations(lastSync)
			if err != nil {
				log.Printf("Error getting operations: %v", err)
				continue
			}

			// Apply each operation
			for _, op := range ops {
				if err := s.applyOperation(op); err != nil {
					log.Printf("Error applying operation: %v", err)
				}
			}
		}
	}
}

// applyOperation applies a single operation to the store
func (s *Server) applyOperation(op *proto.Operation) error {
	switch op.Type {
	case proto.OperationType_SET:
		if len(op.Args) != 2 {
			return fmt.Errorf("invalid SET operation args: expected 2, got %d", len(op.Args))
		}
		key, value := op.Args[0], op.Args[1]
		return s.store.Set(key, value, op.Timestamp, op.ReplicaId, nil)
	default:
		return fmt.Errorf("unknown operation type: %v", op.Type)
	}
}

// Set implements the SET command
func (s *Server) Set(key, value string) error {
	// Create operation
	op := &proto.Operation{
		Type:      proto.OperationType_SET,
		Args:      []string{key, value},
		Timestamp: time.Now().UnixNano(),
		ReplicaId: operation.GetReplicaID(),
	}

	// Add to operation log first
	if err := s.opLog.AddOperation(op); err != nil {
		return fmt.Errorf("failed to add operation to log: %v", err)
	}

	// Then apply it
	return s.applyOperation(op)
}

// Get implements the GET command
func (s *Server) Get(key string) (string, bool) {
	return s.store.Get(key)
}

// Close closes the server and its resources
func (s *Server) Close() error {
	close(s.stopSync) // Signal syncOperations to stop
	<-s.syncDone      // Wait for syncOperations to finish

	// Close resources in reverse order of creation
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close store: %v", err)
	}
	if err := s.opLog.Close(); err != nil {
		return fmt.Errorf("failed to close operation log: %v", err)
	}
	return nil
}
