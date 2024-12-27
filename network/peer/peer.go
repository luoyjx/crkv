package peer

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/network/protocol"
	"github.com/luoyjx/crdt-redis/proto"
)

// Peer represents a remote peer in the CRDT network
type Peer struct {
	ID        string
	Addr      string
	conn      net.Conn
	codec     *protocol.Codec
	sendCh    chan *proto.Operation
	closeCh   chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	lastSeen  time.Time
}

// NewPeer creates a new peer instance
func NewPeer(id string, addr string, conn net.Conn) *Peer {
	return &Peer{
		ID:       id,
		Addr:     addr,
		conn:     conn,
		codec:    protocol.NewCodec(conn),
		sendCh:   make(chan *proto.Operation, 1000),
		closeCh:  make(chan struct{}),
		lastSeen: time.Now(),
	}
}

// Start starts the peer's send and receive loops
func (p *Peer) Start(ctx context.Context, handler OperationHandler) {
	go p.sendLoop(ctx)
	go p.receiveLoop(ctx, handler)
}

// Send sends an operation to the peer
func (p *Peer) Send(op *proto.Operation) error {
	select {
	case p.sendCh <- op:
		return nil
	default:
		return fmt.Errorf("send channel full for peer %s", p.ID)
	}
}

// Close closes the peer connection
func (p *Peer) Close() {
	p.closeOnce.Do(func() {
		close(p.closeCh)
		p.conn.Close()
	})
}

func (p *Peer) sendLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second) // Heartbeat interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.closeCh:
			return
		case op := <-p.sendCh:
			if err := p.codec.WriteMessage(&protocol.Message{
				Type:      protocol.MessageTypeOperation,
				Operation: op,
			}); err != nil {
				p.Close()
				return
			}
		case <-ticker.C:
			// Send heartbeat
			if err := p.codec.WriteMessage(&protocol.Message{
				Type: protocol.MessageTypeHeartbeat,
			}); err != nil {
				p.Close()
				return
			}
		}
	}
}

func (p *Peer) receiveLoop(ctx context.Context, handler OperationHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.closeCh:
			return
		default:
		}

		msg, err := p.codec.ReadMessage()
		if err != nil {
			p.Close()
			return
		}

		p.mu.Lock()
		p.lastSeen = time.Now()
		p.mu.Unlock()

		switch msg.Type {
		case protocol.MessageTypeOperation:
			if msg.Operation != nil && handler != nil {
				if err := handler.HandleOperation(ctx, msg.Operation); err != nil {
					// Log error but continue processing
					continue
				}
			}
		case protocol.MessageTypeHeartbeat:
			// Update last seen time only
			continue
		}
	}
}

// IsAlive returns true if the peer is considered alive
func (p *Peer) IsAlive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Since(p.lastSeen) < 5*time.Second
}

// OperationHandler handles operations received from peers
type OperationHandler interface {
	HandleOperation(ctx context.Context, op *proto.Operation) error
}
