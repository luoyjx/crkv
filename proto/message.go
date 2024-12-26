package proto

// MessageType represents the type of message being sent between peers
type MessageType int

const (
	MessageType_UNKNOWN MessageType = iota
	MessageType_GOSSIP
	MessageType_SYNC_REQUEST
	MessageType_SYNC_RESPONSE
)

// Message represents a message sent between peers
type Message struct {
	Type         MessageType  `json:"type"`
	Operations   []*Operation `json:"operations,omitempty"`
	Timestamp    int64        `json:"timestamp"`
	LastSyncTime int64        `json:"last_sync_time,omitempty"`
	SenderID     string       `json:"sender_id"`
}
