package peer

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
)

// Peer represents a remote peer in the CRDT network
type Peer struct {
	ID        string
	Addr      string
	conn      net.Conn
	mu        sync.RWMutex
	lastSeen  time.Time
	sendChan  chan *proto.Message
	recvChan  chan *proto.Message
	ctx       context.Context
	cancel    context.CancelFunc
	isActive  bool
}

// PeerConfig holds configuration for a peer
type PeerConfig struct {
	ID       string
	Addr     string
	ConnType string // tcp, unix, etc.
}

// NewPeer creates a new peer instance
func NewPeer(cfg PeerConfig) *Peer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Peer{
		ID:       cfg.ID,
		Addr:     cfg.Addr,
		sendChan: make(chan *proto.Message, 100),
		recvChan: make(chan *proto.Message, 100),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Connect establishes connection with the peer
func (p *Peer) Connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isActive {
		return nil
	}

	conn, err := net.Dial("tcp", p.Addr)
	if err != nil {
		return fmt.Errorf("failed to connect to peer %s: %v", p.ID, err)
	}

	p.conn = conn
	p.isActive = true
	p.lastSeen = time.Now()

	// Start send/receive loops
	go p.sendLoop()
	go p.receiveLoop()

	return nil
}

// Close closes the peer connection
func (p *Peer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isActive {
		return nil
	}

	p.cancel()
	p.isActive = false
	close(p.sendChan)
	close(p.recvChan)

	if p.conn != nil {
		return p.conn.Close()
	}

	return nil
}

// Send sends a message to the peer
func (p *Peer) Send(msg *proto.Message) error {
	if !p.isActive {
		return fmt.Errorf("peer %s is not active", p.ID)
	}

	select {
	case p.sendChan <- msg:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("peer %s context cancelled", p.ID)
	default:
		return fmt.Errorf("send channel full for peer %s", p.ID)
	}
}

// Receive returns a channel for receiving messages from the peer
func (p *Peer) Receive() <-chan *proto.Message {
	return p.recvChan
}

// IsActive returns whether the peer is currently active
func (p *Peer) IsActive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isActive
}

func (p *Peer) sendLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case msg := <-p.sendChan:
			if err := p.writeMessage(msg); err != nil {
				p.handleError(err)
				return
			}
		}
	}
}

func (p *Peer) receiveLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			msg, err := p.readMessage()
			if err != nil {
				p.handleError(err)
				return
			}
			p.recvChan <- msg
		}
	}
}

func (p *Peer) writeMessage(msg *proto.Message) error {
	// TODO: Implement message serialization and writing
	return nil
}

func (p *Peer) readMessage() (*proto.Message, error) {
	// TODO: Implement message deserialization and reading
	return nil, nil
}

func (p *Peer) handleError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isActive {
		p.isActive = false
		p.conn.Close()
		// TODO: Implement reconnection logic
	}
}
