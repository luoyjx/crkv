# Redis Hash Commands

Redis Hash is a data structure representing a mapping between string fields and string values, so it's similar to a map data structure. Redis Hashes are ideal for storing objects with multiple fields. Each hash can store up to 2^32 - 1 field-value pairs (more than 4 billion).

## Commands to Implement

### Basic Hash Operations

| Command | Format | Description |
|---------|--------|-------------|
| `HSET` | `HSET key field value [field value ...]` | Sets field(s) to their respective values in the hash |
| `HGET` | `HGET key field` | Returns the value of a field in the hash |
| `HMGET` | `HMGET key field [field ...]` | Returns values for multiple fields in the hash |
| `HDEL` | `HDEL key field [field ...]` | Deletes one or more fields from the hash |
| `HEXISTS` | `HEXISTS key field` | Checks if a field exists in the hash |
| `HLEN` | `HLEN key` | Returns the number of fields in the hash |
| `HGETALL` | `HGETALL key` | Returns all fields and values in the hash |

### Field Query Operations

| Command | Format | Description |
|---------|--------|-------------|
| `HKEYS` | `HKEYS key` | Returns all field names in the hash |
| `HVALS` | `HVALS key` | Returns all values in the hash |
| `HSTRLEN` | `HSTRLEN key field` | Returns string length of the value of a field |
| `HSCAN` | `HSCAN key cursor [MATCH pattern] [COUNT count]` | Incrementally iterates over the hash |
| `HRANDFIELD` | `HRANDFIELD key [count [WITHVALUES]]` | Returns random field(s) from the hash |

### Field Modification Operations 

| Command | Format | Description |
|---------|--------|-------------|
| `HSETNX` | `HSETNX key field value` | Sets field value only if field does not exist |
| `HINCRBY` | `HINCRBY key field increment` | Increments the integer value of a field |
| `HINCRBYFLOAT` | `HINCRBYFLOAT key field increment` | Increments the float value of a field |

### Field Expiration Operations (Redis 7.4+)

| Command | Format | Description |
|---------|--------|-------------|
| `HEXPIRE` | `HEXPIRE key seconds field [field ...]` | Sets expiration time in seconds for field(s) |
| `HEXPIREAT` | `HEXPIREAT key unix-time field [field ...]` | Sets expiration UNIX timestamp for field(s) |
| `HPEXPIRE` | `HPEXPIRE key milliseconds field [field ...]` | Sets expiration time in milliseconds for field(s) |
| `HPEXPIREAT` | `HPEXPIREAT key ms-timestamp field [field ...]` | Sets expiration timestamp in milliseconds |
| `HTTL` | `HTTL key field [field ...]` | Returns remaining time to live in seconds |
| `HPTTL` | `HPTTL key field [field ...]` | Returns remaining time to live in milliseconds |
| `HEXPIRETIME` | `HEXPIRETIME key field [field ...]` | Returns UNIX expiration timestamp in seconds |
| `HPEXPIRETIME` | `HPEXPIRETIME key field [field ...]` | Returns expiration timestamp in milliseconds |
| `HPERSIST` | `HPERSIST key field [field ...]` | Removes expiration from field(s) |

## Available Commands

### HSET

```
HSET key field value [field value ...]
```

Sets the specified fields to their respective values in the hash stored at `key`. This command overwrites the values of specified fields that exist in the hash. If `key` doesn't exist, a new key holding a hash is created.

**Time complexity:** O(1) for each field/value pair added

**Return value:** Integer reply - the number of fields that were added.

**Example:**
```
HSET myhash field1 "Hello"
HGET myhash field1          // Returns "Hello"
HSET myhash field2 "Hi" field3 "World"
HGET myhash field2          // Returns "Hi"
```

### HGET

```
HGET key field
```

Returns the value associated with `field` in the hash stored at `key`.

**Time complexity:** O(1)

**Return value:** Bulk string reply - the value associated with `field`, or `nil` when `field` is not present in the hash or `key` does not exist.

**Example:**
```
HSET myhash field1 "foo"
HGET myhash field1          // Returns "foo"
HGET myhash field2          // Returns nil
```

### HMGET

```
HMGET key field [field ...]
```

Returns the values associated with the specified `fields` in the hash stored at `key`.

**Time complexity:** O(N) where N is the number of fields being requested

**Return value:** Array reply - a list of values associated with the given fields, in the same order as they are requested.

**Example:**
```
HSET myhash field1 "Hello" field2 "World"
HMGET myhash field1 field2 nofield  // Returns ["Hello", "World", nil]
```

### HMSET (deprecated)

```
HMSET key field value [field value ...]
```

Sets the specified fields to their respective values in the hash stored at `key`. This command has been deprecated as of Redis 4.0.0. Use HSET with multiple field-value pairs instead.

**Time complexity:** O(N) where N is the number of fields being set

