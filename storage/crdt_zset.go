package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ZSetElement represents an element in a CRDT sorted set
type ZSetElement struct {
	Member    string       `json:"member"`     // The member value
	Score     float64      `json:"score"`      // The score for sorting
	ID        string       `json:"id"`         // Unique identifier for this element
	Timestamp int64        `json:"timestamp"`  // When this element was added
	ReplicaID string       `json:"replica_id"` // Which replica added this element
	AddedVC   *VectorClock `json:"added_vc"`   // Vector clock when added
	RemovedVC *VectorClock `json:"removed_vc"` // Vector clock when removed (nil if not removed)
	IsRemoved bool         `json:"is_removed"` // Whether this element is marked as removed
}

// CRDTZSet represents a CRDT sorted set using Observed-Remove semantics
// Elements are ordered by score, with ties broken by member lexicographically
type CRDTZSet struct {
	Elements  map[string]*ZSetElement `json:"elements"`   // Map from member to element
	ReplicaID string                  `json:"replica_id"` // This replica's ID
}

// NewCRDTZSet creates a new CRDT sorted set
func NewCRDTZSet(replicaID string) *CRDTZSet {
	return &CRDTZSet{
		Elements:  make(map[string]*ZSetElement),
		ReplicaID: replicaID,
	}
}

// generateTimestamp generates a unique timestamp
func generateTimestamp() int64 {
	return time.Now().UnixNano()
}

// ZAdd adds one or more members with scores to the sorted set
// Returns the number of elements that were added (not updated)
func (zs *CRDTZSet) ZAdd(memberScores map[string]float64, vc *VectorClock) int {
	added := 0
	timestamp := generateTimestamp()

	if vc == nil {
		vc = NewVectorClock()
	}
	vc.Increment(zs.ReplicaID)

	for member, score := range memberScores {
		elementID := fmt.Sprintf("%d-%s", timestamp, member)

		existing, exists := zs.Elements[member]
		if !exists {
			// New element
			zs.Elements[member] = &ZSetElement{
				Member:    member,
				Score:     score,
				ID:        elementID,
				Timestamp: timestamp,
				ReplicaID: zs.ReplicaID,
				AddedVC:   vc.Copy(),
				RemovedVC: nil,
				IsRemoved: false,
			}
			added++
		} else {
			// Update existing element if not removed or if this is a newer update
			if !existing.IsRemoved || (existing.RemovedVC != nil && vc.HappensBefore(existing.RemovedVC)) {
				// Update score if this is a newer update
				shouldUpdate := false
				if existing.AddedVC == nil || existing.AddedVC.HappensBefore(vc) {
					shouldUpdate = true
				} else if existing.AddedVC.IsConcurrent(vc) {
					// For concurrent updates, use timestamp as tie-breaker
					shouldUpdate = timestamp > existing.Timestamp
				}

				if shouldUpdate {
					existing.Score = score
					existing.Timestamp = timestamp
					existing.ReplicaID = zs.ReplicaID
					existing.AddedVC = vc.Copy()
					existing.IsRemoved = false
					existing.RemovedVC = nil
				}
			}
		}
	}

	return added
}

// ZRem removes one or more members from the sorted set
// Returns the number of elements that were removed
func (zs *CRDTZSet) ZRem(members []string, vc *VectorClock) int {
	removed := 0

	if vc == nil {
		vc = NewVectorClock()
	}
	vc.Increment(zs.ReplicaID)

	for _, member := range members {
		if element, exists := zs.Elements[member]; exists && !element.IsRemoved {
			// Only remove if we've observed this element being added
			if element.AddedVC == nil || !vc.HappensBefore(element.AddedVC) {
				element.IsRemoved = true
				element.RemovedVC = vc.Copy()
				removed++
			}
		}
	}

	return removed
}

// ZScore returns the score of a member, or nil if the member doesn't exist
func (zs *CRDTZSet) ZScore(member string) (*float64, bool) {
	if element, exists := zs.Elements[member]; exists && !element.IsRemoved {
		return &element.Score, true
	}
	return nil, false
}

// ZCard returns the number of elements in the sorted set
func (zs *CRDTZSet) ZCard() int {
	count := 0
	for _, element := range zs.Elements {
		if !element.IsRemoved {
			count++
		}
	}
	return count
}

// ZRange returns elements in the specified range, ordered by score
// Start and stop are 0-based indices. Negative indices count from the end.
// If withScores is true, scores are included in the result
func (zs *CRDTZSet) ZRange(start, stop int, withScores bool) ([]string, []float64) {
	// Get all non-removed elements
	var elements []*ZSetElement
	for _, element := range zs.Elements {
		if !element.IsRemoved {
			elements = append(elements, element)
		}
	}

	// Sort by score, then by member lexicographically
	sort.Slice(elements, func(i, j int) bool {
		if elements[i].Score != elements[j].Score {
			return elements[i].Score < elements[j].Score
		}
		return elements[i].Member < elements[j].Member
	})

	// Handle negative indices
	length := len(elements)
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	// Clamp indices
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}

	// Extract range
	var members []string
	var scores []float64

	if start <= stop && start < length {
		for i := start; i <= stop && i < length; i++ {
			members = append(members, elements[i].Member)
			if withScores {
				scores = append(scores, elements[i].Score)
			}
		}
	}

	return members, scores
}

