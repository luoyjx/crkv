package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	// Import crdt package
)

// Store manages the persistent storage of values with CRDT resolution
type Store struct {
	mu              sync.RWMutex
	items           map[string]*Value // In-memory CRDT state
	dataPath        string            // Path to persist CRDT state (legacy)
	redis           *RedisStore       // Local Redis instance
	segmentManager  *SegmentManager   // Optimized persistence with append-only logs
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	closed          bool // Flag to prevent multiple closes
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewStore creates a new store instance with persistence and Redis connection
func NewStore(dataDir string, redisAddr string, redisDB int) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	redis, err := NewRedisStore(redisAddr, redisDB, "") // Add empty replicaID
	if err != nil {
		return nil, err
	}

	// Initialize segment manager for optimized persistence
	segmentManager, err := NewSegmentManager(filepath.Join(dataDir, "segments"))
	if err != nil {
		return nil, fmt.Errorf("failed to create segment manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	store := &Store{
		items:           make(map[string]*Value),
		dataPath:        filepath.Join(dataDir, "store.json"),
		redis:           redis,
		segmentManager:  segmentManager,
		cleanupInterval: time.Second * 1,
		stopCleanup:     make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Load existing CRDT state from segments
	if err := store.loadFromSegments(); err != nil {
		return nil, err
	}

	// Start cleanup goroutine
	go store.cleanupLoop()

	return store, nil
}

// Set stores a value with CRDT metadata and optional TTL using LWW semantics only
func (s *Store) Set(key string, value *Value, ttl *int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Calculate expiration time if TTL is provided
	var expireAt time.Time
	var redisTTL *time.Duration
	if ttl != nil {
		duration := time.Duration(*ttl) * time.Second
		redisTTL = &duration
		expireAt = time.Now().Add(duration)
	}

	existingValue, exists := s.items[key]
	if exists {
		if value.Timestamp <= existingValue.Timestamp {
			return nil // Do not update if new timestamp is not greater
		}
	}

	// Update Redis first
	if err := s.redis.Set(s.ctx, key, value, redisTTL); err != nil {
		return fmt.Errorf("failed to write to Redis: %v", err)
	}

	// Then update CRDT state
	s.items[key] = value
	s.items[key].TTL = ttl
	s.items[key].ExpireAt = expireAt

	// Save to disk
	if err := s.save(); err != nil {
		return fmt.Errorf("failed to save to disk: %v", err)
	}

	return nil
}

// Get retrieves a value, checking both CRDT state and Redis
func (s *Store) Get(key string) (*Value, bool) {
	s.mu.RLock()
	value, exists := s.items[key]
	s.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Check if the value has expired
	if value.TTL != nil && time.Now().After(value.ExpireAt) {
		// Remove expired key
		s.mu.Lock()
		delete(s.items, key)
		s.save() // Save the expired state
		s.mu.Unlock()

		// Also remove from Redis
		s.redis.Delete(s.ctx, key)
		return nil, false
	}

	return value, true
}

// Delete removes a value from both CRDT state and Redis
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, key)
	if err := s.save(); err != nil {
		return err
	}

	return s.redis.Delete(s.ctx, key)
}

// UpdateTTLDuration sets TTL for a key using a duration. If duration <= 0, the key is deleted.
func (s *Store) UpdateTTLDuration(key string, d time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if d <= 0 {
		delete(s.items, key)
		if err := s.save(); err != nil {
			return false, err
		}
		if err := s.redis.Delete(s.ctx, key); err != nil {
			return false, err
		}
		return true, nil
	}
	expireAt := time.Now().Add(d)
	v.SetExpireAt(&expireAt)
	if err := s.save(); err != nil {
		return false, err
	}
	if err := s.redis.SetTTL(s.ctx, key, &d); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateExpireAt sets absolute expiration time for a key.
func (s *Store) UpdateExpireAt(key string, at time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if time.Now().After(at) || time.Now().Equal(at) {
		delete(s.items, key)
		if err := s.save(); err != nil {
			return false, err
		}
		if err := s.redis.Delete(s.ctx, key); err != nil {
			return false, err
		}
		return true, nil
	}
	v.SetExpireAt(&at)
	if err := s.save(); err != nil {
		return false, err
	}
	d := time.Until(at)
	if err := s.redis.SetTTL(s.ctx, key, &d); err != nil {
		return false, err
	}
	return true, nil
}

// Close closes the store and its resources
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Prevent multiple closes
	if s.closed {
		return nil
	}
	s.closed = true

	// Save current state to disk
	if err := s.save(); err != nil {
		return fmt.Errorf("failed to save state on close: %v", err)
	}

	// Stop cleanup goroutine
	close(s.stopCleanup)

	// Cancel context
	s.cancel()

	// Close segment manager
	if s.segmentManager != nil {
		if err := s.segmentManager.Close(); err != nil {
			return fmt.Errorf("failed to close segment manager: %v", err)
		}
	}

	return nil
}

