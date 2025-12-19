package storage

import (
	"testing"
	"time"
)

func TestCRDTList_GC(t *testing.T) {
	list := &CRDTList{Elements: make([]ListElement, 0)}
	now := time.Now().UnixNano()

	// Add elements
	list.LPush("a", now, "r1")
	list.LPush("b", now+1, "r1")
	list.LPush("c", now+2, "r1")

	// Delete "b" (middle)
	list.Rem(0, "b", now+3)

	// Verify "b" is marked deleted
	found := false
	for _, e := range list.Elements {
		if e.Value == "b" {
			if !e.Deleted {
				t.Error("Element 'b' should be marked deleted")
			}
			found = true
		}
	}
	if !found {
		t.Error("Element 'b' should exist in elements list (marked deleted)")
	}

	// GC with cutoff before deletion (should not remove)
	cleaned := list.GC(now)
	if cleaned != 0 {
		t.Errorf("GC(now) removed %d elements, expected 0", cleaned)
	}

	// GC with cutoff after deletion (should remove)
	cleaned = list.GC(now + 4)
	if cleaned != 1 {
		t.Errorf("GC(now+4) removed %d elements, expected 1", cleaned)
	}

	// Verify "b" is gone
	for _, e := range list.Elements {
		if e.Value == "b" {
			t.Error("Element 'b' should be removed after GC")
		}
	}

	// Verify others remain
	if list.Len() != 2 {
		t.Errorf("Expected 2 elements remaining, got %d", list.Len())
	}
}

func TestCRDTSet_GC(t *testing.T) {
	set := NewCRDTSet("r1")
	now := time.Now().UnixNano()

	set.Add("a", now, "r1")
	id := set.Elements["a"].ID
	set.Remove("a", now+1)

	if _, exists := set.Tombstones[id]; !exists {
		// Remove deletes from Elements and adds to Tombstones
	}
	// Note: Remove deletes from Elements map immediately. Tombstones stores the ID.
	// But we can't easily get the ID of "a" after removal unless we stored it.
	// Let's rely on internal implementation knowledge or check Tombstones size.

	if len(set.Tombstones) != 1 {
		t.Errorf("Expected 1 tombstone, got %d", len(set.Tombstones))
	}

	// GC
	set.GC(now + 2)

	if len(set.Tombstones) != 0 {
		t.Errorf("Expected 0 tombstones after GC, got %d", len(set.Tombstones))
	}
}

func TestCRDTHash_GC(t *testing.T) {
	hash := NewCRDTHash("r1")
	now := time.Now().UnixNano()

	hash.Set("k1", "v1", now, "r1")
	hash.Delete("k1", now+1)

	if len(hash.Tombstones) != 1 {
		t.Errorf("Expected 1 tombstone, got %d", len(hash.Tombstones))
	}

	// GC
	hash.GC(now + 2)

	if len(hash.Tombstones) != 0 {
		t.Errorf("Expected 0 tombstones after GC, got %d", len(hash.Tombstones))
	}
}

func TestCRDTZSet_GC(t *testing.T) {
	zset := NewCRDTZSet("r1")
	vc := NewVectorClock()
	now := time.Now().UnixNano()

	zset.ZAdd(map[string]float64{"m1": 1.0}, vc)
	zset.ZRem([]string{"m1"}, now+1, vc)

	// ZRem sets IsRemoved=true
	elem, ok := zset.Elements["m1"]
	if !ok || !elem.IsRemoved {
		t.Error("Element m1 should exist and be marked removed")
	}

	// GC
	zset.GC(now + 2)

	_, ok = zset.Elements["m1"]
	if ok {
		t.Error("Element m1 should be removed from map after GC")
	}
}

func TestStore_GC_Integration(t *testing.T) {
	// Setup store
	dir := t.TempDir()
	store, err := NewStore(dir, "localhost:6379", 0) // Mock/Real redis not strictly needed if we mock or ignore redis errors
	// Assuming local redis might not be available, we should probably mock RedisStore or tolerate errors.
	// But NewStore tries to connect.
	// For unit testing CRDT logic in Store, we might need to bypass Redis or expect failure if we can't connect.
	// However, standard tests use setupTestStore which likely handles this.
	// Let's rely on standard NewStore behavior. If it fails, we skip.
	if err != nil {
		t.Skipf("Skipping store integration test: %v", err)
	}
	defer store.Close()

	// Set short TTL for GC
	store.TombstoneTTL = time.Millisecond * 10

	// Add list and delete item
	key := "list_gc"
	store.LPush(key, []string{"a", "b"}, WithTimestamp(time.Now().UnixNano()))
	// Delete "a"
	_, _, err = store.LPop(key, WithTimestamp(time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("LPop failed: %v", err)
	}

	// Verify "a" is deleted but present as tombstone (List length check sees visible only)
	val, _ := store.Get(key)
	list := val.List()
	foundTombstone := false
	for _, e := range list.Elements {
		if e.Deleted {
			foundTombstone = true
			break
		}
	}
	if !foundTombstone {
		t.Fatal("Expected tombstone before GC")
	}

	// Trigger GC manually
	time.Sleep(time.Millisecond * 20) // Wait for TTL
	store.GC()

	// Verify tombstone is gone
	val, _ = store.Get(key)
	list = val.List()
	foundTombstone = false
	for _, e := range list.Elements {
		if e.Deleted {
			foundTombstone = true
			break
		}
	}
	if foundTombstone {
		t.Fatal("Tombstone should be gone after GC")
	}
}
