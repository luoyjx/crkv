Title: CRDT Redis Server Spec

Overview
- Goal: Build a Redis-compatible active-active proxy server that applies commands to local CRDT state and replicates operations as conflict-free changes to peer nodes.
- Scope: Strings (LWW registers), Counters (PN/accumulative), Lists (RGA), Sets (OR-Set), Hashes (LWW/Counter fields), Sorted Sets.

Top-level Modules
1) Redis Protocol Gateway (`redisprotocol`)
   - Responsibilities: accept TCP connections, parse RESP, map to server APIs, return RESP replies.
   - Submodules:
     - Command parsers (e.g., `commands/set.go`) with option validation.
     - Dispatcher to server interface.

2) CRDT Engine and Storage (`storage`)
   - Responsibilities: maintain local CRDT state, persist to disk, mirror to local Redis for fast read/write.
   - **Critical Rule:** Store methods must accept `Timestamp` and `ReplicaID` from the caller. For local ops, generate new; for replication ops, preserve original.
   - Data types:
     - **String:** LWW register (Last-Write-Wins) by origin timestamp + replica ID tie-breaker.
     - **Counter:** Operation-based accumulate semantics.
     - **List:** **RGA (Replicated Growable Array)** to ensure correct ordering of concurrent inserts.
     - **Set:** OR-Set (Observed-Remove) with unique element IDs.
     - **Hash:** Field-level LWW or Counter semantics.
     - **Sorted Set:** OR-Set for members, LWW/Counter for scores.
   - Submodules:
     - `Store`: in-memory map, persistence (`store.json`), **Garbage Collection (GC)** for tombstones.
     - `RedisStore`: thin adapter to `go-redis`.

3) Operation Log (`operation`)
   - Responsibilities: append-only log of user operations (SET, INCR, DEL, ...), durable on disk, query by timestamp for replication.
   - Messages: protobuf `Operation` and `OperationBatch`.

4) Replication/Syncer (`syncer`)
   - Responsibilities: ship new operations to peers; receive operations and apply via CRDT to local store.
   - Transport: HTTP/gRPC.
   - **Conflict Handling:**
     - Operations are applied idempotently.
     - **Timestamps:** The `Store` MUST use the operation's `Timestamp` and `ReplicaID` when applying replicated updates, NOT the local wall clock.
   - Submodules:
     - Outbound publisher: tail `OperationLog`.
     - Inbound handler: validate, dedupe, apply to `Store`.

5) Server Orchestrator (`server`)
   - Responsibilities: expose typed command APIs to protocol layer, log operations, apply to store, handle peer-applied operations via syncer.
   - Interfaces:
     - Set/Get/Incr, and `HandleOperation(ctx, *proto.Operation)` for inbound replication.

Data and Conflict Semantics
- **Strings:** LWW (Timestamp > ReplicaID).
- **Counters:** Accumulate deltas.
- **Lists:** RGA (Interleaving based on anchor and origin ID).
- **Sets/Hashes/ZSets:** Add-wins / Observed-Remove.
- **Tombstones:** Deleted elements are marked as tombstones and eventually removed by GC.

Persistence
- CRDT state persisted as `store.json` (or segmented files).
- Operation log stored as append-only segment files.

Garbage Collection (GC)
- Periodic process to remove "dead" tombstones.
- Policy: Time-based (e.g., keep tombstones for 1 hour) or Causal (Vector Clock stability).

Testing Strategy
- **Unit:** Storage semantics (LWW, RGA, GC), operation log.
- **Integration:** Multi-server replication, conflict resolution verification.
- **Regression:** `go test ./...` on every commit.
