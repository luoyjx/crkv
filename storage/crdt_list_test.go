package storage

import (
	"testing"
	"time"
)

// ============================================
// Basic List Operations Tests
// ============================================

func TestListLPushRPush(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	// LPUSH adds to head
	list.LPush("a", timestamp, "replica1")
	list.LPush("b", timestamp+1, "replica1")

	visible := list.VisibleElements()
	if len(visible) != 2 {
		t.Errorf("Expected 2 elements, got %d", len(visible))
	}
	if visible[0].Value != "b" || visible[1].Value != "a" {
		t.Errorf("Expected [b, a], got [%s, %s]", visible[0].Value, visible[1].Value)
	}

	// RPUSH adds to tail
	list.RPush("c", timestamp+2, "replica1")
	visible = list.VisibleElements()
	if len(visible) != 3 {
		t.Errorf("Expected 3 elements, got %d", len(visible))
	}
	if visible[2].Value != "c" {
		t.Errorf("Expected last element 'c', got %s", visible[2].Value)
	}
}

func TestListLPopRPop(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")

	// LPOP removes from head
	val, ok := list.LPop()
	if !ok || val != "a" {
		t.Errorf("LPOP expected 'a', got '%s'", val)
	}

	// RPOP removes from tail
	val, ok = list.RPop()
	if !ok || val != "c" {
		t.Errorf("RPOP expected 'c', got '%s'", val)
	}

	// Only 'b' should remain
	if list.Len() != 1 {
		t.Errorf("Expected 1 element, got %d", list.Len())
	}
}

func TestListRange(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")
	list.RPush("d", timestamp+3, "replica1")

	// Basic range
	result := list.Range(0, 2)
	if len(result) != 3 || result[0] != "a" || result[2] != "c" {
		t.Errorf("Range(0,2) expected [a,b,c], got %v", result)
	}

	// Negative indices
	result = list.Range(-2, -1)
	if len(result) != 2 || result[0] != "c" || result[1] != "d" {
		t.Errorf("Range(-2,-1) expected [c,d], got %v", result)
	}

	// Full range
	result = list.Range(0, -1)
	if len(result) != 4 {
		t.Errorf("Range(0,-1) expected 4 elements, got %d", len(result))
	}
}

// ============================================
// LINDEX Tests
// ============================================

func TestLIndexBasicPositive(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")

	// Test positive indices
	val, ok := list.Index(0)
	if !ok || val != "a" {
		t.Errorf("Index(0) expected 'a', got '%s'", val)
	}

	val, ok = list.Index(1)
	if !ok || val != "b" {
		t.Errorf("Index(1) expected 'b', got '%s'", val)
	}

	val, ok = list.Index(2)
	if !ok || val != "c" {
		t.Errorf("Index(2) expected 'c', got '%s'", val)
	}
}

func TestLIndexNegative(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")

	// Test negative indices
	val, ok := list.Index(-1)
	if !ok || val != "c" {
		t.Errorf("Index(-1) expected 'c', got '%s'", val)
	}

	val, ok = list.Index(-2)
	if !ok || val != "b" {
		t.Errorf("Index(-2) expected 'b', got '%s'", val)
	}

	val, ok = list.Index(-3)
	if !ok || val != "a" {
		t.Errorf("Index(-3) expected 'a', got '%s'", val)
	}
}

func TestLIndexOutOfBounds(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")

	// Out of bounds positive
	_, ok := list.Index(10)
	if ok {
		t.Error("Index(10) should return false for 2-element list")
	}

	// Out of bounds negative
	_, ok = list.Index(-10)
	if ok {
		t.Error("Index(-10) should return false for 2-element list")
	}
}

func TestLIndexEmptyList(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}

	_, ok := list.Index(0)
	if ok {
		t.Error("Index(0) should return false for empty list")
	}
}

func TestLIndexWithDeletedElements(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")

	// Delete middle element
	list.LPop() // removes "a"

	// Now index 0 should be "b"
	val, ok := list.Index(0)
	if !ok || val != "b" {
		t.Errorf("After pop, Index(0) expected 'b', got '%s'", val)
	}
}

// ============================================
// LSET Tests
// ============================================

func TestLSetBasic(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")

	// Set first element
	err := list.Set(0, "new_a", timestamp+3, "replica1")
	if err != nil {
		t.Errorf("Set(0) failed: %v", err)
	}

	val, _ := list.Index(0)
	if val != "new_a" {
		t.Errorf("Expected 'new_a', got '%s'", val)
	}
}

