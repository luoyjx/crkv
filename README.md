# CRDT Redis-Compatible Active-Active Server

## Overview
This project implements a Redis-compatible distributed key-value store using Conflict-free Replicated Data Types (CRDTs) and Raft consensus algorithm.

## Features
- Redis-compatible command interface
- Active-active replication
- Conflict resolution using timestamps
- Distributed consensus with Raft

## Architecture
- Uses Hashicorp Raft for consensus
- Implements CRDT merge strategies
- Supports basic Redis commands (SET, GET)

## Running the Server
```bash
go mod tidy
go run main.go --port 6380 --data ./crdt-redis-data
```

## Configuration
- `--port`: Server listening port (default: 6380)
- `--data`: Data directory for persistent storage

## Design Principles
1. Strong eventual consistency
2. Automatic conflict resolution
3. Horizontal scalability

## Limitations
- Currently supports limited Redis commands
- Experimental implementation

## Future Roadmap
- Support more Redis data types
- Enhanced conflict resolution
- Improved network partition handling

## Dependencies
- Hashicorp Raft
- Redcon (Redis protocol)
- BoltDB for persistent storage

## Contributing
Contributions are welcome! Please open issues or submit pull requests.

## License
MIT License
