package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// VectorClock represents a vector clock for tracking causality
type VectorClock struct {
	Clock map[string]int64 `json:"clock"` // replica_id -> logical_time
}

// NewVectorClock creates a new vector clock
func NewVectorClock() *VectorClock {
	return &VectorClock{
		Clock: make(map[string]int64),
	}
}

// Increment increments the clock for the given replica
func (vc *VectorClock) Increment(replicaID string) {
	vc.Clock[replicaID]++
}

// Update updates the clock with the maximum of current and other's values
func (vc *VectorClock) Update(other *VectorClock) {
	if other == nil {
		return
	}

	// Update with maximum values from both clocks
	for replicaID, time := range other.Clock {
		if currentTime, exists := vc.Clock[replicaID]; !exists || time > currentTime {
			vc.Clock[replicaID] = time
		}
	}
}

// Compare compares two vector clocks and returns their relationship
// Returns: -1 if vc < other, 1 if vc > other, 0 if concurrent
func (vc *VectorClock) Compare(other *VectorClock) int {
	if other == nil {
		return 1
	}

	// Get all replica IDs from both clocks
	allReplicas := make(map[string]bool)
	for replicaID := range vc.Clock {
		allReplicas[replicaID] = true
	}
	for replicaID := range other.Clock {
		allReplicas[replicaID] = true
	}

	lessOrEqual := true
	greaterOrEqual := true

	for replicaID := range allReplicas {
		myTime := vc.Clock[replicaID]
		otherTime := other.Clock[replicaID]

		if myTime < otherTime {
			greaterOrEqual = false
		}
		if myTime > otherTime {
			lessOrEqual = false
		}
	}

	if lessOrEqual && greaterOrEqual {
		return 0 // Equal
	} else if lessOrEqual {
		return -1 // vc < other
	} else if greaterOrEqual {
		return 1 // vc > other
	} else {
		return 0 // Concurrent
	}
}

// HappensBefore returns true if this clock happens before the other
func (vc *VectorClock) HappensBefore(other *VectorClock) bool {
	return vc.Compare(other) == -1
}

// HappensAfter returns true if this clock happens after the other
func (vc *VectorClock) HappensAfter(other *VectorClock) bool {
	return vc.Compare(other) == 1
}

// IsConcurrent returns true if this clock is concurrent with the other
func (vc *VectorClock) IsConcurrent(other *VectorClock) bool {
	cmp := vc.Compare(other)
	return cmp == 0 && !vc.Equal(other)
}

// HappensAfterOrConcurrent returns true if this clock happens after or is concurrent with the other
func (vc *VectorClock) HappensAfterOrConcurrent(other *VectorClock) bool {
	return vc.HappensAfter(other) || vc.IsConcurrent(other)
}

// Equal returns true if both clocks are equal
func (vc *VectorClock) Equal(other *VectorClock) bool {
	if other == nil {
		return len(vc.Clock) == 0
	}

	// Get all replica IDs from both clocks
	allReplicas := make(map[string]bool)
	for replicaID := range vc.Clock {
		allReplicas[replicaID] = true
	}
	for replicaID := range other.Clock {
		allReplicas[replicaID] = true
	}

	for replicaID := range allReplicas {
		if vc.Clock[replicaID] != other.Clock[replicaID] {
			return false
		}
	}

	return true
}

// Copy creates a copy of the vector clock
func (vc *VectorClock) Copy() *VectorClock {
	newClock := NewVectorClock()
	for replicaID, time := range vc.Clock {
		newClock.Clock[replicaID] = time
	}
	return newClock
}

// String returns a string representation of the vector clock
func (vc *VectorClock) String() string {
	if len(vc.Clock) == 0 {
		return "{}"
	}

	// Sort replica IDs for consistent output
	var replicas []string
	for replicaID := range vc.Clock {
		replicas = append(replicas, replicaID)
	}
	sort.Strings(replicas)

	var parts []string
	for _, replicaID := range replicas {
		if vc.Clock[replicaID] > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", replicaID, vc.Clock[replicaID]))
		}
	}

	return "{" + strings.Join(parts, ",") + "}"
}

// ToJSON serializes the vector clock to JSON
func (vc *VectorClock) ToJSON() ([]byte, error) {
	return json.Marshal(vc)
}

// FromJSON deserializes the vector clock from JSON
func (vc *VectorClock) FromJSON(data []byte) error {
	return json.Unmarshal(data, vc)
}

// GetTime returns the logical time for a specific replica
func (vc *VectorClock) GetTime(replicaID string) int64 {
	return vc.Clock[replicaID]
}

// SetTime sets the logical time for a specific replica
func (vc *VectorClock) SetTime(replicaID string, time int64) {
	vc.Clock[replicaID] = time
}

// GetMaxTime returns the maximum logical time across all replicas
func (vc *VectorClock) GetMaxTime() int64 {
	var maxTime int64
	for _, time := range vc.Clock {
		if time > maxTime {
			maxTime = time
		}
	}
	return maxTime
}

// GetReplicas returns all replica IDs in the vector clock
func (vc *VectorClock) GetReplicas() []string {
	var replicas []string
	for replicaID := range vc.Clock {
		replicas = append(replicas, replicaID)
	}
	sort.Strings(replicas)
	return replicas
}

// Merge merges another vector clock into this one (taking maximum values)
func (vc *VectorClock) Merge(other *VectorClock) {
	if other == nil {
		return
	}

	for replicaID, otherTime := range other.Clock {
		if currentTime, exists := vc.Clock[replicaID]; !exists || otherTime > currentTime {
			vc.Clock[replicaID] = otherTime
		}
	}
}
