package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/storage/crdt" // Import correct crdt package
	"github.com/redis/go-redis/v9"
)

// RedisClient interface for Redis operations
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
	Close() error
}

// RedisStore wraps a Redis client with CRDT support
type RedisStore struct {
	client RedisClient
	mu     sync.RWMutex
	values map[string]*crdt.Value
}

// NewRedisStore creates a new Redis store with CRDT support
func NewRedisStore(addr string, redisDB int, replicaID string) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   redisDB,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	store := &RedisStore{
		client: client,
		values: make(map[string]*crdt.Value), // Use crdt.Value instead of ValueWithTimestamp
	}
	return store, nil
}

// Set sets a value in Redis with LWW (Last Write Wins) strategy
func (rs *RedisStore) Set(ctx context.Context, key string, value string, ttl *time.Duration) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now().UnixNano()

	// Check if we have a newer value already
	if existing, exists := rs.values[key]; exists && existing.Timestamp > now {
		return nil // Ignore older write
	}

	// Store new value with timestamp
	rs.values[key] = &crdt.Value{
		Value:     value,
		Timestamp: now,
	}

	// Persist to Redis
	if ttl == nil {
		return rs.client.Set(ctx, key, value, 0).Err() // 0 means no expiration
	}
	return rs.client.Set(ctx, key, value, *ttl).Err()
}

// Get gets a value from Redis using LWW strategy
func (rs *RedisStore) Get(ctx context.Context, key string) (string, bool, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	// First check our local CRDT state
	if val, exists := rs.values[key]; exists {
		return val.Value, true, nil
	}

	// Fallback to Redis if not in local state
	val, err := rs.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	return val, true, nil
}

// Delete deletes a key from Redis
func (rs *RedisStore) Delete(ctx context.Context, key string) error {
	return rs.client.Del(ctx, key).Err()
}

// GetTTL gets the TTL of a key from Redis
func (rs *RedisStore) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return rs.client.TTL(ctx, key).Result()
}

// Close closes the Redis connection
func (rs *RedisStore) Close() error {
	return rs.client.Close()
}