**Return value:** Simple string reply - "OK"

**Example:**
```
HMSET myhash field1 "Hello" field2 "World"
HGET myhash field1          // Returns "Hello"
HGET myhash field2          // Returns "World"
```

### HDEL

```
HDEL key field [field ...]
```

Removes the specified fields from the hash stored at `key`. Specified fields that do not exist within this hash are ignored.

**Time complexity:** O(N) where N is the number of fields to be removed

**Return value:** Integer reply - the number of fields that were removed from the hash, not including specified but non-existing fields.

**Example:**
```
HSET myhash field1 "foo" field2 "bar"
HDEL myhash field1          // Returns 1
HDEL myhash field2 field3   // Returns 1
```

### HEXISTS

```
HEXISTS key field
```

Returns if `field` is an existing field in the hash stored at `key`.

**Time complexity:** O(1)

**Return value:** Integer reply - 1 if the hash contains `field`. 0 if the hash does not contain `field`, or `key` does not exist.

**Example:**
```
HSET myhash field1 "foo"
HEXISTS myhash field1       // Returns 1
HEXISTS myhash field2       // Returns 0
```

### HGETALL

```
HGETALL key
```

Returns all fields and values of the hash stored at `key`.

**Time complexity:** O(N) where N is the size of the hash

**Return value:** Array reply - a list of fields and their values stored in the hash, or an empty list when `key` does not exist.

**Example:**
```
HSET myhash field1 "Hello" field2 "World"
HGETALL myhash              // Returns ["field1", "Hello", "field2", "World"]
```

### HINCRBY

```
HINCRBY key field increment
```

Increments the number stored at `field` in the hash stored at `key` by `increment`. If `key` does not exist, a new key holding a hash is created. If `field` does not exist the value is set to 0 before the operation is performed.

**Time complexity:** O(1)

**Return value:** Integer reply - the value at `field` after the increment operation.

**Example:**
```
HSET myhash field 5
HINCRBY myhash field 1      // Returns 6
HINCRBY myhash field -1     // Returns 5
```

### HINCRBYFLOAT

```
HINCRBYFLOAT key field increment
```

Increment the specified `field` of a hash stored at `key`, and representing a floating point number, by the specified `increment`. If `field` does not exist, it is set to 0 before performing the operation.

**Time complexity:** O(1)

**Return value:** Bulk string reply - the value at `field` after the increment operation.

**Example:**
```
HSET myhash field 10.50
HINCRBYFLOAT myhash field 0.1     // Returns "10.6"
HINCRBYFLOAT myhash field -5      // Returns "5.6"
```

### HKEYS

```
HKEYS key
```

Returns all field names in the hash stored at `key`.

**Time complexity:** O(N) where N is the size of the hash

**Return value:** Array reply - a list of fields in the hash, or an empty list when `key` does not exist.

**Example:**
```
HSET myhash field1 "Hello" field2 "World"
HKEYS myhash               // Returns ["field1", "field2"]
```

### HLEN

```
HLEN key
```

Returns the number of fields contained in the hash stored at `key`.

**Time complexity:** O(1)

**Return value:** Integer reply - the number of fields in the hash, or 0 when `key` does not exist.

**Example:**
```
HSET myhash field1 "Hello" field2 "World"
HLEN myhash                // Returns 2
```

### HSETNX

```
HSETNX key field value
```

Sets `field` in the hash stored at `key` to `value`, only if `field` does not yet exist. If `key` does not exist, a new key holding a hash is created.

**Time complexity:** O(1)

**Return value:** Integer reply - 1 if `field` is a new field in the hash and `value` was set. 0 if `field` already exists in the hash and no operation was performed.

**Example:**
```
HSETNX myhash field "Hello"     // Returns 1
HSETNX myhash field "World"     // Returns 0, no operation performed
```

### HSTRLEN

```
HSTRLEN key field
```

Returns the string length of the value associated with `field` in the hash stored at `key`.

**Time complexity:** O(1)

**Return value:** Integer reply - the string length of the value associated with `field`, or 0 when `field` is not present in the hash or `key` does not exist.

**Example:**
```
HSET myhash field1 "Hello" field2 "World"
HSTRLEN myhash field1           // Returns 5
HSTRLEN myhash field2           // Returns 5
HSTRLEN myhash nonexisting      // Returns 0
```

### HVALS

```
HVALS key
```

Returns all values in the hash stored at `key`.

**Time complexity:** O(N) where N is the size of the hash

**Return value:** Array reply - a list of values in the hash, or an empty list when `key` does not exist.

**Example:**
```
HSET myhash field1 "Hello" field2 "World"
HVALS myhash               // Returns ["Hello", "World"]
```

### HSCAN

```
HSCAN key cursor [MATCH pattern] [COUNT count]
```

Incrementally iterates over a hash.

