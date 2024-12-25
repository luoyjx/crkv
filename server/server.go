package server

import (
	"fmt"
	"log"
	"strconv"
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
		val := storage.NewStringValue(value, op.Timestamp, op.ReplicaId)
		return s.store.Set(key, val, nil)
	default:
		return fmt.Errorf("unknown operation type: %v", op.Type)
	}
}

// SetOptions 结构体用于存储 SET 命令的选项
type SetOptions struct {
	NX bool
	XX bool
	// Get     bool // GET 选项通常在协议层处理，不影响数据存储
	EX      *time.Duration
	PX      *time.Duration
	EXAT    *time.Time
	PXAT    *time.Time
	Keepttl bool
}

// Set implements the SET command
func (s *Server) Set(key, value string, options *SetOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().UnixNano()
	var expireAt *time.Time

	// apply options
	if options != nil {
		if options.NX {
			_, exists := s.store.Get(key)
			if exists {
				return nil // Key already exists, do nothing
			}
		}
		if options.XX {
			_, exists := s.store.Get(key)
			if !exists {
				return nil // Key does not exist, do nothing
			}
		}

		if options.EX != nil {
			t := time.Now().Add(*options.EX)
			expireAt = &t
		}
		if options.PX != nil {
			t := time.Now().Add(*options.PX)
			expireAt = &t
		}
		if options.EXAT != nil {
			expireAt = options.EXAT
		}
		if options.PXAT != nil {
			expireAt = options.PXAT
		}
		if options.Keepttl {
			if existingValue, exists := s.store.Get(key); exists {
				expireAt = existingValue.ExpireAt()
			}
		}
	}

	val := storage.NewStringValue(value, timestamp, "server1") // TODO: Make replicaID configurable
	val.SetExpireAt(expireAt)

	if err := s.store.Set(key, val, nil); err != nil {
		return fmt.Errorf("failed to set value: %v", err)
	}

	// Log the operation
	op := &proto.Operation{
		OperationId: fmt.Sprintf("%d-%s", timestamp, key),
		Type:        proto.OperationType_SET,
		Command:     "SET",
		Args:        []string{key, value},
		Timestamp:   timestamp,
	}
	if err := s.opLog.AddOperation(op); err != nil {
		return fmt.Errorf("failed to log operation: %v", err)
	}

	return nil
}

// Get implements the GET command
func (s *Server) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.store.Get(key)
	if !exists {
		return "", false
	}

	return value.String(), true
}

// Incr implements the INCR command
func (s *Server) Incr(key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().UnixNano()
	value, exists := s.store.Get(key)

	var counter int64
	if exists {
		if value.Type != storage.TypeCounter {
			// Convert existing string to counter if possible
			if i, err := strconv.ParseInt(value.String(), 10, 64); err == nil {
				counter = i
			} else {
				return 0, fmt.Errorf("value is not an integer")
			}
		} else {
			counter = value.Counter()
		}
	}

	counter++
	val := storage.NewCounterValue(counter, timestamp, "server1") // TODO: Make replicaID configurable

	if err := s.store.Set(key, val, nil); err != nil {
		return 0, fmt.Errorf("failed to set counter: %v", err)
	}

	// Log the operation
	op := &proto.Operation{
		OperationId: fmt.Sprintf("%d-%s", timestamp, key),
		Type:        proto.OperationType_INCR,
		Command:     "INCR",
		Args:        []string{key, strconv.FormatInt(counter, 10)},
		Timestamp:   timestamp,
	}
	if err := s.opLog.AddOperation(op); err != nil {
		return 0, fmt.Errorf("failed to log operation: %v", err)
	}

	return counter, nil
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