// ZRangeByScore returns elements with scores between min and max (inclusive)
func (zs *CRDTZSet) ZRangeByScore(min, max float64, withScores bool) ([]string, []float64) {
	// Get all non-removed elements within score range
	var elements []*ZSetElement
	for _, element := range zs.Elements {
		if !element.IsRemoved && element.Score >= min && element.Score <= max {
			elements = append(elements, element)
		}
	}

	// Sort by score, then by member lexicographically
	sort.Slice(elements, func(i, j int) bool {
		if elements[i].Score != elements[j].Score {
			return elements[i].Score < elements[j].Score
		}
		return elements[i].Member < elements[j].Member
	})

	// Extract results
	var members []string
	var scores []float64

	for _, element := range elements {
		members = append(members, element.Member)
		if withScores {
			scores = append(scores, element.Score)
		}
	}

	return members, scores
}

// ZRank returns the rank of a member (0-based index in sorted order)
func (zs *CRDTZSet) ZRank(member string) (*int, bool) {
	// Check if member exists
	targetElement, exists := zs.Elements[member]
	if !exists || targetElement.IsRemoved {
		return nil, false
	}

	// Get all non-removed elements
	var elements []*ZSetElement
	for _, element := range zs.Elements {
		if !element.IsRemoved {
			elements = append(elements, element)
		}
	}

	// Sort by score, then by member lexicographically
	sort.Slice(elements, func(i, j int) bool {
		if elements[i].Score != elements[j].Score {
			return elements[i].Score < elements[j].Score
		}
		return elements[i].Member < elements[j].Member
	})

	// Find the rank
	for i, element := range elements {
		if element.Member == member {
			return &i, true
		}
	}

	return nil, false
}

// Merge merges another CRDT sorted set into this one
func (zs *CRDTZSet) Merge(other *CRDTZSet) {
	if other == nil {
		return
	}

	for member, otherElement := range other.Elements {
		existing, exists := zs.Elements[member]

		if !exists {
			// Element doesn't exist locally, add it
			zs.Elements[member] = &ZSetElement{
				Member:    otherElement.Member,
				Score:     otherElement.Score,
				ID:        otherElement.ID,
				Timestamp: otherElement.Timestamp,
				ReplicaID: otherElement.ReplicaID,
				AddedVC:   otherElement.AddedVC.Copy(),
				RemovedVC: nil,
				IsRemoved: false,
			}
			if otherElement.RemovedVC != nil {
				zs.Elements[member].RemovedVC = otherElement.RemovedVC.Copy()
				zs.Elements[member].IsRemoved = otherElement.IsRemoved
			}
		} else {
			// Element exists locally, merge
			// Merge vector clocks
			if existing.AddedVC != nil && otherElement.AddedVC != nil {
				existing.AddedVC.Update(otherElement.AddedVC)
			}

			// Handle removal
			if otherElement.RemovedVC != nil {
				if existing.RemovedVC == nil {
					existing.RemovedVC = otherElement.RemovedVC.Copy()
					existing.IsRemoved = true
				} else {
					existing.RemovedVC.Update(otherElement.RemovedVC)
					existing.IsRemoved = existing.IsRemoved || otherElement.IsRemoved
				}
			}

			// Update score if the other element has a more recent add
			if !existing.IsRemoved && !otherElement.IsRemoved {
				shouldUpdate := false
				if existing.AddedVC == nil && otherElement.AddedVC != nil {
					shouldUpdate = true
				} else if existing.AddedVC != nil && otherElement.AddedVC != nil {
					if existing.AddedVC.HappensBefore(otherElement.AddedVC) {
						shouldUpdate = true
					} else if existing.AddedVC.IsConcurrent(otherElement.AddedVC) {
						// For concurrent updates, use timestamp as tie-breaker
						shouldUpdate = otherElement.Timestamp > existing.Timestamp
					}
				}

				if shouldUpdate {
					existing.Score = otherElement.Score
					existing.Timestamp = otherElement.Timestamp
					existing.ReplicaID = otherElement.ReplicaID
				}
			}
		}
	}
}

// NewZSetValue creates a new Value containing a CRDT sorted set
func NewZSetValue(replicaID string, vc *VectorClock) *Value {
	zset := NewCRDTZSet(replicaID)
	data, _ := json.Marshal(zset)

	if vc == nil {
		vc = NewVectorClock()
	}
	vc.Increment(replicaID)

	return &Value{
		Type:        TypeZSet,
		Data:        data,
		Timestamp:   generateTimestamp(),
		ReplicaID:   replicaID,
		VectorClock: vc,
	}
}

// GetZSet extracts the CRDT sorted set from a Value
func (v *Value) GetZSet() (*CRDTZSet, error) {
	if v.Type != TypeZSet {
		return nil, fmt.Errorf("value is not a sorted set")
	}

	var zset CRDTZSet
	err := json.Unmarshal(v.Data, &zset)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal sorted set: %v", err)
	}

	return &zset, nil
}

// SetZSet updates the Value with a new CRDT sorted set
func (v *Value) SetZSet(zset *CRDTZSet) error {
	if v.Type != TypeZSet {
		return fmt.Errorf("value is not a sorted set")
	}

	data, err := json.Marshal(zset)
	if err != nil {
		return fmt.Errorf("failed to marshal sorted set: %v", err)
	}

	v.Data = data
	v.Timestamp = generateTimestamp()

	return nil
}
