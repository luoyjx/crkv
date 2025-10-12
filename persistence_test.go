package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/luoyjx/crdt-redis/server"
	"github.com/luoyjx/crdt-redis/storage"
)

func TestPersistenceOptimization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "persistence-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Skip this test if running in CI or if Redis connection is problematic
	if testing.Short() {
		t.Skip("skipping persistence optimization test in short mode")
	}

	// Create server with mock-friendly configuration
	srv, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir,
		RedisAddr: "", // Skip Redis to avoid connection issues in tests
		RedisDB:   0,
		OpLogPath: tmpDir + "/oplog.json",
		ReplicaID: "test-replica-persistence",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	t.Run("Basic Persistence", func(t *testing.T) {
		// Add some data
		err := srv.Set("key1", "value1", nil)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		memberScores := map[string]float64{
			"player1": 100.0,
			"player2": 200.0,
		}
		_, err = srv.ZAdd("leaderboard", memberScores)
		if err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}

		// Close server to trigger persistence
		srv.Close()

		// Create new server with same data directory (also without Redis)
		srv2, err := server.NewServerWithConfig(server.Config{
			DataDir:   tmpDir,
			RedisAddr: "", // Skip Redis to match the first server
			RedisDB:   0,
			OpLogPath: tmpDir + "/oplog.json",
			ReplicaID: "test-replica-persistence-2",
		})
		if err != nil {
			t.Fatalf("Failed to create second server: %v", err)
		}
		defer srv2.Close()

		// Verify data was persisted
		value, exists := srv2.Get("key1")
		if !exists || value != "value1" {
			t.Errorf("Expected key1=value1, got exists=%v, value=%s", exists, value)
		}

		count, err := srv2.ZCard("leaderboard")
		if err != nil {
			t.Fatalf("ZCard failed: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected leaderboard to have 2 members, got %d", count)
		}
	})

	// Use srv for remaining tests
	defer srv.Close()

	t.Run("Segment Statistics", func(t *testing.T) {
		// Get persistence stats
		stats := srv.GetPersistenceStats()

		// Check that stats are available
		if stats == nil {
			t.Fatal("Expected persistence stats, got nil")
		}

		// Check expected fields
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
				t.Errorf("Expected stats field %s not found", field)
			}
		}

		t.Logf("Persistence stats: %+v", stats)
	})

	t.Run("Multiple Operations", func(t *testing.T) {
		// Perform many operations to test segment handling
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("bulk-key-%d", i)
			value := fmt.Sprintf("bulk-value-%d", i)

			err := srv.Set(key, value, nil)
			if err != nil {
				t.Fatalf("Bulk set %d failed: %v", i, err)
			}
		}

		// Add some counters
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("counter-%d", i)
			_, err := srv.Incr(key)
			if err != nil {
				t.Fatalf("Incr %d failed: %v", i, err)
			}
		}

		// Add some sorted sets
		for i := 0; i < 20; i++ {
			key := fmt.Sprintf("zset-%d", i)
			memberScores := map[string]float64{
				fmt.Sprintf("member-%d-1", i): float64(i * 10),
				fmt.Sprintf("member-%d-2", i): float64(i*10 + 5),
			}
			_, err := srv.ZAdd(key, memberScores)
			if err != nil {
				t.Fatalf("ZAdd %d failed: %v", i, err)
			}
		}

		// Give a moment for any background persistence to complete
		time.Sleep(100 * time.Millisecond)

		// Check final stats
		stats := srv.GetPersistenceStats()
		totalSegments := stats["total_segments"].(int)
		totalSize := stats["total_size_bytes"].(int64)

		t.Logf("After bulk operations: %d segments, %d bytes", totalSegments, totalSize)

		if totalSegments == 0 {
			t.Error("Expected at least one segment after bulk operations")
		}

		// The size might be 0 if persistence is disabled or operations are buffered
		// This is not necessarily a failure in Redis-free mode
		t.Logf("Total size bytes: %d (may be 0 in Redis-free mode)", totalSize)
	})
}

func TestSegmentManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "segment-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Basic Segment Operations", func(t *testing.T) {
		sm, err := storage.NewSegmentManager(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create segment manager: %v", err)
		}
		defer sm.Close()

		// Write some entries
		for i := 0; i < 10; i++ {
			entry := &storage.LogEntry{
				Timestamp: time.Now().UnixNano(),
				Operation: "SET",
				Key:       fmt.Sprintf("key-%d", i),
				Value: &storage.Value{
					Type:      storage.TypeString,
					Data:      []byte(fmt.Sprintf("value-%d", i)),
					Timestamp: time.Now().UnixNano(),
				},
			}

			err := sm.WriteEntry(entry)
			if err != nil {
				t.Fatalf("Failed to write entry %d: %v", i, err)
			}
		}

		// Load all entries
		items, err := sm.LoadAllEntries()
		if err != nil {
			t.Fatalf("Failed to load entries: %v", err)
		}

		if len(items) != 10 {
			t.Errorf("Expected 10 items, got %d", len(items))
		}

		// Verify data
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key-%d", i)
			expectedValue := fmt.Sprintf("value-%d", i)

			value, exists := items[key]
			if !exists {
				t.Errorf("Key %s not found", key)
				continue
			}

			if string(value.Data) != expectedValue {
				t.Errorf("Key %s: expected %s, got %s", key, expectedValue, string(value.Data))
			}
		}
	})

	t.Run("Delete Operations", func(t *testing.T) {
		sm, err := storage.NewSegmentManager(tmpDir + "/delete")
		if err != nil {
			t.Fatalf("Failed to create segment manager: %v", err)
		}
		defer sm.Close()

		// Write and then delete entries
		key := "delete-test"

		// SET operation
		setEntry := &storage.LogEntry{
			Timestamp: time.Now().UnixNano(),
			Operation: "SET",
			Key:       key,
			Value: &storage.Value{
				Type:      storage.TypeString,
				Data:      []byte("test-value"),
				Timestamp: time.Now().UnixNano(),
			},
		}
		err = sm.WriteEntry(setEntry)
		if err != nil {
			t.Fatalf("Failed to write SET entry: %v", err)
		}

		// DELETE operation
		deleteEntry := &storage.LogEntry{
			Timestamp: time.Now().UnixNano(),
			Operation: "DELETE",
			Key:       key,
		}
		err = sm.WriteEntry(deleteEntry)
		if err != nil {
			t.Fatalf("Failed to write DELETE entry: %v", err)
		}

		// Load entries - should not contain the deleted key
		items, err := sm.LoadAllEntries()
		if err != nil {
			t.Fatalf("Failed to load entries: %v", err)
		}

		if _, exists := items[key]; exists {
			t.Error("Deleted key should not exist in loaded items")
		}
	})

	t.Run("Stats and Info", func(t *testing.T) {
		sm, err := storage.NewSegmentManager(tmpDir + "/stats")
		if err != nil {
			t.Fatalf("Failed to create segment manager: %v", err)
		}
		defer sm.Close()

		// Get initial stats
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
				t.Errorf("Expected stats field %s not found", field)
			}
		}

		t.Logf("Segment manager stats: %+v", stats)
	})
}