// cleanupLoop periodically removes expired keys
func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupExpired()
		case <-s.stopCleanup:
			return
		}
	}
}

// cleanupExpired removes all expired keys
func (s *Store) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	changed := false

	for key, value := range s.items {
		if value.TTL != nil && now.After(value.ExpireAt) {
			delete(s.items, key)
			changed = true
			// Remove from Redis synchronously to ensure it's gone
			s.redis.Delete(s.ctx, key)
		}
	}

	// Save changes to disk if any keys were removed
	if changed {
		if err := s.save(); err != nil {
			log.Printf("Error saving after cleanup: %v", err)
		}
	}
}

// load reads the store data from disk and syncs with Redis
func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := ioutil.ReadFile(s.dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No data file yet
		}
		return fmt.Errorf("failed to read data file: %v", err)
	}

	if err := json.Unmarshal(data, &s.items); err != nil {
		return fmt.Errorf("failed to unmarshal data: %v", err)
	}

	// Sync with Redis and remove expired items
	now := time.Now()
	for key, value := range s.items {
		// Check if the value has expired
		if value.TTL != nil {
			if now.After(value.ExpireAt) {
				delete(s.items, key)
				continue
			}
			// Update TTL for Redis
			remaining := time.Until(value.ExpireAt)
			if remaining <= 0 {
				delete(s.items, key)
				continue
			}
			duration := remaining
			if err := s.redis.Set(s.ctx, key, value, &duration); err != nil {
				return fmt.Errorf("failed to sync key %s to Redis with TTL: %v", key, err)
			}
		} else {
			// No TTL, just set the value
			if err := s.redis.Set(s.ctx, key, value, nil); err != nil {
				return fmt.Errorf("failed to sync key %s to Redis: %v", key, err)
			}
		}
	}

	return nil
}

// save writes the store data to disk
func (s *Store) save() error {
	data, err := json.Marshal(s.items)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(s.dataPath, data, 0644)
}

// GetPath returns the path to the store's data file
func (s *Store) GetPath() string {
	return filepath.Dir(s.dataPath)
}

// GetTTL returns the remaining TTL in seconds for a key
func (s *Store) GetTTL(key string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if value, exists := s.items[key]; exists {
		if value.TTL == nil {
			return -1, true // -1 indicates no TTL
		}
		remaining := time.Until(value.ExpireAt).Seconds()
		if remaining > 0 {
			return int64(remaining), true
		}
		// Key has expired, remove it
		delete(s.items, key)
		s.save()
	}
	return 0, false
}

// Incr increments the value at key by 1 using counter semantics
func (s *Store) Incr(key string, opts ...OpOption) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	options := &WriteOptions{
		Timestamp: time.Now().UnixNano(),
	}
	for _, opt := range opts {
		opt(options)
	}
	timestamp := options.Timestamp

	var counter int64
	if val, exists := s.items[key]; exists {
		if val.Type == TypeCounter {
			counter = val.Counter()
		} else {
			parsed, err := strconv.ParseInt(val.String(), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("value is not an integer")
			}
			counter = parsed
		}
	}
	counter++
	newVal := NewCounterValue(counter, timestamp, options.ReplicaID)
	if err := s.redis.Set(s.ctx, key, newVal, nil); err != nil {
		return counter, fmt.Errorf("failed to write to Redis: %v", err)
	}
	s.items[key] = newVal
	if err := s.save(); err != nil {
		return counter, fmt.Errorf("failed to save to disk: %v", err)
	}
	return counter, nil
}

// WriteOptions holds metadata for write operations (CRDT replication)
type WriteOptions struct {
	Timestamp int64
	ReplicaID string
	TTL       *time.Duration
}

// OpOption is a function that configures WriteOptions
type OpOption func(*WriteOptions)

// WithTimestamp sets the timestamp for the operation
func WithTimestamp(ts int64) OpOption {
	return func(o *WriteOptions) {
		o.Timestamp = ts
	}
}

// WithReplicaID sets the replica ID for the operation
func WithReplicaID(id string) OpOption {
	return func(o *WriteOptions) {
		o.ReplicaID = id
	}
}

