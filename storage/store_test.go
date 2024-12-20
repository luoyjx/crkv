package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

type testStore struct {
	*Store
	mockRedis *MockRedisClient
	cleanup   func()
}

func setupTestStore(t *testing.T) *testStore {
	tmpDir, err := os.MkdirTemp("", "store-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	mockRedis := NewMockRedisClient()
	store := &Store{
		items:           make(map[string]*Value),
		dataPath:        tmpDir + "/store.json",
		redis:           &RedisStore{client: mockRedis},
		cleanupInterval: time.Second * 1,
		stopCleanup:     make(chan struct{}),
		ctx:             context.Background(),
		cancel:          func() {},
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return &testStore{
		Store:     store,
		mockRedis: mockRedis,
		cleanup:   cleanup,
	}
}

func TestStore_SetGet(t *testing.T) {
	ts := setupTestStore(t)
	defer ts.cleanup()

	// Test setting and getting a regular string
	key := "test_key"
	value := "test_value"
	timestamp := time.Now().UnixNano()
	val := NewStringValue(value, timestamp, "test1")

	if err := ts.Store.Set(key, val, nil); err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	got, exists := ts.Store.Get(key)
	if !exists {
		t.Fatal("Value should exist")
	}
	if got.String() != value {
		t.Errorf("Got %v, want %v", got.String(), value)
	}
}

func TestStore_Counter(t *testing.T) {
	ts := setupTestStore(t)
	defer ts.cleanup()

	key := "counter_key"
	timestamp := time.Now().UnixNano()
	val := NewCounterValue(42, timestamp, "test1")

	if err := ts.Store.Set(key, val, nil); err != nil {
		t.Fatalf("Failed to set counter: %v", err)
	}

	got, exists := ts.Store.Get(key)
	if !exists {
		t.Fatal("Counter should exist")
	}
	if got.Counter() != 42 {
		t.Errorf("Got counter %v, want %v", got.Counter(), 42)
	}
	if got.String() != "42" {
		t.Errorf("Got string representation %v, want %v", got.String(), "42")
	}
}

func TestStore_TTL(t *testing.T) {
	ts := setupTestStore(t)
	defer ts.cleanup()

	key := "ttl_key"
	value := "ttl_value"
	timestamp := time.Now().UnixNano()
	val := NewStringValue(value, timestamp, "test1")
	ttl := int64(1) // 1 second TTL

	if err := ts.Store.Set(key, val, &ttl); err != nil {
		t.Fatalf("Failed to set value with TTL: %v", err)
	}

	// Value should exist immediately
	got, exists := ts.Store.Get(key)
	if !exists {
		t.Fatal("Value should exist before TTL expires")
	}
	if got.String() != value {
		t.Errorf("Got %v, want %v", got.String(), value)
	}

	// Wait for TTL to expire
	time.Sleep(2 * time.Second)

	_, exists = ts.Store.Get(key)
	if exists {
		t.Error("Value should not exist after TTL expires")
	}
}

func TestStore_Merge(t *testing.T) {
	ts := setupTestStore(t)
	defer ts.cleanup()

	key := "merge_key"
	timestamp1 := time.Now().UnixNano()
	timestamp2 := timestamp1 + 1

	// Test string merge (LWW)
	val1 := NewStringValue("value1", timestamp1, "test1")
	val2 := NewStringValue("value2", timestamp2, "test2")

	if err := ts.Store.Set(key, val1, nil); err != nil {
		t.Fatalf("Failed to set first value: %v", err)
	}
	if err := ts.Store.Set(key, val2, nil); err != nil {
		t.Fatalf("Failed to set second value: %v", err)
	}

	got, exists := ts.Store.Get(key)
	if !exists {
		t.Fatal("Value should exist")
	}
	if got.String() != "value2" {
		t.Errorf("Got %v, want %v", got.String(), "value2")
	}

	// Test counter merge (accumulative)
	counterKey := "counter_merge"
	counter1 := NewCounterValue(5, timestamp1, "test1")
	counter2 := NewCounterValue(3, timestamp2, "test2")

	if err := ts.Store.Set(counterKey, counter1, nil); err != nil {
		t.Fatalf("Failed to set first counter: %v", err)
	}
	if err := ts.Store.Set(counterKey, counter2, nil); err != nil {
		t.Fatalf("Failed to set second counter: %v", err)
	}

	gotCounter, exists := ts.Store.Get(counterKey)
	if !exists {
		t.Fatal("Counter should exist")
	}
	if gotCounter.Counter() != 8 {
		t.Errorf("Got counter %v, want %v", gotCounter.Counter(), 8)
	}
}

func TestStore_CRDT(t *testing.T) {
	ts := setupTestStore(t)
	defer ts.cleanup()

	replicaID := "test-replica"

	// Test that higher timestamp wins
	err := ts.Store.Set("key1", NewStringValue("value1", 100, replicaID), nil)
	if err != nil {
		t.Errorf("First set failed: %v", err)
	}

	err = ts.Store.Set("key1", NewStringValue("value2", 200, replicaID), nil)
	if err != nil {
		t.Errorf("Second set failed: %v", err)
	}

	// Check CRDT state
	value, exists := ts.Store.Get("key1")
	if !exists {
		t.Error("Get returned not exists for existing key")
	}
	if value.String() != "value2" {
		t.Errorf("Wrong value after CRDT resolution. Expected 'value2', got '%s'", value.String())
	}

	// Check Redis state
	redisValue, exists, err := ts.Store.redis.Get(ts.Store.ctx, "key1")
	if err != nil {
		t.Errorf("Redis Get failed: %v", err)
	}
	if !exists {
		t.Error("Key doesn't exist in Redis")
	}
	if redisValue != "value2" {
		t.Errorf("Redis has wrong value. Expected 'value2', got '%s'", redisValue)
	}

	// Test that lower timestamp doesn't override higher
	err = ts.Store.Set("key1", NewStringValue("value3", 150, replicaID), nil)
	if err != nil {
		t.Errorf("Third set failed: %v", err)
	}

	value, exists = ts.Store.Get("key1")
	if !exists {
		t.Error("Get returned not exists for existing key")
	}
	if value.String() != "value2" {
		t.Errorf("Wrong value after CRDT resolution. Expected 'value2', got '%s'", value.String())
	}

	// Verify Redis wasn't updated
	redisValue, exists, err = ts.Store.redis.Get(ts.Store.ctx, "key1")
	if err != nil {
		t.Errorf("Redis Get failed: %v", err)
	}
	if !exists {
		t.Error("Key doesn't exist in Redis")
	}
	if redisValue != "value2" {
		t.Errorf("Redis has wrong value. Expected 'value2', got '%s'", redisValue)
	}
}

func TestStore_Persistence(t *testing.T) {
	ts := setupTestStore(t)
	tmpDir := ts.Store.GetPath()

	timestamp := time.Now().UnixNano()
	replicaID := "test-replica"

	// Set some data
	err := ts.Store.Set("key1", NewStringValue("value1", timestamp, replicaID), nil)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Close store
	err = ts.Store.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Create new store with same directory
	mockRedis := NewMockRedisClient()
	store2 := &Store{
		items:           make(map[string]*Value),
		dataPath:        tmpDir + "/store.json",
		redis:           &RedisStore{client: mockRedis},
		cleanupInterval: time.Second * 1,
		stopCleanup:     make(chan struct{}),
		ctx:             context.Background(),
		cancel:          func() {},
	}
	defer func() {
		store2.Close()
		os.RemoveAll(tmpDir)
	}()

	// Load existing data
	if err := store2.load(); err != nil {
		t.Fatalf("Failed to load data: %v", err)
	}

	// Verify CRDT state persisted
	value, exists := store2.Get("key1")
	if !exists {
		t.Error("Key doesn't exist after reopening store")
	}
	if value.String() != "value1" {
		t.Errorf("Wrong value after reopening. Expected 'value1', got '%s'", value.String())
	}

	// Verify Redis was updated
	redisValue, exists, err := store2.redis.Get(store2.ctx, "key1")
	if err != nil {
		t.Errorf("Redis Get failed: %v", err)
	}
	if !exists {
		t.Error("Key doesn't exist in Redis after reopening")
	}
	if redisValue != "value1" {
		t.Errorf("Wrong value in Redis after reopening. Expected 'value1', got '%s'", redisValue)
	}
}
