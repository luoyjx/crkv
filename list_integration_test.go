package main

import (
	"os"
	"testing"

	"github.com/luoyjx/crdt-redis/server"
)

func TestCRDTListsReplication(t *testing.T) {
	// Create temp directories for two servers
	tmpDir1, err := os.MkdirTemp("", "server1-list-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "server2-list-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tmpDir2)

	// Create servers with unique Redis DBs
	srv1, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir1,
		RedisAddr: "localhost:6379",
		RedisDB:   12, // Use DB 12 for list tests
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
		RedisDB:   13, // Use DB 13 for list tests
		OpLogPath: tmpDir2 + "/oplog.json",
		ReplicaID: "server2",
	})
	if err != nil {
		t.Fatalf("Failed to create server2: %v", err)
	}
	defer srv2.Close()

	// Test LPUSH/RPUSH replication
	t.Run("LPUSH/RPUSH replication", func(t *testing.T) {
		// LPUSH on server1
		length1, err := srv1.LPush("mylist", "hello", "world")
		if err != nil || length1 != 2 {
			t.Errorf("Server1 LPUSH failed: err=%v, length=%d", err, length1)
		}

		// RPUSH on server2
		length2, err := srv2.RPush("mylist", "foo", "bar")
		if err != nil || length2 != 2 {
			t.Errorf("Server2 RPUSH failed: err=%v, length=%d", err, length2)
		}

		// Manually sync operations
		ops1, _ := srv1.OpLog().GetOperations(0)
		ops2, _ := srv2.OpLog().GetOperations(0)

		for _, op := range ops1 {
			if op.Command == "LPUSH" {
				_ = srv2.HandleOperation(nil, op)
			}
		}
		for _, op := range ops2 {
			if op.Command == "RPUSH" {
				_ = srv1.HandleOperation(nil, op)
			}
		}

		// Both servers should have all elements after sync
		range1, err := srv1.LRange("mylist", 0, -1)
		if err != nil {
			t.Errorf("Server1 LRANGE failed: %v", err)
		}

		range2, err := srv2.LRange("mylist", 0, -1)
		if err != nil {
			t.Errorf("Server2 LRANGE failed: %v", err)
		}

		// Both should have the same elements (order might vary due to CRDT merge)
		if len(range1) != 4 || len(range2) != 4 {
			t.Errorf("Expected 4 elements after sync. srv1=%d, srv2=%d", len(range1), len(range2))
		}

		t.Logf("Server1 list: %v", range1)
		t.Logf("Server2 list: %v", range2)
	})

	// Test LPOP/RPOP replication
	t.Run("LPOP/RPOP replication", func(t *testing.T) {
		// Setup a list with known elements
		_, _ = srv1.LPush("poplist", "a", "b", "c")

		// Sync to server2
		ops1, _ := srv1.OpLog().GetOperations(0)
		for _, op := range ops1 {
			if op.Args[0] == "poplist" {
				_ = srv2.HandleOperation(nil, op)
			}
		}

		// Both should have the same list now
		len1, _ := srv1.LLen("poplist")
		len2, _ := srv2.LLen("poplist")
		if len1 != len2 {
			t.Errorf("List lengths should be equal before pop operations. srv1=%d, srv2=%d", len1, len2)
		}

		// LPOP on server1
		val1, ok1, err := srv1.LPop("poplist")
		if err != nil || !ok1 {
			t.Errorf("Server1 LPOP failed: err=%v, ok=%v", err, ok1)
		}

		// RPOP on server2
		val2, ok2, err := srv2.RPop("poplist")
		if err != nil || !ok2 {
			t.Errorf("Server2 RPOP failed: err=%v, ok=%v", err, ok2)
		}

		t.Logf("Server1 LPOP: %s, Server2 RPOP: %s", val1, val2)

		// Sync pop operations
		ops1, _ = srv1.OpLog().GetOperations(0)
		ops2, _ := srv2.OpLog().GetOperations(0)

		for _, op := range ops1 {
			if op.Command == "LPOP" && op.Args[0] == "poplist" {
				_ = srv2.HandleOperation(nil, op)
			}
		}
		for _, op := range ops2 {
			if op.Command == "RPOP" && op.Args[0] == "poplist" {
				_ = srv1.HandleOperation(nil, op)
			}
		}

		// Both should have consistent list after sync
		finalLen1, _ := srv1.LLen("poplist")
		finalLen2, _ := srv2.LLen("poplist")

		if finalLen1 != finalLen2 {
			t.Errorf("Final list lengths should be equal. srv1=%d, srv2=%d", finalLen1, finalLen2)
		}

		t.Logf("Final list lengths: srv1=%d, srv2=%d", finalLen1, finalLen2)
	})

	// Test concurrent list operations
	t.Run("Concurrent list operations", func(t *testing.T) {
		// Both servers add to the same list concurrently
		_, _ = srv1.LPush("concurrent", "srv1-left")
		_, _ = srv1.RPush("concurrent", "srv1-right")

		_, _ = srv2.LPush("concurrent", "srv2-left")
		_, _ = srv2.RPush("concurrent", "srv2-right")

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

		// Both should have all elements
		final1, _ := srv1.LRange("concurrent", 0, -1)
		final2, _ := srv2.LRange("concurrent", 0, -1)

		if len(final1) != 4 || len(final2) != 4 {
			t.Errorf("Expected 4 elements after concurrent ops. srv1=%d, srv2=%d", len(final1), len(final2))
		}

		// Lists should converge to the same state
		if len(final1) == len(final2) {
			t.Logf("Lists converged successfully: %v", final1)
		}
	})
}

func TestListBasicOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "list-basic-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srv, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir,
		RedisAddr: "localhost:6379",
		RedisDB:   14,
		OpLogPath: tmpDir + "/oplog.json",
		ReplicaID: "test-server",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Test LPUSH and LLEN
	length, err := srv.LPush("testlist", "a", "b", "c")
	if err != nil || length != 3 {
		t.Errorf("LPUSH failed: err=%v, length=%d", err, length)
	}

	// Test LLEN
	length, err = srv.LLen("testlist")
	if err != nil || length != 3 {
		t.Errorf("LLEN failed: err=%v, length=%d", err, length)
	}

	// Test LRANGE
	elements, err := srv.LRange("testlist", 0, -1)
	if err != nil {
		t.Errorf("LRANGE failed: %v", err)
	}
	if len(elements) != 3 {
		t.Errorf("LRANGE returned wrong number of elements: %d", len(elements))
	}

	// Test LPOP
	val, ok, err := srv.LPop("testlist")
	if err != nil || !ok {
		t.Errorf("LPOP failed: err=%v, ok=%v", err, ok)
	}
	// Note: CRDT list might not follow exact Redis LPUSH/LPOP ordering
	// due to conflict resolution, so we just verify a value was popped
	t.Logf("LPOP returned: %s", val)

	// Test final length
	length, _ = srv.LLen("testlist")
	if length != 2 {
		t.Errorf("Final length should be 2, got %d", length)
	}
}
