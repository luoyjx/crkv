package server

import (
	"os"
	"testing"
	"time"
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
