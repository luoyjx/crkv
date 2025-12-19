package storage

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"time"
)

// ValueType represents the type of value stored
type ValueType int

const (
	TypeString       ValueType = iota // Regular string with LWW
	TypeCounter                       // String counter with accumulative semantics (int64)
	TypeFloatCounter                  // Float counter with accumulative semantics (float64)
	TypeList                          // List with CRDT semantics
	TypeSet                           // Set with CRDT semantics
	TypeHash                          // Hash with CRDT semantics
	TypeZSet                          // Sorted Set with CRDT semantics
)

// Value represents a stored value with CRDT metadata
type Value struct {
	Type        ValueType    `json:"type"`      // Type of the value (string or counter)
	Data        []byte       `json:"data"`      // Raw bytes of the value
	Timestamp   int64        `json:"timestamp"` // Wall clock timestamp for LWW
	ReplicaID   string       `json:"replica_id"`
	VectorClock *VectorClock `json:"vector_clock"`        // Vector clock for causality tracking
	TTL         *int64       `json:"ttl,omitempty"`       // TTL in seconds, nil means no expiration
	ExpireAt    time.Time    `json:"expire_at,omitempty"` // Absolute expiration time
}

// NewStringValue creates a new Value for regular strings
func NewStringValue(value string, timestamp int64, replicaID string) *Value {
	vc := NewVectorClock()
	vc.Increment(replicaID)
	return &Value{
		Type:        TypeString,
		Data:        []byte(value),
		Timestamp:   timestamp,
		ReplicaID:   replicaID,
		VectorClock: vc,
	}
}

// NewCounterValue creates a new Value for counters
func NewCounterValue(counter int64, timestamp int64, replicaID string) *Value {
	vc := NewVectorClock()
	vc.Increment(replicaID)
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(counter))
	return &Value{
		Type:        TypeCounter,
		Data:        data,
		Timestamp:   timestamp,
		ReplicaID:   replicaID,
		VectorClock: vc,
	}
}