// WithTTL sets the TTL for the operation
func WithTTL(d time.Duration) OpOption {
	return func(o *WriteOptions) {
		o.TTL = &d
	}
}

// LPush adds elements to the head of a list
func (s *Store) LPush(key string, values []string, opts ...OpOption) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse options (currently ignored to reproduce failure)
	options := &WriteOptions{
		Timestamp: time.Now().UnixNano(),
	}
	for _, opt := range opts {
		opt(options)
	}

	// Use options
	timestamp := options.Timestamp
	var list *CRDTList

	if val, exists := s.items[key]; exists && val.Type == TypeList {
		list = val.List()
		if list == nil {
			return 0, fmt.Errorf("invalid list data")
		}
	} else {
		// Create new list
		newVal := NewListValue(timestamp, options.ReplicaID)
		list = newVal.List()
		s.items[key] = newVal
	}

	// Add values in reverse order to maintain Redis LPUSH semantics
	for i := len(values) - 1; i >= 0; i-- {
		list.LPush(values[i], timestamp, options.ReplicaID)
	}

	// Update the value
	s.items[key].SetList(list, timestamp)

	// Update Redis (store as JSON for now)
	if err := s.redis.Set(s.ctx, key, s.items[key], nil); err != nil {
		return int64(list.Len()), fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return int64(list.Len()), fmt.Errorf("failed to save to disk: %v", err)
	}

	return int64(list.Len()), nil
}

// RPush adds elements to the tail of a list
func (s *Store) RPush(key string, values []string, opts ...OpOption) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	options := &WriteOptions{
		Timestamp: time.Now().UnixNano(),
	}
	for _, opt := range opts {
		opt(options)
	}

	timestamp := options.Timestamp
	var list *CRDTList

	if val, exists := s.items[key]; exists && val.Type == TypeList {
		list = val.List()
		if list == nil {
			return 0, fmt.Errorf("invalid list data")
		}
	} else {
		// Create new list
		newVal := NewListValue(timestamp, options.ReplicaID)
		list = newVal.List()
		s.items[key] = newVal
	}

	// Add values in order
	for _, value := range values {
		list.RPush(value, timestamp, options.ReplicaID)
	}

	// Update the value
	s.items[key].SetList(list, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, s.items[key], nil); err != nil {
		return int64(list.Len()), fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return int64(list.Len()), fmt.Errorf("failed to save to disk: %v", err)
	}

	return int64(list.Len()), nil
}

// LPop removes and returns the first element from a list
func (s *Store) LPop(key string, opts ...OpOption) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	options := &WriteOptions{
		Timestamp: time.Now().UnixNano(),
	}
	for _, opt := range opts {
		opt(options)
	}

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return "", false, nil
	}

	list := val.List()
	if list == nil {
		return "", false, fmt.Errorf("invalid list data")
	}

	value, ok := list.LPop()
	if !ok {
		return "", false, nil
	}

	// Update the value
	timestamp := options.Timestamp
	val.SetList(list, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, val, nil); err != nil {
		return value, true, fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return value, true, fmt.Errorf("failed to save to disk: %v", err)
	}

	return value, true, nil
}

// RPop removes and returns the last element from a list
func (s *Store) RPop(key string, opts ...OpOption) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	options := &WriteOptions{
		Timestamp: time.Now().UnixNano(),
	}
	for _, opt := range opts {
		opt(options)
	}

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return "", false, nil
	}

	list := val.List()
	if list == nil {
		return "", false, fmt.Errorf("invalid list data")
	}

	value, ok := list.RPop()
	if !ok {
		return "", false, nil
	}

	// Update the value
	timestamp := options.Timestamp
	val.SetList(list, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, val, nil); err != nil {
		return value, true, fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return value, true, fmt.Errorf("failed to save to disk: %v", err)
	}

	return value, true, nil
}

// LRange returns elements in the specified range
func (s *Store) LRange(key string, start, stop int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return []string{}, nil
	}

	list := val.List()
	if list == nil {
		return nil, fmt.Errorf("invalid list data")
	}

	return list.Range(start, stop), nil
}

// LLen returns the length of a list
func (s *Store) LLen(key string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return 0, nil
	}

	list := val.List()
	if list == nil {
		return 0, fmt.Errorf("invalid list data")
	}

	return int64(list.Len()), nil
}

// LIndex returns the element at the specified index
func (s *Store) LIndex(key string, index int) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return "", false, nil
	}

	list := val.List()
	if list == nil {
		return "", false, fmt.Errorf("invalid list data")
	}

	value, ok := list.Index(index)
	return value, ok, nil
}

