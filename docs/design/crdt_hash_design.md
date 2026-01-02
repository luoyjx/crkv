# CRDT Hash Design Document

## Overview

This document describes the design for CRDT Hashes with Active-Active database semantics, focusing on field-level LWW for strings and accumulative counter semantics for HINCRBY/HINCRBYFLOAT operations.

## CRDT Semantics (Based on Active-Active Documentation)

### OR-Set Behavior

Hashes in Active-Active databases use OR-Set (Observed-Remove Set) behavior for field management:
- Writes to add new fields across multiple instances are typically **unioned**
- Conflicting writes (delete field while another adds same field) follow **observed-remove rule**
- Remove can only remove fields it has already seen
- **Add/Update wins** over concurrent delete for unobserved fields

### Field Value Types

Hash fields support two types based on initialization command:
1. **String Type**: Initialized via `HSET` or `HMSET` - uses LWW semantics
2. **Counter Type**: Initialized via `HINCRBY` or `HINCRBYFLOAT` - uses accumulative semantics

## Data Structure Design

### HashField Structure

```go
type FieldType int

const (
    FieldTypeString  FieldType = iota // Regular string field with LWW
    FieldTypeCounter                  // Counter field with accumulative semantics
)

type HashField struct {
    Key          string    // Field key
    Value        string    // String value (for FieldTypeString)
    CounterValue int64     // Counter value (for FieldTypeCounter)
    FieldType    FieldType // Type of field
    ID           string    // Unique field ID
    Timestamp    int64     // Wall clock timestamp
    ReplicaID    string    // Replica that created this field
}
```

### CRDTHash Structure

```go
type CRDTHash struct {
    Fields     map[string]*HashField // field key -> field mapping
    Tombstones map[string]struct{}   // Set of IDs of deleted fields
    ReplicaID  string                // This replica's ID
    nextSeq    int64                 // Sequence number for local operations
}
```

## HINCRBY/HINCRBYFLOAT Counter Semantics

### Behavior Specification

Based on Active-Active documentation:
- Fields can be initialized as counters using `HINCRBY` or `HINCRBYFLOAT`
- Counter values are numeric integers (HINCRBY) or floats (HINCRBYFLOAT)
- Counter fields use **accumulative semantics** during merge

### Type Conversion Rules

1. **String to Counter**: When `HINCRBY` is called on a string field, the field type converts to counter
2. **Counter Preservation**: Once a field is a counter, it remains a counter
3. **Type Conflict in Merge**: Counter type takes precedence over string type

### Merge Algorithm

```
For each field in other hash:
    if field is tombstoned in this hash:
        skip (don't add)
    
    if field doesn't exist locally:
        add field as-is
    else:
        if both are counters:
            accumulate CounterValue
            take latest timestamp
        else if type mismatch:
            counter type wins
        else (both are strings):
            apply LWW (timestamp comparison)
```

## Test Scenarios

### T002: HINCRBY Counter Semantics Unit Tests

#### Test Case 1: Basic HINCRBY on New Field
- **Setup**: Empty hash
- **Operation**: `HINCRBY key field1 10`
- **Expected**: field1 created with CounterValue = 10

#### Test Case 2: HINCRBY on Existing Counter Field
- **Setup**: Hash with field1 (CounterValue = 5)
- **Operation**: `HINCRBY key field1 3`
- **Expected**: field1 CounterValue = 8

#### Test Case 3: Multiple HINCRBY Accumulation
- **Setup**: Hash with field1 (CounterValue = 10)
- **Operations**: 
  - `HINCRBY key field1 5`
  - `HINCRBY key field1 3`
  - `HINCRBY key field1 -2`
- **Expected**: field1 CounterValue = 16

#### Test Case 4: HINCRBY with Negative Value
- **Setup**: Hash with field1 (CounterValue = 10)
- **Operation**: `HINCRBY key field1 -15`
- **Expected**: field1 CounterValue = -5

#### Test Case 5: HINCRBY on String Field (Type Conversion)
- **Setup**: Hash with field1 (String Value = "hello")
- **Operation**: `HINCRBY key field1 5`
- **Expected**: field1 converts to Counter with CounterValue = 5 (or 0 + 5)

#### Test Case 6: Concurrent HINCRBY Merge
- **Setup**: 
  - Replica1: Hash with field1 (Counter = 10)
  - Replica2: Hash with field1 (Counter = 10)
