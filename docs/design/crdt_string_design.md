# CRDT String Design Document

## Overview

This document describes the design for CRDT Strings with Active-Active database semantics, focusing on:
1. LWW (Last-Write-Wins) for regular string values
2. Accumulative counter semantics for integer counters (INCR/INCRBY/DECR/DECRBY)
3. **INCRBYFLOAT** - Float counter with accumulative semantics (new)

## CRDT Semantics (Based on Active-Active Documentation)

### String Types

Strings in Active-Active databases have two modes of conflict resolution:

1. **Regular Strings (LWW)**
   - Uses wall-clock timestamp for "last write wins" resolution
   - When replication syncer cannot determine order, the value with latest timestamp wins
   - This is the only case where OS time is used for conflict resolution

2. **String Counters (Accumulative)**
   - INCR, INCRBY, DECR, DECRBY use semantic conflict resolution
   - On conflicting writes, counters accumulate the total operations across all replicas
   - Each sync accumulates private increments/decrements from each region

### 59-bit Counter Limit

Per the Active-Active documentation:
> "Active-Active databases support 59-bit counters. This limitation is to protect from overflowing a counter in a concurrent operation."

The 59-bit limit applies to both integer and float counters to prevent overflow during concurrent operations.

## Data Structure Design

### Current Value Structure

```go
type ValueType int

const (
    TypeString  ValueType = iota // Regular string with LWW
    TypeCounter                  // Integer counter with accumulative semantics
)

type Value struct {
    Type        ValueType    // Type of the value (string or counter)
    Data        []byte       // Raw bytes of the value
    Timestamp   int64        // Wall clock timestamp for LWW
    ReplicaID   string
    VectorClock *VectorClock // Vector clock for causality tracking
    TTL         *int64       // TTL in seconds
    ExpireAt    time.Time    // Absolute expiration time
}
```

### Proposed Enhancement for Float Counter

Add a new type and data structure for float counters:

```go
const (
    TypeString       ValueType = iota // Regular string with LWW
    TypeCounter                       // Integer counter (accumulative)
    TypeFloatCounter                  // Float counter (accumulative) - NEW
)

// FloatCounterValue stores float counter as scaled integer for precision
type FloatCounterData struct {
    Value   float64 // Current float value
    // Note: For CRDT merge, we store accumulated delta separately
    // during replication to enable proper accumulation
}
```

### Alternative: Unified Counter with Float Support

Instead of a new type, extend TypeCounter to support both int and float:

```go
// CounterData stores either integer or float counter value
// Integer counters use IntValue, float counters use FloatValue
type CounterData struct {
    IntValue   int64   // Integer counter value
    FloatValue float64 // Float counter value  
    IsFloat    bool    // Distinguishes between int/float counter
}
```

**Recommendation**: Use the second approach (unified counter) for simpler implementation.

## INCRBYFLOAT Semantics

### Behavior Specification

Based on Redis INCRBYFLOAT documentation and Active-Active counter semantics:

1. **Basic Operation**
   - `INCRBYFLOAT key increment` - Increment the float value stored at key
   - If key does not exist, initialize to 0.0 before operation
   - Returns the new value after increment

2. **Type Handling**
   - If value is a regular string that can be parsed as float, convert to float counter
   - If value is an integer counter, convert to float counter
   - If value cannot be parsed as number, return error

3. **CRDT Conflict Resolution**
   - Use accumulative semantics like integer counters
   - On merge, accumulate float deltas from concurrent operations
   - Example:
     ```
     Region 1: INCRBYFLOAT key 2.5  -> key = 2.5
     Region 2: INCRBYFLOAT key 1.3  -> key = 1.3
     After Sync: key = 3.8 (2.5 + 1.3)
     ```

4. **Float Precision**
   - Use IEEE 754 double precision (float64)
   - Apply standard float comparison with epsilon for equality checks
   - Store with full precision (%.17g format for serialization)

### Implementation Algorithm

```go
func (s *Store) IncrByFloat(key string, increment float64) (float64, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    timestamp := time.Now().UnixNano()
    var currentValue float64 = 0.0
    
    if val, exists := s.items[key]; exists {
        switch val.Type {
        case TypeCounter:
            // Convert integer counter to float
            currentValue = float64(val.Counter())
        case TypeFloatCounter:
            // Already a float counter
            currentValue = val.FloatCounter()
        case TypeString:
            // Try to parse string as float
            parsed, err := strconv.ParseFloat(val.String(), 64)
            if err != nil {
                return 0, fmt.Errorf("value is not a valid float")
            }
            currentValue = parsed
        }
    }
    
    newValue := currentValue + increment
    
    // Create new float counter value
    newVal := NewFloatCounterValue(newValue, timestamp, replicaID)
    
    // Store and persist
    s.items[key] = newVal
    // ... redis sync and disk save
    
    return newValue, nil
}
```

### Merge Algorithm for Float Counters

For CRDT merge during replication:

