# CRDT-based Redis Proxy Server

## Objective

Develop a Redis Proxy Server based on CRDT technology, implementing an Active-Active architecture to provide highly available and low-latency Redis services.

## Core Features

- **Redis Protocol Compatibility:** Fully compatible with Redis client protocol, allowing seamless connection of existing Redis clients.
- **Proximity Access:** Clients can connect to the geographically nearest Proxy Server to reduce access latency.
- **Active-Active Architecture:** Multiple Proxy Server instances provide service simultaneously, with data synchronized between different instances through CRDT.
- **CRDT Data Replication:** Using CRDT algorithms to resolve concurrent modification conflicts of the same key across different nodes.
- **Multi-Master Replication:** All Proxy Server instances are writable, with modifications on any instance synchronized to others.

## Module Division

1. **Network Layer:**
    - Responsible for receiving client connections.
    - Handles network communication between Proxy Servers.
    - Consider using the `net` package for TCP connections and selecting appropriate protocols for inter-Proxy Server communication.

2. **Redis Protocol Layer:**
    - Parses Redis commands sent by clients.
    - Forwards commands to the CRDT engine.
    - Serializes CRDT engine responses into Redis protocol format for client return.
    - Can use the `github.com/gomodule/redcon` library to simplify Redis protocol handling.

3. **CRDT Engine Layer:**
    - Maintains CRDT states for different Redis data types.
    - Implements various CRDT algorithms for handling concurrent modifications of different data types.
    - Provides APIs for the protocol layer to execute CRDT operations.
    - Needs to consider how to store CRDT states, using either memory storage or persistent storage.

4. **Replication Layer:**
    - Responsible for CRDT data synchronization between Proxy Servers.
    - Select appropriate network protocols for communication, such as gRPC or custom Protocol Buffers-based protocols.
    - Implement data broadcasting or Gossip protocol for synchronization.

5. **Routing Layer:**
    - Responsible for routing client requests to appropriate local Proxy Servers.
    - Can route based on client IP addresses or configured geographical location information.
    - Needs to consider service discovery mechanisms for clients to find available Proxy Servers.

## Iteration Plan

### Phase 1: Basic Architecture and Core Command Implementation (Set/Get - LWW)

- **Goal:** Set up basic Proxy Server framework, implement SET and GET commands using Last Write Wins (LWW) CRDT strategy.
- **Tasks:**
    - Set up network layer to accept client connections.
    - Integrate `redcon` library for Redis protocol parsing and serialization.
    - Implement simple CRDT engine with memory storage, supporting LWW strategy for SET and GET operations.
    - Implement basic inter-Proxy Server connection and data synchronization (e.g., simple broadcast).
    - Implement simple routing layer for client connections to specified Proxy Servers.
- **Technology Stack:**
    - `net` package for network connections.
    - `github.com/gomodule/redcon` for Redis protocol handling.
    - `map[string]string` as simple CRDT storage for LWW.
    - `net/rpc` or `net/http` for basic inter-Proxy Server communication.

### Phase 2: Extended Command Support and CRDT Algorithm Implementation

- **Goal:** Extend support for more Redis commands and implement appropriate CRDT algorithms for different data types.
- **Tasks:**
    - Support common commands like INCR, DEL, etc.
    - Implement corresponding CRDT algorithms for different Redis data types (e.g., List, Set, Hash) using Grow-only Counter, Add-wins Set, Observed-Remove Map.
    - Optimize CRDT engine to handle more complex data structures and operations.
    - Improve inter-Proxy Server data synchronization using operation-based replication or state vectors.
- **Technology Stack:**
    - Research and implement various CRDT algorithms.
    - Consider using more efficient serialization libraries like `gogo/protobuf`.

### Phase 3: Proximity Access and Service Discovery

- **Goal:** Implement client proximity connection to Proxy Servers and introduce service discovery mechanisms.
- **Tasks:**
    - Implement routing logic based on client IP or configured geographical location information.
    - Introduce service discovery mechanisms using Consul, Etcd, or ZooKeeper for clients to dynamically discover available Proxy Servers.
    - Consider using load balancers to distribute client requests.
- **Technology Stack:**
    - Service discovery tools like Consul, Etcd, ZooKeeper.
    - Load balancers (e.g., Nginx, HAProxy).

### Phase 4: Stability and Performance Optimization

- **Goal:** Improve system stability and performance.
- **Tasks:**
    - Conduct comprehensive unit tests and integration tests.
    - Perform performance testing and bottleneck analysis.
    - Optimize CRDT algorithms and data synchronization mechanisms.
    - Add monitoring and logging functionality.
    - Consider persisting CRDT data using BoltDB or RocksDB.
- **Technology Stack:**
    - `go test` for unit testing.
    - `pprof` for performance analysis.
    - Prometheus, Grafana for monitoring.
    - BoltDB, RocksDB as embedded databases.

## Future Extensions

- Support more Redis commands and data types.
- Implement more advanced conflict resolution strategies.
- Improve network partition fault tolerance.
- Provide management and monitoring interfaces.