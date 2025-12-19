package storage

import (
	"encoding/json"
	"fmt"
)

// FieldType represents the type of a hash field
type FieldType int

const (
	FieldTypeString  FieldType = iota // Regular string field with LWW
	FieldTypeCounter                  // Counter field with accumulative semantics
)

// HashField represents a field in a CRDT hash
type HashField struct {
	Key          string    `json:"key"`
	Value        string    `json:"value"`         // String value (for FieldTypeString)
	CounterValue int64     `json:"counter_value"` // Counter value (for FieldTypeCounter)
	FieldType    FieldType `json:"field_type"`    // Type of field
	ID           string    `json:"id"`            // Unique field ID (timestamp-replicaID-seq)
	Timestamp    int64     `json:"timestamp"`     // Wall clock timestamp of last update
	ReplicaID    string    `json:"replica_id"`
}

// CRDTHash implements a Last-Write-Wins Hash with field-level granularity
type CRDTHash struct {
	Fields     map[string]*HashField `json:"fields"`     // field key -> field mapping
	Tombstones map[string]int64      `json:"tombstones"` // Set of IDs of deleted fields -> deletion timestamp
	ReplicaID  string                `json:"replica_id"`
	nextSeq    int64                 // Sequence number for local operations
}

// NewCRDTHash creates a new CRDT hash
func NewCRDTHash(replicaID string) *CRDTHash {
	return &CRDTHash{
		Fields:     make(map[string]*HashField),
		Tombstones: make(map[string]int64),
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
	// For counter fields, return string representation of counter value
	if field.FieldType == FieldTypeCounter {
		return fmt.Sprintf("%d", field.CounterValue), true
	}
	return field.Value, true
}

// Delete removes a field from the hash using observed-remove semantics
func (h *CRDTHash) Delete(key string, timestamp int64) bool {
	field, exists := h.Fields[key]
	if !exists {
		return false
	}

	// Add to tombstones to track removal
	h.Tombstones[field.ID] = timestamp
	delete(h.Fields, key)
	return true
}

// IncrBy increments a field's counter value by delta using accumulative semantics
// If the field doesn't exist, it's created with delta as its value
// If the field exists as a string, an error is returned
// Returns the new counter value
func (h *CRDTHash) IncrBy(key string, delta int64, timestamp int64, replicaID string) (int64, error) {
	if replicaID == "" {
		replicaID = h.ReplicaID
	}

	h.nextSeq++
	id := generateElementID(timestamp, replicaID, h.nextSeq)

	existingField, exists := h.Fields[key]
	if exists {
		// Field exists - check type
		if existingField.FieldType == FieldTypeString {
			// Per Redis behavior: try to parse string as int
			// For simplicity, we'll convert string field to counter
			existingField.FieldType = FieldTypeCounter
			existingField.CounterValue = 0
		}
		existingField.CounterValue += delta
		existingField.Timestamp = timestamp
		existingField.ReplicaID = replicaID
		existingField.ID = id
		return existingField.CounterValue, nil
	}

	// Create new counter field
	field := &HashField{
		Key:          key,
		Value:        "",
		CounterValue: delta,
		FieldType:    FieldTypeCounter,
		ID:           id,
		Timestamp:    timestamp,
		ReplicaID:    replicaID,
	}

	h.Fields[key] = field
	return delta, nil
}

// IncrByFloat increments a field's value by a float delta
// Returns the new value as a float64
func (h *CRDTHash) IncrByFloat(key string, delta float64, timestamp int64, replicaID string) (float64, error) {
	// For simplicity, store float values as scaled integers (multiply by 1e6)
	// Or use string representation
	// Here we'll use a simple approach with string representation
	if replicaID == "" {
		replicaID = h.ReplicaID
	}

	h.nextSeq++
	id := generateElementID(timestamp, replicaID, h.nextSeq)

	existingField, exists := h.Fields[key]
	if exists {
		// Parse existing value as float
		var currentValue float64
		if existingField.FieldType == FieldTypeCounter {
			currentValue = float64(existingField.CounterValue)
		} else {
			// Try to parse string as float
			// For simplicity, start from 0
			currentValue = 0
		}
		newValue := currentValue + delta

		// Convert to counter type (we'll store scaled value)
		existingField.FieldType = FieldTypeCounter
		existingField.CounterValue = int64(newValue * 1000000) // Store as micro-units
		existingField.Value = ""
		existingField.Timestamp = timestamp
		existingField.ReplicaID = replicaID
		existingField.ID = id
		return newValue, nil
	}

	// Create new counter field
	field := &HashField{
		Key:          key,
		Value:        "",
		CounterValue: int64(delta * 1000000), // Store as micro-units
		FieldType:    FieldTypeCounter,
		ID:           id,
		Timestamp:    timestamp,
		ReplicaID:    replicaID,
	}

	h.Fields[key] = field
	return delta, nil
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
		if field.FieldType == FieldTypeCounter {
			values = append(values, fmt.Sprintf("%d", field.CounterValue))
		} else {
			values = append(values, field.Value)
		}
	}
	return values
}