// LSet sets the element at the specified index to a new value
func (s *Store) LSet(key string, index int, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return fmt.Errorf("ERR no such key")
	}

	list := val.List()
	if list == nil {
		return fmt.Errorf("invalid list data")
	}

	timestamp := time.Now().UnixNano()
	if err := list.Set(index, value, timestamp, ""); err != nil {
		return err
	}

	// Update the value
	val.SetList(list, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, val, nil); err != nil {
		return fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return fmt.Errorf("failed to save to disk: %v", err)
	}

	return nil
}

// LInsert inserts a value before or after the pivot element
func (s *Store) LInsert(key string, before bool, pivot string, value string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return 0, nil // Key not found, return 0
	}

	list := val.List()
	if list == nil {
		return 0, fmt.Errorf("invalid list data")
	}

	timestamp := time.Now().UnixNano()
	insertAfter := !before
	result := list.Insert(pivot, value, insertAfter, timestamp, "")

	if result == -1 {
		return -1, nil // Pivot not found
	}

	// Update the value
	val.SetList(list, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, val, nil); err != nil {
		return int64(result), fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return int64(result), fmt.Errorf("failed to save to disk: %v", err)
	}

	return int64(result), nil
}

// LTrim trims a list to the specified range
func (s *Store) LTrim(key string, start, stop int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return nil // Key not found, no error per Redis semantics
	}

	list := val.List()
	if list == nil {
		return fmt.Errorf("invalid list data")
	}

	timestamp := time.Now().UnixNano()
	list.Trim(start, stop)

	// Update the value
	val.SetList(list, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, val, nil); err != nil {
		return fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return fmt.Errorf("failed to save to disk: %v", err)
	}

	return nil
}

// LRem removes elements from a list by value
func (s *Store) LRem(key string, count int, value string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeList {
		return 0, nil // Key not found
	}

	list := val.List()
	if list == nil {
		return 0, fmt.Errorf("invalid list data")
	}

	timestamp := time.Now().UnixNano()
	removed := list.Rem(count, value)

	// Update the value
	val.SetList(list, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, val, nil); err != nil {
		return int64(removed), fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return int64(removed), fmt.Errorf("failed to save to disk: %v", err)
	}

	return int64(removed), nil
}

// IncrBy increments the value at key by increment using counter semantics
func (s *Store) IncrBy(key string, increment int64, opts ...OpOption) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	options := &WriteOptions{
		Timestamp: time.Now().UnixNano(),
	}
	for _, opt := range opts {
		opt(options)
	}
	timestamp := options.Timestamp

	var counter int64
	if val, exists := s.items[key]; exists {
		if val.Type == TypeCounter {
			counter = val.Counter()
		} else {
			parsed, err := strconv.ParseInt(val.String(), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("value is not an integer")
			}
			counter = parsed
		}
	}
	counter += increment
	newVal := NewCounterValue(counter, timestamp, options.ReplicaID)
	if err := s.redis.Set(s.ctx, key, newVal, nil); err != nil {
		return counter, fmt.Errorf("failed to write to Redis: %v", err)
	}
	s.items[key] = newVal
	if err := s.save(); err != nil {
		return counter, fmt.Errorf("failed to save to disk: %v", err)
	}
	return counter, nil
}

// IncrByFloat increments the float value at key by increment using counter semantics
func (s *Store) IncrByFloat(key string, increment float64, opts ...OpOption) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	options := &WriteOptions{
		Timestamp: time.Now().UnixNano(),
	}
	for _, opt := range opts {
		opt(options)
	}
	timestamp := options.Timestamp

	var counter float64

	if val, exists := s.items[key]; exists {
		switch val.Type {
		case TypeFloatCounter:
			counter = val.FloatCounter()
		case TypeCounter:
			// Convert integer counter to float
			counter = float64(val.Counter())
		case TypeString:
			// Try to parse string as float
			parsed, err := strconv.ParseFloat(val.String(), 64)
			if err != nil {
				return 0, fmt.Errorf("value is not a valid float")
			}
			counter = parsed
		default:
			return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
	}

	counter += increment
	newVal := NewFloatCounterValue(counter, timestamp, options.ReplicaID)

	if err := s.redis.Set(s.ctx, key, newVal, nil); err != nil {
		return counter, fmt.Errorf("failed to write to Redis: %v", err)
	}
	s.items[key] = newVal
	if err := s.save(); err != nil {
		return counter, fmt.Errorf("failed to save to disk: %v", err)
	}
	return counter, nil
}
