package storage

import (
	"encoding/json"
)

// SetElement represents an element in a CRDT set
type SetElement struct {
	Value     string `json:"value"`
	ID        string `json:"id"`        // Unique element ID (timestamp-replicaID-seq)
	Timestamp int64  `json:"timestamp"` // Wall clock timestamp of addition
	ReplicaID string `json:"replica_id"`
}

// CRDTSet implements an Observed-Remove Set (OR-Set)
type CRDTSet struct {
	Elements   map[string]*SetElement `json:"elements"`   // value -> element mapping
	Tombstones map[string]int64       `json:"tombstones"` // Set of IDs of removed elements -> deletion timestamp
	ReplicaID  string                 `json:"replica_id"`
	nextSeq    int64                  // Sequence number for local operations
}

// NewCRDTSet creates a new CRDT set
func NewCRDTSet(replicaID string) *CRDTSet {
	return &CRDTSet{
		Elements:   make(map[string]*SetElement),
		Tombstones: make(map[string]int64),
		ReplicaID:  replicaID,
		nextSeq:    0,
	}
}

// Add adds an element to the set
func (s *CRDTSet) Add(value string, timestamp int64, replicaID string) {
	if replicaID == "" {
		replicaID = s.ReplicaID
	}

	s.nextSeq++
	id := generateElementID(timestamp, replicaID, s.nextSeq)

	element := &SetElement{
		Value:     value,
		ID:        id,
		Timestamp: timestamp,
		ReplicaID: replicaID,
	}

	s.Elements[value] = element
}

// Remove removes an element from the set using observed-remove semantics
func (s *CRDTSet) Remove(value string, timestamp int64) bool {
	element, exists := s.Elements[value]
	if !exists {
		return false
	}

	// Add to tombstones to track removal with timestamp
	s.Tombstones[element.ID] = timestamp
	delete(s.Elements, value)
	return true
}

// Contains checks if an element is in the set
func (s *CRDTSet) Contains(value string) bool {
	_, exists := s.Elements[value]
	return exists
}

// Members returns all members of the set
func (s *CRDTSet) Members() []string {
	members := make([]string, 0, len(s.Elements))
	for value := range s.Elements {
		members = append(members, value)
	}
	return members
}

// Size returns the number of elements in the set
func (s *CRDTSet) Size() int {
	return len(s.Elements)
}

// Merge merges another CRDT set into this one
func (s *CRDTSet) Merge(other *CRDTSet) {
	// Merge elements (add-wins semantics)
	for value, otherElement := range other.Elements {
		// If element is tombstoned in this set, don't add it
		if _, tombstoned := s.Tombstones[otherElement.ID]; tombstoned {
			continue
		}

		// If we don't have this element, add it
		if _, exists := s.Elements[value]; !exists {
			s.Elements[value] = otherElement
		} else {
			// If we have it, keep the one with later timestamp
			if otherElement.Timestamp > s.Elements[value].Timestamp {
				s.Elements[value] = otherElement
			}
		}
	}

	// Merge tombstones and remove tombstoned elements
	for tombstoneID, deletedAt := range other.Tombstones {
		if existingDeletedAt, exists := s.Tombstones[tombstoneID]; exists {
			// Keep the later timestamp
			if deletedAt > existingDeletedAt {
				s.Tombstones[tombstoneID] = deletedAt
			}
		} else {
			s.Tombstones[tombstoneID] = deletedAt
		}

		// Find and remove any element with this ID
		for value, element := range s.Elements {
			if element.ID == tombstoneID {
				delete(s.Elements, value)
				break
			}
		}
	}
}

// GC removes tombstones older than cutoffTimestamp
func (s *CRDTSet) GC(cutoffTimestamp int64) int {
	cleaned := 0
	for id, deletedAt := range s.Tombstones {
		if deletedAt > 0 && deletedAt < cutoffTimestamp {
			delete(s.Tombstones, id)
			cleaned++
		}
	}
	return cleaned
}

// ToJSON serializes the set to JSON
func (s *CRDTSet) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// FromJSON deserializes the set from JSON
func (s *CRDTSet) FromJSON(data []byte) error {
	return json.Unmarshal(data, s)
}
