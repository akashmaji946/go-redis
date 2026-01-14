# Go-Redis-Server (v1.0)
![go-redis logo](images/go-redis-logo.png)

A lightweight, multi-threaded Redis server implementation in Go.

A Redis-compatible in-memory key-value store server written in Go. This implementation supports core Redis commands, persistence mechanisms (AOF and RDB), authentication, expiration, transactions, monitoring, and memory management with eviction policies.

## Docs
- Refer to our docs for full guide on usage and description of commands.
- Access it here: [Docs](https://akashmaji946.github.io/go-redis/)

## Features

- **Core Commands**: GET, SET, DEL, DELETE, EXISTS, KEYS, DBSIZE, FLUSHDB, DROPDB, STRLEN, SIZE, SELECT, ...
- **In-Memory Storage**: Fast key-value store supporting Strings, Lists, Sets, and Hashes.
- **Persistence**:
  - **AOF (Append-Only File)**: Logs every write operation with configurable fsync modes
  - **RDB (Redis Database)**: Point-in-time snapshots with automatic triggers
- **Pub/Sub**: Real-time messaging with PUBLISH, SUBSCRIBE, PSUBSCRIBE, and pattern support
- **Expiration**: EXPIRE and TTL support for keys with automatic cleanup
- **Authentication**: Password-based authentication with configurable requirements
- **Transactions**: MULTI, EXEC, DISCARD for atomic command execution
- **Monitoring**: MONITOR command for real-time command streaming
- **Server Info**: INFO command for server statistics and metrics
- **Memory Management**: Configurable memory limits with eviction policies
- **Background Operations**: BGSAVE and BGREWRITEAOF for non-blocking persistence
- **Optimistic Locking**: WATCH and UNWATCH support for safe concurrent transactions
- **Thread-Safe**: Concurrent access with read-write locks (RWMutex)
- **Redis Protocol**: Full RESP (Redis Serialization Protocol) compatibility
- **Checksum Verification**: SHA-256 checksums for RDB data integrity
- **Concurrency**: Handles multiple client connections concurrently.
- **Transactions**: Basic `MULTI`, `EXEC`, `DISCARD` support.
- **Eviction**: LRU/Random eviction policies when maxmemory is reached.

## Quick Access:
Go to your terminal and access it here:
```bash
# use redis-cli to access the hosted sever
redis-cli -h go.akashmaji.me -p 7380 --tls
# use pass `dsl` when connected
# 
```

## Prerequisites

- **Go 1.24** or later
- **redis-cli** (for testing and connecting to the server)
- **Linux/Unix environment** (tested on Linux)

## Stopping Default Process (if any)
We will be using port 7379 as default (See `config/redis.conf` file).
```bash
# find and kill process at 7379 forcefully
sudo kill -9 $(sudo lsof -t -i :7379)
```

However, if you want to use port 6379 then do these if redis-server is running:
```bash
## ONE WAY:
sudo systemctl stop redis-server.service
sudo systemctl status redis-server.service
# start later
# systemctl start redis-server.service

## ANOTHER WAY:
# find and kill forcefully
sudo kill -9 $(sudo lsof -t -i :6379)
# start it later
- # systemctl start redis-server.service
```

## Building

Build as:
```bash
# from within project_root
cd cmd
go build -o go-redis .
```
This will create an executable named `go-redis` that you can run.

Run as:
```bash
# from within project_root
cd cmd
./go-redis ../config/redis.conf ../data/
```

## Running
Run the script `run_server.sh` as:
```bash
# from within project_root
chmod +x run_server.sh
./run.sh ./config/redis.conf ./data/
```

## Connecting to local server so built and ran
```bash
# TCP
redis-cli -p 7379 
# TLS
redis-cli -p 7380 --tls
```

## Configuration

The server reads configuration from a `redis.conf` file. Create a configuration file with the following options:

```conf
# Data directory (where persistence files are stored)
dir ./data

# Server Configuration
port 7379

# Command case sensitivity (yes|no)
sensitive no

# AOF Configuration
appendonly yes
appendfilename backup.aof
appendfsync always

# RDB Configuration
save 5 3
dbfilename backup.rdb

# Authentication
requirepass dsl

# Encryption (AES-GCM)
encrypt yes
nonce 1234567890

# Memory Management
maxmemory 1024
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

When you run the server (or run `./run_server,sh`), you'll see console output or log like:

Console Output:
```bash
[INFO] Building go-redis...
[INFO] Running go-redis...

                   ██████╗  ██████╗ 
                  ██╔════╝ ██╔═══██╗
                  ██║  ███╗██║   ██║
                  ██║   ██║██║   ██║
                  ╚██████╔╝╚██████╔╝
                   ╚═════╝  ╚═════╝ 

           ██████╗ ███████╗██████╗ ██╗███████╗
           ██╔══██╗██╔════╝██╔══██╗██║██╔════╝
           ██████╔╝█████╗  ██║  ██║██║███████╗
           ██╔══██╗██╔══╝  ██║  ██║██║╚════██║
           ██║  ██║███████╗██████╔╝██║███████║
           ╚═╝  ╚═╝╚══════╝╚═════╝ ╚═╝╚══════╝

   ███████╗███████╗██████╗ ██╗   ██╗███████╗██████╗ 
   ██╔════╝██╔════╝██╔══██╗██║   ██║██╔════╝██╔══██╗
   ███████╗█████╗  ██████╔╝██║   ██║█████╗  ██████╔╝
   ╚════██║██╔══╝  ██╔══██╗╚██╗ ██╔╝██╔══╝  ██╔══██╗
   ███████║███████╗██║  ██║ ╚████╔╝ ███████╗██║  ██║
   ╚══════╝╚══════╝╚═╝  ╚═╝  ╚═══╝  ╚══════╝╚═╝  ╚═╝
             >>> Go-Redis Server v1.0 <<<      

[INFO] Go-Redis Server is up on port: 7379 (TCP)
[INFO] Go-Redis Server is up on port: 7380 (TLS)
```
Log Output:
```bash
2026/01/14 09:17:18 >>>> Go-Redis Server v1.0 <<<<
2026/01/14 09:17:18 reading the config file...
2026/01/14 09:17:18 [INFO] config file   : ./config/redis.conf
2026/01/14 09:17:18 [INFO] data directory: ./data/
2026/01/14 09:17:18 using data directory from command line: /home/akashmaji/Documents/go-redis/data
2026/01/14 09:17:18 data directory: /home/akashmaji/Documents/go-redis/data
2026/01/14 09:17:18 syncing records
2026/01/14 09:17:18 error in ReadArray: EOF
2026/01/14 09:17:18 records synchronized: 0
2026/01/14 09:17:18 Listening on 0.0.0.0:7379 (TCP)
2026/01/14 09:17:18 Listening on 0.0.0.0:7380 (TLS)
```

### Server Connection
Multiple clients can connect to the server
```bash
redis-cli -p <port_number>
# e.g. redis-cli -p 7379
```

## Available Commands

**Connection**
`AUTH`, `USERADD`, `PASSWD`, `USERS`, `WHOAMI`, `PING`, `SELECT`, `SEL`

**Persistence**
`BGREWRITEAOF`, `BGSAVE`, `SAVE`

**Server**
`COMMAND`, `COMMANDS`, `DBSIZE`, `FLUSHDB`, `DROPDB`, `INFO`, `MONITOR`, `SIZE`

**String**
`DECR`, `DECRBY`, `GET`, `INCR`, `INCRBY`, `MGET`, `MSET`, `SET`, `STRLEN`

**Key**
`DEL`, `DELETE`, `EXISTS`, `EXPIRE`, `KEYS`, `PERSIST`, `RENAME`, `TTL`, `TYPE`

**Transaction**
`DISCARD`, `EXEC`, `MULTI`, `UNWATCH`, `WATCH`

**Hash**
`HDEL`, `HDELALL`, `HEXISTS`, `HEXPIRE`, `HGET`, `HGETALL`, `HINCRBY`, `HKEYS`, `HLEN`, `HMSET`, `HSET`, `HVALS`

**List**
`LGET`, `LINDEX`, `LLEN`, `LPOP`, `LPUSH`, `LRANGE`, `RPOP`, `RPUSH`

**PubSub**
`PSUBSCRIBE`, `PSUB`, `PUBLISH`, `PUB`, `PUNSUBSCRIBE`, `PUNSUB`, `SUBSCRIBE`, `SUB`, `UNSUBSCRIBE`, `UNSUB`

**Set**
`SADD`, `SCARD`, `SDIFF`, `SINTER`, `SISMEMBER`, `SMEMBERS`, `SRANDMEMBER`, `SREM`, `SUNION`

**ZSet**
`ZADD`, `ZCARD`, `ZGET`, `ZRANGE`, `ZREM`, `ZREVRANGE`, `ZSCORE`

#### For more details:
- Refer to our docs for full guide on usage and description of commands.
- Access it here: [Docs](https://akashmaji946.github.io/go-redis/)

## Persistence
### AOF

### AOF
## Getting Started

* Logs every write
* Replayed on startup
* Supports `always`, `everysec`, `no` fsync modes
* Rewritten using BGREWRITEAOF
1. **Build the server:**
   ```bash
   go build -o go-redis .
   ```

### RDB
2. **Run the server:**
   ```bash
   ./go-redis
   ```
   *Optionally specify config file and data directory:*
   ```bash
   ./go-redis ./config/redis.conf ./data/
   ```

* Snapshot-based persistence
* Triggered via `save` rules
* Uses Go `gob` encoding
* SHA-256 checksum verification

3. **Connect using `redis-cli`:**
   ```bash
   # normal TCP
   redis-cli -p 7379
   # secured TLS
   redis-cli -p 7380
   ```

## Memory Management

### Eviction Policies

* no-eviction
* allkeys-random
* allkeys-lru
* allkeys-lfu

## Architecture

* RWMutex-based concurrency
* Per-connection goroutines
* Background workers for persistence
* Lazy expiration
* Automatic eviction

## Project Structure

```bash
>> tree .
.
├── bench
│   └── benchmark.txt
├── cmd
│   ├── go-redis.service
│   └── main.go
├── config
│   ├── akashmaji.me
│   ├── cert_gen.sh
│   ├── cert.pem
│   ├── go-redis.service
│   ├── key.pem
│   └── redis.conf
├── data
├── Dockerfile
├── DOCKER.md
├── DOCS.md
├── go.mod
├── go-redis.code-workspace
├── go.sum
├── images
│   ├── go-redis-logo.png
│   └── go-redis.png
├── internal
│   ├── common
│   │   ├── aof.go
│   │   ├── appstate.go
│   │   ├── client.go
│   │   ├── conf.go
│   │   ├── constants.go
│   │   ├── helpers.go
│   │   ├── info.go
│   │   ├── rdb.go
│   │   ├── transaction.go
│   │   ├── value.go
│   │   └── writer.go
│   ├── database
│   │   ├── database.go
│   │   └── mem.go
│   ├── handlers
│   │   ├── handler_connection.go
│   │   ├── handler_generic.go
│   │   ├── handler_hash.go
│   │   ├── handler_key.go
│   │   ├── handler_list.go
│   │   ├── handler_persistence.go
│   │   ├── handler_pubsub.go
│   │   ├── handler_set.go
│   │   ├── handlers.go
│   │   ├── handler_string.go
│   │   ├── handler_transaction.go
│   │   └── handler_zset.go
│   └── info
├── LICENSE
├── README.md
├── run_clean.sh
├── run_client.sh
├── run_server.sh
├── static
│   ├── commands.json
│   └── notes.txt
└── VSCODE.md

12 directories, 50 files
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
docker run -d -p 7379:7379 \
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
docker run -d -p 7379:7379 -v $(pwd)/data:/app/data go-redis:latest

# Run with custom paths
docker run -d -p 7379:7379 \
  -v $(pwd)/config/redis.conf:/app/config/redis.conf:ro \
  -v $(pwd)/data:/app/data \
  go-redis:latest /app/config/redis.conf /app/data

## Access it from host
redis-cli
```
See `DOCKER.md` for more detail

## Limitations

* No replication
* No Lua scripting

## License

Educational project implementing Redis-like functionality in Go.

## Version

**v1.0**

## Author
**Akash Maji (akashmaji@iisc.ac.in) - Contact for bugs and support**
## Configuration
Configuration is handled via `redis.conf`. See `config/redis.conf` for available options like port, persistence settings, and memory limits.
