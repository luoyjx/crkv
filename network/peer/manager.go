package peer

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
	"github.com/luoyjx/crdt-redis/routing"
)

// Manager manages peer connections and message broadcasting
type Manager struct {
	mu           sync.RWMutex
	peers        map[string]*Peer
	router       *routing.Router
	handler      OperationHandler
	listenAddr   string
	listener     net.Listener
	ctx          context.Context
	cancel       context.CancelFunc
	maxRetries   int
	retryBackoff time.Duration
}

// ManagerConfig holds configuration for the peer manager
type ManagerConfig struct {
	ListenAddr   string
	Router       *routing.Router
	Handler      OperationHandler
	MaxRetries   int
	RetryBackoff time.Duration
}

// NewManager creates a new peer manager
func NewManager(cfg ManagerConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		peers:        make(map[string]*Peer),
		router:       cfg.Router,
		handler:      cfg.Handler,
		listenAddr:   cfg.ListenAddr,
		ctx:          ctx,
		cancel:       cancel,
		maxRetries:   cfg.MaxRetries,
		retryBackoff: cfg.RetryBackoff,
	}
}

// Start starts the peer manager
func (m *Manager) Start() error {
	listener, err := net.Listen("tcp", m.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to start peer listener: %v", err)
	}
	m.listener = listener

	go m.acceptLoop()
	go m.monitorPeers()

	return nil
}

// Stop stops the peer manager
func (m *Manager) Stop() {
	m.cancel()
	if m.listener != nil {
		m.listener.Close()
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, peer := range m.peers {
		peer.Close()
	}
}

// Broadcast sends an operation to all connected peers
func (m *Manager) Broadcast(op *proto.Operation) {
	m.mu.RLock()
	peers := make([]*Peer, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	m.mu.RUnlock()

	for _, peer := range peers {
		if err := peer.Send(op); err != nil {
			// Log error but continue broadcasting
			continue
		}
	}
}

// Connect establishes a connection to a new peer
func (m *Manager) Connect(ctx context.Context, node *routing.Node) error {
	m.mu.Lock()
	if _, exists := m.peers[node.ID]; exists {
		m.mu.Unlock()
		return nil // Already connected
	}
	m.mu.Unlock()

	var (
		conn net.Conn
		err  error
	)

	// Try to connect with retries
	for i := 0; i < m.maxRetries; i++ {
		conn, err = net.DialTimeout("tcp", node.Addr, 5*time.Second)
		if err == nil {
			break
		}
		if i < m.maxRetries-1 {
			time.Sleep(m.retryBackoff)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to peer %s after %d retries: %v", node.ID, m.maxRetries, err)
	}

	peer := NewPeer(node.ID, node.Addr, conn)
	peer.Start(ctx, m.handler)

	m.mu.Lock()
	m.peers[node.ID] = peer
	m.mu.Unlock()

	return nil
}

func (m *Manager) acceptLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		conn, err := m.listener.Accept()
		if err != nil {
			if m.ctx.Err() != nil {
				return
			}
			// Log error and continue accepting
			continue
		}

		go m.handleIncomingConnection(conn)
	}
}

func (m *Manager) handleIncomingConnection(conn net.Conn) {
	// Set deadline for initial handshake
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// TODO: Implement handshake protocol to exchange peer IDs
	// For now, use remote address as peer ID
	peerID := conn.RemoteAddr().String()
	peerAddr := conn.RemoteAddr().String()

	// Reset deadline
	conn.SetDeadline(time.Time{})

	peer := NewPeer(peerID, peerAddr, conn)
	peer.Start(m.ctx, m.handler)

	m.mu.Lock()
	m.peers[peerID] = peer
	m.mu.Unlock()
}

func (m *Manager) monitorPeers() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkPeers()
		}
	}
}

func (m *Manager) checkPeers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, peer := range m.peers {
		if !peer.IsAlive() {
			peer.Close()
			delete(m.peers, id)
		}
	}
}

// GetPeers returns all connected peers
func (m *Manager) GetPeers() []*Peer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]*Peer, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetPeer returns a specific peer by ID
func (m *Manager) GetPeer(id string) (*Peer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer, ok := m.peers[id]
	return peer, ok
}
