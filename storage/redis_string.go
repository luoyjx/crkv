package storage

import (
	"encoding/binary"
	"strconv"
	"time"
)

// ValueType represents the type of value stored
type ValueType int

const (
	TypeString  ValueType = iota // Regular string with LWW
	TypeCounter                  // String counter with accumulative semantics
)

// Value represents a stored value with CRDT metadata
type Value struct {
	Type      ValueType `json:"type"`      // Type of the value (string or counter)
	Data      []byte    `json:"data"`      // Raw bytes of the value
	Timestamp int64     `json:"timestamp"` // Wall clock timestamp for LWW
	ReplicaID string    `json:"replica_id"`
	TTL       *int64    `json:"ttl,omitempty"`       // TTL in seconds, nil means no expiration
	ExpireAt  time.Time `json:"expire_at,omitempty"` // Absolute expiration time
}

// NewStringValue creates a new Value for regular strings
func NewStringValue(value string, timestamp int64, replicaID string) *Value {
	return &Value{
		Type:      TypeString,
		Data:      []byte(value),
		Timestamp: timestamp,
		ReplicaID: replicaID,
	}
}

// NewCounterValue creates a new Value for counters
func NewCounterValue(counter int64, timestamp int64, replicaID string) *Value {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(counter))
	return &Value{
		Type:      TypeCounter,
		Data:      data,
		Timestamp: timestamp,
		ReplicaID: replicaID,
	}
}

// String returns the string representation of the value
func (v *Value) String() string {
	switch v.Type {
	case TypeString:
		return string(v.Data)
	case TypeCounter:
		counter := v.Counter()
		return strconv.FormatInt(counter, 10)
	default:
		return ""
	}
}

// Counter returns the counter value if type is TypeCounter
func (v *Value) Counter() int64 {
	if v.Type != TypeCounter || len(v.Data) != 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(v.Data))
}

// SetCounter sets the counter value and updates the internal bytes
func (v *Value) SetCounter(counter int64) {
	if v.Type != TypeCounter {
		return
	}
	if len(v.Data) != 8 {
		v.Data = make([]byte, 8)
	}
	binary.BigEndian.PutUint64(v.Data, uint64(counter))
}

func (v *Value) SetExpireAt(expireAt *time.Time) {
	v.TTL = new(int64)
	*v.TTL = int64(time.Until(*expireAt).Seconds())
	v.ExpireAt = *expireAt
}

// Merge merges another Value into this one according to CRDT rules
func (v *Value) Merge(other *Value) {
	if v.Type != other.Type {
		// If types don't match, use the most recent value
		if other.Timestamp > v.Timestamp {
			*v = *other
		}
		return
	}

	switch v.Type {
	case TypeString:
		// Last-Write-Wins for regular strings
		if other.Timestamp > v.Timestamp {
			v.Data = other.Data
			v.Timestamp = other.Timestamp
			v.ReplicaID = other.ReplicaID
		}
	case TypeCounter:
		// Accumulate counter values
		newCounter := v.Counter() + other.Counter()
		v.SetCounter(newCounter)
		// Take the latest timestamp
		if other.Timestamp > v.Timestamp {
			v.Timestamp = other.Timestamp
			v.ReplicaID = other.ReplicaID
		}
	}

	// Merge TTL - take the longer TTL if both exist
	if v.TTL != nil && other.TTL != nil {
		if time.Until(other.ExpireAt) > time.Until(v.ExpireAt) {
			v.TTL = other.TTL
			v.ExpireAt = other.ExpireAt
		}
	} else if other.TTL != nil {
		v.TTL = other.TTL
		v.ExpireAt = other.ExpireAt
	}
}