- **Operations**:
  - Replica1: `HINCRBY key field1 5`
  - Replica2: `HINCRBY key field1 3`
- **Merge**: Merge Replica2 into Replica1
- **Expected**: field1 CounterValue = 10 + 5 + 3 = 18

#### Test Case 7: HINCRBYFLOAT Basic
- **Setup**: Empty hash
- **Operation**: `HINCRBYFLOAT key field1 2.5`
- **Expected**: field1 created with float value 2.5

#### Test Case 8: HINCRBYFLOAT Accumulation
- **Setup**: Hash with field1 (float = 10.5)
- **Operations**: 
  - `HINCRBYFLOAT key field1 0.3`
  - `HINCRBYFLOAT key field1 -2.8`
- **Expected**: field1 = 10.5 + 0.3 + (-2.8) = 8.0

#### Test Case 9: String Field LWW Merge
- **Setup**:
  - Replica1: field1 = "value1" at t1
  - Replica2: field1 = "value2" at t2 (t2 > t1)
- **Merge**: Merge Replica2 into Replica1
- **Expected**: field1 = "value2" (LWW)

#### Test Case 10: Mixed Field Types Merge
- **Setup**:
  - Replica1: field1 (String = "hello"), field2 (Counter = 10)
  - Replica2: field1 (String = "world"), field2 (Counter = 10)
- **Operations**:
  - Replica1: HINCRBY field2 5
  - Replica2: HINCRBY field2 3
- **Merge**: Merge Replica2 into Replica1
- **Expected**: 
  - field1 = "world" (LWW - assuming t2 > t1)
  - field2 = 18 (10 + 5 + 3, accumulate)

#### Test Case 11: HDEL + HINCRBY Concurrent
- **Setup**: Both replicas have field1 (Counter = 10)
- **Operations**:
  - Replica1: HDEL key field1
  - Replica2: HINCRBY key field1 5
- **Merge**: Merge Replica2 into Replica1
- **Expected**: Depends on observed-remove semantics (add may win)

#### Test Case 12: Counter Field Get Returns String Representation
- **Setup**: Hash with field1 (CounterValue = 42)
- **Operation**: `HGET key field1`
- **Expected**: Returns "42" as string

#### Test Case 13: HGETALL with Mixed Field Types
- **Setup**: Hash with:
  - field1 (String = "hello")
  - field2 (Counter = 100)
- **Operation**: `HGETALL key`
- **Expected**: Returns {"field1": "hello", "field2": "100"}

## Implementation Details

### HINCRBY Method

```go
func (h *CRDTHash) IncrBy(key string, delta int64, timestamp int64, replicaID string) (int64, error) {
    // 1. If field doesn't exist, create counter field with delta as value
    // 2. If field exists as string, convert to counter (start from 0)
    // 3. Accumulate delta to CounterValue
    // 4. Update timestamp and metadata
}
```

### HINCRBYFLOAT Method

```go
func (h *CRDTHash) IncrByFloat(key string, delta float64, timestamp int64, replicaID string) (float64, error) {
    // Similar to IncrBy but handles float values
    // May use scaled integer storage (multiply by 1e6) for precision
}
```

### Merge Method

```go
func (h *CRDTHash) Merge(other *CRDTHash) {
    // 1. Check tombstones, skip tombstoned fields
    // 2. For each field in other:
    //    - If not exists locally: add as-is
    //    - If both counters: accumulate values
    //    - If type mismatch: counter wins
    //    - If both strings: LWW
    // 3. Merge tombstone sets
}
```

## File Locations

- Implementation: `storage/crdt_hash.go`
- Unit Tests: `storage/crdt_hash_test.go`
- Integration Tests: `hash_integration_test.go`

## Example from Active-Active Documentation

### Add Wins Case

```
| Time | Instance 1              | Instance 2              |
|------|-------------------------|-------------------------|
| t1   | HSET key1 field1 "a"    |                         |
| t2   |                         | HSET key1 field2 "b"    |
| t4   | — Sync —                | — Sync —                |
| t5   | HGETALL key1            | HGETALL key1            |
|      | field2="b", field1="a"  | field2="b", field1="a"  |
```

Both fields are preserved after sync (union of fields).

## References

- [Active-Active Hashes Documentation](../active_active/data_types/3_hashs.md)
- [Redis HINCRBY Command](https://redis.io/commands/hincrby/)
- [Redis HINCRBYFLOAT Command](https://redis.io/commands/hincrbyfloat/)
