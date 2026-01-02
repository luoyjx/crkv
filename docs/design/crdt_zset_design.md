# CRDT Sorted Set Design Document

## Overview

This document describes the design for CRDT Sorted Sets with Active-Active database semantics, specifically focusing on conflict resolution for concurrent operations and counter accumulation for ZINCRBY.

## CRDT Semantics (Based on Active-Active Documentation)

### Two-Phase Conflict Resolution

CRDT Sorted Sets use a two-phase conflict resolution strategy:

1. **Set Level: OR-Set (Observed-Remove Set)**
   - Writes across multiple instances are typically unioned
   - ZREM can only remove elements it has already "observed" (seen)
   - Add/Update wins over concurrent delete for unobserved elements

2. **Score Level: Dual Semantics**
   - **ZADD**: Uses Last-Write-Wins (LWW) based on timestamp
   - **ZINCRBY**: Uses counter accumulation semantics (sum all increments)

### Key Behaviors

| Operation | Semantics | Conflict Resolution |
|-----------|-----------|---------------------|
| ZADD | LWW | Later timestamp wins; tie-break by replica ID |
| ZINCRBY | Counter | Accumulate all deltas across replicas |
| ZREM | Observed-Remove | Remove only observed elements; add wins for unobserved |

## Data Structure Design

### ZSetElement Structure

```go
type ZSetElement struct {
    Member    string       // The member value
    Score     float64      // Base score (from ZADD with LWW)
    Delta     float64      // Accumulated delta from ZINCRBY (counter semantics)
    ID        string       // Unique identifier
    Timestamp int64        // Wall clock timestamp of last update
    ReplicaID string       // Which replica added this element
    AddedVC   *VectorClock // Vector clock when added
    RemovedVC *VectorClock // Vector clock when removed
    IsRemoved bool         // Whether element is marked as removed
}
```

### Effective Score Calculation

```
EffectiveScore = Score (base from ZADD) + Delta (accumulated from ZINCRBY)
```

## ZINCRBY Counter Semantics

### Behavior Specification

Based on Active-Active documentation example:

```
| Time | Instance 1       | Instance 2       |
|------|------------------|------------------|
| t1   | ZADD Z 1.1 x     |                  |
| t2   | — Sync —         | — Sync —         |
| t3   | ZINCRBY Z 1.0 x  | ZINCRBY Z 1.0 x  |
| t4   | — Sync —         | — Sync —         |
| t5   | ZSCORE Z x => 3.1| ZSCORE Z x => 3.1|
```

**Result**: The final score is the sum of base score (1.1) + all ZINCRBY operations (1.0 + 1.0 = 2.0) = 3.1

### ZREM + ZINCRBY Concurrent Behavior

Based on Active-Active documentation example:

```
| Time | Instance 1         | Instance 2        |
|------|--------------------|--------------------|
| t1   | ZADD Z 4.1 x       |                    |
| t2   | — Sync —           | — Sync —           |
| t3   | ZSCORE Z x => 4.1  | ZSCORE Z x => 4.1  |
| t4   | ZREM Z x           | ZINCRBY Z 2.0 x    |
| t5   | ZSCORE Z x => nil  | ZSCORE Z x => 6.1  |
| t6   | — Sync —           | — Sync —           |
| t7   | ZSCORE Z x => 2.0  | ZSCORE Z x => 2.0  |
```

**Explanation**:
- At t4-t5: ZREM on Instance 1 removes element locally; ZINCRBY on Instance 2 increases score
- At t7 after sync: ZREM removes only observed value (4.1), leaving unobserved increment (2.0)

### Implementation Algorithm

1. **On ZINCRBY call**:
   - If element doesn't exist: create with `Score = increment`, `Delta = 0`
   - If element exists: `Delta += increment`

2. **On Merge** (for existing elements):
   - Accumulate `Delta` values from both replicas (counter semantics)
   - Apply LWW for `Score` field (for concurrent ZADD operations)
   - Handle removal with observed-remove semantics

## Test Scenarios

### T001: ZINCRBY Counter Semantics Unit Tests

#### Test Case 1: Basic ZINCRBY on New Element
- **Setup**: Empty ZSet
- **Operation**: `ZINCRBY key 10 member`
- **Expected**: Score = 10, member added to set

#### Test Case 2: ZINCRBY on Existing Element
- **Setup**: ZSet with member score 5
- **Operation**: `ZINCRBY key 3 member`
- **Expected**: EffectiveScore = 8 (5 + 3)

#### Test Case 3: Multiple ZINCRBY Accumulation
- **Setup**: ZSet with member score 10
- **Operations**: 
  - `ZINCRBY key 5 member`
  - `ZINCRBY key 3 member`
  - `ZINCRBY key -2 member`
- **Expected**: EffectiveScore = 16 (10 + 5 + 3 + (-2))

#### Test Case 4: ZINCRBY with Negative Increment
- **Setup**: ZSet with member score 10
- **Operation**: `ZINCRBY key -15 member`
- **Expected**: EffectiveScore = -5

#### Test Case 5: Concurrent ZINCRBY Merge
- **Setup**: 
  - Replica1 ZSet with member score 10, delta 0
  - Replica2 ZSet with member score 10, delta 0
- **Operations**:
  - Replica1: `ZINCRBY key 5 member` → delta = 5
  - Replica2: `ZINCRBY key 3 member` → delta = 3
- **Merge**: Merge Replica2 into Replica1
- **Expected**: EffectiveScore = 18 (10 + 5 + 3)

#### Test Case 6: ZADD LWW + ZINCRBY Counter Merge
- **Setup**:
  - Replica1: ZADD member score 10 at t1, then ZINCRBY +5 at t3
  - Replica2: ZADD member score 20 at t2 (t2 > t1), then ZINCRBY +3 at t4
- **Merge**: Merge Replica2 into Replica1
- **Expected**: 
  - Score = 20 (LWW: t2 > t1)
  - Delta = 8 (5 + 3)
  - EffectiveScore = 28

#### Test Case 7: ZREM + ZINCRBY Concurrent (Observed-Remove)
- **Setup**:
  - Replica1: Has member with score 4.1 (synced)
  - Replica2: Has member with score 4.1 (synced)
- **Operations**:
  - Replica1: ZREM member
  - Replica2: ZINCRBY +2.0 member
- **Merge**: Merge Replica2 into Replica1
- **Expected**: Element exists with EffectiveScore = 2.0 (ZINCRBY preserves unobserved increment)

#### Test Case 8: ZINCRBY on Removed Element (Re-Add)
- **Setup**: ZSet with member previously removed
- **Operation**: `ZINCRBY key 5 member`
- **Expected**: Member is re-added with Score = 5

#### Test Case 9: Float Increment Precision
- **Setup**: ZSet with member score 0
- **Operations**: Multiple small float increments
- **Expected**: Correct floating point accumulation

#### Test Case 10: ZRange/ZRangeByScore with EffectiveScore
- **Setup**: ZSet with multiple members having both Score and Delta
- **Expected**: Sorting and filtering use EffectiveScore, not just Score

## File Locations

- Implementation: `storage/crdt_zset.go`
- Unit Tests: `storage/crdt_zset_test.go`
- Integration Tests: `zset_integration_test.go`

## Dependencies

- `storage/vector_clock.go` - Vector clock for causality tracking
- `storage/types.go` - Value type definitions

## References

- [Active-Active Sorted Sets Documentation](../active_active/data_types/5_sorted-sets.md)
- [Redis ZINCRBY Command](https://redis.io/commands/zincrby/)