**Time complexity:** O(1) for every call. O(N) for a complete iteration, where N is the number of elements inside the hash.

**Return value:** Array reply - a list containing two elements: a new cursor and an array of elements.

**Example:**
```
HSCAN myhash 0 MATCH f* COUNT 2
```

### HRANDFIELD

```
HRANDFIELD key [count [WITHVALUES]]
```

Returns a random field from the hash stored at `key`.

**Time complexity:** O(N) where N is the absolute value of the passed count.

**Return value:** Bulk string reply: without additional arguments the command returns a randomly selected field, or nil when `key` does not exist.

**Example:**
```
HSET coin heads obverse tails reverse edge null
HRANDFIELD coin            // Returns either "heads", "tails", or "edge"
HRANDFIELD coin 2          // Returns an array of 2 fields
HRANDFIELD coin 2 WITHVALUES  // Returns an array of 2 fields and their values
```

## Field Expiration Commands (Redis 7.4+)

Redis 7.4 adds field-level expiration for hash data types:

### HEXPIRE

```
HEXPIRE key seconds field [field ...]
```

Sets the expiration time for specified fields in the hash stored at `key`.

**Example:**
```
HEXPIRE sensor:1 60 temperature humidity  // Set fields to expire in 60 seconds
```

### HEXPIREAT

```
HEXPIREAT key unix-time field [field ...]
```

Sets the expiration time for specified fields to a UNIX timestamp (in seconds).

### HPEXPIRE

```
HPEXPIRE key milliseconds field [field ...]
```

Sets the expiration time for specified fields in milliseconds.

### HPEXPIREAT

```
HPEXPIREAT key milliseconds-timestamp field [field ...]
```

Sets the expiration time for specified fields to a UNIX timestamp (in milliseconds).

### HTTL

```
HTTL key field [field ...]
```

Returns the remaining time to live in seconds for specified hash fields.

### HPTTL

```
HPTTL key field [field ...]
```

Returns the remaining time to live in milliseconds for specified hash fields.

### HEXPIRETIME/HPEXPIRETIME

```
HEXPIRETIME key field [field ...]
HPEXPIRETIME key field [field ...]
```

Returns the UNIX timestamp at which the specified fields will expire (in seconds or milliseconds).

### HPERSIST

```
HPERSIST key field [field ...]
```

Removes the expiration from specified fields in the hash.

## Performance Considerations

- Most Redis hash commands are O(1), making them very efficient.
- A few commands like HKEYS, HVALS, HGETALL, and HSCAN are O(N) where N is the number of field-value pairs.
- Hash structures with only a few fields are encoded in a space-efficient way that makes them memory efficient.
- For objects with many fields, consider using multiple smaller hashes to improve performance. 

## Redis Hash in Active-Active Databases (CRDT)

Redis Hash in Active-Active geo-distributed databases implements CRDT (Conflict-free Replicated Data Type) semantics to handle concurrent operations across multiple instances.

### CRDT Behavior

Hashes in Active-Active databases maintain additional metadata to achieve an "OR-Set" behavior to handle concurrent conflicting writes. With this behavior:

1. **Add Wins Semantics**: Writes to add new fields across multiple Active-Active database instances are typically unioned together.

2. **Conflict Resolution**: When conflicting instance writes occur (e.g., one instance deletes a field while another adds the same field), an "observed remove" rule is followed. That is, a remove operation can only remove fields it has already seen, and in all other cases, element add/update wins.

3. **Field Values**: Field values behave just like CRDT strings. String values can be regular strings or counter integers based on the command used for initialization of the field value.

### Example of "Add Wins" Case:

| **Time** | **CRDB Instance1**                             | **CRDB Instance2**                             |
| -------- | ---------------------------------------------- | ---------------------------------------------- |
| t1       | HSET key1 field1 "a"                           |                                                |
| t2       | HSET key1 field2 "b"                           |                                                |
| t4       | \- Sync -                                      | \- Sync -                                      |
| t5       | HGETALL key1<br>1) "field2"<br>2) "b"<br>3) "field1"<br>4) "a" | HGETALL key1<br>1) "field2"<br>2) "b"<br>3) "field1"<br>4) "a" |

### Use Cases in Distributed Systems

Hashes in Active-Active databases are particularly useful for:
- Managing distributed user or application session state
- Storing user preferences 
- Handling form data
- Maintaining structured data that needs to be accessed and modified across multiple geographical regions

### Implementation Considerations

When implementing Redis Hash with CRDT support:
1. Ensure correct handling of metadata for conflict resolution
2. Consider the different behavior of string fields vs. counter fields
3. Be aware of the "add wins" semantics in conflict scenarios

*Source: [Redis Documentation - Hashes in Active-Active databases](https://redis.io/docs/latest/operate/rs/databases/active-active/develop/data-types/hashes/)* 