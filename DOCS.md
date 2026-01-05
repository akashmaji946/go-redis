# Go-Redis — Developer Documentation (DOCS.md)

This document describes the **internal architecture, command semantics, persistence model, concurrency behavior, and design decisions** of **Go-Redis**, a Redis-compatible in-memory key-value store written in Go.

> 📌 For build, configuration, and usage instructions, see `README.md` at [link](https://github.com/akashmaji946/go-redis/blob/main/README.md)

---

## Table of Contents

1. Overview  
2. Architecture  
3. Data Model  
4. Concurrency Model  
5. Command Execution Pipeline  
6. Command Reference  
7. Transactions  
8. Persistence  
9. Memory Management  
10. Monitoring & Observability  
11. RESP Protocol Support  
12. Startup & Shutdown Lifecycle  
13. Limitations  
14. Intended Use  
15. Versioning  

---

## 1. Overview

**Go-Redis** is a Redis-compatible, single-node, in-memory database implemented in Go.  
It supports the RESP protocol and a subset of Redis commands, with a focus on **clarity, correctness, and educational value**.

### Design Goals

- Compatible with `redis-cli`
- Simple, readable Go implementation
- Deterministic persistence behavior
- Clear separation of concerns
- Educational rather than production focus

### Non-Goals

- Replication or clustering
- Redis modules
- Full Redis command parity
- Maximum performance tuning

---

## 2. Architecture

### High-Level Flow

```

Client (redis-cli)
|
TCP
|
RESP Parser
|
Command Dispatcher
|
+--------------------------+
| In-Memory Database       |
| map[string]*Value        |
| RWMutex protected        |
+--------------------------+
|
+--------------------------+
| Persistence Layer        |
|  - AOF                  |
|  - RDB                  |
+--------------------------+

```

### Core Components

| File | Responsibility |
|----|----------------|
| `main.go` | Server startup, TCP listener |
| `client.go` | Per-client lifecycle |
| `handlers.go` | Command handlers |
| `database.go` | Thread-safe datastore |
| `value.go` | Value + TTL metadata |
| `writer.go` | RESP encoding |
| `aof.go` | Append-only file |
| `rdb.go` | Snapshot persistence |
| `mem.go` | Memory accounting & eviction |
| `info.go` | INFO command |

---

## 3. Data Model

Each key maps to a `Value` object containing:

- Stored value (string or hash)
- Optional expiration timestamp
- Last access timestamp
- Access frequency counter

This enables:

- Lazy expiration
- TTL inspection
- Future LRU/LFU eviction policies

---

## 4. Concurrency Model

### Threading Strategy

- One goroutine per client connection
- Single shared database
- Synchronization via `sync.RWMutex`

### Locking Rules

| Operation | Lock Type |
|---------|----------|
| Read-only (GET, TTL) | `RLock` |
| Write (SET, DEL) | `Lock` |
| RDB snapshot | Read lock + copy |

> The system prioritizes **correctness and simplicity** over fine-grained parallelism.

---

## 5. Command Execution Pipeline

1. Client sends RESP request
2. RESP parser decodes command and arguments
3. Authentication check (if enabled)
4. Transaction state check (MULTI mode)
5. Command handler execution
6. RESP-encoded response sent
7. Optional AOF append

---

## 6. Command Reference

### 6.1 Authentication

#### `AUTH <password>`

- Validates against `requirepass` from config
- Required for most commands if authentication is enabled
- Marks client as authenticated on success

---

### 6.2 String Commands

#### `SET <key> <value>`

- Overwrites existing key
- Clears previous TTL
- Updates memory counters
- Appended to AOF (if enabled)

#### `GET <key>`

- Performs lazy expiration
- Updates access metadata
- Returns NULL if key is missing or expired

---

### 6.3 Key Commands

#### `DEL <key1> [key2 ...]`

- Deletes keys immediately
- Frees memory and expiration metadata

#### `KEYS <pattern>`

- Glob-style matching
- Iterates entire keyspace (O(N))
- Intended for debugging

#### `DBSIZE`

- Returns number of keys in database
- O(1) operation

#### `FLUSHDB`

- Removes all keys
- Frees all memory
- Irreversible

---

### 6.4 Expiration Commands

#### `EXPIRE <key> <seconds>`

- Stores absolute expiration timestamp
- No background timer
- Key removed lazily on access

#### `TTL <key>`

Return values:
- `> 0` → seconds remaining
- `-1` → key exists without expiration
- `-2` → key does not exist

---

## 7. Transactions

### MULTI / EXEC

- Commands queued per client
- No `WATCH` support
- `EXEC` executes atomically under write lock
- Errors inside transaction do **not** abort execution

### DISCARD

- Clears queued commands
- Leaves database unchanged

---

## 8. Hash Commands

Each hash key stores a `map[field]value`.  
TTL applies to the **entire hash**, not individual fields.

Supported commands:

- `HSET`
- `HGET`
- `HDEL`
- `HDELALL`
- `HGETALL`
- `HMSET`
- `HINCRBY`
- `HEXISTS`
- `HLEN`
- `HKEYS`
- `HVALS`
- `HEXPIRE`

All hash commands:
- Perform lazy expiration
- Update memory accounting
- Remove hash key if empty

---

## 9. Persistence

### 9.1 AOF (Append-Only File)

#### Write Path

1. Command executes in memory
2. Serialized in RESP
3. Appended to AOF buffer
4. Flushed based on fsync policy

#### fsync Modes

- `always` — fsync after every write
- `everysec` — background fsync goroutine
- `no` — OS-managed flushing

#### AOF Replay

- Replayed on startup
- Rebuilds dataset deterministically

---

### 9.2 RDB (Snapshot)

#### Snapshot Flow

1. Copy database state
2. Serialize using Go `gob`
3. Compute SHA-256 checksum
4. Write atomically to disk

#### Save Commands

- `SAVE` — synchronous, blocks writes
- `BGSAVE` — background snapshot
- `BGREWRITEAOF` — AOF compaction

Snapshots with checksum mismatch are rejected.

---

## 10. Memory Management

### Memory Accounting Includes

- Key size
- Value size
- Hash fields
- Expiration metadata
- Map overhead

### Eviction

Triggered when `maxmemory` is exceeded.

Supported policies:
- `no-eviction`
- `allkeys-random`

(LRU/LFU scaffolding exists but is inactive.)

---

## 11. Monitoring & Observability

### `MONITOR`

- Streams all executed commands
- Includes timestamp and client address
- Runs until connection closes

### `INFO`

Reports sections:
- Server
- Clients
- Memory
- Persistence
- General stats

---

## 12. RESP Protocol Support

Supported RESP types:

- `+` Simple String
- `-` Error
- `:` Integer
- `$` Bulk String
- `$-1` Null
- `*` Array

Fully compatible with `redis-cli`.

---

## 13. Startup & Shutdown Lifecycle

### Startup

1. Parse `redis.conf`
2. Load AOF (if enabled)
3. Load RDB (if present)
4. Start TCP listener
5. Launch background workers

### Shutdown

- No signal handling yet
- Intended for controlled educational use

---

## 14. Limitations (Intentional)

- Single database only
- No replication or clustering
- No Pub/Sub
- No Lua scripting
- No background expiration sweeps
- No Redis modules

---

## 15. Intended Use

Go-Redis is ideal for:

- Learning Redis internals
- Understanding RESP
- Studying persistence tradeoffs
- Exploring Go concurrency
- Systems programming coursework

---

## 16. Versioning

Current version: **v0.1**

Semantic versioning not yet enforced.

---

## 17. Report and Bugs
- Contact: `Akash Maji` 
- Email: `akashmaji@iisc.ac.in`

---



