package storage

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// MockRedisClient implements RedisClient interface for testing
type MockRedisClient struct {
	mu     sync.RWMutex
	data   map[string]string
	ttls   map[string]time.Duration
	closed bool
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]string),
		ttls: make(map[string]time.Duration),
	}
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewStatusCmd(ctx, "set")
	}

	m.data[key] = value.(string)
	if expiration > 0 {
		m.ttls[key] = expiration
	} else {
		delete(m.ttls, key)
	}
	return redis.NewStatusCmd(ctx, "set")
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return redis.NewStringCmd(ctx, "get")
	}

	if ttl, ok := m.ttls[key]; ok {
		if ttl > 0 && time.Now().After(time.Now().Add(ttl)) {
			delete(m.data, key)
			delete(m.ttls, key)
			return redis.NewStringCmd(ctx, "get")
		}
	}

	if val, ok := m.data[key]; ok {
		cmd := redis.NewStringCmd(ctx, "get")
		cmd.SetVal(val)
		return cmd
	}

	cmd := redis.NewStringCmd(ctx, "get")
	cmd.SetErr(redis.Nil)
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewIntCmd(ctx, "del")
	}

	count := int64(0)
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			delete(m.data, key)
			delete(m.ttls, key)
			count++
		}
	}
	cmd := redis.NewIntCmd(ctx, "del")
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) TTL(ctx context.Context, key string) *redis.DurationCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		cmd := redis.NewDurationCmd(ctx, 0)
		cmd.SetVal(time.Duration(-2) * time.Second) // -2 for closed connection
		return cmd
	}

	if ttl, ok := m.ttls[key]; ok {
		cmd := redis.NewDurationCmd(ctx, 0)
		cmd.SetVal(ttl)
		return cmd
	}
	if _, ok := m.data[key]; ok {
		cmd := redis.NewDurationCmd(ctx, 0)
		cmd.SetVal(time.Duration(-1) * time.Second) // -1 for key exists but has no TTL
		return cmd
	}
	cmd := redis.NewDurationCmd(ctx, 0)
	cmd.SetVal(time.Duration(-2) * time.Second) // -2 for key does not exist
	return cmd
}

func (m *MockRedisClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}
