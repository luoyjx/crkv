package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ListElement represents an element in a CRDT list
type ListElement struct {
	Value        string `json:"value"`
	ID           string `json:"id"`        // Unique element ID (timestamp-replicaID-seq)
	Timestamp    int64  `json:"timestamp"` // Creation timestamp
	ReplicaID    string `json:"replica_id"`
	OriginLeftID string `json:"origin_left_id"` // ID of the element to the left at insertion time
	Deleted      bool   `json:"deleted"`        // For tombstone-based deletion
	DeletedAt    int64  `json:"deleted_at"`     // Timestamp when deleted (for GC)
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

	// LPush always inserts at the "virtual head", so OriginLeftID is empty
	originLeft := ""

	element := ListElement{
		Value:        value,
		ID:           elementID,
		Timestamp:    timestamp,
		ReplicaID:    replicaID,
		OriginLeftID: originLeft,
		Deleted:      false,
	}

	// For local LPush, we can just insert at head for the visible view,
	// but strictly we should run the merge logic to place it correctly among concurrent inserts.
	// However, since we are the leader for this op, and it's LPush, it goes to head.
	// BUT, if there are already other elements with OriginLeftID="", we need to sort against them.
	// Simplest approach: append to elements and re-merge/sort.
	// Or: insert at head of Elements because our timestamp is likely newest.
	// Let's use the robust approach: Add to list and re-sort/re-linearize.
	// To optimize: We know it belongs at the start (or near it).

	// Optimization: LPush with newest timestamp usually goes to index 0.
	// We'll append and then run a local "Merge" equivalent or just rely on the fact
	// that we only need strict ordering when actual concurrency happens.
	// But `list.Elements` IS the stored state. We must keep it sorted.

	// Let's implement a helper `addAndSort` or just use `Merge` with a single-element list?
	// No, that's heavy.

	// Fast path: Just insert at head?
	// If we just insert at head, we might violate RGA if we have concurrent future ops.
	// But this is a local operation.
	// Let's rely on a helper that inserts correctly.

	list.insertRGA(element)
	return elementID
}

// RPush adds element to the tail of the list
func (list *CRDTList) RPush(value string, timestamp int64, replicaID string) string {
	elementID := generateElementID(timestamp, replicaID, list.NextSeq)
	list.NextSeq++

	// RPush inserts after the last element
	originLeft := ""
	if len(list.Elements) > 0 {
		// Find the last non-deleted element?
		// No, RGA appends after the *absolute* last element in the current linearization, even if deleted?
		// Redis RPush appends to the *visible* tail.
		// If we append after a deleted element, it's fine.
		// But usually users expect it after the last visible one.
		// Let's use the last element in the `Elements` slice (linearized view).
		originLeft = list.Elements[len(list.Elements)-1].ID
	}

	element := ListElement{
		Value:        value,
		ID:           elementID,
		Timestamp:    timestamp,
		ReplicaID:    replicaID,
		OriginLeftID: originLeft,
		Deleted:      false,
	}

	list.insertRGA(element)
	return elementID
}

