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
	dataPath        string            // Path to persist CRDT state
	redis           *RedisStore       // Local Redis instance
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
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

	ctx, cancel := context.WithCancel(context.Background())
	store := &Store{
		items:           make(map[string]*Value),
		dataPath:        filepath.Join(dataDir, "store.json"),
		redis:           redis,
		cleanupInterval: time.Second * 1,
		stopCleanup:     make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Load existing CRDT state if any
	if err := store.load(); err != nil {
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

// Close closes the store and its resources
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Save current state to disk
	if err := s.save(); err != nil {
		return fmt.Errorf("failed to save state on close: %v", err)
	}

	// Stop cleanup goroutine
	close(s.stopCleanup)

	// Cancel context
	s.cancel()

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
func (s *Store) Incr(key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().UnixNano()
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
	newVal := NewCounterValue(counter, timestamp, "")
	if err := s.redis.Set(s.ctx, key, newVal, nil); err != nil {
		return counter, fmt.Errorf("failed to write to Redis: %v", err)
	}
	s.items[key] = newVal
	if err := s.save(); err != nil {
		return counter, fmt.Errorf("failed to save to disk: %v", err)
	}
	return counter, nil
}

// IncrBy increments the value at key by increment using counter semantics
func (s *Store) IncrBy(key string, increment int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().UnixNano()
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
	newVal := NewCounterValue(counter, timestamp, "")
	if err := s.redis.Set(s.ctx, key, newVal, nil); err != nil {
		return counter, fmt.Errorf("failed to write to Redis: %v", err)
	}
	s.items[key] = newVal
	if err := s.save(); err != nil {
		return counter, fmt.Errorf("failed to save to disk: %v", err)
	}
	return counter, nil
}
