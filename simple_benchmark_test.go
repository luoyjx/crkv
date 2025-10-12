package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/luoyjx/crdt-redis/server"
)

// Simple benchmark for basic operations
func BenchmarkBasicOperations(b *testing.B) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "simple-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create server
	srv, err := server.NewServer(
		tmpDir,
		"localhost:6379",
		tmpDir+"/oplog.json",
	)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	b.Run("SET_Operations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)
			err := srv.Set(key, value, nil)
			if err != nil {
				b.Fatalf("SET failed: %v", err)
			}
		}
	})

	b.Run("GET_Operations", func(b *testing.B) {
		// Pre-populate with data
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("get-key-%d", i)
			value := fmt.Sprintf("get-value-%d", i)
			srv.Set(key, value, nil)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("get-key-%d", i%100)
			srv.Get(key)
		}
	})

	b.Run("INCR_Operations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("counter-%d", i%10)
			srv.Incr(key)
		}
	})
}
