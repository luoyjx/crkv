package protocol

import (
	"encoding/binary"
	"fmt"
	"io"

	pb "google.golang.org/protobuf/proto"

	"github.com/luoyjx/crdt-redis/proto"
)

// MessageType represents the type of protocol message
type MessageType uint8

const (
	// MessageTypeOperation represents an operation message
	MessageTypeOperation MessageType = iota + 1
	// MessageTypeHeartbeat represents a heartbeat message
	MessageTypeHeartbeat
	// MessageTypeHandshake represents a handshake message
	MessageTypeHandshake
)

// Message represents a protocol message
type Message struct {
	Type      MessageType
	Operation *proto.Operation
	Handshake *HandshakeData
}

// HandshakeData represents handshake information
type HandshakeData struct {
	NodeID   string
	Addr     string
	Version  string
	Metadata map[string]string
}

// Protocol constants
const (
	MaxMessageSize = 10 * 1024 * 1024 // 10MB
	HeaderSize     = 5                // 1 byte type + 4 bytes length
)

// EncodeMessage encodes a message to a byte slice
func EncodeMessage(msg *Message) ([]byte, error) {
	var payload []byte
	var err error

	switch msg.Type {
	case MessageTypeOperation:
		if msg.Operation == nil {
			return nil, fmt.Errorf("operation message with nil operation")
		}
		payload, err = pb.Marshal(msg.Operation)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal operation: %v", err)
		}

	case MessageTypeHandshake:
		if msg.Handshake == nil {
			return nil, fmt.Errorf("handshake message with nil handshake data")
		}
		payload, err = encodeHandshake(msg.Handshake)
		if err != nil {
			return nil, fmt.Errorf("failed to encode handshake: %v", err)
		}

	case MessageTypeHeartbeat:
		// Heartbeat has no payload
		payload = []byte{}
	}

	if len(payload) > MaxMessageSize-HeaderSize {
		return nil, fmt.Errorf("message payload too large: %d bytes", len(payload))
	}

	// Allocate buffer for header + payload
	buf := make([]byte, HeaderSize+len(payload))

	// Write message type
	buf[0] = byte(msg.Type)

	// Write payload length
	binary.BigEndian.PutUint32(buf[1:HeaderSize], uint32(len(payload)))

	// Copy payload
	copy(buf[HeaderSize:], payload)

	return buf, nil
}

// DecodeMessage decodes a message from a reader
func DecodeMessage(r io.Reader) (*Message, error) {
	// Read header
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("failed to read message header: %v", err)
	}

	// Parse message type
	msgType := MessageType(header[0])

	// Parse payload length
	payloadLen := binary.BigEndian.Uint32(header[1:HeaderSize])

	if payloadLen > MaxMessageSize-HeaderSize {
		return nil, fmt.Errorf("message payload too large: %d bytes", payloadLen)
	}

	// Read payload if present
	var payload []byte
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("failed to read message payload: %v", err)
		}
	}

	msg := &Message{Type: msgType}

	// Parse payload based on message type
	switch msgType {
	case MessageTypeOperation:
		op := &proto.Operation{}
		if err := pb.Unmarshal(payload, op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal operation: %v", err)
		}
		msg.Operation = op

	case MessageTypeHandshake:
		handshake, err := decodeHandshake(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode handshake: %v", err)
		}
		msg.Handshake = handshake

	case MessageTypeHeartbeat:
		// No payload to parse
	}

	return msg, nil
}

// encodeHandshake encodes handshake data to bytes
func encodeHandshake(data *HandshakeData) ([]byte, error) {
	// Calculate total size needed
	size := 4 + len(data.NodeID) + // NodeID (length + string)
		4 + len(data.Addr) + // Addr (length + string)
		4 + len(data.Version) + // Version (length + string)
		4 // Number of metadata entries

	// Add size for metadata entries
	for k, v := range data.Metadata {
		size += 4 + len(k) + 4 + len(v) // Key and value (length + string)
	}

	buf := make([]byte, size)
	offset := 0

	// Write NodeID
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(data.NodeID)))
	offset += 4
	copy(buf[offset:], data.NodeID)
	offset += len(data.NodeID)

	// Write Addr
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(data.Addr)))
	offset += 4
	copy(buf[offset:], data.Addr)
	offset += len(data.Addr)

	// Write Version
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(data.Version)))
	offset += 4
	copy(buf[offset:], data.Version)
	offset += len(data.Version)

	// Write metadata count
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(data.Metadata)))
	offset += 4

	// Write metadata entries
	for k, v := range data.Metadata {
		binary.BigEndian.PutUint32(buf[offset:], uint32(len(k)))
		offset += 4
		copy(buf[offset:], k)
		offset += len(k)

		binary.BigEndian.PutUint32(buf[offset:], uint32(len(v)))
		offset += 4
		copy(buf[offset:], v)
		offset += len(v)
	}

	return buf, nil
}

// decodeHandshake decodes handshake data from bytes
func decodeHandshake(data []byte) (*HandshakeData, error) {
	if len(data) < 16 { // Minimum size for empty strings and metadata count
		return nil, fmt.Errorf("handshake data too short")
	}

	offset := 0
	result := &HandshakeData{
		Metadata: make(map[string]string),
	}

	// Read NodeID
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid handshake data")
	}
	nodeIDLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(nodeIDLen) > len(data) {
		return nil, fmt.Errorf("invalid handshake data")
	}
	result.NodeID = string(data[offset : offset+int(nodeIDLen)])
	offset += int(nodeIDLen)

	// Read Addr
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid handshake data")
	}
	addrLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(addrLen) > len(data) {
		return nil, fmt.Errorf("invalid handshake data")
	}
	result.Addr = string(data[offset : offset+int(addrLen)])
	offset += int(addrLen)

	// Read Version
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid handshake data")
	}
	versionLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(versionLen) > len(data) {
		return nil, fmt.Errorf("invalid handshake data")
	}
	result.Version = string(data[offset : offset+int(versionLen)])
	offset += int(versionLen)

	// Read metadata count
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid handshake data")
	}
	metadataCount := binary.BigEndian.Uint32(data[offset:])
	offset += 4

	// Read metadata entries
	for i := uint32(0); i < metadataCount; i++ {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("invalid handshake data")
		}
		keyLen := binary.BigEndian.Uint32(data[offset:])
		offset += 4
		if offset+int(keyLen) > len(data) {
			return nil, fmt.Errorf("invalid handshake data")
		}
		key := string(data[offset : offset+int(keyLen)])
		offset += int(keyLen)

		if offset+4 > len(data) {
			return nil, fmt.Errorf("invalid handshake data")
		}
		valueLen := binary.BigEndian.Uint32(data[offset:])
		offset += 4
		if offset+int(valueLen) > len(data) {
			return nil, fmt.Errorf("invalid handshake data")
		}
		value := string(data[offset : offset+int(valueLen)])
		offset += int(valueLen)

		result.Metadata[key] = value
	}

	return result, nil
}
