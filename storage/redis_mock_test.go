package storage

import (
	"context"
	"fmt"
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

func NewMockRedisClient(addr string, redisDB int) *MockRedisClient {
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

// Append implements the RedisClient.Append method
func (m *MockRedisClient) Append(ctx context.Context, key, value string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewIntCmd(ctx, "append")
	}

	cmd := redis.NewIntCmd(ctx, "append")
	if val, ok := m.data[key]; ok {
		newVal := val + value
		m.data[key] = newVal
		cmd.SetVal(int64(len(newVal)))
	} else {
		m.data[key] = value
		cmd.SetVal(int64(len(value)))
	}
	return cmd
}

// GetRange implements the RedisClient.GetRange method
func (m *MockRedisClient) GetRange(ctx context.Context, key string, start, end int64) *redis.StringCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return redis.NewStringCmd(ctx, "getrange")
	}

	cmd := redis.NewStringCmd(ctx, "getrange")
	if val, ok := m.data[key]; ok {
		runes := []rune(val)
		length := int64(len(runes))

		if start < 0 {
			start = length + start
			if start < 0 {
				start = 0
			}
		}
		if end < 0 {
			end = length + end
			if end < 0 {
				end = 0
			}
		}
		if end >= length {
			end = length - 1
		}

		if start > end || start >= length {
			cmd.SetVal("")
			return cmd
		}

		result := string(runes[start : end+1])
		cmd.SetVal(result)
		return cmd
	}

	cmd.SetVal("")
	return cmd
}

// SetRange implements the RedisClient.SetRange method
func (m *MockRedisClient) SetRange(ctx context.Context, key string, offset int64, value string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewIntCmd(ctx, "setrange")
	}

	cmd := redis.NewIntCmd(ctx, "setrange")
	if offset < 0 {
		cmd.SetErr(redis.Nil)
		return cmd
	}

	var currentValue string
	if val, ok := m.data[key]; ok {
		currentValue = val
	} else {
		currentValue = ""
	}

	currentRunes := []rune(currentValue)
	valueRunes := []rune(value)

	if int64(len(currentRunes)) < offset {
		// Pad with zero bytes
		padding := make([]rune, offset-int64(len(currentRunes)))
		for i := range padding {
			padding[i] = '\x00'
		}
		currentRunes = append(currentRunes, padding...)
	}

	// Insert value at offset
	resultRunes := make([]rune, max(int64(len(currentRunes)), offset+int64(len(valueRunes))))
	copy(resultRunes, currentRunes)
	copy(resultRunes[offset:], valueRunes)

	m.data[key] = string(resultRunes)
	cmd.SetVal(int64(len(resultRunes)))
	return cmd
}

// StrLen implements the RedisClient.StrLen method
func (m *MockRedisClient) StrLen(ctx context.Context, key string) *redis.IntCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return redis.NewIntCmd(ctx, "strlen")
	}

	cmd := redis.NewIntCmd(ctx, "strlen")
	if val, ok := m.data[key]; ok {
		cmd.SetVal(int64(len(val)))
	} else {
		cmd.SetVal(0)
	}
	return cmd
}

// GetSet implements the RedisClient.GetSet method
func (m *MockRedisClient) GetSet(ctx context.Context, key string, value interface{}) *redis.StringCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewStringCmd(ctx, "getset")
	}

	cmd := redis.NewStringCmd(ctx, "getset")
	if val, ok := m.data[key]; ok {
		cmd.SetVal(val)
	} else {
		cmd.SetErr(redis.Nil)
	}

	m.data[key] = value.(string)
	return cmd
}

// GetDel implements the RedisClient.GetDel method
func (m *MockRedisClient) GetDel(ctx context.Context, key string) *redis.StringCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewStringCmd(ctx, "getdel")
	}

	cmd := redis.NewStringCmd(ctx, "getdel")
	if val, ok := m.data[key]; ok {
		cmd.SetVal(val)
		delete(m.data, key)
		delete(m.ttls, key)
	} else {
		cmd.SetErr(redis.Nil)
	}

	return cmd
}

// GetEx implements the RedisClient.GetEx method
func (m *MockRedisClient) GetEx(ctx context.Context, key string, expiration time.Duration) *redis.StringCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewStringCmd(ctx, "getex")
	}

	cmd := redis.NewStringCmd(ctx, "getex")
	if val, ok := m.data[key]; ok {
		cmd.SetVal(val)
		if expiration > 0 {
			m.ttls[key] = expiration
		} else {
			delete(m.ttls, key)
		}
	} else {
		cmd.SetErr(redis.Nil)
	}

	return cmd
}

// MGet implements the RedisClient.MGet method
func (m *MockRedisClient) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return redis.NewSliceCmd(ctx, "mget")
	}

	cmd := redis.NewSliceCmd(ctx, "mget")
	result := make([]interface{}, len(keys))
	for i, key := range keys {
		if val, ok := m.data[key]; ok {
			result[i] = val
		} else {
			result[i] = nil
		}
	}
	cmd.SetVal(result)
	return cmd
}

