# Go-Redis User Guide

This guide provides documentation for all available commands in Go-Redis. Each command includes syntax, examples, and behavior description.

## Table of Contents

- [String Operations](#string-operations)
- [Key Management](#key-management)
- [Expiration](#expiration)
- [Transactions](#transactions)
- [Persistence](#persistence)
- [Authentication](#authentication)
- [Monitoring & Information](#monitoring--information)
- [Utility](#utility)

---

## String Operations

### GET

**Description:** Retrieves the value associated with a key.

**Syntax:**
```
GET <key>
```

**Example:**
```
127.0.0.1:6379> GET name
"John"
127.0.0.1:6379> GET nonexistent
(nil)
```

**Returns:**
- Bulk string: The value if key exists and is not expired
- NULL: If key doesn't exist or has expired

**Behavior:**
- Automatically deletes expired keys when accessed (lazy expiration)
- Thread-safe read operation
- Updates last accessed time and access count for the key

---

### SET

**Description:** Sets a key to hold a string value.

**Syntax:**
```
SET <key> <value>
```

**Example:**
```
127.0.0.1:6379> SET name "John"
OK
127.0.0.1:6379> SET counter "100"
OK
```

**Returns:** `OK` on success

**Behavior:**
- If key already exists, overwrites the previous value
- Automatically triggers eviction if maxmemory limit is reached
- Appends command to AOF file if AOF persistence is enabled
- Flushes AOF immediately if `appendfsync=always`
- Updates RDB change tracker if RDB persistence is configured
- Thread-safe write operation

---

## Key Management

### DEL

**Description:** Deletes one or more keys from the database.

**Syntax:**
```
DEL <key1> [key2 ...]
```

**Example:**
```
127.0.0.1:6379> DEL name
(integer) 1
127.0.0.1:6379> DEL key1 key2 key3
(integer) 2
```

**Returns:** Integer representing the number of keys actually deleted

**Behavior:**
- Non-existent keys are ignored (not counted)
- Returns 0 if none of the specified keys exist
- Thread-safe operation
- Frees memory associated with deleted keys

---

### EXISTS

**Description:** Checks if one or more keys exist in the database.

**Syntax:**
```
EXISTS <key1> [key2 ...]
```

**Example:**
```
127.0.0.1:6379> EXISTS name
(integer) 1
127.0.0.1:6379> EXISTS name age
(integer) 1
127.0.0.1:6379> EXISTS nonexistent
(integer) 0
```

**Returns:** Integer count of keys that exist

**Behavior:**
- Returns 0 if none of the keys exist
- Returns count of existing keys (may be less than number of keys checked)
- Thread-safe read operation

---

### KEYS

**Description:** Finds all keys matching a given pattern using glob-style matching.

**Syntax:**
```
KEYS <pattern>
```

**Example:**
```
127.0.0.1:6379> KEYS *
1) "name"
2) "age"
3) "user:1"
127.0.0.1:6379> KEYS user:*
1) "user:1"
2) "user:2"
127.0.0.1:6379> KEYS *name*
1) "firstname"
2) "lastname"
```

**Pattern Rules:**
- `*` - Matches any sequence of characters
- `?` - Matches a single character
- `[abc]` - Matches any character in brackets

**Returns:** Array of matching keys (empty array if no matches)

**Behavior:**
- Uses filepath.Match for pattern matching
- Thread-safe read operation
- May be slow on large databases (scans all keys)

---

### DBSIZE

**Description:** Returns the total number of keys in the current database.

**Syntax:**
```
DBSIZE
```

**Example:**
```
127.0.0.1:6379> DBSIZE
(integer) 42
```

**Returns:** Integer count of keys

**Behavior:**
- Counts all keys including expired ones (until accessed)
- Thread-safe read operation
- Fast operation (O(1) - map length)

---

### FLUSHDB

**Description:** Removes all keys from the current database.

**Syntax:**
```
FLUSHDB
```

**Example:**
```
127.0.0.1:6379> FLUSHDB
OK
```

**Returns:** `OK`

**Behavior:**
- Efficiently clears database by replacing the store map
- Irreversible operation - all data is permanently deleted
- Thread-safe write operation
- Resets memory usage counter

**Warning:** This operation cannot be undone!

---

## Expiration

### EXPIRE

**Description:** Sets a timeout on a key. After the timeout expires, the key will be automatically deleted.

**Syntax:**
```
EXPIRE <key> <seconds>
```

**Example:**
```
127.0.0.1:6379> EXPIRE session:123 3600
(integer) 1
127.0.0.1:6379> EXPIRE nonexistent 60
(integer) 0
```

**Returns:**
- `1`: Timeout was set successfully
- `0`: Key doesn't exist

**Behavior:**
- Sets expiration as absolute timestamp (current time + seconds)
- Overwrites any existing expiration time
- Expired keys are deleted when accessed (lazy expiration)
- Thread-safe operation

---

### TTL

**Description:** Returns the remaining time to live (TTL) of a key in seconds.

**Syntax:**
```
TTL <key>
```

**Example:**
```
127.0.0.1:6379> TTL session:123
(integer) 3598
127.0.0.1:6379> TTL noexpiration
(integer) -1
127.0.0.1:6379> TTL nonexistent
(integer) -2
```

**Returns:**
- Positive integer: Remaining TTL in seconds
- `-1`: Key exists but has no expiration set
- `-2`: Key doesn't exist or has expired

**Behavior:**
- Automatically deletes key if expired when checked
- Returns seconds remaining until expiration
- Thread-safe read operation

---

## Transactions

### MULTI

**Description:** Begins a transaction. All subsequent commands will be queued until EXEC or DISCARD is called.

**Syntax:**
```
MULTI
```

**Example:**
```
127.0.0.1:6379> MULTI
OK
127.0.0.1:6379> SET key1 "value1"
QUEUED
127.0.0.1:6379> SET key2 "value2"
QUEUED
```

**Returns:** `OK` (or error if transaction already running)

**Behavior:**
- Creates a new transaction context
- Commands return "QUEUED" instead of executing immediately
- Only one transaction can be active per client
- Transaction control commands (MULTI, EXEC, DISCARD) execute immediately

---

### EXEC

**Description:** Executes all commands queued in the current transaction atomically.

**Syntax:**
```
EXEC
```

**Example:**
```
127.0.0.1:6379> MULTI
OK
127.0.0.1:6379> SET a "1"
QUEUED
127.0.0.1:6379> SET b "2"
QUEUED
127.0.0.1:6379> EXEC
1) OK
2) OK
```

**Returns:**
- Array of replies: One reply per queued command, in order
- Error if no transaction is running

**Behavior:**
- Executes all queued commands sequentially
- Commands succeed or fail individually (no rollback)
- Clears transaction context after execution
- Updates transaction statistics

---

### DISCARD

**Description:** Aborts the current transaction by discarding all queued commands without executing them.

**Syntax:**
```
DISCARD
```

**Example:**
```
127.0.0.1:6379> MULTI
OK
127.0.0.1:6379> SET key1 "value1"
QUEUED
127.0.0.1:6379> SET key2 "value2"
QUEUED
127.0.0.1:6379> DISCARD
OK
```

**Returns:** `OK` (or error if no transaction is running)

**Behavior:**
- Clears transaction context
- All queued commands are discarded
- No changes are made to the database
- Client can start a new transaction with MULTI

---

## Persistence

### SAVE

**Description:** Synchronously saves the database snapshot to disk.

**Syntax:**
```
SAVE
```

**Example:**
```
127.0.0.1:6379> SAVE
OK
```

**Returns:** `OK`

**Behavior:**
- Blocks the server until save completes
- Uses read lock, preventing write operations during save
- Computes SHA-256 checksum for data integrity
- Saves to RDB file configured in redis.conf

**Note:** Use BGSAVE for non-blocking saves.

---

### BGSAVE

**Description:** Performs an asynchronous background save of the database snapshot.

**Syntax:**
```
BGSAVE
```

**Example:**
```
127.0.0.1:6379> BGSAVE
OK
127.0.0.1:6379> BGSAVE
(error) already in progress
```

**Returns:**
- `OK`: Background save started successfully
- Error: If a background save is already in progress

**Behavior:**
- Creates a copy of database state before saving
- Runs in a separate goroutine (non-blocking)
- Server continues to handle commands during save
- Prevents concurrent background saves

---

### BGREWRITEAOF

**Description:** Rewrites the AOF file in the background to remove duplicates and optimize size.

**Syntax:**
```
BGREWRITEAOF
```

**Example:**
```
127.0.0.1:6379> BGREWRITEAOF
Started.
```

**Returns:** `Started.`

**Behavior:**
- Creates compact AOF with only SET commands for current keys
- Buffers new commands during rewrite and appends them after
- Significantly reduces AOF file size
- Runs in a separate goroutine
- Updates AOF rewrite statistics

---

## Authentication

### AUTH

**Description:** Authenticates the client with the server using a password.

**Syntax:**
```
AUTH <password>
```

**Example:**
```
127.0.0.1:6379> AUTH dsl
OK
127.0.0.1:6379> AUTH wrongpassword
(error) ERR invalid password, given=wrongpassword
```

**Returns:**
- `OK`: Authentication successful
- Error: If password is incorrect

**Behavior:**
- Sets client's authenticated flag to true on success
- Sets authenticated flag to false on failure
- Required before executing other commands if `requirepass` is set
- Safe command (can be executed without prior authentication)

**Note:** Authentication state persists for the connection lifetime.

---

## Monitoring & Information

### MONITOR

**Description:** Enables real-time monitoring mode. All commands executed by other clients are streamed to this connection.

**Syntax:**
```
MONITOR
```

**Example:**
```
# Terminal 1: Enable monitoring
127.0.0.1:6379> MONITOR
OK

# Terminal 2: Execute commands
127.0.0.1:6379> SET test "value"
OK

# Terminal 1: Receives
1704067200 [127.0.0.1:54321] "SET" "test" "value"
```

**Returns:** `OK`

**Behavior:**
- Adds client to monitoring list
- Client remains in monitoring mode until connection closes
- All commands from other clients are streamed
- Monitoring client does not receive its own commands
- Multiple clients can monitor simultaneously
- Logs sent asynchronously (doesn't block command execution)

**Format:** `<timestamp> [<client_ip>] "<command>" "<arg1>" ... "<argN>"`

---

### INFO

**Description:** Returns server information and statistics in a human-readable format.

**Syntax:**
```
INFO
```

**Example:**
```
127.0.0.1:6379> INFO
# Server
redis_version  : 0.1
process_id     : 12345
server_uptime  : 3600
...

# Memory
used_memory: 1024 B
eviction_policy: allkeys-random
...
```

**Returns:** Bulk string containing formatted server information

**Information Categories:**
- **Server**: Version, PID, port, uptime, paths
- **Clients**: Number of connected clients
- **Memory**: Used/peak/total memory, eviction policy
- **Persistence**: RDB/AOF status, save times, counts
- **General**: Connections, commands, transactions, expired/evicted keys

**Behavior:**
- Information generated dynamically on each call
- Statistics are cumulative since server startup
- Thread-safe read operation

---

## Utility

### COMMAND

**Description:** Utility command for connection testing and protocol compliance.

**Syntax:**
```
COMMAND
```

**Example:**
```
127.0.0.1:6379> COMMAND
OK
```

**Returns:** `OK`

**Behavior:**
- Simple connection test
- Can be executed without authentication
- Used for protocol compliance

---

## Command Response Types

Go-Redis uses the RESP (Redis Serialization Protocol) format for responses:

- **Simple String**: `+OK\r\n`
- **Bulk String**: `$5\r\nhello\r\n`
- **Integer**: `:42\r\n`
- **Array**: `*2\r\n$3\r\nGET\r\n$4\r\nname\r\n`
- **Error**: `-ERR message\r\n`
- **Null**: `$-1\r\n`

---

## Error Messages

Common error messages you may encounter:

- `ERR no such command` - Command doesn't exist
- `NOAUTH client not authenticated` - Authentication required
- `ERR invalid command usage` - Wrong number of arguments
- `ERR tx already running` - Transaction already active
- `ERR tx already NOT running` - No transaction to execute/discard
- `already in progress` - Background save already running
- `ERR maxmemory reached` - Memory limit reached, eviction failed

---

## Best Practices

1. **Use BGSAVE instead of SAVE** - Non-blocking saves don't freeze the server
2. **Monitor memory usage** - Use INFO command to check memory statistics
3. **Set appropriate expiration** - Use EXPIRE for temporary data to prevent memory buildup
4. **Use transactions for atomicity** - MULTI/EXEC for operations that must succeed together
5. **Enable AOF for durability** - Better data safety than RDB alone
6. **Use KEYS sparingly** - Can be slow on large databases

---

## Quick Reference

| Command | Category | Purpose |
|---------|----------|---------|
| GET | String | Retrieve value |
| SET | String | Store value |
| DEL | Key | Delete key(s) |
| EXISTS | Key | Check existence |
| KEYS | Key | Find by pattern |
| DBSIZE | Key | Count keys |
| FLUSHDB | Key | Delete all keys |
| EXPIRE | Expiration | Set timeout |
| TTL | Expiration | Check remaining time |
| MULTI | Transaction | Start transaction |
| EXEC | Transaction | Execute transaction |
| DISCARD | Transaction | Abort transaction |
| SAVE | Persistence | Sync save |
| BGSAVE | Persistence | Async save |
| BGREWRITEAOF | Persistence | Rewrite AOF |
| AUTH | Auth | Authenticate |
| MONITOR | Monitoring | Enable monitoring |
| INFO | Information | Server stats |
| COMMAND | Utility | Connection test |

---

For more information, see the main README.md file.