func TestLSetNegativeIndex(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")

	// Set last element using negative index
	err := list.Set(-1, "new_c", timestamp+3, "replica1")
	if err != nil {
		t.Errorf("Set(-1) failed: %v", err)
	}

	val, _ := list.Index(-1)
	if val != "new_c" {
		t.Errorf("Expected 'new_c', got '%s'", val)
	}
}

func TestLSetOutOfBounds(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")

	err := list.Set(10, "x", timestamp+1, "replica1")
	if err == nil {
		t.Error("Set(10) should return error for 1-element list")
	}
}

func TestLSetEmptyList(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	err := list.Set(0, "x", timestamp, "replica1")
	if err == nil {
		t.Error("Set(0) should return error for empty list")
	}
}

// ============================================
// LINSERT Tests
// ============================================

func TestLInsertAfter(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("c", timestamp+1, "replica1")

	// Insert "b" after "a"
	result := list.Insert("a", "b", true, timestamp+2, "replica1")
	if result == -1 {
		t.Error("Insert AFTER should not return -1")
	}

	visible := list.VisibleElements()
	if len(visible) != 3 {
		t.Errorf("Expected 3 elements, got %d", len(visible))
	}

	// Check order: a, b, c
	values := make([]string, len(visible))
	for i, e := range visible {
		values[i] = e.Value
	}
	if values[0] != "a" || values[1] != "b" || values[2] != "c" {
		t.Errorf("Expected [a,b,c], got %v", values)
	}
}

func TestLInsertBefore(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("c", timestamp+1, "replica1")

	// Insert "b" before "c"
	result := list.Insert("c", "b", false, timestamp+2, "replica1")
	if result == -1 {
		t.Error("Insert BEFORE should not return -1")
	}

	visible := list.VisibleElements()
	values := make([]string, len(visible))
	for i, e := range visible {
		values[i] = e.Value
	}
	if values[0] != "a" || values[1] != "b" || values[2] != "c" {
		t.Errorf("Expected [a,b,c], got %v", values)
	}
}

func TestLInsertPivotNotFound(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")

	result := list.Insert("notexist", "b", true, timestamp+1, "replica1")
	if result != -1 {
		t.Errorf("Insert with non-existent pivot should return -1, got %d", result)
	}
}

func TestLInsertConcurrent(t *testing.T) {
	// Test concurrent insertions after same pivot (CRDT behavior)
	list1 := &CRDTList{Elements: make([]ListElement, 0)}
	list2 := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	// Both start with "x"
	list1.RPush("x", timestamp, "replica1")
	list2.RPush("x", timestamp, "replica1")

	// Concurrent insertions after "x"
	list1.Insert("x", "y1", true, timestamp+1, "replica1")
	list2.Insert("x", "y2", true, timestamp+1, "replica2")

	// Merge
	list1.Merge(list2)
	list2.Merge(list1)

	// Both should have same elements
	if list1.Len() != list2.Len() {
		t.Errorf("Lists should have same length after merge: list1=%d, list2=%d",
			list1.Len(), list2.Len())
	}

	// Both should have x, y1, y2 (order may vary but consistent)
	visible1 := list1.VisibleElements()
	visible2 := list2.VisibleElements()

	// Verify x is first
	if visible1[0].Value != "x" || visible2[0].Value != "x" {
		t.Error("First element should be 'x'")
	}

	// Verify y1 and y2 are present (order determined by element ID)
	hasY1, hasY2 := false, false
	for _, e := range visible1 {
		if e.Value == "y1" {
			hasY1 = true
		}
		if e.Value == "y2" {
			hasY2 = true
		}
	}
	if !hasY1 || !hasY2 {
		t.Error("Both y1 and y2 should be present after merge")
	}

	t.Logf("List1 after merge: %v", getValues(visible1))
	t.Logf("List2 after merge: %v", getValues(visible2))
}

// ============================================
// LTRIM Tests
// ============================================

func TestLTrimBasic(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")
	list.RPush("d", timestamp+3, "replica1")

	// Keep first 2 elements
	list.Trim(0, 1)

	visible := list.VisibleElements()
	if len(visible) != 2 {
		t.Errorf("Expected 2 elements after trim, got %d", len(visible))
	}
	if visible[0].Value != "a" || visible[1].Value != "b" {
		t.Errorf("Expected [a,b], got %v", getValues(visible))
	}
}

