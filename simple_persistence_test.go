package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/luoyjx/crdt-redis/storage"
)

func TestSegmentManagerBasic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "segment-basic-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Write and Read Entries", func(t *testing.T) {
		sm, err := storage.NewSegmentManager(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create segment manager: %v", err)
		}
		defer sm.Close()

		// Write entries
		entries := []*storage.LogEntry{
			{
				Timestamp: time.Now().UnixNano(),
				Operation: "SET",
				Key:       "key1",
				Value: &storage.Value{
					Type:      storage.TypeString,
					Data:      []byte("value1"),
					Timestamp: time.Now().UnixNano(),
				},
			},
			{
				Timestamp: time.Now().UnixNano(),
				Operation: "SET",
				Key:       "key2",
				Value: &storage.Value{
					Type:      storage.TypeString,
					Data:      []byte("value2"),
					Timestamp: time.Now().UnixNano(),
				},
			},
		}

		for i, entry := range entries {
			err := sm.WriteEntry(entry)
			if err != nil {
				t.Fatalf("Failed to write entry %d: %v", i, err)
			}
		}

		// Read all entries
		items, err := sm.LoadAllEntries()
		if err != nil {
			t.Fatalf("Failed to load entries: %v", err)
		}

		if len(items) != 2 {
			t.Errorf("Expected 2 items, got %d", len(items))
		}

		// Verify content
		for _, entry := range entries {
			value, exists := items[entry.Key]
			if !exists {
				t.Errorf("Key %s not found", entry.Key)
				continue
			}

			if string(value.Data) != string(entry.Value.Data) {
				t.Errorf("Key %s: expected %s, got %s",
					entry.Key, string(entry.Value.Data), string(value.Data))
			}
		}
	})

	t.Run("Delete Operations", func(t *testing.T) {
		sm, err := storage.NewSegmentManager(tmpDir + "/delete")
		if err != nil {
			t.Fatalf("Failed to create segment manager: %v", err)
		}
		defer sm.Close()

		key := "delete-key"

		// Write entry
		setEntry := &storage.LogEntry{
			Timestamp: time.Now().UnixNano(),
			Operation: "SET",
			Key:       key,
			Value: &storage.Value{
				Type:      storage.TypeString,
				Data:      []byte("delete-value"),
				Timestamp: time.Now().UnixNano(),
			},
		}
		err = sm.WriteEntry(setEntry)
		if err != nil {
			t.Fatalf("Failed to write SET entry: %v", err)
		}

		// Delete entry
		deleteEntry := &storage.LogEntry{
			Timestamp: time.Now().UnixNano(),
			Operation: "DELETE",
			Key:       key,
		}
		err = sm.WriteEntry(deleteEntry)
		if err != nil {
			t.Fatalf("Failed to write DELETE entry: %v", err)
		}

		// Load - should not contain deleted key
		items, err := sm.LoadAllEntries()
		if err != nil {
			t.Fatalf("Failed to load entries: %v", err)
		}

		if _, exists := items[key]; exists {
			t.Error("Deleted key should not exist")
		}
	})

	t.Run("Statistics", func(t *testing.T) {
		sm, err := storage.NewSegmentManager(tmpDir + "/stats")
		if err != nil {
			t.Fatalf("Failed to create segment manager: %v", err)
		}
		defer sm.Close()

		// Write some entries
		for i := 0; i < 5; i++ {
			entry := &storage.LogEntry{
				Timestamp: time.Now().UnixNano(),
				Operation: "SET",
				Key:       fmt.Sprintf("stats-key-%d", i),
				Value: &storage.Value{
					Type:      storage.TypeString,
					Data:      []byte(fmt.Sprintf("stats-value-%d", i)),
					Timestamp: time.Now().UnixNano(),
				},
			}
			err := sm.WriteEntry(entry)
			if err != nil {
				t.Fatalf("Failed to write entry %d: %v", i, err)
			}
		}

		// Get stats
		stats := sm.GetStats()

		expectedFields := []string{
			"total_segments",
			"current_segment_id",
			"max_segment_size",
			"compaction_threshold",
			"last_compaction",
			"total_size_bytes",
		}

		for _, field := range expectedFields {
			if _, exists := stats[field]; !exists {
				t.Errorf("Missing stats field: %s", field)
			}
		}

		// Check reasonable values
		totalSegments := stats["total_segments"].(int)
		totalSize := stats["total_size_bytes"].(int64)

		if totalSegments < 1 {
			t.Errorf("Expected at least 1 segment, got %d", totalSegments)
		}

		if totalSize <= 0 {
			t.Errorf("Expected positive total size, got %d", totalSize)
		}

		t.Logf("Segment stats: %+v", stats)
	})

	t.Run("Persistence Across Restarts", func(t *testing.T) {
		persistDir := tmpDir + "/persist"

		// First session: write data
		sm1, err := storage.NewSegmentManager(persistDir)
		if err != nil {
			t.Fatalf("Failed to create first segment manager: %v", err)
		}

		testData := map[string]string{
			"persist1": "value1",
			"persist2": "value2",
			"persist3": "value3",
		}

		for key, value := range testData {
			entry := &storage.LogEntry{
				Timestamp: time.Now().UnixNano(),
				Operation: "SET",
				Key:       key,
				Value: &storage.Value{
					Type:      storage.TypeString,
					Data:      []byte(value),
					Timestamp: time.Now().UnixNano(),
				},
			}
			err := sm1.WriteEntry(entry)
			if err != nil {
				t.Fatalf("Failed to write entry %s: %v", key, err)
			}
		}

		sm1.Close()

		// Second session: read data
		sm2, err := storage.NewSegmentManager(persistDir)
		if err != nil {
			t.Fatalf("Failed to create second segment manager: %v", err)
		}
		defer sm2.Close()

		items, err := sm2.LoadAllEntries()
		if err != nil {
			t.Fatalf("Failed to load entries in second session: %v", err)
		}

		if len(items) != len(testData) {
			t.Errorf("Expected %d items, got %d", len(testData), len(items))
		}

		for key, expectedValue := range testData {
			value, exists := items[key]
			if !exists {
				t.Errorf("Key %s not found in second session", key)
				continue
			}

			if string(value.Data) != expectedValue {
				t.Errorf("Key %s: expected %s, got %s",
					key, expectedValue, string(value.Data))
			}
		}
	})
}

