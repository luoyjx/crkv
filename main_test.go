package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
	"github.com/luoyjx/crdt-redis/server"
	"github.com/luoyjx/crdt-redis/syncer"
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
	// Skip if running short tests (since this involves HTTP servers)
	if testing.Short() {
		t.Skip("skipping multi-server integration test in short mode")
	}

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

	// Create servers with different Redis DBs to avoid conflicts
	srv1, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir1 + "/store",
		RedisAddr: "localhost:6379",
		RedisDB:   10, // Use higher DB numbers to avoid conflicts
		OpLogPath: tmpDir1 + "/oplog/operations.log",
		ReplicaID: "test-server-1",
	})
	if err != nil {
		t.Fatalf("Failed to create server 1: %v", err)
	}
	defer srv1.Close()

	srv2, err := server.NewServerWithConfig(server.Config{
		DataDir:   tmpDir2 + "/store",
		RedisAddr: "localhost:6379",
		RedisDB:   11, // Use higher DB numbers to avoid conflicts
		OpLogPath: tmpDir2 + "/oplog/operations.log",
		ReplicaID: "test-server-2",
	})
	if err != nil {
		t.Fatalf("Failed to create server 2: %v", err)
	}
	defer srv2.Close()

	// Set up HTTP sync endpoints for both servers
	httpPort1 := 18081
	httpPort2 := 18082

	// Setup HTTP endpoints for server 1
	mux1 := http.NewServeMux()
	mux1.HandleFunc("/ops", func(w http.ResponseWriter, r *http.Request) {
		sinceStr := r.URL.Query().Get("since")
		var since int64
		if sinceStr != "" {
			if v, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
				since = v
			}
		}
		ops, _ := srv1.OpLog().GetOperations(since)
		batch := proto.OperationBatch{Operations: ops}
		json.NewEncoder(w).Encode(batch)
	})
	mux1.HandleFunc("/apply", func(w http.ResponseWriter, r *http.Request) {
		var batch proto.OperationBatch
		if err := json.NewDecoder(r.Body).Decode(&batch); err == nil {
			for _, op := range batch.Operations {
				srv1.HandleOperation(r.Context(), op)
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	// Setup HTTP endpoints for server 2
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/ops", func(w http.ResponseWriter, r *http.Request) {
		sinceStr := r.URL.Query().Get("since")
		var since int64
		if sinceStr != "" {
			if v, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
				since = v
			}
		}
		ops, _ := srv2.OpLog().GetOperations(since)
		batch := proto.OperationBatch{Operations: ops}
		json.NewEncoder(w).Encode(batch)
	})
	mux2.HandleFunc("/apply", func(w http.ResponseWriter, r *http.Request) {
		var batch proto.OperationBatch
		if err := json.NewDecoder(r.Body).Decode(&batch); err == nil {
			for _, op := range batch.Operations {
				srv2.HandleOperation(r.Context(), op)
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	// Start HTTP servers
	server1 := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort1),
		Handler: mux1,
	}
	server2 := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort2),
		Handler: mux2,
	}

	go server1.ListenAndServe()
	go server2.ListenAndServe()

	defer func() {
		server1.Close()
		server2.Close()
	}()

	// Give HTTP servers time to start
	time.Sleep(100 * time.Millisecond)

	// Create syncers with peer configuration
	peer1Addr := fmt.Sprintf("http://localhost:%d", httpPort1)
	peer2Addr := fmt.Sprintf("http://localhost:%d", httpPort2)

	syncer1 := syncer.New(syncer.Config{
		SelfAddress: peer1Addr,
		Peers:       []syncer.Peer{{Address: peer2Addr}},
		Interval:    200 * time.Millisecond, // Faster sync for testing
	}, srv1)

	syncer2 := syncer.New(syncer.Config{
		SelfAddress: peer2Addr,
		Peers:       []syncer.Peer{{Address: peer1Addr}},
		Interval:    200 * time.Millisecond, // Faster sync for testing
	}, srv2)

	// Start syncers
	stopSync := make(chan struct{})
	defer close(stopSync)

	syncer1.Start(stopSync)
	syncer2.Start(stopSync)

	// Give syncers time to start
	time.Sleep(200 * time.Millisecond)

	// Set data on server 1
	if err := srv1.Set("key1", "value1", nil); err != nil {
		t.Fatalf("Failed to set key1 on server 1: %v", err)
	}

	// Set data on server 2
	if err := srv2.Set("key2", "value2", nil); err != nil {
		t.Fatalf("Failed to set key2 on server 2: %v", err)
	}

	// Wait for sync with timeout
	timeout := time.After(10 * time.Second) // Increased timeout
	syncDone := make(chan struct{})

	go func() {
		// Verify data with retries
		ticker := time.NewTicker(200 * time.Millisecond)
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