// NewFloatCounterValue creates a new Value for float counters
func NewFloatCounterValue(counter float64, timestamp int64, replicaID string) *Value {
	vc := NewVectorClock()
	if replicaID != "" {
		vc.Increment(replicaID)
	}
	// Store float64 as IEEE 754 binary representation
	data := make([]byte, 8)
	bits := math.Float64bits(counter)
	binary.BigEndian.PutUint64(data, bits)
	return &Value{
		Type:        TypeFloatCounter,
		Data:        data,
		Timestamp:   timestamp,
		ReplicaID:   replicaID,
		VectorClock: vc,
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
	case TypeFloatCounter:
		counter := v.FloatCounter()
		return strconv.FormatFloat(counter, 'g', -1, 64)
	case TypeList:
		list := v.List()
		if list == nil {
			return ""
		}
		return fmt.Sprintf("(list with %d elements)", list.Len())
	case TypeSet:
		set := v.Set()
		if set == nil {
			return ""
		}
		return fmt.Sprintf("(set with %d elements)", set.Size())
	case TypeHash:
		hash := v.Hash()
		if hash == nil {
			return ""
		}
		return fmt.Sprintf("(hash with %d fields)", hash.Len())
	case TypeZSet:
		zset, err := v.GetZSet()
		if err != nil || zset == nil {
			return ""
		}
		return fmt.Sprintf("(zset with %d members)", zset.ZCard())
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

// FloatCounter returns the float counter value if type is TypeFloatCounter
func (v *Value) FloatCounter() float64 {
	if v.Type != TypeFloatCounter || len(v.Data) != 8 {
		return 0
	}
	bits := binary.BigEndian.Uint64(v.Data)
	return math.Float64frombits(bits)
}

// SetFloatCounter sets the float counter value and updates the internal bytes
func (v *Value) SetFloatCounter(counter float64) {
	if v.Type != TypeFloatCounter {
		return
	}
	if len(v.Data) != 8 {
		v.Data = make([]byte, 8)
	}
	bits := math.Float64bits(counter)
	binary.BigEndian.PutUint64(v.Data, bits)
}

func (v *Value) SetExpireAt(expireAt *time.Time) {
	v.TTL = new(int64)
	*v.TTL = int64(time.Until(*expireAt).Seconds())
	v.ExpireAt = *expireAt
}

// Merge merges another Value into this one according to CRDT rules
func (v *Value) Merge(other *Value) {
	// Merge vector clocks first
	if v.VectorClock == nil {
		v.VectorClock = NewVectorClock()
	}
	if other.VectorClock != nil {
		v.VectorClock.Update(other.VectorClock)
	}

	if v.Type != other.Type {
		// If types don't match, use the most recent value based on vector clock
		shouldUpdate := false
		if v.VectorClock != nil && other.VectorClock != nil {
			if v.VectorClock.HappensBefore(other.VectorClock) {
				shouldUpdate = true
			} else if v.VectorClock.IsConcurrent(other.VectorClock) {
				// For concurrent updates, use timestamp as tie-breaker
				shouldUpdate = other.Timestamp > v.Timestamp
			}
		} else {
			// Fallback to timestamp comparison
			shouldUpdate = other.Timestamp > v.Timestamp
		}

		if shouldUpdate {
			*v = *other
			if v.VectorClock != nil && other.VectorClock != nil {
				v.VectorClock.Update(other.VectorClock)
			}
		}
		return
	}

	switch v.Type {
	case TypeString:
		// Last-Write-Wins for regular strings using vector clock
		shouldUpdate := false
		if v.VectorClock != nil && other.VectorClock != nil {
			if v.VectorClock.HappensBefore(other.VectorClock) {
				shouldUpdate = true
			} else if v.VectorClock.IsConcurrent(other.VectorClock) {
				// For concurrent updates, use timestamp as tie-breaker
				shouldUpdate = other.Timestamp > v.Timestamp
			}
		} else {
			// Fallback to timestamp comparison
			shouldUpdate = other.Timestamp > v.Timestamp
		}

		if shouldUpdate {
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
		}
	case TypeFloatCounter:
		// Accumulate float counter values (same semantics as integer counter)
		newCounter := v.FloatCounter() + other.FloatCounter()
		v.SetFloatCounter(newCounter)
		// Take the latest timestamp
		if other.Timestamp > v.Timestamp {
			v.Timestamp = other.Timestamp
		}
	case TypeList:
		// Merge CRDT lists
		myList := v.List()
		otherList := other.List()
		if myList != nil && otherList != nil {
			myList.Merge(otherList)
			v.SetList(myList, other.Timestamp)
		} else if otherList != nil {
			v.Data = other.Data
			v.Timestamp = other.Timestamp
		}
	case TypeSet:
		// Merge CRDT sets
		mySet := v.Set()
		otherSet := other.Set()
		if mySet != nil && otherSet != nil {
			mySet.Merge(otherSet)
			v.SetSet(mySet, other.Timestamp)
		} else if otherSet != nil {
			v.Data = other.Data
			v.Timestamp = other.Timestamp
		}
	case TypeHash:
		// Merge CRDT hashes
		myHash := v.Hash()
		otherHash := other.Hash()
		if myHash != nil && otherHash != nil {
			myHash.Merge(otherHash)
			v.SetHash(myHash, other.Timestamp)
		} else if otherHash != nil {
			v.Data = other.Data
			v.Timestamp = other.Timestamp
		}
	case TypeZSet:
		// Merge CRDT sorted sets
		myZSet, _ := v.GetZSet()
		otherZSet, _ := other.GetZSet()
		if myZSet != nil && otherZSet != nil {
			myZSet.Merge(otherZSet)
			v.SetZSet(myZSet)
		} else if otherZSet != nil {
			v.Data = other.Data
			v.Timestamp = other.Timestamp
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