// MSet implements the RedisClient.MSet method
func (m *MockRedisClient) MSet(ctx context.Context, values ...interface{}) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed || len(values)%2 != 0 {
		return redis.NewStatusCmd(ctx, "mset")
	}

	for i := 0; i < len(values); i += 2 {
		key, value := values[i].(string), values[i+1].(string)
		m.data[key] = value
	}

	cmd := redis.NewStatusCmd(ctx, "mset")
	cmd.SetVal("OK")
	return cmd
}

// MSetNX implements the RedisClient.MSetNX method
func (m *MockRedisClient) MSetNX(ctx context.Context, values ...interface{}) *redis.BoolCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed || len(values)%2 != 0 {
		return redis.NewBoolCmd(ctx, "msetnx")
	}

	cmd := redis.NewBoolCmd(ctx, "msetnx")

	// Check if any key exists
	for i := 0; i < len(values); i += 2 {
		key := values[i].(string)
		if _, ok := m.data[key]; ok {
			cmd.SetVal(false)
			return cmd
		}
	}

	// Set all keys
	for i := 0; i < len(values); i += 2 {
		key, value := values[i].(string), values[i+1].(string)
		m.data[key] = value
	}

	cmd.SetVal(true)
	return cmd
}

// SetEX implements the RedisClient.SetEX method
func (m *MockRedisClient) SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewStatusCmd(ctx, "setex")
	}

	m.data[key] = value.(string)
	m.ttls[key] = expiration

	cmd := redis.NewStatusCmd(ctx, "setex")
	cmd.SetVal("OK")
	return cmd
}

// PSetEX implements the RedisClient.PSetEX method
func (m *MockRedisClient) PSetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewStatusCmd(ctx, "psetex")
	}

	m.data[key] = value.(string)
	m.ttls[key] = expiration // We store as duration directly

	cmd := redis.NewStatusCmd(ctx, "psetex")
	cmd.SetVal("OK")
	return cmd
}

// SetNX implements the RedisClient.SetNX method
func (m *MockRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewBoolCmd(ctx, "setnx")
	}

	cmd := redis.NewBoolCmd(ctx, "setnx")
	if _, ok := m.data[key]; ok {
		cmd.SetVal(false)
		return cmd
	}

	m.data[key] = value.(string)
	if expiration > 0 {
		m.ttls[key] = expiration
	}
	cmd.SetVal(true)
	return cmd
}

// Incr implements the RedisClient.Incr method
func (m *MockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewIntCmd(ctx, "incr")
	}

	cmd := redis.NewIntCmd(ctx, "incr")
	var val int64 = 0
	if existingVal, ok := m.data[key]; ok {
		var err error
		val, err = parseInt(existingVal)
		if err != nil {
			cmd.SetErr(err)
			return cmd
		}
	}

	val++
	m.data[key] = intToString(val)
	cmd.SetVal(val)
	return cmd
}

// Decr implements the RedisClient.Decr method
func (m *MockRedisClient) Decr(ctx context.Context, key string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewIntCmd(ctx, "decr")
	}

	cmd := redis.NewIntCmd(ctx, "decr")
	var val int64 = 0
	if existingVal, ok := m.data[key]; ok {
		var err error
		val, err = parseInt(existingVal)
		if err != nil {
			cmd.SetErr(err)
			return cmd
		}
	}

	val--
	m.data[key] = intToString(val)
	cmd.SetVal(val)
	return cmd
}

// IncrBy implements the RedisClient.IncrBy method
func (m *MockRedisClient) IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewIntCmd(ctx, "incrby")
	}

	cmd := redis.NewIntCmd(ctx, "incrby")
	var val int64 = 0
	if existingVal, ok := m.data[key]; ok {
		var err error
		val, err = parseInt(existingVal)
		if err != nil {
			cmd.SetErr(err)
			return cmd
		}
	}

	val += value
	m.data[key] = intToString(val)
	cmd.SetVal(val)
	return cmd
}

// DecrBy implements the RedisClient.DecrBy method
func (m *MockRedisClient) DecrBy(ctx context.Context, key string, value int64) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewIntCmd(ctx, "decrby")
	}

	cmd := redis.NewIntCmd(ctx, "decrby")
	var val int64 = 0
	if existingVal, ok := m.data[key]; ok {
		var err error
		val, err = parseInt(existingVal)
		if err != nil {
			cmd.SetErr(err)
			return cmd
		}
	}

	val -= value
	m.data[key] = intToString(val)
	cmd.SetVal(val)
	return cmd
}

// IncrByFloat implements the RedisClient.IncrByFloat method
func (m *MockRedisClient) IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return redis.NewFloatCmd(ctx, "incrbyfloat")
	}

	cmd := redis.NewFloatCmd(ctx, "incrbyfloat")
	var val float64 = 0
	if existingVal, ok := m.data[key]; ok {
		var err error
		val, err = parseFloat(existingVal)
		if err != nil {
			cmd.SetErr(err)
			return cmd
		}
	}

	val += value
	m.data[key] = floatToString(val)
	cmd.SetVal(val)
	return cmd
}

// Helper functions for numeric operations
func parseInt(s string) (int64, error) {
	var i int64
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func intToString(i int64) string {
	return fmt.Sprintf("%d", i)
}

func floatToString(f float64) string {
	return fmt.Sprintf("%f", f)
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
