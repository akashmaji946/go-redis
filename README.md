# Go-Redis

A Redis-compatible in-memory key-value store server written in Go. This implementation supports core Redis commands, persistence mechanisms (AOF and RDB), authentication, expiration, transactions, monitoring, and memory management with eviction policies.

## Features

- **Core Commands**: GET, SET, DEL, EXISTS, KEYS, DBSIZE, FLUSHDB
- **Persistence**:
  - **AOF (Append-Only File)**: Logs every write operation with configurable fsync modes
  - **RDB (Redis Database)**: Point-in-time snapshots with automatic triggers
- **Expiration**: EXPIRE and TTL support for keys with automatic cleanup
- **Authentication**: Password-based authentication with configurable requirements
- **Transactions**: MULTI, EXEC, DISCARD for atomic command execution
- **Monitoring**: MONITOR command for real-time command streaming
- **Server Info**: INFO command for server statistics and metrics
- **Memory Management**: Configurable memory limits with eviction policies
- **Background Operations**: BGSAVE and BGREWRITEAOF for non-blocking persistence
- **Thread-Safe**: Concurrent access with read-write locks (RWMutex)
- **Redis Protocol**: Full RESP (Redis Serialization Protocol) compatibility
- **Checksum Verification**: SHA-256 checksums for RDB data integrity

## Prerequisites

- **Go 1.24.4** or later
- **redis-cli** (for testing and connecting to the server)
- **Linux/Unix environment** (tested on Linux)

### Stopping Default Redis (if running)

```bash
sudo systemctl stop redis-server.service
sudo systemctl status redis-server.service
````

## Building

```bash
go build
```

## Configuration

The server reads configuration from `redis.conf`:

```conf
dir ./data

appendonly yes
appendfilename backup.aof
appendfsync always

save 5 3
dbfilename backup.rdb

requirepass dsl

maxmemory 256
maxmemory-policy allkeys-random
maxmemory-samples 5
```

## Running

```bash
./go-redis
```

Server listens on port **6379**.

## Available Commands

### String Operations

* GET
* SET

### Key Management

* DEL
* EXISTS
* KEYS
* DBSIZE
* FLUSHDB

### Expiration

* EXPIRE
* TTL

### Transactions

* MULTI
* EXEC
* DISCARD

### Persistence

* SAVE
* BGSAVE
* BGREWRITEAOF

### Monitoring and Information

* MONITOR
* INFO

### Authentication

* AUTH

### Utility

* COMMAND

## Persistence

### AOF

* Logs every write
* Replayed on startup
* Supports `always`, `everysec`, `no` fsync modes
* Rewritten using BGREWRITEAOF

### RDB

* Snapshot-based persistence
* Triggered via `save` rules
* Uses Go `gob` encoding
* SHA-256 checksum verification

## Memory Management

### Eviction Policies

* no-eviction
* allkeys-random (implemented)
* allkeys-lru (not implemented)
* allkeys-lfu (not implemented)

## Architecture

* RWMutex-based concurrency
* Per-connection goroutines
* Background workers for persistence
* Lazy expiration
* Automatic eviction

## Project Structure

```
go-redis/
├── main.go
├── handlers.go
├── database.go
├── value.go
├── writer.go
├── conf.go
├── aof.go
├── rdb.go
├── client.go
├── appstate.go
├── info.go
├── mem.go
├── redis.conf
├── go.mod
└── data/
```

## Protocol

Implements full Redis RESP:

* Simple Strings
* Bulk Strings
* Arrays
* Integers
* Errors
* Null

## Limitations

* Single database only
* No replication
* No Pub/Sub
* Only string data type
* No WATCH
* Partial eviction support

## License

Educational project implementing Redis-like functionality in Go.

## Version

**v0.1**

## Author
**Akash Maji (akashmaji@iisc.ac.in) - Contact for bugs and support**