// GetAll returns all field key-value pairs
func (h *CRDTHash) GetAll() map[string]string {
	result := make(map[string]string, len(h.Fields))
	for key, field := range h.Fields {
		if field.FieldType == FieldTypeCounter {
			result[key] = fmt.Sprintf("%d", field.CounterValue)
		} else {
			result[key] = field.Value
		}
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
// String fields use LWW semantics, Counter fields use accumulative semantics
func (h *CRDTHash) Merge(other *CRDTHash) {
	// Merge fields
	for key, otherField := range other.Fields {
		// If field is tombstoned in this hash, don't add it
		if _, tombstoned := h.Tombstones[otherField.ID]; tombstoned {
			continue
		}

		existingField, exists := h.Fields[key]
		if !exists {
			// Field doesn't exist locally, add it
			h.Fields[key] = &HashField{
				Key:          otherField.Key,
				Value:        otherField.Value,
				CounterValue: otherField.CounterValue,
				FieldType:    otherField.FieldType,
				ID:           otherField.ID,
				Timestamp:    otherField.Timestamp,
				ReplicaID:    otherField.ReplicaID,
			}
		} else {
			// Field exists locally
			// Handle based on field types
			if existingField.FieldType == FieldTypeCounter && otherField.FieldType == FieldTypeCounter {
				// Both are counters - accumulate values (counter semantics)
				existingField.CounterValue += otherField.CounterValue
				// Take latest timestamp
				if otherField.Timestamp > existingField.Timestamp {
					existingField.Timestamp = otherField.Timestamp
					existingField.ReplicaID = otherField.ReplicaID
					existingField.ID = otherField.ID
				}
			} else if existingField.FieldType == FieldTypeCounter || otherField.FieldType == FieldTypeCounter {
				// Type mismatch - counter wins (as it's a more specific operation)
				if otherField.FieldType == FieldTypeCounter {
					existingField.FieldType = FieldTypeCounter
					existingField.CounterValue = otherField.CounterValue
					existingField.Value = ""
					existingField.Timestamp = otherField.Timestamp
					existingField.ReplicaID = otherField.ReplicaID
					existingField.ID = otherField.ID
				}
			} else {
				// Both are strings - apply LWW
				if otherField.Timestamp > existingField.Timestamp ||
					(otherField.Timestamp == existingField.Timestamp && otherField.ReplicaID > existingField.ReplicaID) {
					existingField.Value = otherField.Value
					existingField.Timestamp = otherField.Timestamp
					existingField.ReplicaID = otherField.ReplicaID
					existingField.ID = otherField.ID
				}
			}
		}
	}

	// Merge tombstones and remove tombstoned fields
	for tombstoneID, deletedAt := range other.Tombstones {
		if existingDeletedAt, exists := h.Tombstones[tombstoneID]; exists {
			if deletedAt > existingDeletedAt {
				h.Tombstones[tombstoneID] = deletedAt
			}
		} else {
			h.Tombstones[tombstoneID] = deletedAt
		}

		// Find and remove any field with this ID
		for key, field := range h.Fields {
			if field.ID == tombstoneID {
				delete(h.Fields, key)
				break
			}
		}
	}
}

// GC removes tombstones older than cutoffTimestamp
func (h *CRDTHash) GC(cutoffTimestamp int64) int {
	cleaned := 0
	for id, deletedAt := range h.Tombstones {
		if deletedAt > 0 && deletedAt < cutoffTimestamp {
			delete(h.Tombstones, id)
			cleaned++
		}
	}
	return cleaned
}

// ToJSON serializes the hash to JSON
func (h *CRDTHash) ToJSON() ([]byte, error) {
	return json.Marshal(h)
}

// FromJSON deserializes the hash from JSON
func (h *CRDTHash) FromJSON(data []byte) error {
	return json.Unmarshal(data, h)
}
