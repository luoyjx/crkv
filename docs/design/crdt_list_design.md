# CRDT List Design Document

## Overview

This document describes the design for CRDT List commands with Active-Active database semantics, focusing on:
1. LINDEX - Get element by index
2. LSET - Set element at index
3. LINSERT - Insert before/after pivot (key for concurrent insertions)
4. LTRIM - Trim list to range
5. LREM - Remove elements by value

## CRDT Semantics (Based on Active-Active Documentation)

### Core Principles

1. **Unique Element IDs**: Each element has a unique ID (timestamp-replicaID-seq) for conflict resolution.

2. **Tombstone-based Deletion**: Elements are marked as deleted (soft delete) rather than physically removed, enabling CRDT merge.

3. **Observed-Remove Semantics**: DEL only deletes elements that were observed at the time of deletion. New elements added concurrently survive.

4. **At-Least-Once POP**: Concurrent POP operations may return the same element. Each element is guaranteed to be POPped at least once, but not exactly once.

### Concurrent Insertion Behavior (from Documentation)

Per the Active-Active documentation example at t3-t7:

```
t1: Instance 1: LPUSH L x
t2: — Sync —
t3: Instance 1: LINSERT L AFTER x y1
t4: Instance 2: LINSERT L AFTER x y2
t5: Instance 1: LRANGE L 0 -1 => x y1
    Instance 2: LRANGE L 0 -1 => x y2
t6: — Sync —
t7: Both: LRANGE L 0 -1 => x y1 y2
```

**Key insight**: When two instances concurrently insert after the same pivot:
- Both insertions are preserved
- Order is resolved arbitrarily but consistently across instances
- Resolution typically uses element ID (timestamp-replicaID-seq) for ordering

## Data Structure

### Current ListElement Structure

```go
type ListElement struct {
    Value     string `json:"value"`
    ID        string `json:"id"`        // Unique: timestamp-replicaID-seq
    Timestamp int64  `json:"timestamp"` // Creation timestamp
    ReplicaID string `json:"replica_id"`
    Deleted   bool   `json:"deleted"`   // Tombstone marker
}
```

### Enhanced Structure for LINSERT

To support LINSERT with concurrent insertion resolution, we need to track the "parent" element:

```go
type ListElement struct {
    Value       string `json:"value"`
    ID          string `json:"id"`
    Timestamp   int64  `json:"timestamp"`
    ReplicaID   string `json:"replica_id"`
    Deleted     bool   `json:"deleted"`
    ParentID    string `json:"parent_id,omitempty"`    // ID of pivot element (for LINSERT)
    InsertAfter bool   `json:"insert_after,omitempty"` // true=AFTER, false=BEFORE
}
```

## Command Specifications

### 1. LINDEX - Get Element by Index

**Syntax**: `LINDEX key index`

**Behavior**:
- Returns the element at the specified index
- Index is 0-based
- Negative indices count from the end (-1 = last element)
- Returns nil if index is out of range
- Only considers visible (non-deleted) elements

**Implementation**:
```go
func (list *CRDTList) Index(index int) (string, bool) {
    visible := list.VisibleElements()
    length := len(visible)
    
    // Handle negative index
    if index < 0 {
        index = length + index
    }
    
    // Bounds check
    if index < 0 || index >= length {
        return "", false
    }
    
    return visible[index].Value, true
}
```

**CRDT Notes**: No special CRDT handling needed - read-only operation.

### 2. LSET - Set Element at Index

**Syntax**: `LSET key index value`

**Behavior**:
- Sets the element at index to the new value
- Returns error if index is out of range
- Index is 0-based, negative indices supported

**Implementation**:
```go
func (list *CRDTList) Set(index int, value string, timestamp int64, replicaID string) error {
    visible := list.VisibleElements()
    length := len(visible)
    
    // Handle negative index
    if index < 0 {
        index = length + index
    }
    
    // Bounds check
    if index < 0 || index >= length {
        return fmt.Errorf("ERR index out of range")
    }
    
    // Find and update the element
    targetID := visible[index].ID
    for i := range list.Elements {
        if list.Elements[i].ID == targetID && !list.Elements[i].Deleted {
            list.Elements[i].Value = value
            list.Elements[i].Timestamp = timestamp
            return nil
        }
    }
    return fmt.Errorf("ERR element not found")
}
```

**CRDT Notes**: Uses LWW (Last-Write-Wins) for concurrent updates to the same element. The element ID remains the same, only the value is updated.

### 3. LINSERT - Insert Before/After Pivot

