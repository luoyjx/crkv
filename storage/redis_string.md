# Redis String Commands for CRDT Implementation

This document outlines the Redis string commands that need to be implemented for CRDT (Conflict-free Replicated Data Type) support, following the design principles described in the Redis Active-Active architecture.

## Replication Semantics

Redis string operations in a CRDT context follow two main approaches:
1. **Last Write Wins (LWW)** - For regular string operations
2. **Semantic Resolution** - For counter operations

## Commands to Implement

### Basic String Operations (LWW semantics)

| Command | Format | Description |
|---------|--------|-------------|
| `GET` | `GET key` | Returns the string value of a key |
| `SET` | `SET key value [EX seconds] [PX milliseconds] [NX\|XX]` | Sets the string value of a key with optional expiration and condition flags |
| `APPEND` | `APPEND key value` | Appends a value to an existing string |
| `GETRANGE` | `GETRANGE key start end` | Returns a substring of the string stored at a key |
| `SETRANGE` | `SETRANGE key offset value` | Overwrites part of a string at key starting at the specified offset |
| `STRLEN` | `STRLEN key` | Returns the length of the string value stored at key |
| `GETSET` | `GETSET key value` | Sets a new value and returns the old value |
| `GETDEL` | `GETDEL key` | Gets the value and deletes the key |
| `GETEX` | `GETEX key [EX seconds] [PX milliseconds]` | Gets the value and sets expiration |
| `MGET` | `MGET key [key ...]` | Returns the values of all specified keys |
| `MSET` | `MSET key value [key value ...]` | Sets multiple key-value pairs |
| `MSETNX` | `MSETNX key value [key value ...]` | Sets multiple key-value pairs only if none exist |
| `SETEX` | `SETEX key seconds value` | Sets string value with expiration time in seconds |
| `PSETEX` | `PSETEX key milliseconds value` | Sets string value with expiration time in milliseconds |
| `SETNX` | `SETNX key value` | Sets a key only if it doesn't exist |

### Counter Operations (Semantic Resolution)

| Command | Format | Description |
|---------|--------|-------------|
| `INCR` | `INCR key` | Increments the integer value of a key by one |
| `DECR` | `DECR key` | Decrements the integer value of a key by one |
| `INCRBY` | `INCRBY key increment` | Increments the integer value of a key by a specified amount |
| `DECRBY` | `DECRBY key decrement` | Decrements the integer value of a key by a specified amount |
| `INCRBYFLOAT` | `INCRBYFLOAT key increment` | Increments the float value of a key by a specified amount |

## Implementation Requirements

1. **Metadata Management**:
   - Each string operation must store a wall-clock timestamp in its metadata
   - For counter operations, maintain a 59-bit counter to prevent overflow in concurrent operations

2. **Conflict Resolution**:
   - For regular string operations: implement "last write wins" based on timestamps
   - For counter operations: implement semantic resolution by accumulating the counter operations across replicas

3. **Data Structure**:
   - Design string data structure to include both value and metadata components
   - Ensure counter operations can track increments/decrements across replicas

4. **Synchronization**:
   - Implement efficient synchronization mechanism for string values between replicas
   - Special handling for counter values to ensure proper accumulation

## Limitations

- Counter operations are limited to 59-bit integers (to protect from overflows in concurrent operations)
- String operations follow LWW semantics, which may result in some updates being lost in conflict situations 