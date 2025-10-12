package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/luoyjx/crdt-redis/server"
	"github.com/luoyjx/crdt-redis/storage"
)

// BenchmarkStringOperations tests string SET/GET performance
func BenchmarkStringOperations(b *testing.B) {
	srv := setupBenchmarkServer(b)
	defer srv.Close()

	b.Run("SET", func(b *testing.B) {
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

	b.Run("GET", func(b *testing.B) {
		// Pre-populate with data
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("bench-key-%d", i)
			value := fmt.Sprintf("bench-value-%d", i)
			srv.Set(key, value, nil)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench-key-%d", i%1000)
			_, _ = srv.Get(key)
		}
	})

	b.Run("SET_GET_Mixed", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("mixed-key-%d", i%100)
			if i%2 == 0 {
				value := fmt.Sprintf("mixed-value-%d", i)
				srv.Set(key, value, nil)
			} else {
				srv.Get(key)
			}
		}
	})
}

// BenchmarkCounterOperations tests counter INCR/DECR performance
func BenchmarkCounterOperations(b *testing.B) {
	srv := setupBenchmarkServer(b)
	defer srv.Close()

	b.Run("INCR", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("counter-%d", i%10)
			_, err := srv.Incr(key)
			if err != nil {
				b.Fatalf("INCR failed: %v", err)
			}
		}
	})

	b.Run("INCRBY", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("counter-by-%d", i%10)
			_, err := srv.IncrBy(key, int64(i%100))
			if err != nil {
				b.Fatalf("INCRBY failed: %v", err)
			}
		}
	})
}

// BenchmarkListOperations tests list operations performance
func BenchmarkListOperations(b *testing.B) {
	srv := setupBenchmarkServer(b)
	defer srv.Close()

	b.Run("LPUSH", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("list-%d", i%10)
			value := fmt.Sprintf("item-%d", i)
			_, err := srv.LPush(key, value)
			if err != nil {
				b.Fatalf("LPUSH failed: %v", err)
			}
		}
	})

	b.Run("RPUSH", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("rlist-%d", i%10)
			value := fmt.Sprintf("item-%d", i)
			_, err := srv.RPush(key, value)
			if err != nil {
				b.Fatalf("RPUSH failed: %v", err)
			}
		}
	})

	b.Run("LPOP", func(b *testing.B) {
		// Pre-populate lists
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("pop-list-%d", i)
			for j := 0; j < 1000; j++ {
				srv.LPush(key, fmt.Sprintf("item-%d", j))
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("pop-list-%d", i%10)
			_, _, err := srv.LPop(key)
			if err != nil {
				b.Fatalf("LPOP failed: %v", err)
			}
		}
	})

	b.Run("LRANGE", func(b *testing.B) {
		// Pre-populate a list
		key := "range-list"
		for i := 0; i < 1000; i++ {
			srv.LPush(key, fmt.Sprintf("item-%d", i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := srv.LRange(key, 0, 10)
			if err != nil {
				b.Fatalf("LRANGE failed: %v", err)
			}
		}
	})
}

// BenchmarkSetOperations tests set operations performance
func BenchmarkSetOperations(b *testing.B) {
	srv := setupBenchmarkServer(b)
	defer srv.Close()

	b.Run("SADD", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("set-%d", i%10)
			member := fmt.Sprintf("member-%d", i)
			_, err := srv.SAdd(key, member)
			if err != nil {
				b.Fatalf("SADD failed: %v", err)
			}
		}
	})

	b.Run("SREM", func(b *testing.B) {
		// Pre-populate sets
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("rem-set-%d", i)
			for j := 0; j < 1000; j++ {
				srv.SAdd(key, fmt.Sprintf("member-%d", j))
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("rem-set-%d", i%10)
			member := fmt.Sprintf("member-%d", i%1000)
			_, err := srv.SRem(key, member)
			if err != nil {
				b.Fatalf("SREM failed: %v", err)
			}
		}
	})

	b.Run("SMEMBERS", func(b *testing.B) {
		// Pre-populate a set
		key := "members-set"
		for i := 0; i < 100; i++ {
			srv.SAdd(key, fmt.Sprintf("member-%d", i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := srv.SMembers(key)
			if err != nil {
				b.Fatalf("SMEMBERS failed: %v", err)
			}
		}
	})

	b.Run("SISMEMBER", func(b *testing.B) {
		// Pre-populate a set
		key := "ismember-set"
		for i := 0; i < 1000; i++ {
			srv.SAdd(key, fmt.Sprintf("member-%d", i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			member := fmt.Sprintf("member-%d", i%1000)
			_, err := srv.SIsMember(key, member)
			if err != nil {
				b.Fatalf("SISMEMBER failed: %v", err)
			}
		}
	})
}

// BenchmarkHashOperations tests hash operations performance
func BenchmarkHashOperations(b *testing.B) {
	srv := setupBenchmarkServer(b)
	defer srv.Close()

	b.Run("HSET", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("hash-%d", i%10)
			field := fmt.Sprintf("field-%d", i)
			value := fmt.Sprintf("value-%d", i)
			_, err := srv.HSet(key, field, value)
			if err != nil {
				b.Fatalf("HSET failed: %v", err)
			}
		}
	})

	b.Run("HGET", func(b *testing.B) {
		// Pre-populate hashes
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("get-hash-%d", i)
			for j := 0; j < 100; j++ {
				field := fmt.Sprintf("field-%d", j)
				value := fmt.Sprintf("value-%d", j)
				srv.HSet(key, field, value)
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("get-hash-%d", i%10)
			field := fmt.Sprintf("field-%d", i%100)
			_, _, err := srv.HGet(key, field)
			if err != nil {
				b.Fatalf("HGET failed: %v", err)
			}
		}
	})

	b.Run("HGETALL", func(b *testing.B) {
		// Pre-populate a hash
		key := "getall-hash"
		for i := 0; i < 100; i++ {
			field := fmt.Sprintf("field-%d", i)
			value := fmt.Sprintf("value-%d", i)
			srv.HSet(key, field, value)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := srv.HGetAll(key)
			if err != nil {
				b.Fatalf("HGETALL failed: %v", err)
			}
		}
	})
}

// BenchmarkVectorClockOperations tests vector clock performance
func BenchmarkVectorClockOperations(b *testing.B) {
	b.Run("VectorClock_Increment", func(b *testing.B) {
		vc := storage.NewVectorClock()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			replicaID := fmt.Sprintf("replica-%d", i%10)
			vc.Increment(replicaID)
		}
	})

	b.Run("VectorClock_Compare", func(b *testing.B) {
		vc1 := storage.NewVectorClock()
		vc2 := storage.NewVectorClock()

		// Setup different clocks
		for i := 0; i < 10; i++ {
			vc1.Increment(fmt.Sprintf("replica-%d", i))
			vc2.Increment(fmt.Sprintf("replica-%d", i))
		}
		vc1.Increment("replica-1")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vc1.Compare(vc2)
		}
	})

	b.Run("VectorClock_Merge", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vc1 := storage.NewVectorClock()
			vc2 := storage.NewVectorClock()

			// Setup clocks
			for j := 0; j < 5; j++ {
				vc1.Increment(fmt.Sprintf("replica-%d", j))
				vc2.Increment(fmt.Sprintf("replica-%d", j+2))
			}

			vc1.Update(vc2)
		}
	})
}

