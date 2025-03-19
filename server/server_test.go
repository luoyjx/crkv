package server

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/luoyjx/crdt-redis/routing"
)

func TestServerSetGet(t *testing.T) {
	// Create temporary directory for server data
	tmpDir, err := os.MkdirTemp("", "server-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories for different components
	if err := os.MkdirAll(tmpDir+"/store", 0755); err != nil {
		t.Fatalf("Failed to create store directory: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/oplog", 0755); err != nil {
		t.Fatalf("Failed to create oplog directory: %v", err)
	}

	// Create server
	server, err := NewServer(tmpDir+"/store", "localhost:6379", tmpDir+"/oplog/operations.log")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Test basic set and get
	err = server.Set("key1", "value1", nil)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, exists := server.Get("key1")
	if !exists {
		t.Error("Get returned not exists for existing key")
	}
	if value != "value1" {
		t.Errorf("Get returned wrong value. Expected 'value1', got '%s'", value)
	}

	// Test get non-existent key
	_, exists = server.Get("nonexistent")
	if exists {
		t.Error("Get returned exists for non-existent key")
	}
}

// getTestServerConfig returns a unique server configuration for testing
func getTestServerConfig(t *testing.T, portOffset int) (string, string, string) {
	tmpDir, err := os.MkdirTemp("", "server-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create subdirectories
	if err := os.MkdirAll(tmpDir+"/store", 0755); err != nil {
		t.Fatalf("Failed to create store directory: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/oplog", 0755); err != nil {
		t.Fatalf("Failed to create oplog directory: %v", err)
	}

	// Use unique ports based on portOffset
	redisPort := 6379
	peerPort := 8081 + portOffset*2

	return tmpDir, fmt.Sprintf("localhost:%d", redisPort), fmt.Sprintf("localhost:%d", peerPort)
}

func TestServerInterServerSync(t *testing.T) {
	// Get unique configurations for two servers
	tmpDir1, redisAddr1, peerAddr1 := getTestServerConfig(t, 0)
	defer os.RemoveAll(tmpDir1)

	tmpDir2, redisAddr2, peerAddr2 := getTestServerConfig(t, 1)
	defer os.RemoveAll(tmpDir2)

	// Create two servers with unique configurations
	server1, err := NewServerWithConfig(Config{
		DataDir:   tmpDir1 + "/store",
		RedisAddr: redisAddr1,
		OpLogPath: tmpDir1 + "/oplog/operations.log",
		ReplicaID: "server1",
		RouterConfig: routing.Config{
			SelfID: "server1",
			Addr:   peerAddr1,
		},
		ListenAddr: peerAddr1,
	})
	if err != nil {
		t.Fatalf("Failed to create server1: %v", err)
	}
	defer server1.Close()

	server2, err := NewServerWithConfig(Config{
		DataDir:   tmpDir2 + "/store",
		RedisAddr: redisAddr2,
		OpLogPath: tmpDir2 + "/oplog/operations.log",
		ReplicaID: "server2",
		RouterConfig: routing.Config{
			SelfID: "server2",
			Addr:   peerAddr2,
		},
		ListenAddr: peerAddr2,
	})
	if err != nil {
		t.Fatalf("Failed to create server2: %v", err)
	}
	defer server2.Close()

	// Wait for servers to initialize
	time.Sleep(500 * time.Millisecond)

	// Connect server1 to server2
	node2 := &routing.Node{
		ID:   "server2",
		Addr: peerAddr2,
	}
	err = server1.peerManager.Connect(context.Background(), node2)
	if err != nil {
		t.Fatalf("Failed to connect server1 to server2: %v", err)
	}

	// Set value on server1
	err = server1.Set("key1", "value1", nil)
	if err != nil {
		t.Errorf("Set on server1 failed: %v", err)
	}

	// Wait for sync
	time.Sleep(2 * time.Second)

	// Get value from server2
	value2, exists := server2.Get("key1")
	if !exists {
		t.Errorf("Value not synced to server2")
	}
	if value2 != "value1" {
		t.Errorf("Server2 has wrong value. Expected 'value1', got '%s'", value2)
	}
}

func TestServerOperationSync(t *testing.T) {
	// Create temporary directory for server data
	tmpDir, err := os.MkdirTemp("", "server-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories for different components
	if err := os.MkdirAll(tmpDir+"/store", 0755); err != nil {
		t.Fatalf("Failed to create store directory: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/oplog", 0755); err != nil {
		t.Fatalf("Failed to create oplog directory: %v", err)
	}

	// Create server
	server, err := NewServer(tmpDir+"/store", "localhost:6379", tmpDir+"/oplog/operations.log")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Add some operations
	err = server.Set("key1", "value1", nil)
	if err != nil {
		t.Errorf("Set key1 failed: %v", err)
	}

	err = server.Set("key2", "value2", nil)
	if err != nil {
		t.Errorf("Set key2 failed: %v", err)
	}

	// Wait for sync to complete
	time.Sleep(time.Second)

	// Verify both values are present
	value1, exists := server.Get("key1")
	if !exists || value1 != "value1" {
		t.Errorf("After sync, key1 has wrong value. Expected 'value1', got '%s'", value1)
	}

	value2, exists := server.Get("key2")
	if !exists || value2 != "value2" {
		t.Errorf("After sync, key2 has wrong value. Expected 'value2', got '%s'", value2)
	}
}

func TestServerClose(t *testing.T) {
	// Create temporary directory for server data
	tmpDir, err := os.MkdirTemp("", "server-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories for different components
	if err := os.MkdirAll(tmpDir+"/store", 0755); err != nil {
		t.Fatalf("Failed to create store directory: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/oplog", 0755); err != nil {
		t.Fatalf("Failed to create oplog directory: %v", err)
	}

	// Create first server
	server, err := NewServer(tmpDir+"/store", "localhost:6379", tmpDir+"/oplog/operations.log")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Set some data
	err = server.Set("key1", "value1", nil)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Close server
	err = server.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Create new server with same directories
	server2, err := NewServer(tmpDir+"/store", "localhost:6379", tmpDir+"/oplog/operations.log")
	if err != nil {
		t.Fatalf("Failed to create second server: %v", err)
	}
	defer server2.Close()

	// Verify data persisted
	value, exists := server2.Get("key1")
	if !exists || value != "value1" {
		t.Errorf("After close and reopen, wrong value. Expected 'value1', got '%s'", value)
	}
}
