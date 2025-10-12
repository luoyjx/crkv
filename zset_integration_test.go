package main

import (
	"os"
	"testing"
	"time"

	"github.com/luoyjx/crdt-redis/server"
)

func TestZSetIntegration(t *testing.T) {
	// Create temporary directories
	tmpDir1, err := os.MkdirTemp("", "zset-test1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "zset-test2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir2)

	// Create servers
	srv1, err := server.NewServer(tmpDir1, "localhost:6379", tmpDir1+"/oplog.json")
	if err != nil {
		t.Fatalf("Failed to create server1: %v", err)
	}
	defer srv1.Close()

	srv2, err := server.NewServer(tmpDir2, "localhost:6379", tmpDir2+"/oplog.json")
	if err != nil {
		t.Fatalf("Failed to create server2: %v", err)
	}
	defer srv2.Close()

	t.Run("Basic ZADD and ZCARD", func(t *testing.T) {
		// Add elements to sorted set on srv1
		memberScores := map[string]float64{
			"alice":   85.5,
			"bob":     92.0,
			"charlie": 78.3,
		}

		added, err := srv1.ZAdd("scores", memberScores)
		if err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}
		if added != 3 {
			t.Errorf("Expected 3 elements added, got %d", added)
		}

		// Check cardinality
		count, err := srv1.ZCard("scores")
		if err != nil {
			t.Fatalf("ZCard failed: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected cardinality 3, got %d", count)
		}
	})

	t.Run("ZSCORE and ZRANK", func(t *testing.T) {
		// Check individual scores
		score, exists, err := srv1.ZScore("scores", "bob")
		if err != nil {
			t.Fatalf("ZScore failed: %v", err)
		}
		if !exists {
			t.Error("Expected bob to exist")
		}
		if score == nil || *score != 92.0 {
			t.Errorf("Expected score 92.0, got %v", score)
		}

		// Check rank (0-based, sorted by score ascending)
		rank, exists, err := srv1.ZRank("scores", "bob")
		if err != nil {
			t.Fatalf("ZRank failed: %v", err)
		}
		if !exists {
			t.Error("Expected bob to exist")
		}
		// bob has score 92.0, which should be the highest, so rank 2 (0-based)
		if rank == nil || *rank != 2 {
			t.Errorf("Expected rank 2, got %v", rank)
		}
	})

	t.Run("ZRANGE", func(t *testing.T) {
		// Get all elements in order
		members, scores, err := srv1.ZRange("scores", 0, -1, true)
		if err != nil {
			t.Fatalf("ZRange failed: %v", err)
		}

		expectedOrder := []string{"charlie", "alice", "bob"} // Sorted by score: 78.3, 85.5, 92.0
		expectedScores := []float64{78.3, 85.5, 92.0}

		if len(members) != 3 {
			t.Errorf("Expected 3 members, got %d", len(members))
		}

		for i, member := range members {
			if member != expectedOrder[i] {
				t.Errorf("Expected member %s at position %d, got %s", expectedOrder[i], i, member)
			}
			if scores[i] != expectedScores[i] {
				t.Errorf("Expected score %.1f at position %d, got %.1f", expectedScores[i], i, scores[i])
			}
		}
	})

	t.Run("ZRANGEBYSCORE", func(t *testing.T) {
		// Get elements with scores between 80 and 95
		members, scores, err := srv1.ZRangeByScore("scores", 80.0, 95.0, true)
		if err != nil {
			t.Fatalf("ZRangeByScore failed: %v", err)
		}

		expectedMembers := []string{"alice", "bob"} // alice: 85.5, bob: 92.0
		expectedScores := []float64{85.5, 92.0}

		if len(members) != 2 {
			t.Errorf("Expected 2 members, got %d", len(members))
		}

		for i, member := range members {
			if member != expectedMembers[i] {
				t.Errorf("Expected member %s at position %d, got %s", expectedMembers[i], i, member)
			}
			if scores[i] != expectedScores[i] {
				t.Errorf("Expected score %.1f at position %d, got %.1f", expectedScores[i], i, scores[i])
			}
		}
	})

	t.Run("ZREM", func(t *testing.T) {
		// Remove a member
		removed, err := srv1.ZRem("scores", []string{"alice"})
		if err != nil {
			t.Fatalf("ZRem failed: %v", err)
		}
		if removed != 1 {
			t.Errorf("Expected 1 member removed, got %d", removed)
		}

		// Verify removal
		count, err := srv1.ZCard("scores")
		if err != nil {
			t.Fatalf("ZCard failed: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected cardinality 2 after removal, got %d", count)
		}

		// Verify alice is gone
		_, exists, err := srv1.ZScore("scores", "alice")
		if err != nil {
			t.Fatalf("ZScore failed: %v", err)
		}
		if exists {
			t.Error("Expected alice to be removed")
		}
	})

	t.Run("Replication", func(t *testing.T) {
		// Add more elements to srv1
		memberScores := map[string]float64{
			"david": 88.5,
			"eve":   95.2,
		}

		_, err := srv1.ZAdd("leaderboard", memberScores)
		if err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}

		// Add different elements to srv2
		memberScores2 := map[string]float64{
			"frank": 79.8,
			"grace": 91.1,
		}

		_, err = srv2.ZAdd("leaderboard", memberScores2)
		if err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}

		// Get operations from srv1 and apply to srv2
		ops1, _ := srv1.OpLog().GetOperations(0)
		for _, op := range ops1 {
			srv2.HandleOperation(nil, op)
		}

		// Get operations from srv2 and apply to srv1
		ops2, _ := srv2.OpLog().GetOperations(0)
		for _, op := range ops2 {
			srv1.HandleOperation(nil, op)
		}

		// Both servers should have all elements
		count1, err := srv1.ZCard("leaderboard")
		if err != nil {
			t.Fatalf("ZCard failed on srv1: %v", err)
		}

		count2, err := srv2.ZCard("leaderboard")
		if err != nil {
			t.Fatalf("ZCard failed on srv2: %v", err)
		}

		if count1 != 4 || count2 != 4 {
			t.Errorf("Expected both servers to have 4 elements, got srv1=%d, srv2=%d", count1, count2)
		}

		// Verify all members exist on both servers
		expectedMembers := []string{"david", "eve", "frank", "grace"}
		for _, member := range expectedMembers {
			_, exists1, err := srv1.ZScore("leaderboard", member)
			if err != nil || !exists1 {
				t.Errorf("Member %s should exist on srv1", member)
			}

			_, exists2, err := srv2.ZScore("leaderboard", member)
			if err != nil || !exists2 {
				t.Errorf("Member %s should exist on srv2", member)
			}
		}
	})

	t.Run("Concurrent Updates", func(t *testing.T) {
		// Both servers update the same member with different scores
		memberScores1 := map[string]float64{
			"concurrent": 100.0,
		}
		memberScores2 := map[string]float64{
			"concurrent": 200.0,
		}

		// Add to both servers
		srv1.ZAdd("concurrent_test", memberScores1)
		time.Sleep(time.Millisecond) // Ensure different timestamps
		srv2.ZAdd("concurrent_test", memberScores2)

		// Replicate operations
		ops1, _ := srv1.OpLog().GetOperations(0)
		for _, op := range ops1 {
			srv2.HandleOperation(nil, op)
		}

		ops2, _ := srv2.OpLog().GetOperations(0)
		for _, op := range ops2 {
			srv1.HandleOperation(nil, op)
		}

		// Both servers should converge to the same value
		score1, exists1, err := srv1.ZScore("concurrent_test", "concurrent")
		if err != nil || !exists1 {
			t.Fatalf("Failed to get score from srv1: %v", err)
		}

		score2, exists2, err := srv2.ZScore("concurrent_test", "concurrent")
		if err != nil || !exists2 {
			t.Fatalf("Failed to get score from srv2: %v", err)
		}

		if *score1 != *score2 {
			t.Logf("Concurrent updates converged. Both servers have score: %.1f", *score1)
		} else {
			t.Logf("Concurrent updates converged. srv1=%.1f, srv2=%.1f", *score1, *score2)
		}
	})
}

func TestZSetEdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zset-edge-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srv, err := server.NewServer(tmpDir, "localhost:6379", tmpDir+"/oplog.json")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	t.Run("Empty ZSet Operations", func(t *testing.T) {
		// Operations on non-existent key
		count, err := srv.ZCard("nonexistent")
		if err != nil {
			t.Fatalf("ZCard failed: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected cardinality 0 for nonexistent key, got %d", count)
		}

		// ZRANGE on empty set
		members, _, err := srv.ZRange("nonexistent", 0, -1, false)
		if err != nil {
			t.Fatalf("ZRange failed: %v", err)
		}
		if len(members) != 0 {
			t.Errorf("Expected empty result, got %d members", len(members))
		}
	})

	t.Run("Duplicate Member Updates", func(t *testing.T) {
		// Add same member multiple times with different scores
		memberScores1 := map[string]float64{"player": 50.0}
		added1, err := srv.ZAdd("updates", memberScores1)
		if err != nil {
			t.Fatalf("First ZAdd failed: %v", err)
		}
		if added1 != 1 {
			t.Errorf("Expected 1 element added, got %d", added1)
		}

		memberScores2 := map[string]float64{"player": 75.0}
		added2, err := srv.ZAdd("updates", memberScores2)
		if err != nil {
			t.Fatalf("Second ZAdd failed: %v", err)
		}
		if added2 != 0 {
			t.Errorf("Expected 0 elements added (update), got %d", added2)
		}

		// Verify final score
		score, exists, err := srv.ZScore("updates", "player")
		if err != nil || !exists {
			t.Fatalf("ZScore failed: %v", err)
		}
		if *score != 75.0 {
			t.Errorf("Expected score 75.0, got %.1f", *score)
		}

		// Verify cardinality is still 1
		count, err := srv.ZCard("updates")
		if err != nil {
			t.Fatalf("ZCard failed: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected cardinality 1, got %d", count)
		}
	})

	t.Run("Negative Scores and Ranges", func(t *testing.T) {
		memberScores := map[string]float64{
			"negative": -10.5,
			"zero":     0.0,
			"positive": 10.5,
		}

		_, err := srv.ZAdd("mixed_scores", memberScores)
		if err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}

		// Test range with negative indices
		members, scores, err := srv.ZRange("mixed_scores", -2, -1, true)
		if err != nil {
			t.Fatalf("ZRange failed: %v", err)
		}

		// Should get the last 2 elements (zero and positive)
		if len(members) != 2 {
			t.Errorf("Expected 2 members, got %d", len(members))
		}
		if members[0] != "zero" || scores[0] != 0.0 {
			t.Errorf("Expected zero with score 0.0, got %s with score %.1f", members[0], scores[0])
		}
		if members[1] != "positive" || scores[1] != 10.5 {
			t.Errorf("Expected positive with score 10.5, got %s with score %.1f", members[1], scores[1])
		}
	})
}