func TestLTrimNegativeIndices(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("c", timestamp+2, "replica1")
	list.RPush("d", timestamp+3, "replica1")

	// Keep last 2 elements
	list.Trim(-2, -1)

	visible := list.VisibleElements()
	if len(visible) != 2 {
		t.Errorf("Expected 2 elements after trim, got %d", len(visible))
	}
	if visible[0].Value != "c" || visible[1].Value != "d" {
		t.Errorf("Expected [c,d], got %v", getValues(visible))
	}
}

func TestLTrimKeepAll(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")

	// Keep all elements
	list.Trim(0, -1)

	if list.Len() != 2 {
		t.Errorf("Expected 2 elements after trim(0,-1), got %d", list.Len())
	}
}

func TestLTrimEmptyResult(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")

	// Trim beyond range
	list.Trim(10, 20)

	if list.Len() != 0 {
		t.Errorf("Expected 0 elements after trim(10,20), got %d", list.Len())
	}
}

// ============================================
// LREM Tests
// ============================================

func TestLRemAll(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("a", timestamp+2, "replica1")
	list.RPush("c", timestamp+3, "replica1")
	list.RPush("a", timestamp+4, "replica1")

	// Remove all "a"
	removed := list.Rem(0, "a")
	if removed != 3 {
		t.Errorf("Expected 3 removals, got %d", removed)
	}

	visible := list.VisibleElements()
	for _, e := range visible {
		if e.Value == "a" {
			t.Error("Found 'a' after LREM 0 'a'")
		}
	}
}

func TestLRemFromHead(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("a", timestamp+2, "replica1")
	list.RPush("c", timestamp+3, "replica1")
	list.RPush("a", timestamp+4, "replica1")

	// Remove first 2 "a" from head
	removed := list.Rem(2, "a")
	if removed != 2 {
		t.Errorf("Expected 2 removals, got %d", removed)
	}

	// Should have: b, c, a (last 'a' remains)
	visible := list.VisibleElements()
	values := getValues(visible)
	if len(values) != 3 || values[2] != "a" {
		t.Errorf("Expected last 'a' to remain, got %v", values)
	}
}

func TestLRemFromTail(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")
	list.RPush("a", timestamp+2, "replica1")
	list.RPush("c", timestamp+3, "replica1")
	list.RPush("a", timestamp+4, "replica1")

	// Remove first 2 "a" from tail
	removed := list.Rem(-2, "a")
	if removed != 2 {
		t.Errorf("Expected 2 removals, got %d", removed)
	}

	// Should have: a, b, c (first 'a' remains)
	visible := list.VisibleElements()
	values := getValues(visible)
	if len(values) != 3 || values[0] != "a" {
		t.Errorf("Expected first 'a' to remain, got %v", values)
	}
}

func TestLRemValueNotFound(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list.RPush("a", timestamp, "replica1")
	list.RPush("b", timestamp+1, "replica1")

	removed := list.Rem(0, "notexist")
	if removed != 0 {
		t.Errorf("Expected 0 removals for non-existent value, got %d", removed)
	}
}

// ============================================
// CRDT Merge Tests
// ============================================

func TestListMergeBasic(t *testing.T) {
	list1 := &CRDTList{Elements: make([]ListElement, 0)}
	list2 := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	list1.RPush("a", timestamp, "replica1")
	list2.RPush("b", timestamp+1, "replica2")

	list1.Merge(list2)

	if list1.Len() != 2 {
		t.Errorf("Expected 2 elements after merge, got %d", list1.Len())
	}
}

func TestListMergeDeletionWins(t *testing.T) {
	list1 := &CRDTList{Elements: make([]ListElement, 0)}
	list2 := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	// Both have same element
	id := list1.RPush("a", timestamp, "replica1")
	list2.Elements = append(list2.Elements, ListElement{
		Value:     "a",
		ID:        id,
		Timestamp: timestamp,
		ReplicaID: "replica1",
		Deleted:   false,
	})

	// Delete in list2
	list2.Elements[0].Deleted = true

	// Merge - deletion should win
	list1.Merge(list2)

	if list1.Len() != 0 {
		t.Errorf("Expected 0 visible elements after merge (deletion wins), got %d", list1.Len())
	}
}