func TestSegmentRotation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "segment-rotation-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create segment manager with small segment size for testing
	sm, err := storage.NewSegmentManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create segment manager: %v", err)
	}
	defer sm.Close()

	// Write many entries to trigger segment rotation
	numEntries := 100
	for i := 0; i < numEntries; i++ {
		entry := &storage.LogEntry{
			Timestamp: time.Now().UnixNano(),
			Operation: "SET",
			Key:       fmt.Sprintf("rotation-key-%d", i),
			Value: &storage.Value{
				Type:      storage.TypeString,
				Data:      []byte(fmt.Sprintf("rotation-value-%d-with-some-extra-data-to-make-it-larger", i)),
				Timestamp: time.Now().UnixNano(),
			},
		}
		err := sm.WriteEntry(entry)
		if err != nil {
			t.Fatalf("Failed to write entry %d: %v", i, err)
		}
	}

	// Verify all entries are still accessible
	items, err := sm.LoadAllEntries()
	if err != nil {
		t.Fatalf("Failed to load entries after rotation: %v", err)
	}

	if len(items) != numEntries {
		t.Errorf("Expected %d items after rotation, got %d", numEntries, len(items))
	}

	// Check stats
	stats := sm.GetStats()
	totalSegments := stats["total_segments"].(int)

	t.Logf("After %d entries: %d segments", numEntries, totalSegments)

	// Should have at least 1 segment
	if totalSegments < 1 {
		t.Errorf("Expected at least 1 segment, got %d", totalSegments)
	}
}
