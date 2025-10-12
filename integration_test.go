package main

import (
	"os"
	"testing"

	"github.com/luoyjx/crdt-redis/server"
)

func TestTwoServerReplication(t *testing.T) {
	// Create temp directories for two servers
	tmpDir1, err := os.MkdirTemp("", "server1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "server2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tmpDir2)

	// Create servers with unique Redis DBs and configs
	srv1, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir1,
		RedisAddr: "localhost:6379",
		RedisDB:   10, // Use DB 10 for tests
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
		RedisDB:   11, // Use DB 11 for tests
		OpLogPath: tmpDir2 + "/oplog.json",
		ReplicaID: "server2",
	})
	if err != nil {
		t.Fatalf("Failed to create server2: %v", err)
	}
	defer srv2.Close()

	// Create syncers with cross-replication (in real setup, would use HTTP endpoints)
	// For test simplicity, we'll manually sync operations between servers
	stopSync := make(chan struct{})
	defer close(stopSync)

	// Test basic SET/GET replication
	t.Run("SET/GET replication", func(t *testing.T) {
		// Set on server1
		err := srv1.Set("key1", "value1", nil)
		if err != nil {
			t.Errorf("Server1 SET failed: %v", err)
		}

		// Manually sync operations from srv1 to srv2
		ops1, _ := srv1.OpLog().GetOperations(0)
		for _, op := range ops1 {
			_ = srv2.HandleOperation(nil, op)
		}

		// Check if server2 has the value
		val2, exists := srv2.Get("key1")
		if !exists || val2 != "value1" {
			t.Errorf("Server2 GET failed. Expected 'value1', got '%s', exists=%v", val2, exists)
		}
	})

	// Test counter replication
	t.Run("INCR replication", func(t *testing.T) {
		// Increment on both servers
		val1, err := srv1.Incr("counter")
		if err != nil {
			t.Errorf("Server1 INCR failed: %v", err)
		}
		if val1 != 1 {
			t.Errorf("Server1 INCR expected 1, got %d", val1)
		}

		val2, err := srv2.IncrBy("counter", 3)
		if err != nil {
			t.Errorf("Server2 INCRBY failed: %v", err)
		}
		if val2 != 3 {
			t.Errorf("Server2 INCRBY expected 3, got %d", val2)
		}

		// Cross-sync operations
		ops1, _ := srv1.OpLog().GetOperations(0)
		ops2, _ := srv2.OpLog().GetOperations(0)

		for _, op := range ops1 {
			if op.Command == "INCR" && op.Args[0] == "counter" {
				_ = srv2.HandleOperation(nil, op)
			}
		}
		for _, op := range ops2 {
			if op.Command == "INCRBY" && op.Args[0] == "counter" {
				_ = srv1.HandleOperation(nil, op)
			}
		}

		// Both servers should have accumulated the counter operations
		// Server1: 1 (local) + 3 (from server2) = 4
		// Server2: 3 (local) + 1 (from server1) = 4
		finalVal1, exists1 := srv1.Get("counter")
		finalVal2, exists2 := srv2.Get("counter")

		if !exists1 || !exists2 {
			t.Errorf("Counter should exist on both servers. exists1=%v, exists2=%v", exists1, exists2)
		}

		// Both should converge to the same value after replication
		if finalVal1 != finalVal2 {
			t.Errorf("Counter values should be equal after sync. srv1=%s, srv2=%s", finalVal1, finalVal2)
		}
	})

	// Test DELETE replication
	t.Run("DEL replication", func(t *testing.T) {
		// Set a key on server1
		err := srv1.Set("delkey", "delvalue", nil)
		if err != nil {
			t.Errorf("Server1 SET failed: %v", err)
		}

		// Sync to server2
		ops1, _ := srv1.OpLog().GetOperations(0)
		for _, op := range ops1 {
			if op.Args[0] == "delkey" {
				_ = srv2.HandleOperation(nil, op)
			}
		}

		// Verify both have the key
		_, exists1 := srv1.Get("delkey")
		_, exists2 := srv2.Get("delkey")
		if !exists1 || !exists2 {
			t.Errorf("Key should exist on both servers before delete")
		}

		// Delete on server2
		removed, err := srv2.Del("delkey")
		if err != nil || removed != 1 {
			t.Errorf("Server2 DEL failed: err=%v, removed=%d", err, removed)
		}

		// Sync delete operation to server1
		ops2, _ := srv2.OpLog().GetOperations(0)
		for _, op := range ops2 {
			if op.Type.String() == "DELETE" && op.Args[0] == "delkey" {
				_ = srv1.HandleOperation(nil, op)
			}
		}

		// Both servers should not have the key
		_, exists1 = srv1.Get("delkey")
		_, exists2 = srv2.Get("delkey")
		if exists1 || exists2 {
			t.Errorf("Key should not exist on either server after delete. exists1=%v, exists2=%v", exists1, exists2)
		}
	})
}

func TestSyncerIntegration(t *testing.T) {
	// This test would require HTTP endpoints to be running
	// For now, we'll skip it as it requires more complex setup
	t.Skip("Syncer integration test requires HTTP server setup")

	// Future: start two servers with HTTP sync endpoints
	// and test actual network-based replication
}