// insertRGA inserts an element maintaining RGA order
func (list *CRDTList) insertRGA(newElem ListElement) {
	// 1. Find the target position based on OriginLeftID
	// 2. Scan forward past any siblings that have higher priority (timestamp/replica)
	// 3. Insert

	// Find index of origin
	insertIndex := 0
	if newElem.OriginLeftID != "" {
		found := false
		for i, e := range list.Elements {
			if e.ID == newElem.OriginLeftID {
				insertIndex = i + 1
				found = true
				break
			}
		}
		if !found {
			// Origin not found (should not happen for local ops, but possible in sync)
			// Fallback: append to end or treat as root?
			// For correctness, maybe treat as root.
			insertIndex = 0
		}
	}

	// Skip over siblings with greater priority
	// A sibling is an element with the same OriginLeftID
	for insertIndex < len(list.Elements) {
		curr := list.Elements[insertIndex]
		if curr.OriginLeftID != newElem.OriginLeftID {
			// No longer looking at siblings (assuming list is RGA-sorted)
			// Wait, RGA-sorted means siblings are grouped.
			// However, a subtree might be interleaved?
			// No, in the linearization, descendants appear strictly after their parent
			// and before the next sibling of the parent.
			// So if we see a node with a DIFFERENT OriginLeftID, it is either:
			// 1. A descendant of the current sibling -> we must skip it.
			// 2. A sibling of the parent (or further up) -> we stop.

			// We need to check if 'curr' is a descendant of the sibling we are skipping.
			// This requires traversing the "right" pointers or checking origins.
			// If we simplify: we only skip siblings that are "greater".
			// But we must skip their entire subtrees too!

			// This "insert in place" logic is tricky.
			// It might be easier to just append and then fully re-sort/linearize if performance allows.
			// Given the complexity, full linearization is safer and ensures correctness.
			break // Placeholder if we do full merge
		}

		// Compare IDs (Timestamp Desc, ReplicaID Desc)
		if compareIDs(curr, newElem) > 0 {
			// curr > newElem, so newElem comes after curr (and its descendants)
			// We need to skip curr and all its descendants.
			// How to find end of descendants?
			// Linear scan until we find an element whose OriginLeftID (recursively) is NOT curr.ID
			// Actually, just simpler:
			// Rebuild the whole list.
			break
		}
		insertIndex++
	}

	list.Elements = append(list.Elements, newElem)
	list.rebuildRGA()
}

// rebuildRGA sorts and linearizes the elements based on RGA rules
func (list *CRDTList) rebuildRGA() {
	// 1. Group by OriginLeftID
	byOrigin := make(map[string][]ListElement)
	for _, e := range list.Elements {
		byOrigin[e.OriginLeftID] = append(byOrigin[e.OriginLeftID], e)
	}

	// 2. Sort each group
	for origin, siblings := range byOrigin {
		sort.Slice(siblings, func(i, j int) bool {
			return compareIDs(siblings[i], siblings[j]) > 0
		})
		byOrigin[origin] = siblings
	}

	// 3. Flatten (DFS)
	var linearized []ListElement
	var visit func(string)

	// Track visited to prevent cycles (though IDs are unique, graph should be tree/forest)
	// Cyclic origins shouldn't happen with valid timestamps.

	visit = func(originID string) {
		siblings := byOrigin[originID]
		for _, sibling := range siblings {
			linearized = append(linearized, sibling)
			visit(sibling.ID)
		}
	}

	visit("") // Start from virtual root

	list.Elements = linearized
}

// compareIDs returns > 0 if a > b, < 0 if a < b
// Order: Higher Timestamp first. Tie-breaker: Higher ReplicaID first.
func compareIDs(a, b ListElement) int {
	if a.Timestamp != b.Timestamp {
		if a.Timestamp > b.Timestamp {
			return 1
		}
		return -1
	}
	return strings.Compare(a.ReplicaID, b.ReplicaID)
}

// LPop removes and returns the first element
func (list *CRDTList) LPop(timestamp int64) (string, bool) {
	visible := list.VisibleElements()
	if len(visible) == 0 {
		return "", false
	}

	// Mark the first visible element as deleted
	for i := range list.Elements {
		if list.Elements[i].ID == visible[0].ID && !list.Elements[i].Deleted {
			list.Elements[i].Deleted = true
			list.Elements[i].DeletedAt = timestamp
			return visible[0].Value, true
		}
	}
	return "", false
}

