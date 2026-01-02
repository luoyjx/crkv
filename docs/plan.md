# CRDT Redis - Active-Active CRDB Plan (Revamp & Stabilization)

## Ultimate Goal
Build a robust, correct, and self-cleaning Active-Active Redis CRDB that supports multi-master replication with strong eventual consistency.

## Gap Analysis (Current Status)
1.  **Replication Correctness (Critical):** Current `Store` methods (e.g., `Set`, `LPush`) generate a new local timestamp (`time.Now()`) when applying operations. This is incorrect for replication, where the **original** timestamp and replica ID must be preserved to ensure convergence (LWW).
2.  **List CRDT (Critical):** The current `CRDTList` merge logic is append-only. Concurrent insertions at the same index will result in inconsistent or non-interleaved orders. It requires an **RGA (Replicated Growable Array)** implementation.
3.  **Garbage Collection (Robustness):** Deleted elements (Tombstones) are never removed, leading to unbounded memory growth. A GC mechanism based on stable vector clocks or time thresholds is needed.
4.  **Testing:** Lack of specific tests for concurrent interleaving and timestamp preservation.

## Implementation Steps

### Phase 0: Foundations & Standards
1.  **Create Cursor Rules:** Enforce TDD, replication correctness, and CRDT principles (`.cursor/rules/crdt-standards.mdc`). **(Done)**
2.  **Stabilize Baseline:** Fix currently failing `TestServerInterServerSync`. **(Done)**

### Phase 1: Fix Replication & Timestamp Propagation (Critical)
1.  **Test:** Create unit tests in `store_test.go` asserting that `Store` methods respect explicit `Timestamp` and `ReplicaID` options. **(Done)**
2.  **Refactor Store:** Update `storage/store.go` interfaces to accept `Context` or `Options` containing metadata. **(Done)**
3.  **Update Server:** Modify `server/server.go`'s `applyOperation` to pass the operation's original timestamp/ID to the store. **(Done)**
4.  **Verify:** Ensure all replication tests pass and data converges correctly. **(Done)**

### Phase 2: Fix List CRDT (Critical)
1.  **Test:** Write unit tests in `crdt_list_test.go` simulating concurrent inserts (e.g., User A inserts "A" at 0, User B inserts "B" at 0) to verify deterministic ordering (RGA). **(Done)**
2.  **Implement RGA:** Rewrite `CRDTList` to use a linked-list-like structure (or logical RGA) where elements are ordered by `(origin_timestamp, origin_replica_id)`. **(Done)**
3.  **Merge Logic:** Update `Merge` to respect RGA ordering rules. **(Done)**

### Phase 3: Garbage Collection (Robustness)
1.  **Test:** Create tests for GC (add -> delete -> GC -> verify removal). **(Done)**
2.  **Implement GC:** Add a periodic cleaner in `Store` that purges tombstones safe to remove (e.g., older than `clean_interval` or causally stable). **(Done)**

### Phase 4: Verification & Optimization
1.  **Chaos Testing:** Simulate network partitions and concurrent conflicting writes.
2.  **Optimization:** Optimize `syncer` payload if necessary.

## Task List
(See `docs/todos.md` for granular tracking)
