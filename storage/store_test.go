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

func TestStoreSetGet(t *testing.T) {
	ts := setupTestStore(t)
	defer ts.cleanup()

	timestamp := time.Now().UnixNano()
	replicaID := "test-replica"

	// Test basic set and get
	err := ts.Set("key1", "value1", timestamp, replicaID, nil)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Verify value in CRDT state
	value, exists := ts.Get("key1")
	if !exists {
		t.Error("Get returned not exists for existing key")
	}
	if value != "value1" {
		t.Errorf("Get returned wrong value. Expected 'value1', got '%s'", value)
	}

	// Verify value in Redis
	redisValue, exists, err := ts.redis.Get(ts.ctx, "key1")
	if err != nil {
		t.Errorf("Redis Get failed: %v", err)
	}
	if !exists {
		t.Error("Key doesn't exist in Redis")
	}
	if redisValue != "value1" {
		t.Errorf("Redis has wrong value. Expected 'value1', got '%s'", redisValue)
	}

	// Test get non-existent key
	val, exists := ts.Get("nonexistent")
	if exists {
		t.Error("Get returned exists for non-existent key, value:", val)
	}
}

func TestStoreCRDT(t *testing.T) {
	ts := setupTestStore(t)
	defer ts.cleanup()

	replicaID := "test-replica"

	// Test that higher timestamp wins
	err := ts.Set("key1", "value1", 100, replicaID, nil)
	if err != nil {
		t.Errorf("First set failed: %v", err)
	}

	err = ts.Set("key1", "value2", 200, replicaID, nil)
	if err != nil {
		t.Errorf("Second set failed: %v", err)
	}

	// Check CRDT state
	value, exists := ts.Get("key1")
	if !exists {
		t.Error("Get returned not exists for existing key")
	}
	if value != "value2" {
		t.Errorf("Wrong value after CRDT resolution. Expected 'value2', got '%s'", value)
	}

	// Check Redis state
	redisValue, exists, err := ts.redis.Get(ts.ctx, "key1")
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
	err = ts.Set("key1", "value3", 150, replicaID, nil)
	if err != nil {
		t.Errorf("Third set failed: %v", err)
	}

	value, exists = ts.Get("key1")
	if !exists {
		t.Error("Get returned not exists for existing key")
	}
	if value != "value2" {
		t.Errorf("Wrong value after CRDT resolution. Expected 'value2', got '%s'", value)
	}

	// Verify Redis wasn't updated
	redisValue, exists, err = ts.redis.Get(ts.ctx, "key1")
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

func TestStoreTTL(t *testing.T) {
	ts := setupTestStore(t)
	defer ts.cleanup()

	timestamp := time.Now().UnixNano()
	replicaID := "test-replica"

	// Test setting with TTL
	ttl := int64(2) // 2 seconds
	err := ts.Set("key1", "value1", timestamp, replicaID, &ttl)
	if err != nil {
		t.Errorf("Set with TTL failed: %v", err)
	}

	// Verify TTL is set in CRDT state
	remaining, exists := ts.GetTTL("key1")
	if !exists {
		t.Error("GetTTL returned not exists for existing key")
	}
	if remaining <= 0 || remaining > 2 {
		t.Errorf("Wrong TTL in CRDT state. Expected between 0 and 2, got %d", remaining)
	}

	// Verify TTL is set in Redis
	redisTTL, err := ts.redis.GetTTL(ts.ctx, "key1")
	if err != nil {
		t.Errorf("Redis GetTTL failed: %v", err)
	}
	if redisTTL <= 0 || redisTTL > time.Second*2 {
		t.Errorf("Wrong TTL in Redis. Expected between 0 and 2 seconds, got %v", redisTTL)
	}

	// Test that value exists
	value, exists := ts.Get("key1")
	if !exists {
		t.Error("Get returned not exists for existing key")
	}
	if value != "value1" {
		t.Errorf("Get returned wrong value. Expected 'value1', got '%s'", value)
	}

	// Wait for expiration
	time.Sleep(3 * time.Second)

	// Verify key has expired in CRDT state
	_, exists = ts.Get("key1")
	if exists {
		t.Error("Key still exists in CRDT state after TTL expiration")
	}

	// Verify key has expired in Redis
	_, exists, err = ts.redis.Get(ts.ctx, "key1")
	if err != nil {
		t.Errorf("Redis Get failed: %v", err)
	}
	if exists {
		t.Error("Key still exists in Redis after TTL expiration")
	}

	// Test TTL conflict resolution
	shortTTL := int64(5) // 5 seconds
	longTTL := int64(10) // 10 seconds

	// Set with short TTL
	err = ts.Set("key2", "short", timestamp, replicaID, &shortTTL)
	if err != nil {
		t.Errorf("Set with short TTL failed: %v", err)
	}

	// Try to set with long TTL (should win)
	err = ts.Set("key2", "long", timestamp+1, replicaID, &longTTL)
	if err != nil {
		t.Errorf("Set with long TTL failed: %v", err)
	}

	// Verify CRDT state
	value, exists = ts.Get("key2")
	if !exists || value != "long" {
		t.Errorf("Long TTL didn't win in CRDT state. Got value: %s", value)
	}

	// Verify Redis state
	redisValue, exists, err := ts.redis.Get(ts.ctx, "key2")
	if err != nil {
		t.Errorf("Redis Get failed: %v", err)
	}
	if !exists || redisValue != "long" {
		t.Errorf("Long TTL didn't win in Redis. Got value: %s", redisValue)
	}

	// Try to set with no TTL (should win)
	err = ts.Set("key2", "no-ttl", timestamp+2, replicaID, nil)
	if err != nil {
		t.Errorf("Set with no TTL failed: %v", err)
	}

	// Verify CRDT state
	value, exists = ts.Get("key2")
	if !exists || value != "no-ttl" {
		t.Errorf("No TTL didn't win in CRDT state. Got value: %s", value)
	}

	// Verify Redis state
	redisValue, exists, err = ts.redis.Get(ts.ctx, "key2")
	if err != nil {
		t.Errorf("Redis Get failed: %v", err)
	}
	if !exists || redisValue != "no-ttl" {
		t.Errorf("No TTL didn't win in Redis. Got value: %s", redisValue)
	}

	// Verify no TTL in CRDT state
	remaining, exists = ts.GetTTL("key2")
	if !exists || remaining != -1 {
		t.Errorf("Expected TTL -1 for no TTL in CRDT state, got %d", remaining)
	}

	// Verify no TTL in Redis
	redisTTL, err = ts.redis.GetTTL(ts.ctx, "key2")
	if err != nil {
		t.Errorf("Redis GetTTL failed: %v", err)
	}
	if redisTTL != -1*time.Second {
		t.Errorf("Expected TTL -1s for no TTL in Redis, got %v", redisTTL)
	}
}

func TestStorePersistence(t *testing.T) {
	ts := setupTestStore(t)
	tmpDir := ts.GetPath()

	timestamp := time.Now().UnixNano()
	replicaID := "test-replica"

	// Set some data
	err := ts.Set("key1", "value1", timestamp, replicaID, nil)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Close store
	err = ts.Close()
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
	if value != "value1" {
		t.Errorf("Wrong value after reopening. Expected 'value1', got '%s'", value)
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