**Syntax**: `LINSERT key BEFORE|AFTER pivot value`

**Behavior**:
- Inserts value before or after the first occurrence of pivot
- Returns the list length after insert, or -1 if pivot not found
- Returns 0 if key doesn't exist or list is empty

**Implementation**:
```go
func (list *CRDTList) Insert(pivot string, value string, insertAfter bool, 
                              timestamp int64, replicaID string) int {
    visible := list.VisibleElements()
    
    // Find pivot element
    pivotIndex := -1
    var pivotElem *ListElement
    for i, elem := range visible {
        if elem.Value == pivot {
            pivotIndex = i
            pivotElem = &visible[i]
            break
        }
    }
    
    if pivotIndex == -1 {
        return -1 // Pivot not found
    }
    
    // Create new element with parent tracking
    elementID := generateElementID(timestamp, replicaID, list.NextSeq)
    list.NextSeq++
    
    newElem := ListElement{
        Value:       value,
        ID:          elementID,
        Timestamp:   timestamp,
        ReplicaID:   replicaID,
        Deleted:     false,
        ParentID:    pivotElem.ID,
        InsertAfter: insertAfter,
    }
    
    // Find actual position in Elements slice
    actualPivotIndex := -1
    for i, elem := range list.Elements {
        if elem.ID == pivotElem.ID {
            actualPivotIndex = i
            break
        }
    }
    
    // Insert at correct position
    insertPos := actualPivotIndex
    if insertAfter {
        insertPos++
    }
    
    // Insert into slice
    list.Elements = append(list.Elements[:insertPos], 
                          append([]ListElement{newElem}, list.Elements[insertPos:]...)...)
    
    return list.Len()
}
```

**CRDT Merge for Concurrent LINSERT**:

When merging lists with concurrent insertions after the same pivot:

```go
func (list *CRDTList) Merge(other *CRDTList) {
    // ... existing merge logic ...
    
    // For elements with same ParentID, order by element ID
    // This ensures consistent ordering across all replicas
    sort.SliceStable(list.Elements, func(i, j int) bool {
        // Elements with same parent are ordered by their ID
        if list.Elements[i].ParentID == list.Elements[j].ParentID &&
           list.Elements[i].ParentID != "" {
            return list.Elements[i].ID < list.Elements[j].ID
        }
        // Otherwise maintain original order
        return false
    })
}
```

### 4. LTRIM - Trim List to Range

**Syntax**: `LTRIM key start stop`

**Behavior**:
- Trims the list to contain only elements in the specified range
- Elements outside the range are marked as deleted
- Indices work like LRANGE (0-based, negative supported)

**Implementation**:
```go
func (list *CRDTList) Trim(start, stop int) {
    visible := list.VisibleElements()
    length := len(visible)
    
    // Handle negative indices
    if start < 0 {
        start = length + start
    }
    if stop < 0 {
        stop = length + stop
    }
    
    // Bounds adjustment
    if start < 0 {
        start = 0
    }
    if stop >= length {
        stop = length - 1
    }
    
    // Build set of IDs to keep
    keepIDs := make(map[string]bool)
    for i := start; i <= stop && i < length; i++ {
        keepIDs[visible[i].ID] = true
    }
    
    // Mark elements outside range as deleted
    for i := range list.Elements {
        if !list.Elements[i].Deleted && !keepIDs[list.Elements[i].ID] {
            list.Elements[i].Deleted = true
        }
    }
}
```

**CRDT Notes**: Uses tombstone marking. Concurrent LTRIM operations may have different visible elements, but after merge, deletion state is union (any delete wins).

### 5. LREM - Remove Elements by Value

**Syntax**: `LREM key count value`

**Behavior**:
- `count > 0`: Remove first `count` occurrences from head to tail
- `count < 0`: Remove first `|count|` occurrences from tail to head
- `count = 0`: Remove all occurrences
- Returns the number of removed elements

**Implementation**:
```go
func (list *CRDTList) Rem(count int, value string) int {
    visible := list.VisibleElements()
    removed := 0
    
    // Determine direction and max removals
    fromHead := count >= 0
    maxRemove := count
    if count < 0 {
        maxRemove = -count
    }
    if count == 0 {
        maxRemove = len(visible) // Remove all
    }
    
    // Find elements to remove
    var toRemove []string
    if fromHead {
        for _, elem := range visible {
            if elem.Value == value && len(toRemove) < maxRemove {
                toRemove = append(toRemove, elem.ID)
            }
        }
    } else {
        // From tail
        for i := len(visible) - 1; i >= 0 && len(toRemove) < maxRemove; i-- {
            if visible[i].Value == value {
                toRemove = append(toRemove, visible[i].ID)
            }
        }
    }
    
    // Mark as deleted
    removeSet := make(map[string]bool)
    for _, id := range toRemove {
        removeSet[id] = true
    }
    
    for i := range list.Elements {
        if removeSet[list.Elements[i].ID] && !list.Elements[i].Deleted {
            list.Elements[i].Deleted = true
            removed++
        }
    }
    
    return removed
}
```

