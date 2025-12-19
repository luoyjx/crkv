package storage

import (
	"encoding/json"
	"fmt"
)

// ListElement represents an element in a CRDT list
type ListElement struct {
	Value     string `json:"value"`
	ID        string `json:"id"`        // Unique element ID (timestamp-replicaID-seq)
	Timestamp int64  `json:"timestamp"` // Creation timestamp
	ReplicaID string `json:"replica_id"`
	Deleted   bool   `json:"deleted"` // For tombstone-based deletion
}

// CRDTList represents a CRDT list with observed-remove semantics
type CRDTList struct {
	Elements []ListElement `json:"elements"`
	NextSeq  int64         `json:"next_seq"` // Sequence counter for this replica
}

// NewListValue creates a new Value for CRDT lists
func NewListValue(timestamp int64, replicaID string) *Value {
	vc := NewVectorClock()
	vc.Increment(replicaID)
	list := &CRDTList{
		Elements: make([]ListElement, 0),
		NextSeq:  0,
	}
	data, _ := json.Marshal(list)
	return &Value{
		Type:        TypeList,
		Data:        data,
		Timestamp:   timestamp,
		ReplicaID:   replicaID,
		VectorClock: vc,
	}
}

// NewSetValue creates a new Value of type TypeSet
func NewSetValue(timestamp int64, replicaID string) *Value {
	vc := NewVectorClock()
	vc.Increment(replicaID)
	set := NewCRDTSet(replicaID)
	data, _ := json.Marshal(set)
	return &Value{
		Type:        TypeSet,
		Data:        data,
		Timestamp:   timestamp,
		ReplicaID:   replicaID,
		VectorClock: vc,
	}
}

// NewHashValue creates a new Value of type TypeHash
func NewHashValue(timestamp int64, replicaID string) *Value {
	vc := NewVectorClock()
	vc.Increment(replicaID)
	hash := NewCRDTHash(replicaID)
	data, _ := json.Marshal(hash)
	return &Value{
		Type:        TypeHash,
		Data:        data,
		Timestamp:   timestamp,
		ReplicaID:   replicaID,
		VectorClock: vc,
	}
}

// List returns the CRDTList if type is TypeList
func (v *Value) List() *CRDTList {
	if v.Type != TypeList {
		return nil
	}
	var list CRDTList
	if err := json.Unmarshal(v.Data, &list); err != nil {
		return nil
	}
	return &list
}

// SetList updates the list data and timestamp
func (v *Value) SetList(list *CRDTList, timestamp int64) {
	if v.Type != TypeList {
		return
	}
	data, _ := json.Marshal(list)
	v.Data = data
	v.Timestamp = timestamp
}

// Set returns the CRDT set if type is TypeSet
func (v *Value) Set() *CRDTSet {
	if v.Type != TypeSet {
		return nil
	}
	var set CRDTSet
	if err := json.Unmarshal(v.Data, &set); err != nil {
		return nil
	}
	return &set
}

// SetSet updates the set data and timestamp
func (v *Value) SetSet(set *CRDTSet, timestamp int64) {
	if v.Type != TypeSet {
		return
	}
	data, _ := json.Marshal(set)
	v.Data = data
	v.Timestamp = timestamp
}

// Hash returns the CRDT hash if type is TypeHash
func (v *Value) Hash() *CRDTHash {
	if v.Type != TypeHash {
		return nil
	}
	var hash CRDTHash
	if err := json.Unmarshal(v.Data, &hash); err != nil {
		return nil
	}
	return &hash
}

// SetHash updates the hash data and timestamp
func (v *Value) SetHash(hash *CRDTHash, timestamp int64) {
	if v.Type != TypeHash {
		return
	}
	data, _ := json.Marshal(hash)
	v.Data = data
	v.Timestamp = timestamp
}

// LPush adds element to the head of the list
func (list *CRDTList) LPush(value string, timestamp int64, replicaID string) string {
	elementID := generateElementID(timestamp, replicaID, list.NextSeq)
	list.NextSeq++

	element := ListElement{
		Value:     value,
		ID:        elementID,
		Timestamp: timestamp,
		ReplicaID: replicaID,
		Deleted:   false,
	}

	// Insert at head
	list.Elements = append([]ListElement{element}, list.Elements...)
	return elementID
}

