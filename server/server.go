package server

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/network/peer"
	"github.com/luoyjx/crdt-redis/operation"
	"github.com/luoyjx/crdt-redis/proto"
	"github.com/luoyjx/crdt-redis/routing"
	"github.com/luoyjx/crdt-redis/storage"
)

// Server represents the main CRDT Redis server
type Server struct {
	mu          sync.RWMutex
	store       *storage.RedisStore
	opLog       *operation.OperationLog
	replicaID   string
	peerManager *peer.Manager // Peer manager for inter-server communication
	router      *routing.Router
}

// Ensure Server implements peer.OperationHandler
var _ peer.OperationHandler = (*Server)(nil)

// HandleOperation implements the peer.OperationHandler interface
func (s *Server) HandleOperation(ctx context.Context, op *proto.Operation) error {
	log.Printf("Received operation from peer: %v", op)
	return s.applyOperation(op)
}

// Config holds server configuration
type Config struct {
	DataDir      string
	RedisAddr    string
	RedisDB      int
	OpLogPath    string
	ReplicaID    string
	ListenAddr   string // Address to listen for peer connections
	RouterConfig routing.Config
}

// NewServer creates a new CRDT Redis server instance with default configuration
func NewServer(dataDir string, redisAddr string, oplogPath string) (*Server, error) {
	routerCfg := routing.DefaultConfig() // 使用默认路由配置
	routerCfg.SelfID = "server-1"        // 假设一个默认的 Server ID
	routerCfg.Addr = "localhost:8081"    // 假设一个默认的监听地址
	return NewServerWithConfig(Config{
		DataDir:      dataDir,
		RedisAddr:    redisAddr,
		OpLogPath:    oplogPath,
		ReplicaID:    "",        // Will be auto-generated
		RouterConfig: routerCfg, // 传递路由配置
		ListenAddr:   "localhost:8082",
	})
}

// NewServerWithConfig creates a new CRDT Redis server instance with configuration
func NewServerWithConfig(cfg Config) (*Server, error) {
	store, err := storage.NewRedisStore(cfg.RedisAddr, cfg.RedisDB, cfg.ReplicaID)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %v", err)
	}

	opLog, err := operation.NewOperationLog(cfg.OpLogPath)
	if err != nil {
		store.Close() // Clean up store if oplog creation fails
		return nil, fmt.Errorf("failed to create operation log: %v", err)
	}

	// Generate replicaID if not provided
	replicaID := cfg.ReplicaID
	if replicaID == "" {
		replicaID = fmt.Sprintf("replica-%d", time.Now().UnixNano())
	}

	server := &Server{
		store:     store,
		opLog:     opLog,
		replicaID: replicaID,
		router:    routing.NewRouter(cfg.RouterConfig), // Initialize router
	}

	// Initialize peer manager
	peerCfg := peer.ManagerConfig{
		ListenAddr:   cfg.ListenAddr,
		Handler:      server,      // Server itself implements OperationHandler
		MaxRetries:   3,           // Default max retries
		RetryBackoff: time.Second, // Default retry backoff
		Router:       server.router,
	}
	server.peerManager = peer.NewManager(peerCfg)

	// Start peer manager
	if err := server.peerManager.Start(); err != nil {
		store.Close()
		opLog.Close()
		return nil, fmt.Errorf("failed to start peer manager: %v", err)
	}

	// Connect to other peers
	for _, node := range server.router.GetAllNodes() {
		if node.ID != server.replicaID {
			if err := server.peerManager.Connect(context.Background(), node); err != nil {
				log.Printf("Failed to connect to peer %s: %v", node.ID, err)
			}
		}
	}

	return server, nil
}

// applyOperation applies a single operation to the store
func (s *Server) applyOperation(op *proto.Operation) error {
	switch op.Type {
	case proto.OperationType_SET:
		if len(op.Args) != 2 {
			return fmt.Errorf("invalid SET operation args: expected 2, got %d", len(op.Args))
		}
		key, value := op.Args[0], op.Args[1]
		return s.store.Set(context.Background(), key, value, nil)
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
func (s *Server) Set(key string, value string, options *SetOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().UnixNano()
	var expireAt *time.Time

	// apply options
	if options != nil {
		if options.NX {
			if options.NX {
				_, _, _ = s.store.Get(context.Background(), key)
				if options.NX {
					return nil // Key already exists, do nothing
				}
			}
			if options.XX {
				_, _, _ = s.store.Get(context.Background(), key)
				if options.XX {
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
				if _, exists, _ := s.store.Get(context.Background(), key); exists {
					// expireAt = existingValue.ExpireAt // 假设 RedisStore 返回包含 ExpireAt 的 Value 类型
				}
			}
		}

		var ttl *time.Duration
		if expireAt != nil {
			duration := time.Until(*expireAt)
			ttl = &duration
		}

		if err := s.store.Set(context.Background(), key, value, ttl); err != nil {
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

		// Broadcast the operation to other peers
		s.peerManager.Broadcast(op)

		return nil
	}
}

// Get implements the GET command
func (s *Server) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists, _ := s.store.Get(context.Background(), key)
	if !exists {
		return "", false
	}

	return value, exists
}

// Incr implements the INCR command
func (s *Server) Incr(key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().UnixNano()
	_, exists, _ := s.store.Get(context.Background(), key)

	var counter int64
	if exists {
		// always start from 0 for incr command in phase 1
		counter = 0
	}

	counter++

	if err := s.store.Set(context.Background(), key, strconv.FormatInt(counter, 10), nil); err != nil {
		return counter, fmt.Errorf("failed to set counter: %v", err)
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
	// Stop peer manager
	if s.peerManager != nil {
		s.peerManager.Stop()
	}

	// Close resources in reverse order of creation
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close store: %v", err)
	}
	if err := s.opLog.Close(); err != nil {
		return fmt.Errorf("failed to close operation log: %v", err)
	}
	return nil
}
