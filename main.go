package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/luoyjx/crdt-redis/consensus"
	"github.com/luoyjx/crdt-redis/redisprotocol"
	"github.com/luoyjx/crdt-redis/server"
)

func main() {
	// Parse command line flags
	dataDir := flag.String("data", "./crdt-redis-data", "directory for persistent storage")
	port := flag.Int("port", 6380, "port to listen on")
	raftPort := flag.Int("raft-port", 6381, "port for Raft consensus")
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

	// Initialize Raft consensus
	raftAddr := fmt.Sprintf(":%d", *raftPort)
	raftNode, err := consensus.NewRaftNode(*dataDir, raftAddr)
	if err != nil {
		log.Fatalf("Failed to initialize Raft: %v", err)
	}
	defer raftNode.Close()

	// Initialize Redis protocol server
	redisServer := redisprotocol.NewRedisServer(srv)
	listenAddr := fmt.Sprintf(":%d", *port)

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
	case err := <-errChan:
		log.Printf("Server error: %v", err)
	}
}
