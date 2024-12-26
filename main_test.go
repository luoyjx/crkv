package main

import (
	"os"
	"testing"
	"time"

	"github.com/luoyjx/crdt-redis/server"
)

func TestIntegrationBasic(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "crdt-redis-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories
	if err := os.MkdirAll(tmpDir+"/store", 0755); err != nil {
		t.Fatalf("Failed to create store directory: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/oplog", 0755); err != nil {
		t.Fatalf("Failed to create oplog directory: %v", err)
	}

	// Create server
	srv, err := server.NewServer(tmpDir+"/store", "localhost:6379", tmpDir+"/oplog/operations.log")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Test basic operations
	err = srv.Set("key1", "value1", nil)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, exists := srv.Get("key1")
	if !exists || value != "value1" {
		t.Errorf("Get failed. Expected 'value1', got '%s'", value)
	}
}

func TestIntegrationMultiServer(t *testing.T) {
	// Create temporary directories for servers
	tmpDir1, err := os.MkdirTemp("", "server1-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "server2-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tmpDir2)

	// Create directories for both servers
	for _, dir := range []string{tmpDir1, tmpDir2} {
		if err := os.MkdirAll(dir+"/store", 0755); err != nil {
			t.Fatalf("Failed to create store directory: %v", err)
		}
		if err := os.MkdirAll(dir+"/oplog", 0755); err != nil {
			t.Fatalf("Failed to create oplog directory: %v", err)
		}
	}

	// Create servers with shorter sync interval
	srv1, err := server.NewServerWithConfig(server.Config{
		DataDir:      tmpDir1 + "/store",
		RedisAddr:    "localhost:6379",
		RedisDB:      0,
		OpLogPath:    tmpDir1 + "/oplog/operations.log",
		SyncInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create server 1: %v", err)
	}
	defer srv1.Close()

	srv2, err := server.NewServerWithConfig(server.Config{
		DataDir:      tmpDir2 + "/store",
		RedisAddr:    "localhost:6379",
		RedisDB:      1,
		OpLogPath:    tmpDir2 + "/oplog/operations.log",
		SyncInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create server 2: %v", err)
	}
	defer srv2.Close()

	// Set data on server 1
	if err := srv1.Set("key1", "value1", nil); err != nil {
		t.Fatalf("Failed to set key1 on server 1: %v", err)
	}

	// Set data on server 2
	if err := srv2.Set("key2", "value2", nil); err != nil {
		t.Fatalf("Failed to set key2 on server 2: %v", err)
	}

	// Wait for sync with timeout
	timeout := time.After(5 * time.Second)
	syncDone := make(chan struct{})

	go func() {
		// Verify data with retries
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				val1, exists1 := srv1.Get("key2")
				val2, exists2 := srv2.Get("key1")

				if exists1 && exists2 && val1 == "value2" && val2 == "value1" {
					close(syncDone)
					return
				}
			case <-timeout:
				return
			}
		}
	}()

	// Wait for either sync completion or timeout
	select {
	case <-syncDone:
		// Success - data has synced
	case <-timeout:
		// Dump debug info
		val1, exists1 := srv1.Get("key2")
		val2, exists2 := srv2.Get("key1")
		t.Fatalf("Test timed out waiting for servers to sync. Server1: key2=%v (exists=%v), Server2: key1=%v (exists=%v)",
			val1, exists1, val2, exists2)
	}
}
