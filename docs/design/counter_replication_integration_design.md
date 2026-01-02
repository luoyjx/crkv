# Counter Replication Integration Test Design

## Overview

This document describes the design for integration tests that verify CRDT counter replication between multiple server instances. The tests validate that ZINCRBY and HINCRBY operations correctly replicate and converge using counter accumulation semantics.

## Test Environment

### Multi-Server Setup

```
┌─────────────────┐     ┌─────────────────┐
│    Server 1     │     │    Server 2     │
│  (replica1)     │     │  (replica2)     │
│                 │◄───►│                 │
│  OperationLog   │     │  OperationLog   │
└─────────────────┘     └─────────────────┘
         │                       │
         └───────────┬───────────┘
                     │
              (Sync Operations)
```

### Test Configuration

- Two or more server instances with separate data directories
- Each server has unique replica ID
- Operations are synced via OperationLog exchange

## Test Scenarios

### T003-1: ZINCRBY Replication Basic

**Description**: Verify ZINCRBY operations replicate correctly.

```
| Time | Server1                | Server2                |
|------|------------------------|------------------------|
| t1   | ZINCRBY z x 5.0        |                        |
| t2   | — Sync —               | — Sync —               |
| t3   | ZSCORE z x => 5.0      | ZSCORE z x => 5.0      |
```

**Expected**: Both servers have member x with score 5.0.

### T003-2: ZINCRBY Concurrent Counter Accumulation

**Description**: Verify concurrent ZINCRBY operations accumulate.

```
| Time | Server1                | Server2                |
|------|------------------------|------------------------|
| t1   | ZADD z 1.0 x           |                        |
| t2   | — Sync —               | — Sync —               |
| t3   | ZINCRBY z x 1.0        | ZINCRBY z x 1.0        |
| t4   | — Sync —               | — Sync —               |
| t5   | ZSCORE z x => 3.0      | ZSCORE z x => 3.0      |
```

**Expected**: Both servers converge to score 3.0 (1.0 + 1.0 + 1.0).

### T003-3: HINCRBY Replication Basic

**Description**: Verify HINCRBY operations replicate correctly.

```
| Time | Server1                | Server2                |
|------|------------------------|------------------------|
| t1   | HINCRBY h f 10         |                        |
| t2   | — Sync —               | — Sync —               |
| t3   | HGET h f => "10"       | HGET h f => "10"       |
```

**Expected**: Both servers have field f with value 10.

### T003-4: HINCRBY Concurrent Counter Accumulation

**Description**: Verify concurrent HINCRBY operations accumulate.

```
| Time | Server1                | Server2                |
|------|------------------------|------------------------|
| t1   | HINCRBY h f 100        | HINCRBY h f 200        |
| t2   | — Sync —               | — Sync —               |
| t3   | HGET h f => 300        | HGET h f => 300        |
```

**Expected**: Both servers converge to counter value 300 (100 + 200).

### T003-5: Mixed ZADD/ZINCRBY Replication

**Description**: Verify ZADD (LWW) and ZINCRBY (counter) work together.

```
| Time | Server1                | Server2                |
|------|------------------------|------------------------|
| t1   | ZADD z 10 x            |                        |
| t2   |                        | ZADD z 20 x            |
| t3   | ZINCRBY z x 5          | ZINCRBY z x 3          |
| t4   | — Sync —               | — Sync —               |
| t5   | ZSCORE z x => 28       | ZSCORE z x => 28       |
```

**Expected**: Base score 20 (LWW) + deltas (5 + 3) = 28.

### T003-6: ZREM + ZINCRBY Concurrent Replication

**Description**: Verify observed-remove semantics with ZINCRBY.

```
| Time | Server1                | Server2                |
|------|------------------------|------------------------|
| t1   | ZADD z 10 x            |                        |
| t2   | — Sync —               | — Sync —               |
| t3   | ZREM z x               | ZINCRBY z x 5          |
| t4   | — Sync —               | — Sync —               |
| t5   | ZSCORE z x => ?        | ZSCORE z x => ?        |
```

**Expected**: Element survives with ZINCRBY delta preserved (observed-remove semantics).

### T003-7: HDEL + HINCRBY Concurrent Replication

**Description**: Verify observed-remove semantics with HINCRBY.

```
| Time | Server1                | Server2                |
|------|------------------------|------------------------|
| t1   | HSET h f "value"       |                        |
| t2   | — Sync —               | — Sync —               |
| t3   | HDEL h f               | HINCRBY h f 100        |
| t4   | — Sync —               | — Sync —               |
| t5   | HGET h f => ?          | HGET h f => ?          |
```

**Expected**: Field survives with HINCRBY value (add wins over observed-remove).

### T003-8: Large Scale Counter Operations

**Description**: Verify many concurrent counter operations accumulate correctly.

- Server1: 100 ZINCRBY operations (+1 each)
- Server2: 100 ZINCRBY operations (+1 each)
- After sync: Both have score 200

### T003-9: Eventual Consistency Check

**Description**: Verify eventual consistency after multiple sync rounds.

- Perform operations on both servers
- Sync partially
- Perform more operations
- Sync fully
- Verify both servers have identical state

## Implementation Details

### Test Helper Functions

```go
// syncServers synchronizes operations between two servers
func syncServers(srv1, srv2 *server.Server) {
    ops1, _ := srv1.OpLog().GetOperations(0)
    ops2, _ := srv2.OpLog().GetOperations(0)
    
    for _, op := range ops1 {
        srv2.HandleOperation(nil, op)
    }
    for _, op := range ops2 {
        srv1.HandleOperation(nil, op)
    }
}

// assertZScoreEqual asserts both servers have same score for member
func assertZScoreEqual(t *testing.T, srv1, srv2 *server.Server, key, member string) {
    score1, _, _ := srv1.ZScore(key, member)
    score2, _, _ := srv2.ZScore(key, member)
    
    if *score1 != *score2 {
        t.Errorf("Score mismatch: srv1=%f, srv2=%f", *score1, *score2)
    }
}

// assertHGetEqual asserts both servers have same value for field
func assertHGetEqual(t *testing.T, srv1, srv2 *server.Server, key, field string) {
    val1, _, _ := srv1.HGet(key, field)
    val2, _, _ := srv2.HGet(key, field)
    
    if val1 != val2 {
        t.Errorf("Value mismatch: srv1=%s, srv2=%s", val1, val2)
    }
}
```

### File Location

- Integration Tests: `counter_replication_test.go`

## References

- [ZINCRBY Design](./crdt_zset_design.md)
- [HINCRBY Design](./crdt_hash_design.md)
- [Active-Active Sorted Sets](../active_active/data_types/5_sorted-sets.md)
- [Active-Active Hashes](../active_active/data_types/3_hashs.md)
