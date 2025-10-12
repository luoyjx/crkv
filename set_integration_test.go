package main

import (
	"os"
	"testing"

	"github.com/luoyjx/crdt-redis/server"
)

func TestCRDTSetsReplication(t *testing.T) {
	// Create temp directories for two servers
	tmpDir1, err := os.MkdirTemp("", "server1-set-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "server2-set-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tmpDir2)

	// Create servers with unique Redis DBs
	srv1, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir1,
		RedisAddr: "localhost:6379",
		RedisDB:   5, // Use DB 5 for set tests
		OpLogPath: tmpDir1 + "/oplog.json",
		ReplicaID: "server1",
	})
	if err != nil {
		t.Fatalf("Failed to create server1: %v", err)
	}
	defer srv1.Close()

	srv2, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir2,
		RedisAddr: "localhost:6379",
		RedisDB:   6, // Use DB 6 for set tests
		OpLogPath: tmpDir2 + "/oplog.json",
		ReplicaID: "server2",
	})
	if err != nil {
		t.Fatalf("Failed to create server2: %v", err)
	}
	defer srv2.Close()

	// Test SADD/SREM replication
	t.Run("SADD/SREM replication", func(t *testing.T) {
		// SADD on server1
		added1, err := srv1.SAdd("myset", "a", "b", "c")
		if err != nil || added1 != 3 {
			t.Errorf("Server1 SADD failed: err=%v, added=%d", err, added1)
		}

		// SADD on server2 (note: 'a' doesn't exist yet in server2's local view)
		added2, err := srv2.SAdd("myset", "d", "e", "a")
		if err != nil || added2 != 3 { // All 3 elements are new to server2
			t.Errorf("Server2 SADD failed: err=%v, added=%d", err, added2)
		}

		// Manually sync operations
		ops1, _ := srv1.OpLog().GetOperations(0)
		ops2, _ := srv2.OpLog().GetOperations(0)

		for _, op := range ops1 {
			if op.Command == "SADD" && len(op.Args) > 0 && op.Args[0] == "myset" {
				_ = srv2.HandleOperation(nil, op)
			}
		}
		for _, op := range ops2 {
			if op.Command == "SADD" && len(op.Args) > 0 && op.Args[0] == "myset" {
				_ = srv1.HandleOperation(nil, op)
			}
		}

		// Both servers should have all elements after sync
		members1, err := srv1.SMembers("myset")
		if err != nil {
			t.Errorf("Server1 SMEMBERS failed: %v", err)
		}

		members2, err := srv2.SMembers("myset")
		if err != nil {
			t.Errorf("Server2 SMEMBERS failed: %v", err)
		}

		// Both should have the same elements
		if len(members1) != 5 || len(members2) != 5 {
			t.Errorf("Expected 5 elements after sync. srv1=%d, srv2=%d", len(members1), len(members2))
		}

		// Check cardinality
		card1, _ := srv1.SCard("myset")
		card2, _ := srv2.SCard("myset")
		if card1 != 5 || card2 != 5 {
			t.Errorf("Expected cardinality 5. srv1=%d, srv2=%d", card1, card2)
		}

		t.Logf("Server1 set: %v", members1)
		t.Logf("Server2 set: %v", members2)
	})

	// Test SREM operations
	t.Run("SREM operations", func(t *testing.T) {
		// Setup a set with known elements
		_, _ = srv1.SAdd("removeset", "x", "y", "z")

		// Sync to server2
		ops1, _ := srv1.OpLog().GetOperations(0)
		for _, op := range ops1 {
			if len(op.Args) > 0 && op.Args[0] == "removeset" {
				_ = srv2.HandleOperation(nil, op)
			}
		}

		// Both should have the same set now
		card1, _ := srv1.SCard("removeset")
		card2, _ := srv2.SCard("removeset")
		if card1 != card2 {
			t.Errorf("Set cardinalities should be equal before remove operations. srv1=%d, srv2=%d", card1, card2)
		}

		// SREM on server1
		removed1, err := srv1.SRem("removeset", "x", "nonexistent")
		if err != nil || removed1 != 1 {
			t.Errorf("Server1 SREM failed: err=%v, removed=%d", err, removed1)
		}

		// SREM on server2
		removed2, err := srv2.SRem("removeset", "y")
		if err != nil || removed2 != 1 {
			t.Errorf("Server2 SREM failed: err=%v, removed=%d", err, removed2)
		}

		// Sync remove operations
		ops1, _ = srv1.OpLog().GetOperations(0)
		ops2, _ := srv2.OpLog().GetOperations(0)

		for _, op := range ops1 {
			if op.Command == "SREM" && len(op.Args) > 0 && op.Args[0] == "removeset" {
				_ = srv2.HandleOperation(nil, op)
			}
		}
		for _, op := range ops2 {
			if op.Command == "SREM" && len(op.Args) > 0 && op.Args[0] == "removeset" {
				_ = srv1.HandleOperation(nil, op)
			}
		}

		// Both should have consistent set after sync
		finalCard1, _ := srv1.SCard("removeset")
		finalCard2, _ := srv2.SCard("removeset")

		if finalCard1 != finalCard2 {
			t.Errorf("Final set cardinalities should be equal. srv1=%d, srv2=%d", finalCard1, finalCard2)
		}

		// Should have only 'z' remaining
		if finalCard1 != 1 {
			t.Errorf("Expected final cardinality 1, got %d", finalCard1)
		}

		// Check membership
		exists1, _ := srv1.SIsMember("removeset", "z")
		exists2, _ := srv2.SIsMember("removeset", "z")
		if !exists1 || !exists2 {
			t.Errorf("Element 'z' should exist in both sets")
		}

		t.Logf("Final set cardinalities: srv1=%d, srv2=%d", finalCard1, finalCard2)
	})

	// Test concurrent set operations
	t.Run("Concurrent set operations", func(t *testing.T) {
		// Both servers add to the same set concurrently
		_, _ = srv1.SAdd("concurrent", "srv1-a", "srv1-b", "common")
		_, _ = srv2.SAdd("concurrent", "srv2-x", "srv2-y", "common")

		// Cross-sync all operations
		ops1, _ := srv1.OpLog().GetOperations(0)
		ops2, _ := srv2.OpLog().GetOperations(0)

		for _, op := range ops1 {
			if len(op.Args) > 0 && op.Args[0] == "concurrent" {
				_ = srv2.HandleOperation(nil, op)
			}
		}
		for _, op := range ops2 {
			if len(op.Args) > 0 && op.Args[0] == "concurrent" {
				_ = srv1.HandleOperation(nil, op)
			}
		}

		// Both should have all unique elements
		final1, _ := srv1.SMembers("concurrent")
		final2, _ := srv2.SMembers("concurrent")

		if len(final1) != 5 || len(final2) != 5 {
			t.Errorf("Expected 5 unique elements after concurrent ops. srv1=%d, srv2=%d", len(final1), len(final2))
		}

		// Sets should converge to the same state
		if len(final1) == len(final2) {
			t.Logf("Sets converged successfully with %d elements", len(final1))
		}
	})
}

func TestSetBasicOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "set-basic-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srv, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir,
		RedisAddr: "localhost:6379",
		RedisDB:   7,
		OpLogPath: tmpDir + "/oplog.json",
		ReplicaID: "test-server",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Test SADD and SCARD
	added, err := srv.SAdd("testset", "a", "b", "c", "a") // duplicate 'a'
	if err != nil || added != 3 {
		t.Errorf("SADD failed: err=%v, added=%d", err, added)
	}

	// Test SCARD
	cardinality, err := srv.SCard("testset")
	if err != nil || cardinality != 3 {
		t.Errorf("SCARD failed: err=%v, cardinality=%d", err, cardinality)
	}

	// Test SISMEMBER
	exists, err := srv.SIsMember("testset", "a")
	if err != nil || !exists {
		t.Errorf("SISMEMBER failed: err=%v, exists=%v", err, exists)
	}

	notExists, err := srv.SIsMember("testset", "nonexistent")
	if err != nil || notExists {
		t.Errorf("SISMEMBER should return false for nonexistent: err=%v, exists=%v", err, notExists)
	}

	// Test SMEMBERS
	members, err := srv.SMembers("testset")
	if err != nil {
		t.Errorf("SMEMBERS failed: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("SMEMBERS returned wrong number of elements: %d", len(members))
	}

	// Test SREM
	removed, err := srv.SRem("testset", "a", "nonexistent")
	if err != nil || removed != 1 {
		t.Errorf("SREM failed: err=%v, removed=%d", err, removed)
	}

	// Test final cardinality
	finalCard, _ := srv.SCard("testset")
	if finalCard != 2 {
		t.Errorf("Final cardinality should be 2, got %d", finalCard)
	}
}