// RPush adds element to the tail of the list
func (list *CRDTList) RPush(value string, timestamp int64, replicaID string) string {
	elementID := generateElementID(timestamp, replicaID, list.NextSeq)
	list.NextSeq++

	element := ListElement{
		Value:     value,
		ID:        elementID,
		Timestamp: timestamp,
		ReplicaID: replicaID,
		Deleted:   false,
	}

	// Insert at tail
	list.Elements = append(list.Elements, element)
	return elementID
}

// LPop removes and returns the first element
func (list *CRDTList) LPop() (string, bool) {
	visible := list.VisibleElements()
	if len(visible) == 0 {
		return "", false
	}

	// Mark the first visible element as deleted
	for i := range list.Elements {
		if list.Elements[i].ID == visible[0].ID && !list.Elements[i].Deleted {
			list.Elements[i].Deleted = true
			return visible[0].Value, true
		}
	}
	return "", false
}

// RPop removes and returns the last element
func (list *CRDTList) RPop() (string, bool) {
	visible := list.VisibleElements()
	if len(visible) == 0 {
		return "", false
	}

	// Mark the last visible element as deleted
	lastVisible := visible[len(visible)-1]
	for i := range list.Elements {
		if list.Elements[i].ID == lastVisible.ID && !list.Elements[i].Deleted {
			list.Elements[i].Deleted = true
			return lastVisible.Value, true
		}
	}
	return "", false
}

// VisibleElements returns non-deleted elements in order
func (list *CRDTList) VisibleElements() []ListElement {
	var visible []ListElement
	for _, elem := range list.Elements {
		if !elem.Deleted {
			visible = append(visible, elem)
		}
	}
	return visible
}

// Range returns elements in the specified range (Redis LRANGE semantics)
func (list *CRDTList) Range(start, stop int) []string {
	visible := list.VisibleElements()
	length := len(visible)

	// Handle negative indices
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	// Bounds checking
	if start < 0 {
		start = 0
	}
	if start >= length {
		return []string{}
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		return []string{}
	}

	var result []string
	for i := start; i <= stop; i++ {
		result = append(result, visible[i].Value)
	}
	return result
}

// Len returns the number of visible elements
func (list *CRDTList) Len() int {
	count := 0
	for _, elem := range list.Elements {
		if !elem.Deleted {
			count++
		}
	}
	return count
}

// Index returns the element at the specified index (LINDEX command)
func (list *CRDTList) Index(index int) (string, bool) {
	visible := list.VisibleElements()
	length := len(visible)

	// Handle negative index
	if index < 0 {
		index = length + index
	}

	// Bounds check
	if index < 0 || index >= length {
		return "", false
	}

	return visible[index].Value, true
}

// Set updates the element at the specified index (LSET command)
func (list *CRDTList) Set(index int, value string, timestamp int64, replicaID string) error {
	visible := list.VisibleElements()
	length := len(visible)

	// Handle negative index
	if index < 0 {
		index = length + index
	}

	// Bounds check
	if index < 0 || index >= length {
		return fmt.Errorf("ERR index out of range")
	}

	// Find and update the element
	targetID := visible[index].ID
	for i := range list.Elements {
		if list.Elements[i].ID == targetID && !list.Elements[i].Deleted {
			list.Elements[i].Value = value
			list.Elements[i].Timestamp = timestamp
			return nil
		}
	}
	return fmt.Errorf("ERR element not found")
}