**CRDT Notes**: Uses tombstone marking. Concurrent LREM operations targeting the same elements will both mark them as deleted (idempotent).

## Test Scenarios

### LINDEX Tests

1. **Basic positive index**: `LINDEX list 0` - Get first element
2. **Basic negative index**: `LINDEX list -1` - Get last element
3. **Middle element**: `LINDEX list 2` - Get element at index 2
4. **Out of bounds positive**: `LINDEX list 100` - Return nil
5. **Out of bounds negative**: `LINDEX list -100` - Return nil
6. **Empty list**: `LINDEX emptylist 0` - Return nil
7. **With deleted elements**: Elements marked as deleted should not be counted

### LSET Tests

1. **Basic set**: `LSET list 0 "new"` - Update first element
2. **Negative index**: `LSET list -1 "new"` - Update last element
3. **Out of bounds**: `LSET list 100 "new"` - Return error
4. **Empty list**: `LSET emptylist 0 "new"` - Return error
5. **Concurrent LSET (LWW)**: Two replicas set same index, newer wins
6. **LSET on deleted position**: Handle gracefully

### LINSERT Tests

1. **Insert AFTER pivot**: `LINSERT list AFTER "x" "y"` - y appears after x
2. **Insert BEFORE pivot**: `LINSERT list BEFORE "x" "y"` - y appears before x
3. **Pivot not found**: `LINSERT list AFTER "notexist" "y"` - Return -1
4. **Multiple same value**: Insert after first occurrence only
5. **Concurrent LINSERT same pivot** (Critical):
   - Replica 1: `LINSERT list AFTER "x" "y1"`
   - Replica 2: `LINSERT list AFTER "x" "y2"`
   - After merge: list = [x, y1, y2] or [x, y2, y1] (consistent across replicas)
6. **LINSERT on deleted pivot**: Handle gracefully

### LTRIM Tests

1. **Basic trim**: `LTRIM list 0 2` - Keep first 3 elements
2. **Negative indices**: `LTRIM list -3 -1` - Keep last 3 elements
3. **Empty result**: `LTRIM list 5 10` (list has 3 elements) - Empty list
4. **Keep all**: `LTRIM list 0 -1` - No change
5. **Concurrent LTRIM**: Both replicas trim, union of deletions
6. **LTRIM on empty list**: No error

### LREM Tests

1. **Remove all (count=0)**: `LREM list 0 "x"` - Remove all "x"
2. **Remove from head (count>0)**: `LREM list 2 "x"` - Remove first 2 "x"
3. **Remove from tail (count<0)**: `LREM list -2 "x"` - Remove last 2 "x"
4. **Value not found**: `LREM list 0 "notexist"` - Return 0
5. **Concurrent LREM**: Both replicas remove same elements (idempotent)
6. **LREM on empty list**: Return 0

### Integration/Merge Tests

1. **Concurrent insertion preservation**: Per documentation example
2. **DEL + PUSH concurrent**: DEL only removes observed elements
3. **LTRIM + LPUSH concurrent**: New elements may survive trim
4. **Merge ordering consistency**: Same order across all replicas

## File Locations

- Implementation: `storage/crdt_list.go`
- Unit Tests: `storage/crdt_list_test.go`
- Store Methods: `storage/store.go` (add new methods)
- Server Methods: `server/server.go` (add new handlers)
- Protocol: `redisprotocol/redis.go` (add new commands)

## Implementation Priority

1. **LINDEX** - Simplest, read-only, commonly used
2. **LINSERT** - Critical for CRDT semantics (documented behavior)
3. **LREM** - Common use case
4. **LSET** - Less common but useful
5. **LTRIM** - Memory management

## Notes

- All modifications use tombstone marking for CRDT compatibility
- Element ordering after concurrent insertions is deterministic via ID comparison
- The "at-least-once" POP behavior is a documented limitation, not a bug
- Blocking operations (BLPOP, etc.) are lower priority and may not need CRDT semantics
