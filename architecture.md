# CRDT Redis Architecture

This document describes the architecture of the CRDT Redis implementation.

## System Architecture Diagram

```mermaid
graph TB
    %% Define Client nodes
    Client1[Redis Client 1]
    Client2[Redis Client 2]

    %% Define Instance 1 components
    subgraph Instance 1
        subgraph Network Layer 1
            RP1[Redis Protocol Handler]
        end

        subgraph Application Layer 1
            Server1[CRDT Server]
            OpLog1[Operation Log]
        end

        subgraph Storage Layer 1
            Store1[CRDT Store]
            Redis1[(Redis)]
            Disk1[(Disk Storage)]
        end

        subgraph Consensus Layer 1
            Raft1[Raft Node]
        end
    end

    %% Define Instance 2 components
    subgraph Instance 2
        subgraph Network Layer 2
            RP2[Redis Protocol Handler]
        end

        subgraph Application Layer 2
            Server2[CRDT Server]
            OpLog2[Operation Log]
        end

        subgraph Storage Layer 2
            Store2[CRDT Store]
            Redis2[(Redis)]
            Disk2[(Disk Storage)]
        end

        subgraph Consensus Layer 2
            Raft2[Raft Node]
        end
    end

    %% Client connections
    Client1 -->|Redis Protocol| RP1
    Client2 -->|Redis Protocol| RP2
    
    %% Instance 1 internal connections
    RP1 -->|Commands| Server1
    Server1 -->|Writes| OpLog1
    Server1 -->|Reads/Writes| Store1
    Store1 -->|Persistence| Disk1
    Store1 -->|Cache| Redis1
    OpLog1 -->|Persistence| Disk1
    Server1 <-->|State| Raft1

    %% Instance 2 internal connections
    RP2 -->|Commands| Server2
    Server2 -->|Writes| OpLog2
    Server2 -->|Reads/Writes| Store2
    Store2 -->|Persistence| Disk2
    Store2 -->|Cache| Redis2
    OpLog2 -->|Persistence| Disk2
    Server2 <-->|State| Raft2

    %% Inter-instance connections
    Raft1 <-->|Consensus Protocol| Raft2
    OpLog1 <-.->|Operation Sync| OpLog2
    Server1 <-.->|CRDT Merge| Server2

    %% Styling
    classDef primary fill:#f9f,stroke:#333,stroke-width:2px
    classDef secondary fill:#bbf,stroke:#333,stroke-width:1px
    classDef storage fill:#dfd,stroke:#333,stroke-width:1px
    classDef client fill:#ffd,stroke:#333,stroke-width:1px
    
    class Server1,Server2,Store1,Store2 primary
    class OpLog1,OpLog2,RP1,RP2,Raft1,Raft2 secondary
    class Redis1,Redis2,Disk1,Disk2 storage
    class Client1,Client2 client
```

## Component Description

### Client Layer
- **Redis Client**: Standard Redis clients can connect to the system using the Redis protocol

### Network Layer
- **Redis Protocol Handler**: Handles Redis protocol communication and command parsing
- **Redis Server**: Manages client connections and routes commands to the CRDT server

### Application Layer
- **CRDT Server**: Core server component that handles command processing and CRDT operations
- **Operation Log**: Maintains a log of all operations for CRDT convergence and replication

### Storage Layer
- **CRDT Store**: Implements CRDT logic and manages data storage
- **Redis**: Used as a fast cache layer
- **Disk Storage**: Persistent storage for both CRDT state and operation logs

### Consensus Layer
- **Raft Consensus**: Ensures consistent replication across nodes in the cluster

## Key Features

1. **CRDT Implementation**
   - Conflict-free replicated data types
   - Automatic conflict resolution
   - Eventually consistent

2. **Hybrid Storage**
   - In-memory state
   - Redis cache layer
   - Persistent disk storage

3. **Distributed Consensus**
   - Raft-based replication
   - Operation log synchronization
   - Leader election

4. **Redis Compatibility**
   - Redis protocol support
   - Standard Redis client support
   - Familiar Redis commands

## Data Flow

1. Clients connect using standard Redis protocol
2. Commands are processed by the CRDT server
3. Operations are logged and replicated via Raft
4. Data is stored in memory, Redis cache, and disk
5. Changes are propagated to other nodes through operation log sync

## Consistency Model

The system provides eventual consistency through CRDT operations:
- All replicas eventually converge to the same state
- Conflicts are automatically resolved using CRDT rules
- Operations are timestamped and replicated across nodes
