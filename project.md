.  // Root directory of the CRDT Redis project
├── server/  // Main CRDT Redis server implementation and tests
│   ├── server.go  // Core server logic for CRDT Redis
│   └── server_test.go  // Unit and integration tests for server
├── storage/  // Persistent storage and CRDT logic
│   ├── store.go  // Persistent store with CRDT resolution
│   ├── store_test.go  // Tests for persistent store
│   ├── redis_client.go  // Redis client wrapper with CRDT support
│   ├── redis_mock_test.go  // Mock Redis client for testing
│   ├── crdt_string.go  // CRDT value types and merge logic
│   └── redis_string.md  // Documentation for Redis string CRDT implementation
├── redisprotocol/  // Redis protocol implementation
│   ├── redis.go  // Redis protocol server logic
│   └── commands/  // Redis command handlers
│       └── set.go  // Implementation of the SET command
├── proto/  // Protobuf definitions and generated code
│   ├── operation.pb.go  // Generated Go code for protobuf
│   └── operation.proto  // Protobuf schema for operations
├── docs/  // Documentation files
│   └── crdt/  // CRDT-related documentation
│       ├── redis-string-incr.md  // Redis string increment CRDT doc
│       └── strings.md  // CRDT string documentation
├── operation/  // Operation log and related logic
│   ├── log_test.go  // Tests for operation log
│   └── oplog.go  // Operation log implementation
├── main.go  // Entry point for the CRDT Redis server
├── main_test.go  // Integration tests for the main server
├── go.mod  // Go module definition
├── go.sum  // Go module dependency checksums
├── TODO.md  // Project TODO list
├── run_tests.sh  // Shell script to run all tests
├── Makefile  // Build and test automation
├── README.md  // Project overview and usage
├── .gitignore  // Git ignore file
```