package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ZAdd adds one or more members with scores to the sorted set
func (s *Store) ZAdd(key string, memberScores map[string]float64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.items[key]
	var zset *CRDTZSet

	if !exists {
		// Create new sorted set
		value = NewZSetValue("", nil)
		s.items[key] = value
		var err error
		zset, err = value.GetZSet()
		if err != nil {
			return 0, fmt.Errorf("failed to get zset: %v", err)
		}
	} else {
		if value.Type != TypeZSet {
			return 0, fmt.Errorf("key is not a sorted set")
		}
		var err error
		zset, err = value.GetZSet()
		if err != nil {
			return 0, fmt.Errorf("failed to get zset: %v", err)
		}
	}

	// Add members to the sorted set
	added := zset.ZAdd(memberScores, value.VectorClock)

	// Update the value in the store
	value.SetZSet(zset)

	return added, nil
}

// ZIncrBy increments the score of a member by increment using counter semantics
// If the member does not exist, it is added with increment as its score
// Returns the new effective score
func (s *Store) ZIncrBy(key string, member string, increment float64) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.items[key]
	var zset *CRDTZSet

	if !exists {
		// Create new sorted set
		value = NewZSetValue("", nil)
		s.items[key] = value
		var err error
		zset, err = value.GetZSet()
		if err != nil {
			return 0, fmt.Errorf("failed to get zset: %v", err)
		}
	} else {
		if value.Type != TypeZSet {
			return 0, fmt.Errorf("key is not a sorted set")
		}
		var err error
		zset, err = value.GetZSet()
		if err != nil {
			return 0, fmt.Errorf("failed to get zset: %v", err)
		}
	}

	// Increment using counter semantics
	newScore := zset.ZIncrBy(member, increment, value.VectorClock)

	// Update the value in the store
	value.SetZSet(zset)

	return newScore, nil
}

// ZRem removes one or more members from the sorted set
func (s *Store) ZRem(key string, members []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.items[key]
	if !exists {
		return 0, nil // Key doesn't exist
	}

	if value.Type != TypeZSet {
		return 0, fmt.Errorf("key is not a sorted set")
	}

	zset, err := value.GetZSet()
	if err != nil {
		return 0, fmt.Errorf("failed to get zset: %v", err)
	}

	// Remove members from the sorted set
	timestamp := time.Now().UnixNano()
	removed := zset.ZRem(members, timestamp, value.VectorClock)

	// Update the value
	value.SetZSet(zset)

	return removed, nil
}

// ZScore returns the score of a member
func (s *Store) ZScore(key, member string) (*float64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.items[key]
	if !exists {
		return nil, false, nil // Key doesn't exist
	}

	if value.Type != TypeZSet {
		return nil, false, fmt.Errorf("key is not a sorted set")
	}

	zset, err := value.GetZSet()
	if err != nil {
		return nil, false, fmt.Errorf("failed to get zset: %v", err)
	}

	score, found := zset.ZScore(member)
	return score, found, nil
}

// ZCard returns the number of elements in the sorted set
func (s *Store) ZCard(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.items[key]
	if !exists {
		return 0, nil // Key doesn't exist
	}

	if value.Type != TypeZSet {
		return 0, fmt.Errorf("key is not a sorted set")
	}

	zset, err := value.GetZSet()
	if err != nil {
		return 0, fmt.Errorf("failed to get zset: %v", err)
	}

	return zset.ZCard(), nil
}

// ZRange returns elements in the specified range
func (s *Store) ZRange(key string, start, stop int, withScores bool) ([]string, []float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.items[key]
	if !exists {
		return []string{}, []float64{}, nil // Key doesn't exist
	}

	if value.Type != TypeZSet {
		return nil, nil, fmt.Errorf("key is not a sorted set")
	}

	zset, err := value.GetZSet()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get zset: %v", err)
	}

	members, scores := zset.ZRange(start, stop, withScores)
	return members, scores, nil
}

// ZRangeByScore returns elements with scores between min and max
func (s *Store) ZRangeByScore(key string, min, max float64, withScores bool) ([]string, []float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.items[key]
	if !exists {
		return []string{}, []float64{}, nil // Key doesn't exist
	}

	if value.Type != TypeZSet {
		return nil, nil, fmt.Errorf("key is not a sorted set")
	}

	zset, err := value.GetZSet()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get zset: %v", err)
	}

	members, scores := zset.ZRangeByScore(min, max, withScores)
	return members, scores, nil
}

// ZRank returns the rank of a member
func (s *Store) ZRank(key, member string) (*int, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.items[key]
	if !exists {
		return nil, false, nil // Key doesn't exist
	}

	if value.Type != TypeZSet {
		return nil, false, fmt.Errorf("key is not a sorted set")
	}

	zset, err := value.GetZSet()
	if err != nil {
		return nil, false, fmt.Errorf("failed to get zset: %v", err)
	}

	rank, found := zset.ZRank(member)
	return rank, found, nil
}

// ParseZAddArgs parses ZADD command arguments into member-score pairs
func ParseZAddArgs(args []string) (map[string]float64, error) {
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("wrong number of arguments: ZADD expects score-member pairs")
	}

	memberScores := make(map[string]float64)
	for i := 0; i < len(args); i += 2 {
		scoreStr := args[i]
		member := args[i+1]

		score, err := strconv.ParseFloat(scoreStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid score '%s': %v", scoreStr, err)
		}

		memberScores[member] = score
	}

	return memberScores, nil
}

// ParseZRangeArgs parses ZRANGE command arguments
func ParseZRangeArgs(args []string) (start, stop int, withScores bool, err error) {
	if len(args) < 2 {
		return 0, 0, false, fmt.Errorf("wrong number of arguments for ZRANGE")
	}

	start, err = strconv.Atoi(args[0])
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid start index: %v", err)
	}

	stop, err = strconv.Atoi(args[1])
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid stop index: %v", err)
	}

	// Check for WITHSCORES option
	withScores = false
	if len(args) > 2 {
		for i := 2; i < len(args); i++ {
			if strings.ToUpper(args[i]) == "WITHSCORES" {
				withScores = true
				break
			}
		}
	}

	return start, stop, withScores, nil
}

// ParseZRangeByScoreArgs parses ZRANGEBYSCORE command arguments
func ParseZRangeByScoreArgs(args []string) (min, max float64, withScores bool, err error) {
	if len(args) < 2 {
		return 0, 0, false, fmt.Errorf("wrong number of arguments for ZRANGEBYSCORE")
	}

	// Parse min score (support -inf)
	if args[0] == "-inf" {
		min = -1e308 // Use a very large negative number
	} else {
		min, err = strconv.ParseFloat(args[0], 64)
		if err != nil {
			return 0, 0, false, fmt.Errorf("invalid min score: %v", err)
		}
	}

	// Parse max score (support +inf)
	if args[1] == "+inf" || args[1] == "inf" {
		max = 1e308 // Use a very large positive number
	} else {
		max, err = strconv.ParseFloat(args[1], 64)
		if err != nil {
			return 0, 0, false, fmt.Errorf("invalid max score: %v", err)
		}
	}

	// Check for WITHSCORES option
	withScores = false
	if len(args) > 2 {
		for i := 2; i < len(args); i++ {
			if strings.ToUpper(args[i]) == "WITHSCORES" {
				withScores = true
				break
			}
		}
	}

	return min, max, withScores, nil
}