// BenchmarkConcurrentOperations tests concurrent access performance
func BenchmarkConcurrentOperations(b *testing.B) {
	srv := setupBenchmarkServer(b)
	defer srv.Close()

	b.Run("Concurrent_SET", func(b *testing.B) {
		numWorkers := runtime.NumCPU()
		var wg sync.WaitGroup

		b.ResetTimer()
		for worker := 0; worker < numWorkers; worker++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for i := 0; i < b.N/numWorkers; i++ {
					key := fmt.Sprintf("concurrent-key-%d-%d", workerID, i)
					value := fmt.Sprintf("value-%d", i)
					srv.Set(key, value, nil)
				}
			}(worker)
		}
		wg.Wait()
	})

	b.Run("Concurrent_Mixed", func(b *testing.B) {
		// Pre-populate some data
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("mixed-key-%d", i)
			value := fmt.Sprintf("value-%d", i)
			srv.Set(key, value, nil)
		}

		numWorkers := runtime.NumCPU()
		var wg sync.WaitGroup

		b.ResetTimer()
		for worker := 0; worker < numWorkers; worker++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for i := 0; i < b.N/numWorkers; i++ {
					key := fmt.Sprintf("mixed-key-%d", i%1000)

					switch i % 4 {
					case 0:
						srv.Set(key, fmt.Sprintf("new-value-%d", i), nil)
					case 1:
						srv.Get(key)
					case 2:
						srv.Incr(fmt.Sprintf("counter-%d", i%100))
					case 3:
						srv.SAdd(fmt.Sprintf("set-%d", i%50), fmt.Sprintf("member-%d", i))
					}
				}
			}(worker)
		}
		wg.Wait()
	})
}

// BenchmarkMemoryUsage tests memory efficiency
func BenchmarkMemoryUsage(b *testing.B) {
	srv := setupBenchmarkServer(b)
	defer srv.Close()

	b.Run("Memory_Strings", func(b *testing.B) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("mem-key-%d", i)
			value := fmt.Sprintf("mem-value-%d", i)
			srv.Set(key, value, nil)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		bytesPerOp := (m2.Alloc - m1.Alloc) / uint64(b.N)
		b.ReportMetric(float64(bytesPerOp), "bytes/op")
	})
}

// BenchmarkReplication tests replication performance
func BenchmarkReplication(b *testing.B) {
	srv1 := setupBenchmarkServer(b)
	defer srv1.Close()

	srv2 := setupBenchmarkServer(b)
	defer srv2.Close()

	b.Run("Operation_Logging", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("repl-key-%d", i)
			value := fmt.Sprintf("repl-value-%d", i)
			srv1.Set(key, value, nil)
		}

		// Measure operation log size
		ops, _ := srv1.OpLog().GetOperations(0)
		b.ReportMetric(float64(len(ops)), "ops-logged")
	})

	b.Run("Cross_Server_Sync", func(b *testing.B) {
		// Pre-populate server1 with operations
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("sync-key-%d", i)
			value := fmt.Sprintf("sync-value-%d", i)
			srv1.Set(key, value, nil)
		}

		ops, _ := srv1.OpLog().GetOperations(0)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate applying operations from peer
			for _, op := range ops[:10] { // Apply 10 ops per iteration
				srv2.HandleOperation(nil, op)
			}
		}
	})
}

// setupBenchmarkServer creates a server for benchmarking
func setupBenchmarkServer(b *testing.B) *server.Server {
	tmpDir, err := os.MkdirTemp("", "benchmark-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}

	srv, err := server.NewServer(
		tmpDir,
		"localhost:6379",
		tmpDir+"/oplog.json",
	)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	return srv
}