func TestListMergeObservedRemove(t *testing.T) {
	// Per documentation: DEL deletes only observed elements
	// New elements added concurrently survive
	list1 := &CRDTList{Elements: make([]ListElement, 0)}
	list2 := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	// Both start with "x"
	id := list1.RPush("x", timestamp, "replica1")
	list2.Elements = append(list2.Elements, ListElement{
		Value:     "x",
		ID:        id,
		Timestamp: timestamp,
		ReplicaID: "replica1",
		Deleted:   false,
	})

	// list1 adds "y" (concurrent with list2's delete)
	list1.RPush("y", timestamp+1, "replica1")

	// list2 deletes "x" (only observed element)
	list2.Elements[0].Deleted = true

	// Merge
	list1.Merge(list2)

	// "x" should be deleted, "y" should survive
	visible := list1.VisibleElements()
	if len(visible) != 1 || visible[0].Value != "y" {
		t.Errorf("Expected only 'y' to survive, got %v", getValues(visible))
	}
}

func TestListMergeRGAInterleaving(t *testing.T) {
	// Test RGA convergence for concurrent insertions at the same position
	// Current implementation is append-only for new elements, which may fail convergence check
	// if merge order differs.

	timestamp := time.Now().UnixNano()

	// Scenario: Both users insert at head (LPUSH) concurrently

	// Replica 1 inserts "A"
	list1 := &CRDTList{Elements: make([]ListElement, 0)}
	list1.LPush("A", timestamp, "r1")

	// Replica 2 inserts "B"
	list2 := &CRDTList{Elements: make([]ListElement, 0)}
	list2.LPush("B", timestamp, "r2") // Same timestamp

	// Create clones for two-way merge
	list1Clone := copyList(list1)
	list2Clone := copyList(list2)

	// Merge list2 into list1
	list1.Merge(list2)

	// Merge list1 into list2
	list2Clone.Merge(list1Clone)

	// Check convergence: list1 and list2Clone should be identical in order
	vals1 := getValues(list1.VisibleElements())
	vals2 := getValues(list2Clone.VisibleElements())

	if len(vals1) != len(vals2) {
		t.Errorf("Divergence! Lengths differ: %d vs %d", len(vals1), len(vals2))
	}

	if len(vals1) == 2 {
		if vals1[0] != vals2[0] || vals1[1] != vals2[1] {
			t.Errorf("Divergence! Orders differ:\nList1 merged: %v\nList2 merged: %v", vals1, vals2)
			// Debug dump
			t.Logf("List1 Dump:")
			for _, e := range list1.Elements {
				t.Logf("  Val: %s, ID: %s, Origin: '%s', TS: %d, Rep: %s", e.Value, e.ID, e.OriginLeftID, e.Timestamp, e.ReplicaID)
			}
			t.Logf("List2 Dump:")
			for _, e := range list2Clone.Elements {
				t.Logf("  Val: %s, ID: %s, Origin: '%s', TS: %d, Rep: %s", e.Value, e.ID, e.OriginLeftID, e.Timestamp, e.ReplicaID)
			}
		}
	} else {
		t.Errorf("Expected length 2, got %v and %v", vals1, vals2)
	}
}

func copyList(l *CRDTList) *CRDTList {
	newList := &CRDTList{
		Elements: make([]ListElement, len(l.Elements)),
		NextSeq:  l.NextSeq,
	}
	copy(newList.Elements, l.Elements)
	return newList
}

// ============================================
// Benchmark Tests
// ============================================

func BenchmarkListLPush(b *testing.B) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.LPush("value", timestamp+int64(i), "replica1")
	}
}

func BenchmarkListMerge(b *testing.B) {
	list1 := &CRDTList{Elements: make([]ListElement, 0)}
	timestamp := time.Now().UnixNano()

	// Populate list1
	for i := 0; i < 100; i++ {
		list1.RPush("value", timestamp+int64(i), "replica1")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list2 := &CRDTList{Elements: make([]ListElement, 0)}
		for j := 0; j < 10; j++ {
			list2.RPush("new", timestamp+int64(1000+j), "replica2")
		}
		list1.Merge(list2)
	}
}

// ============================================
// Helper Functions
// ============================================

func getValues(elements []ListElement) []string {
	values := make([]string, len(elements))
	for i, e := range elements {
		values[i] = e.Value
	}
	return values
}
