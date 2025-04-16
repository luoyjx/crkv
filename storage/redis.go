package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	// Import correct crdt package
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
	values map[string]*Value
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
		values: make(map[string]*Value),
	}
	return store, nil
}

// Set sets a value in Redis with LWW semantics only
func (rs *RedisStore) Set(ctx context.Context, key string, value *Value, ttl *time.Duration) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	existing, exists := rs.values[key]
	if exists {
		if value.Timestamp <= existing.Timestamp {
			return nil // Ignore older write
		}
	}

	rs.values[key] = value
	if ttl == nil {
		return rs.client.Set(ctx, key, value.String(), 0).Err()
	}
	return rs.client.Set(ctx, key, value.String(), *ttl).Err()
}

// Get gets a value from Redis using LWW or counter semantics
func (rs *RedisStore) Get(ctx context.Context, key string) (*Value, bool, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if val, exists := rs.values[key]; exists {
		return val, true, nil
	}

	valStr, err := rs.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	// Default to string type if not found in local state
	return NewStringValue(valStr, time.Now().UnixNano(), ""), true, nil
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
