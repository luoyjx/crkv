package protocol

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/luoyjx/crdt-redis/proto"
)

// Handler handles protocol messages
type Handler struct {
	mu               sync.RWMutex
	operationHandler OperationHandler
	handshakeHandler HandshakeHandler
	errorHandler     ErrorHandler
}

// OperationHandler handles operation messages
type OperationHandler interface {
	HandleOperation(ctx context.Context, op *proto.Operation) error
}

// HandshakeHandler handles handshake messages
type HandshakeHandler interface {
	HandleHandshake(ctx context.Context, handshake *HandshakeData) error
}

// ErrorHandler handles protocol errors
type ErrorHandler interface {
	HandleError(ctx context.Context, err error)
}

// NewHandler creates a new protocol handler
func NewHandler(opHandler OperationHandler, handshakeHandler HandshakeHandler, errorHandler ErrorHandler) *Handler {
	return &Handler{
		operationHandler: opHandler,
		handshakeHandler: handshakeHandler,
		errorHandler:     errorHandler,
	}
}

// HandleMessage handles an incoming protocol message
func (h *Handler) HandleMessage(ctx context.Context, msg *Message) error {
	switch msg.Type {
	case MessageTypeOperation:
		if msg.Operation == nil {
			return fmt.Errorf("operation message with nil operation")
		}
		if h.operationHandler != nil {
			return h.operationHandler.HandleOperation(ctx, msg.Operation)
		}

	case MessageTypeHandshake:
		if msg.Handshake == nil {
			return fmt.Errorf("handshake message with nil handshake data")
		}
		if h.handshakeHandler != nil {
			return h.handshakeHandler.HandleHandshake(ctx, msg.Handshake)
		}

	case MessageTypeHeartbeat:
		// Heartbeat messages are handled at the peer level
		return nil

	default:
		return fmt.Errorf("unknown message type: %d", msg.Type)
	}

	return nil
}

// HandleError handles a protocol error
func (h *Handler) HandleError(ctx context.Context, err error) {
	if h.errorHandler != nil {
		h.errorHandler.HandleError(ctx, err)
	} else {
		log.Printf("Protocol error: %v", err)
	}
}

// DefaultErrorHandler provides a default implementation of ErrorHandler
type DefaultErrorHandler struct{}

// HandleError implements ErrorHandler
func (h *DefaultErrorHandler) HandleError(ctx context.Context, err error) {
	log.Printf("Protocol error: %v", err)
}

// NoOpHandshakeHandler provides a no-op implementation of HandshakeHandler
type NoOpHandshakeHandler struct{}

// HandleHandshake implements HandshakeHandler
func (h *NoOpHandshakeHandler) HandleHandshake(ctx context.Context, handshake *HandshakeData) error {
	return nil
}
