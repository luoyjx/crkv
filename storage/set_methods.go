package storage

import (
	"fmt"
	"time"
)

// SAdd adds members to a set
func (s *Store) SAdd(key string, members ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().UnixNano()
	var set *CRDTSet
	var added int64

	if val, exists := s.items[key]; exists && val.Type == TypeSet {
		set = val.Set()
		if set == nil {
			return 0, fmt.Errorf("invalid set data")
		}
	} else {
		// Create new set
		newVal := NewSetValue(timestamp, "")
		set = newVal.Set()
		s.items[key] = newVal
	}

	// Add members and count new additions
	for _, member := range members {
		if !set.Contains(member) {
			set.Add(member, timestamp, "")
			added++
		}
	}

	// Update the value
	s.items[key].SetSet(set, timestamp)

	// Update Redis
	if err := s.redis.Set(s.ctx, key, s.items[key], nil); err != nil {
		return added, fmt.Errorf("failed to write to Redis: %v", err)
	}

	if err := s.save(); err != nil {
		return added, fmt.Errorf("failed to save to disk: %v", err)
	}

	return added, nil
}

// SRem removes members from a set
func (s *Store) SRem(key string, members ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeSet {
		return 0, nil
	}

	set := val.Set()
	if set == nil {
		return 0, fmt.Errorf("invalid set data")
	}

	var removed int64
	timestamp := time.Now().UnixNano()
	for _, member := range members {
		if set.Remove(member, timestamp) {
			removed++
		}
	}

	if removed > 0 {
		// Update the value
		val.SetSet(set, timestamp)

		// Update Redis
		if err := s.redis.Set(s.ctx, key, val, nil); err != nil {
			return removed, fmt.Errorf("failed to write to Redis: %v", err)
		}

		if err := s.save(); err != nil {
			return removed, fmt.Errorf("failed to save to disk: %v", err)
		}
	}

	return removed, nil
}

// SMembers returns all members of a set
func (s *Store) SMembers(key string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeSet {
		return []string{}, nil
	}

	set := val.Set()
	if set == nil {
		return nil, fmt.Errorf("invalid set data")
	}

	return set.Members(), nil
}

// SCard returns the cardinality (size) of a set
func (s *Store) SCard(key string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeSet {
		return 0, nil
	}

	set := val.Set()
	if set == nil {
		return 0, fmt.Errorf("invalid set data")
	}

	return int64(set.Size()), nil
}

// SIsMember checks if a member exists in a set
func (s *Store) SIsMember(key string, member string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.items[key]
	if !exists || val.Type != TypeSet {
		return false, nil
	}

	set := val.Set()
	if set == nil {
		return false, fmt.Errorf("invalid set data")
	}

	return set.Contains(member), nil
}
