# Go-Redis — Developer Documentation (DOCS)

This document provides **detailed technical documentation** for the Go-Redis project, covering internal architecture, command semantics, persistence internals, concurrency model, and lifecycle behavior.  
For build/run instructions, see `README.md`. Access it here: [README](https://github.com/akashmaji946/go-redis/blob/main/README.md)

---

## 1. System Overview

Go-Redis is a **Redis-compatible, in-memory key-value store** written in Go, implementing the RESP protocol and a subset of Redis functionality.

### Design Goals
- Protocol compatibility with `redis-cli`
- Clear, readable Go implementation
- Deterministic persistence behavior
- Educational clarity over maximum performance

### Non-Goals
- Cluster support
- Replication
- Advanced Redis data types
- Full Redis command coverage

---

## 2. High-Level Architecture

```

Client (redis-cli)
|
TCP
|
RESP Parser
|
Command Dispatcher
|
+-------------------------+
| In-Memory Database      |
| map[string]*Value      |
| RWMutex protected      |
+-------------------------+
|
+-------------------------+
| Persistence Layer       |
|  - AOF                 |
|  - RDB                 |
+-------------------------+

```

### Key Components

| Component | Responsibility |
|---------|----------------|
| `main.go` | Server startup, TCP accept loop |
| `client.go` | Per-client connection lifecycle |
| `handlers.go` | Command execution logic |
| `database.go` | Thread-safe key-value storage |
| `value.go` | Data model & expiration metadata |
| `writer.go` | RESP serialization |
| `aof.go` | Append-Only File persistence |
| `rdb.go` | Snapshot persistence |
| `mem.go` | Memory tracking & eviction |
| `info.go` | INFO command reporting |

---

## 3. Data Model

### Value Structure

Each key maps to a `Value` object containing:

- Raw string value
- Expiration timestamp (optional)
- Last access time
- Access frequency counter

This enables:
- TTL handling
- Lazy expiration
- Future LRU/LFU eviction support

---

## 4. Concurrency Model

### Threading Strategy

- **One goroutine per client connection**
- **Single shared database**
- Protected by `sync.RWMutex`

### Locking Rules

- Read-only commands (GET, EXISTS, TTL) acquire `RLock`
- Write commands (SET, DEL, EXPIRE) acquire `Lock`
- Persistence snapshots copy state to avoid blocking clients

This design favors **simplicity and correctness** over extreme parallelism.

---

## 5. Command Execution Pipeline

1. Client sends RESP-encoded request
2. RESP parser decodes command + arguments
3. Authentication check (if enabled)
4. Transaction check (MULTI mode)
5. Command handler execution
6. Response encoded in RESP
7. Optional AOF append

---

## 6. Command Semantics

### String Commands

#### SET
- Overwrites existing key
- Clears previous expiration
- Triggers eviction if memory limit exceeded
- Appended to AOF (if enabled)

#### GET
- Performs lazy expiration
- Updates access metadata
- Returns NULL if expired or missing

---

### Key Commands

#### DEL
- Deletes key immediately
- Removes expiration metadata
- Updates memory counters

#### KEYS
- Uses glob-style pattern matching
- Iterates entire keyspace (O(N))
- Intended for debugging, not production

---

### Expiration Commands

#### EXPIRE
- Stores absolute expiration timestamp
- Does not create background timer
- Key removed lazily on access

#### TTL
Return values:
- `>0` seconds remaining
- `-1` key exists without expiration
- `-2` key does not exist

---

## 7. Transactions

### MULTI / EXEC Model

- Commands queued per client
- No optimistic locking (`WATCH` unsupported)
- EXEC executes atomically under write lock
- Errors inside transaction do **not** abort execution

### DISCARD
- Clears transaction queue
- Leaves database unchanged

---

## 8. Persistence Internals

## 8.1 AOF (Append-Only File)

### Write Path
1. Command executes in memory
2. Serialized in RESP
3. Appended to AOF buffer
4. Flushed based on fsync policy

### fsync Modes
- `always` — fsync after every write
- `everysec` — background fsync goroutine
- `no` — OS-managed flushing

### AOF Replay
- On startup, AOF is replayed command-by-command
- Rebuilds in-memory state deterministically

---

## 8.2 RDB (Snapshot)

### Snapshot Trigger
- Based on `save <seconds> <keys>` rules
- Tracks key mutation count

### Snapshot Flow
1. Copy database state
2. Serialize using Go `gob`
3. Compute SHA-256 checksum
4. Write to disk atomically

### Verification
- Snapshot rejected if checksum mismatch
- Prevents corrupted saves

---

## 9. Memory Management

### Memory Accounting Includes
- Key size
- Value size
- Expiration metadata
- Map overhead

### Eviction Flow
1. Write exceeds `maxmemory`
2. Sample keys (`maxmemory-samples`)
3. Select eviction candidates
4. Delete keys until enough memory freed

### Supported Policies
- `no-eviction`
- `allkeys-random`

(LRU/LFU scaffolding exists but not active)

---

## 10. Monitoring & Observability

### MONITOR
- Streams all executed commands
- Includes timestamp and client address
- Useful for debugging and education

### INFO
Reports:
- Server uptime
- Client count
- Memory usage
- Persistence state
- Eviction statistics

---

## 11. RESP Protocol Support

Supported RESP Types:
- `+` Simple String
- `$` Bulk String
- `*` Array
- `:` Integer
- `-` Error
- `$-1` Null

RESP compliance allows unmodified use of `redis-cli`.

---

## 12. Startup & Shutdown Lifecycle

### Startup
1. Parse `redis.conf`
2. Load AOF (if enabled)
3. Load RDB (if present)
4. Start TCP listener
5. Launch background workers

### Shutdown
- No signal handling yet
- OS termination may interrupt persistence
- Intended for controlled educational use

---

## 13. Limitations (Intentional)

- Single database only
- No replication / clustering
- No Pub/Sub
- No advanced Redis types
- No background expiration sweep
- No Lua scripting

---

## 14. Intended Use

This project is best suited for:
- Learning Redis internals
- Studying RESP
- Understanding persistence tradeoffs
- Go concurrency patterns
- Systems programming coursework

---

## 15. Versioning

Current version: **v0.1**

Semantic versioning not yet enforced.

---

## 16. Further Reading

- Redis Design Notes
- Redis Persistence Internals
- Go net/http and net TCP patterns
- RWMutex performance tradeoffs
