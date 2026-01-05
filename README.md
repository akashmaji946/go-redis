# Go-Redis

![go-redis logo](go-redis.png)

A Redis-compatible in-memory key-value store server written in Go. This implementation supports core Redis commands, persistence mechanisms (AOF and RDB), authentication, expiration, transactions, monitoring, and memory management with eviction policies.

## Docs
Access it here: [Docs](https://akashmaji946.github.io/go-redis/)

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
```

## Building

```bash
go build
```

This will create an executable named `go-redis` that you can run.

## Configuration

The server reads configuration from a `redis.conf` file. Create a configuration file with the following options:

```conf
# Data directory (where persistence files are stored)
dir ./data

# AOF Configuration
appendonly yes
appendfilename backup.aof
appendfsync always

# RDB Configuration
save 5 3
dbfilename backup.rdb

# Authentication
requirepass dsl

# Memory Management
maxmemory 256
maxmemory-policy allkeys-random
maxmemory-samples 5
```

## Running

### Basic Usage

The server accepts command-line arguments for configuration file and data directory:

```bash
./go-redis [config_file] [data_directory]
```

**Arguments:**
- `config_file` (optional): Path to the configuration file
  - Default: `./config/redis.conf`
- `data_directory` (optional): Path to the data directory for persistence files
  - Default: `./data/` (or value from config file if specified)

### Examples

**1. Default configuration (uses `./config/redis.conf` and `./data/`):**
```bash
./go-redis
```

**2. Custom configuration file:**
```bash
./go-redis /etc/go-redis/redis.conf
```

**3. Custom configuration and data directory:**
```bash
./go-redis /etc/go-redis/redis.conf /var/lib/go-redis
```

**4. Relative paths:**
```bash
./go-redis ./myconfig.conf ./mydata
```

**5. Absolute paths:**
```bash
./go-redis /home/user/config/redis.conf /home/user/data
```

### Behavior

- If the configuration file doesn't exist, the server will warn and use default settings
- The data directory will be created automatically if it doesn't exist
- If both command-line argument and config file specify a data directory, the command-line argument takes precedence
- The server listens on port **6379** (default Redis port)

### Server Startup

When you run the server, you'll see output like:

```
>>> Go-Redis Server v0.1 <<<
reading the config file...
Data directory: /app/data
listening on port 6379
```

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

### Hash Operations

* **HSET**: Set field in a hash
* **HGET**: Get field from a hash
* **HDEL**: Delete one or more fields from a hash
* **HGETALL**: Get all fields and values in a hash
* **HDELALL**: Delete the entire hash key and all its fields
* **HINCRBY**: Increment a hash field by a given integer
* **HMSET**: Set multiple fields in a hash
* **HEXISTS**: Check if a field exists in a hash
* **HLEN**: Get number of fields in a hash
* **HKEYS**: Get all field names in a hash
* **HVALS**: Get all values in a hash
* **HEXPIRE**: Set TTL on a hash key

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
├── config/
│   └── redis.conf
├── data/
│   ├── backup.aof
│   └── backup.rdb
└── go.mod
```

## Protocol

Implements full Redis RESP:

* Simple Strings
* Bulk Strings
* Arrays
* Integers
* Errors
* Null

## Docker Deployment

**Prerequisites**
- docker
- redis-cli


The project includes a Dockerfile for containerized deployment. See the `Dockerfile` for details.

**Very Quick Docker usage:**
Use an image:
```bash
# Pull the image
docker pull akashmaji/go-redis:latest

# Run it
docker run -d -p 6379:6379 \
  -v $(pwd)/data:/app/data \
  akashmaji/go-redis:latest

## Access it from host
redis-cli

```

**Quick Docker usage:**
Build the image:
```bash
# Build
docker build -t go-redis:latest .

# Run with default config
docker run -d -p 6379:6379 -v $(pwd)/data:/app/data go-redis:latest

# Run with custom paths
docker run -d -p 6379:6379 \
  -v $(pwd)/config/redis.conf:/app/config/redis.conf:ro \
  -v $(pwd)/data:/app/data \
  go-redis:latest /app/config/redis.conf /app/data

## Access it from host
redis-cli
```
See `DOCKER.md` for more detail

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
