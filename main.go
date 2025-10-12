package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
	"github.com/luoyjx/crdt-redis/redisprotocol"
	"github.com/luoyjx/crdt-redis/server"
	"github.com/luoyjx/crdt-redis/syncer"
)

func main() {
	// Parse command line flags
	dataDir := flag.String("data", "./crdt-redis-data", "directory for persistent storage")
	port := flag.Int("port", 6380, "port to listen on")
	raftPort := flag.Int("raft-port", 6381, "port for Raft consensus")
	httpSyncPort := flag.Int("sync-port", 8083, "http sync port")
	peerAddrs := flag.String("peers", "", "comma-separated http peer addresses, e.g. http://127.0.0.1:8084")
	redisAddr := flag.String("redis", "localhost:6379", "address of local Redis server")
	flag.Parse()

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize CRDT Redis Server
	srv, err := server.NewServer(*dataDir+"/store", *redisAddr, *dataDir+"/oplog")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Initialize Redis protocol server
	redisServer := redisprotocol.NewRedisServer(srv)
	listenAddr := fmt.Sprintf(":%d", *port)

	// Start HTTP sync endpoints and background syncer (MVP)
	stopSync := make(chan struct{})
	go func() {
		// very simple HTTP mux for ops
		httpAddr := fmt.Sprintf(":%d", *httpSyncPort)
		http.HandleFunc("/ops", func(w http.ResponseWriter, r *http.Request) {
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
		http.HandleFunc("/apply", func(w http.ResponseWriter, r *http.Request) {
			var batch proto.OperationBatch
			if err := json.NewDecoder(r.Body).Decode(&batch); err == nil {
				for _, op := range batch.Operations {
					_ = srv.HandleOperation(r.Context(), op)
				}
			}
			w.WriteHeader(http.StatusOK)
		})
		log.Printf("Starting HTTP sync endpoint on %s", httpAddr)
		_ = http.ListenAndServe(httpAddr, nil)
	}()

	// Background syncer pushing/pulling to peers
	var peers []syncer.Peer
	if *peerAddrs != "" {
		for _, addr := range strings.Split(*peerAddrs, ",") {
			peers = append(peers, syncer.Peer{Address: strings.TrimSpace(addr)})
		}
	}
	syncComponent := syncer.New(syncer.Config{SelfAddress: fmt.Sprintf("http://127.0.0.1:%d", *httpSyncPort), Peers: peers, Interval: time.Second}, srv)
	syncComponent.Start(stopSync)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start Redis server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Printf("Starting Redis server on port %d", *port)
		log.Printf("Starting Raft server on port %d", *raftPort)
		if err := redisServer.Start(listenAddr); err != nil {
			errChan <- fmt.Errorf("Redis server error: %v", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Println("Shutting down gracefully...")
		close(stopSync) // Stop syncer
	case err := <-errChan:
		log.Printf("Server error: %v", err)
		close(stopSync) // Stop syncer
	}

	// Give a moment for syncer to stop
	time.Sleep(100 * time.Millisecond)
}