// Insert inserts a value before or after the pivot element (LINSERT command)
// insertAfter: true = AFTER, false = BEFORE
// Returns the list length after insert, or -1 if pivot not found
func (list *CRDTList) Insert(pivot string, value string, insertAfter bool, timestamp int64, replicaID string) int {
	visible := list.VisibleElements()

	// Find pivot element
	var pivotElem *ListElement
	var pivotActualIndex int = -1
	for i, elem := range visible {
		if elem.Value == pivot {
			// Find actual index in Elements slice
			for j := range list.Elements {
				if list.Elements[j].ID == elem.ID {
					pivotElem = &list.Elements[j]
					pivotActualIndex = j
					break
				}
			}
			_ = i // pivot index in visible (not used currently)
			break
		}
	}

	if pivotElem == nil || pivotActualIndex == -1 {
		return -1 // Pivot not found
	}

	// Create new element
	elementID := generateElementID(timestamp, replicaID, list.NextSeq)
	list.NextSeq++

	newElem := ListElement{
		Value:     value,
		ID:        elementID,
		Timestamp: timestamp,
		ReplicaID: replicaID,
		Deleted:   false,
	}

	// Calculate insert position
	insertPos := pivotActualIndex
	if insertAfter {
		insertPos++
	}

	// Insert into slice
	if insertPos >= len(list.Elements) {
		list.Elements = append(list.Elements, newElem)
	} else {
		list.Elements = append(list.Elements[:insertPos+1], list.Elements[insertPos:]...)
		list.Elements[insertPos] = newElem
	}

	return list.Len()
}

// Trim trims the list to the specified range (LTRIM command)
func (list *CRDTList) Trim(start, stop int) {
	visible := list.VisibleElements()
	length := len(visible)

	if length == 0 {
		return
	}

	// Handle negative indices
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	// Bounds adjustment
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}

	// If start > stop, delete all
	if start > stop {
		for i := range list.Elements {
			if !list.Elements[i].Deleted {
				list.Elements[i].Deleted = true
			}
		}
		return
	}

	// Build set of IDs to keep
	keepIDs := make(map[string]bool)
	for i := start; i <= stop && i < length; i++ {
		keepIDs[visible[i].ID] = true
	}

	// Mark elements outside range as deleted
	for i := range list.Elements {
		if !list.Elements[i].Deleted && !keepIDs[list.Elements[i].ID] {
			list.Elements[i].Deleted = true
		}
	}
}

// Rem removes elements by value (LREM command)
// count > 0: Remove first count occurrences from head to tail
// count < 0: Remove first |count| occurrences from tail to head
// count = 0: Remove all occurrences
// Returns the number of removed elements
func (list *CRDTList) Rem(count int, value string) int {
	visible := list.VisibleElements()
	removed := 0

	// Determine direction and max removals
	fromHead := count >= 0
	maxRemove := count
	if count < 0 {
		maxRemove = -count
	}
	if count == 0 {
		maxRemove = len(visible) // Remove all
	}

	// Find elements to remove
	var toRemove []string
	if fromHead {
		for _, elem := range visible {
			if elem.Value == value && len(toRemove) < maxRemove {
				toRemove = append(toRemove, elem.ID)
			}
		}
	} else {
		// From tail
		for i := len(visible) - 1; i >= 0 && len(toRemove) < maxRemove; i-- {
			if visible[i].Value == value {
				toRemove = append(toRemove, visible[i].ID)
			}
		}
	}

	// Mark as deleted
	removeSet := make(map[string]bool)
	for _, id := range toRemove {
		removeSet[id] = true
	}

	for i := range list.Elements {
		if removeSet[list.Elements[i].ID] && !list.Elements[i].Deleted {
			list.Elements[i].Deleted = true
			removed++
		}
	}

	return removed
}

// Merge merges another CRDTList into this one (for replication)
func (list *CRDTList) Merge(other *CRDTList) {
	// Create a map of existing element IDs for fast lookup
	existing := make(map[string]*ListElement)
	for i := range list.Elements {
		existing[list.Elements[i].ID] = &list.Elements[i]
	}

	// Add new elements from other list
	for _, otherElem := range other.Elements {
		if existingElem, found := existing[otherElem.ID]; found {
			// Element exists, merge deletion state (delete wins)
			if otherElem.Deleted {
				existingElem.Deleted = true
			}
		} else {
			// New element, add it
			list.Elements = append(list.Elements, otherElem)
		}
	}

	// Update sequence counter
	if other.NextSeq > list.NextSeq {
		list.NextSeq = other.NextSeq
	}
}

// generateElementID creates a unique element ID
func generateElementID(timestamp int64, replicaID string, seq int64) string {
	return fmt.Sprintf("%d-%s-%d", timestamp, replicaID, seq)
}
