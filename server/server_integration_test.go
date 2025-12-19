package server_test

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
	peerPort := 8081 + portOffset*2

	return tmpDir, "", fmt.Sprintf("localhost:%d", peerPort)
}

func TestServerInterServerSync(t *testing.T) {
	// Get unique configurations for two servers
	tmpDir1, redisAddr1, peerAddr1 := getTestServerConfig(t, 0)
	defer os.RemoveAll(tmpDir1)

	tmpDir2, redisAddr2, peerAddr2 := getTestServerConfig(t, 1)
	defer os.RemoveAll(tmpDir2)

	// Create two servers with unique configurations
	server1, err := server.NewServerWithConfig(server.Config{
		DataDir:    tmpDir1 + "/store",
		RedisAddr:  redisAddr1,
		OpLogPath:  tmpDir1 + "/oplog/operations.log",
		ReplicaID:  "server1",
		ListenAddr: peerAddr1,
	})
	if err != nil {
		t.Fatalf("Failed to create server1: %v", err)
	}
	defer server1.Close()

	server2, err := server.NewServerWithConfig(server.Config{
		DataDir:    tmpDir2 + "/store",
		RedisAddr:  redisAddr2,
		OpLogPath:  tmpDir2 + "/oplog/operations.log",
		ReplicaID:  "server2",
		ListenAddr: peerAddr2,
	})
	if err != nil {
		t.Fatalf("Failed to create server2: %v", err)
	}
	defer server2.Close()

	// Helper to start HTTP sync server
	startSyncServer := func(srv *server.Server, addr string) *http.Server {
		mux := http.NewServeMux()
		mux.HandleFunc("/ops", func(w http.ResponseWriter, r *http.Request) {
			sinceStr := r.URL.Query().Get("since")
			var since int64
			if sinceStr != "" {
				if v, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
					since = v
				}
			}
			ops, _ := srv.OpLog().GetOperations(since)
			batch := proto.OperationBatch{Operations: ops}
			_ = json.NewEncoder(w).Encode(batch)
		})
		mux.HandleFunc("/apply", func(w http.ResponseWriter, r *http.Request) {
			var batch proto.OperationBatch
			if err := json.NewDecoder(r.Body).Decode(&batch); err == nil {
				for _, op := range batch.Operations {
					_ = srv.HandleOperation(r.Context(), op)
				}
			}
			w.WriteHeader(http.StatusOK)
		})
		httpServer := &http.Server{Addr: addr, Handler: mux}
		go func() {
			if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
				t.Logf("HTTP server %s error: %v", addr, err)
			}
		}()
		return httpServer
	}

	// Start HTTP servers
	http1 := startSyncServer(server1, peerAddr1)
	defer http1.Close()
	http2 := startSyncServer(server2, peerAddr2)
	defer http2.Close()

	// Wait for HTTP servers to start
	time.Sleep(100 * time.Millisecond)

	// Start Syncers
	stopSync := make(chan struct{})
	defer close(stopSync)

	// Syncer 1 (pushes to/pulls from server 2)
	syncer1 := syncer.New(syncer.Config{
		SelfAddress: "http://" + peerAddr1,
		Peers:       []syncer.Peer{{Address: "http://" + peerAddr2}},
		Interval:    100 * time.Millisecond,
	}, server1)
	syncer1.Start(stopSync)

	// Syncer 2 (pushes to/pulls from server 1)
	syncer2 := syncer.New(syncer.Config{
		SelfAddress: "http://" + peerAddr2,
		Peers:       []syncer.Peer{{Address: "http://" + peerAddr1}},
		Interval:    100 * time.Millisecond,
	}, server2)
	syncer2.Start(stopSync)

	// Wait for servers to initialize
	time.Sleep(500 * time.Millisecond)

	// Set value on server1
	err = server1.Set("key1", "value1", nil)
	if err != nil {
		t.Errorf("Set on server1 failed: %v", err)
	}

	// Wait for sync (needs enough time for interval + network)
	time.Sleep(1 * time.Second)

	// Get value from server2
	value2, exists := server2.Get("key1")
	if !exists {
		t.Errorf("Value not synced to server2")
	}
	if value2 != "value1" {
		t.Errorf("Server2 has wrong value. Expected 'value1', got '%s'", value2)
	}
}