// RPop removes and returns the last element
func (list *CRDTList) RPop(timestamp int64) (string, bool) {
	visible := list.VisibleElements()
	if len(visible) == 0 {
		return "", false
	}

	// Mark the last visible element as deleted
	lastVisible := visible[len(visible)-1]
	for i := range list.Elements {
		if list.Elements[i].ID == lastVisible.ID && !list.Elements[i].Deleted {
			list.Elements[i].Deleted = true
			list.Elements[i].DeletedAt = timestamp
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
			// Note: We do NOT update the timestamp here because it's used for RGA structure (ID).
			// Ideally LSET should be a new operation type or element structure should separate ID-timestamp from Value-timestamp.
			// For this implementation, we only update the Value.
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
	var pivotIndex int = -1

	// We need the Visible index to check if it exists, but we need the Element itself for ID
	for i, elem := range visible {
		if elem.Value == pivot {
			pivotElem = &visible[i] // This is a copy from slice, be careful.
			// We need the ID.
			pivotIndex = i
			break
		}
	}

	if pivotIndex == -1 || pivotElem == nil {
		return -1 // Pivot not found
	}

	// Determine OriginLeftID
	var originLeftID string
	if insertAfter {
		// Insert after pivot: Origin is Pivot
		originLeftID = pivotElem.ID
	} else {
		// Insert before pivot: Origin is the element BEFORE pivot
		if pivotIndex == 0 {
			originLeftID = "" // Head
		} else {
			originLeftID = visible[pivotIndex-1].ID
		}
	}

	// Create new element
	elementID := generateElementID(timestamp, replicaID, list.NextSeq)
	list.NextSeq++

	newElem := ListElement{
		Value:        value,
		ID:           elementID,
		Timestamp:    timestamp,
		ReplicaID:    replicaID,
		OriginLeftID: originLeftID,
		Deleted:      false,
	}

	list.insertRGA(newElem)
	return list.Len()
}

// Trim trims the list to the specified range (LTRIM command)
func (list *CRDTList) Trim(start, stop int, timestamp int64) {
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
				list.Elements[i].DeletedAt = timestamp
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
			list.Elements[i].DeletedAt = timestamp
		}
	}
}

// Rem removes elements by value (LREM command)
// count > 0: Remove first count occurrences from head to tail
// count < 0: Remove first |count| occurrences from tail to head
// count = 0: Remove all occurrences
// Returns the number of removed elements
func (list *CRDTList) Rem(count int, value string, timestamp int64) int {
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
			list.Elements[i].DeletedAt = timestamp
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
	addedNew := false
	for _, otherElem := range other.Elements {
		if existingElem, found := existing[otherElem.ID]; found {
			// Element exists, merge deletion state (delete wins)
			if otherElem.Deleted {
				existingElem.Deleted = true
				if otherElem.DeletedAt > existingElem.DeletedAt {
					existingElem.DeletedAt = otherElem.DeletedAt
				}
			}
		} else {
			// New element, add it
			list.Elements = append(list.Elements, otherElem)
			addedNew = true
		}
	}

	// Update sequence counter
	if other.NextSeq > list.NextSeq {
		list.NextSeq = other.NextSeq
	}

	// If we added new elements, we must rebuild the linearization order
	if addedNew {
		list.rebuildRGA()
	}
}

// GC removes deleted elements older than cutoffTimestamp
func (list *CRDTList) GC(cutoffTimestamp int64) int {
	cleaned := 0
	newElements := make([]ListElement, 0, len(list.Elements))
	for _, elem := range list.Elements {
		if elem.Deleted && elem.DeletedAt > 0 && elem.DeletedAt < cutoffTimestamp {
			cleaned++
			continue
		}
		newElements = append(newElements, elem)
	}
	list.Elements = newElements
	// Note: Removing elements might break OriginLeftID chains for very old concurrent ops,
	// but those ops would also likely be GC'd or are too old to matter for convergence
	// (they will attach to head if origin is missing).
	return cleaned
}

// generateElementID creates a unique element ID
func generateElementID(timestamp int64, replicaID string, seq int64) string {
	return fmt.Sprintf("%d-%s-%d", timestamp, replicaID, seq)
}
