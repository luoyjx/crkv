package storage

import (
	"context"
	"errors"
	"time"

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

// RedisStore wraps a Redis client
type RedisStore struct {
	client RedisClient
}

// NewRedisStore creates a new Redis store
func NewRedisStore(addr string, redisDB int) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   redisDB,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisStore{
		client: client,
	}, nil
}

// Set sets a value in Redis
func (rs *RedisStore) Set(ctx context.Context, key string, value string, ttl *time.Duration) error {
	if ttl == nil {
		return rs.client.Set(ctx, key, value, 0).Err() // 0 means no expiration
	}
	return rs.client.Set(ctx, key, value, *ttl).Err()
}

// Get gets a value from Redis
func (rs *RedisStore) Get(ctx context.Context, key string) (string, bool, error) {
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
