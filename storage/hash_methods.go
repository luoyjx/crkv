package storage

import (
	"fmt"
	"time"
)

// HSet sets a field in a hash
func (s *Store) HSet(key, field, value string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().UnixNano()
	var hash *CRDTHash
	var isNewField int64

	if val, exists := s.items[key]; exists && val.Type == TypeHash {
		hash = val.Hash()
		if hash == nil {
			return 0, fmt.Errorf("invalid hash data")
		}
	} else {
		// Create new hash
		newVal := NewHashValue(timestamp, "")
		hash = newVal.Hash()
		s.items[key] = newVal
	}

	// Check if field is new
	if !hash.Exists(field) {
		isNewField = 1
	}

	// Set the field
	hash.Set(field, value, timestamp, "")

	// Update the value
	s.items[key].SetHash(hash, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, s.items[key], nil); err != nil {
		return isNewField, fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return isNewField, fmt.Errorf("failed to save to disk: %v", err)
	}

	return isNewField, nil
}

// HGet gets a field from a hash
func (s *Store) HGet(key, field string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeHash {
		return "", false, nil
	}

	hash := val.Hash()
	if hash == nil {
		return "", false, fmt.Errorf("invalid hash data")
	}

	value, exists := hash.Get(field)
	return value, exists, nil
}

// HDel deletes fields from a hash
func (s *Store) HDel(key string, fields ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeHash {
		return 0, nil
	}

	hash := val.Hash()
	if hash == nil {
		return 0, fmt.Errorf("invalid hash data")
	}

	var deleted int64
	for _, field := range fields {
		if hash.Delete(field) {
			deleted++
		}
	}

	if deleted > 0 {
		// Update the value
		timestamp := time.Now().UnixNano()
		val.SetHash(hash, timestamp)

		// Update Redis
		if err := s.redis.Set(s.ctx, key, val, nil); err != nil {
			return deleted, fmt.Errorf("failed to write to Redis: %v", err)
		}

		if err := s.save(); err != nil {
			return deleted, fmt.Errorf("failed to save to disk: %v", err)
		}
	}

	return deleted, nil
}

// HKeys returns all field names in a hash
func (s *Store) HKeys(key string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeHash {
		return []string{}, nil
	}

	hash := val.Hash()
	if hash == nil {
		return nil, fmt.Errorf("invalid hash data")
	}

	return hash.Keys(), nil
}

// HVals returns all values in a hash
func (s *Store) HVals(key string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeHash {
		return []string{}, nil
	}

	hash := val.Hash()
	if hash == nil {
		return nil, fmt.Errorf("invalid hash data")
	}

	return hash.Values(), nil
}

// HGetAll returns all field-value pairs in a hash
func (s *Store) HGetAll(key string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeHash {
		return map[string]string{}, nil
	}

	hash := val.Hash()
	if hash == nil {
		return nil, fmt.Errorf("invalid hash data")
	}

	return hash.GetAll(), nil
}

// HLen returns the number of fields in a hash
func (s *Store) HLen(key string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeHash {
		return 0, nil
	}

	hash := val.Hash()
	if hash == nil {
		return 0, fmt.Errorf("invalid hash data")
	}

	return int64(hash.Len()), nil
}

// HExists checks if a field exists in a hash
func (s *Store) HExists(key, field string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeHash {
		return false, nil
	}

	hash := val.Hash()
	if hash == nil {
		return false, fmt.Errorf("invalid hash data")
	}

	return hash.Exists(field), nil
}
