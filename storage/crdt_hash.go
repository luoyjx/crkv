package storage

import (
	"encoding/json"
)

// HashField represents a field in a CRDT hash
type HashField struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	ID        string `json:"id"`        // Unique field ID (timestamp-replicaID-seq)
	Timestamp int64  `json:"timestamp"` // Wall clock timestamp of last update
	ReplicaID string `json:"replica_id"`
}

// CRDTHash implements a Last-Write-Wins Hash with field-level granularity
type CRDTHash struct {
	Fields     map[string]*HashField `json:"fields"`     // field key -> field mapping
	Tombstones map[string]struct{}   `json:"tombstones"` // Set of IDs of deleted fields
	ReplicaID  string                `json:"replica_id"`
	nextSeq    int64                 // Sequence number for local operations
}

// NewCRDTHash creates a new CRDT hash
func NewCRDTHash(replicaID string) *CRDTHash {
	return &CRDTHash{
		Fields:     make(map[string]*HashField),
		Tombstones: make(map[string]struct{}),
		ReplicaID:  replicaID,
		nextSeq:    0,
	}
}

// Set sets a field in the hash (Last-Write-Wins semantics)
func (h *CRDTHash) Set(key, value string, timestamp int64, replicaID string) bool {
	if replicaID == "" {
		replicaID = h.ReplicaID
	}

	h.nextSeq++
	id := generateElementID(timestamp, replicaID, h.nextSeq)

	// Check if field exists and compare timestamps
	if existingField, exists := h.Fields[key]; exists {
		if timestamp < existingField.Timestamp {
			return false // Existing field is newer
		}
		if timestamp == existingField.Timestamp && replicaID <= existingField.ReplicaID {
			return false // Tie-breaker: use replica ID
		}
	}

	field := &HashField{
		Key:       key,
		Value:     value,
		ID:        id,
		Timestamp: timestamp,
		ReplicaID: replicaID,
	}

	h.Fields[key] = field
	return true
}

// Get gets a field from the hash
func (h *CRDTHash) Get(key string) (string, bool) {
	field, exists := h.Fields[key]
	if !exists {
		return "", false
	}
	return field.Value, true
}

// Delete removes a field from the hash using observed-remove semantics
func (h *CRDTHash) Delete(key string) bool {
	field, exists := h.Fields[key]
	if !exists {
		return false
	}

	// Add to tombstones to track removal
	h.Tombstones[field.ID] = struct{}{}
	delete(h.Fields, key)
	return true
}

// Keys returns all field keys in the hash
func (h *CRDTHash) Keys() []string {
	keys := make([]string, 0, len(h.Fields))
	for key := range h.Fields {
		keys = append(keys, key)
	}
	return keys
}

// Values returns all field values in the hash
func (h *CRDTHash) Values() []string {
	values := make([]string, 0, len(h.Fields))
	for _, field := range h.Fields {
		values = append(values, field.Value)
	}
	return values
}

// GetAll returns all field key-value pairs
func (h *CRDTHash) GetAll() map[string]string {
	result := make(map[string]string, len(h.Fields))
	for key, field := range h.Fields {
		result[key] = field.Value
	}
	return result
}

// Len returns the number of fields in the hash
func (h *CRDTHash) Len() int {
	return len(h.Fields)
}

// Exists checks if a field exists in the hash
func (h *CRDTHash) Exists(key string) bool {
	_, exists := h.Fields[key]
	return exists
}

// Merge merges another CRDT hash into this one
func (h *CRDTHash) Merge(other *CRDTHash) {
	// Merge fields (LWW semantics)
	for key, otherField := range other.Fields {
		// If field is tombstoned in this hash, don't add it
		if _, tombstoned := h.Tombstones[otherField.ID]; tombstoned {
			continue
		}

		// If we don't have this field, add it
		if _, exists := h.Fields[key]; !exists {
			h.Fields[key] = otherField
		} else {
			// We have the field, apply LWW
			existingField := h.Fields[key]
			if otherField.Timestamp > existingField.Timestamp ||
				(otherField.Timestamp == existingField.Timestamp && otherField.ReplicaID > existingField.ReplicaID) {
				h.Fields[key] = otherField
			}
		}
	}

	// Merge tombstones and remove tombstoned fields
	for tombstoneID := range other.Tombstones {
		h.Tombstones[tombstoneID] = struct{}{}

		// Find and remove any field with this ID
		for key, field := range h.Fields {
			if field.ID == tombstoneID {
				delete(h.Fields, key)
				break
			}
		}
	}
}

// ToJSON serializes the hash to JSON
func (h *CRDTHash) ToJSON() ([]byte, error) {
	return json.Marshal(h)
}

// FromJSON deserializes the hash from JSON
func (h *CRDTHash) FromJSON(data []byte) error {
	return json.Unmarshal(data, h)
}