```go
func (v *Value) Merge(other *Value) {
    // ... existing code ...
    
    case TypeFloatCounter:
        // Accumulate float counter values
        newValue := v.FloatCounter() + other.FloatCounter()
        v.SetFloatCounter(newValue)
        // Take the latest timestamp
        if other.Timestamp > v.Timestamp {
            v.Timestamp = other.Timestamp
        }
}
```

**Important**: The merge must be carefully designed to handle the difference between:
- Initial SET operations (use LWW, not accumulate)
- INCRBYFLOAT operations (accumulate deltas)

### Operation Logging for Replication

Each INCRBYFLOAT operation logs the **delta**, not the final value:

```go
op := &proto.Operation{
    Type:      proto.OperationType_INCRBYFLOAT,
    Command:   "INCRBYFLOAT",
    Args:      []string{key, fmt.Sprintf("%.17g", increment)}, // Log delta
    Timestamp: timestamp,
    ReplicaId: replicaID,
}
```

## Test Scenarios

### 1. Basic INCRBYFLOAT on New Key
- `INCRBYFLOAT newkey 2.5`
- Expected: newkey = 2.5
- Verify: Type is float counter

### 2. INCRBYFLOAT on Existing Float Counter
- Setup: key = 5.0 (float counter)
- `INCRBYFLOAT key 2.5`
- Expected: key = 7.5

### 3. Multiple INCRBYFLOAT Accumulation
- `INCRBYFLOAT key 1.1`
- `INCRBYFLOAT key 2.2`
- `INCRBYFLOAT key 3.3`
- Expected: key = 6.6

### 4. INCRBYFLOAT with Negative Value
- Setup: key = 10.0
- `INCRBYFLOAT key -3.5`
- Expected: key = 6.5

### 5. INCRBYFLOAT on Integer Counter
- Setup: key = 5 (integer counter via INCRBY)
- `INCRBYFLOAT key 2.5`
- Expected: key = 7.5, type converted to float counter

### 6. INCRBYFLOAT on String Value
- Setup: SET key "10.5"
- `INCRBYFLOAT key 2.5`
- Expected: key = 13.0, type converted to float counter

### 7. INCRBYFLOAT on Non-Numeric String
- Setup: SET key "hello"
- `INCRBYFLOAT key 2.5`
- Expected: Error "value is not a valid float"

### 8. Concurrent INCRBYFLOAT Merge (Accumulation)
- Replica 1: INCRBYFLOAT key 5.5
- Replica 2: INCRBYFLOAT key 3.3
- After merge: key = 8.8

### 9. SET + INCRBYFLOAT Concurrent (LWW + Accumulation)
- Replica 1: SET key "100"
- Replica 2: INCRBYFLOAT key 5.5
- Expected behavior depends on timing:
  - If SET happens after INCRBYFLOAT is observed: key = 100
  - If concurrent: accumulate delta on top of SET result

### 10. Float Precision Test
- `INCRBYFLOAT key 0.1` repeated 10 times
- Expected: key ≈ 1.0 (within float precision tolerance)

### 11. Large Number Test
- `INCRBYFLOAT key 1e15`
- `INCRBYFLOAT key 1e15`
- Verify: No overflow, correct accumulation

### 12. 59-bit Counter Limit Test
- Try to exceed 59-bit limit
- Expected: Proper handling (either error or cap)

### 13. String LWW Merge Test
- Replica 1: SET key "value1" at t1
- Replica 2: SET key "value2" at t2 (t2 > t1)
- After merge: key = "value2" (LWW)

### 14. Integer Counter Accumulation Test
- Replica 1: INCRBY key 10
- Replica 2: INCRBY key 5
- After merge: key = 15

### 15. TTL with Float Counter
- `INCRBYFLOAT key 5.5`
- `EXPIRE key 10`
- After 10 seconds: key should not exist

## File Locations

- Implementation: `storage/crdt_string.go`, `storage/store.go`
- Unit Tests: `storage/crdt_string_test.go` (new)
- Integration Tests: `string_integration_test.go` (new if needed)
- Server Layer: `server/server.go` (add IncrByFloat method)
- Protocol Layer: `redisprotocol/redis.go` (add incrbyfloat case)

## Implementation Plan

### Phase 1: Core Implementation
1. Add `NewFloatCounterValue` function
2. Add `FloatCounter()` and `SetFloatCounter()` methods to Value
3. Implement `Store.IncrByFloat()` with CRDT semantics
4. Update `Value.Merge()` to handle float counters

### Phase 2: Server Layer
1. Add `Server.IncrByFloat()` method
2. Log INCRBYFLOAT operations for replication
3. Handle INCRBYFLOAT in `applyOperation()`

### Phase 3: Protocol Layer
1. Add `incrbyfloat` case in redis protocol handler
2. Parse float argument and call server method
3. Return formatted float response

### Phase 4: Testing
1. Unit tests for all scenarios above
2. Integration tests for replication
3. Benchmark tests for performance

## Compatibility Notes

- Compatible with existing integer counter operations
- Type conversion from integer to float is one-way
- String to counter conversion follows existing patterns
- TTL behavior unchanged from current implementation
