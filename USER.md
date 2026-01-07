# Go-Redis: User Guide

This guide provides comprehensive documentation for installing, configuring, and using Go-Redis. It covers everything from building the server to detailed explanations of all available commands.

![go-redis logo](go-redis-logo.png)

---

## Table of Contents

1.  [Features](#features)
2.  [Getting Started](#getting-started)
    -   [Prerequisites](#prerequisites)
    -   [Building the Server](#building-the-server)
    -   [Configuration](#configuration)
    -   [Running the Server](#running-the-server)
    -   [Connecting with `redis-cli`](#connecting-with-redis-cli)
3.  [Docker Deployment](#docker-deployment)
4.  [Command Reference](#command-reference)
    -   [String Operations](#string-operations)
    -   [Key Management](#key-management)
    -   [List Operations](#list-operations)
    -   [Set Operations](#set-operations)
    -   [Hash Operations](#hash-operations)
    -   [Sorted Set Operations](#sorted-set-operations)
    -   [Expiration](#expiration)
    -   [Transactions](#transactions)
    -   [Persistence](#persistence)
    -   [Server & Connection](#server--connection)
    -   [Monitoring & Information](#monitoring--information)
5.  [Persistence Explained](#persistence-explained)
    -   [AOF (Append-Only File)](#aof-append-only-file)
    -   [RDB (Snapshot)](#rdb-snapshot)
6.  [Memory Management](#memory-management)
7.  [Limitations](#limitations)

---

## 1. Features

-   **Core Redis Commands**: Supports a wide subset of commands for strings, lists, sets, hashes, and sorted sets.
-   **In-Memory Storage**: Fast, thread-safe key-value store.
-   **Dual Persistence**:
    -   **AOF (Append-Only File)**: Logs every write operation with configurable `fsync` modes for durability.
    -   **RDB (Redis Database)**: Creates point-in-time snapshots of your dataset.
-   **Key Expiration**: `EXPIRE` and `TTL` support with lazy cleanup.
-   **Authentication**: Secure your server with password protection.
-   **Atomic Transactions**: Group commands using `MULTI`/`EXEC`.
-   **Real-time Monitoring**: Use the `MONITOR` command to inspect live traffic.
-   **Server Introspection**: The `INFO` command provides detailed server statistics.
-   **Memory Management**: Set memory limits (`maxmemory`) and eviction policies.
-   **RESP Compatibility**: Fully compatible with `redis-cli` and the Redis Serialization Protocol.

---

## 2. Getting Started

### Prerequisites

-   **Go**: Version 1.24.4 or later.
-   **redis-cli**: The standard Redis command-line interface, used to connect to the server.
-   **Operating System**: Tested primarily on Linux/Unix environments.

Before you start, ensure no other Redis instance is running on the default port `6379`. You can stop the default Redis service with:
```bash
sudo systemctl stop redis-server.service
```

### Building the Server

1.  Clone the repository or navigate to the project directory.
2.  Build the executable:
    ```bash
    go build
    ```
This command compiles the source code and creates a binary file named `go-redis` in the current directory.

### Configuration

Go-Redis is configured via a `redis.conf` file. The server looks for this file at `./config/redis.conf` by default.

Here is an example `redis.conf`:
```conf
# Set the data directory for persistence files
dir ./data

# Enable AOF persistence
appendonly yes
appendfilename backup.aof
# fsync policy: always, everysec, or no
appendfsync always

# RDB snapshot configuration (save if 3 changes happen in 5 seconds)
save 5 3
dbfilename backup.rdb

# Set a password for authentication
requirepass your-secret-password

# Set a memory limit (e.g., 1GB) and eviction policy
maxmemory 1073741824
maxmemory-policy allkeys-random
```

### Running the Server

You can run the server with default settings or provide custom paths for the configuration and data directory.

**Syntax:**
```bash
./go-redis [config_file_path] [data_directory_path]
```

**Examples:**

-   **Run with defaults:** (Uses `./config/redis.conf` and `./data/`)
    ```bash
    ./go-redis
    ```

-   **Run with a custom config file:**
    ```bash
    ./go-redis /etc/go-redis/redis.conf
    ```

-   **Run with custom config and data directories:**
    ```bash
    ./go-redis ./my.conf ./my-data
    ```

The server will start and listen on port **6379**.

### Connecting with `redis-cli`

Open a new terminal and connect to the server:
```bash
redis-cli -p 6379
```

If you have authentication enabled in your `redis.conf`, you must authenticate first:
```
127.0.0.1:6379> AUTH your-secret-password
OK
```

You are now ready to execute commands!

---

## 3. Docker Deployment

For ease of use, you can run Go-Redis in a Docker container.

### Pull and Run the Pre-built Image

This is the fastest way to get started.

```bash
# 1. Pull the latest image from Docker Hub
docker pull akashmaji/go-redis:latest

# 2. Run the container, mapping the data directory to your host
docker run -d -p 6379:6379 \
  -v $(pwd)/data:/app/data \
  akashmaji/go-redis:latest

# 3. Connect from your host machine
redis-cli
```

### Build and Run Your Own Image

If you've made changes to the code, you can build the image yourself.

```bash
# 1. Build the Docker image
docker build -t go-redis:latest .

# 2. Run the container
# This example mounts a custom config file and data directory
docker run -d -p 6379:6379 \
  -v $(pwd)/config/redis.conf:/app/config/redis.conf:ro \
  -v $(pwd)/data:/app/data \
  go-redis:latest
```

For more advanced Docker usage, see `DOCKER.md`.

---

## 4. Command Reference

### String Operations

| Command | Description | Syntax | 
|---|---|---|
| `GET` | Get the value of a key. | `GET <key>` |
| `SET` | Set the string value of a key. | `SET <key> <value>` |
| `INCR` | Increment the integer value of a key by one. | `INCR <key>` |
| `DECR` | Decrement the integer value of a key by one. | `DECR <key>` |
| `INCRBY`| Increment the integer value of a key by a given amount. | `INCRBY <key> <amount>` |
| `DECRBY`| Decrement the integer value of a key by a given amount. | `DECRBY <key> <amount>` |
| `MGET` | Get the values of all the given keys. | `MGET <key> [key ...]` |
| `MSET` | Set multiple keys to multiple values. | `MSET <key> <value> [key value ...]` |

### Key Management

| Command | Description | Syntax | 
|---|---|---|
| `DEL` | Delete one or more keys. | `DEL <key> [key ...]` |
| `EXISTS`| Check if a key exists. | `EXISTS <key>` |
| `KEYS` | Find all keys matching a given pattern. **Warning: O(N) complexity.** | `KEYS <pattern>` |
| `RENAME`| Rename a key. | `RENAME <key> <newkey>` |
| `TYPE` | Get the type of value stored at a key. | `TYPE <key>` |
| `FLUSHDB`| Remove all keys from the database. **Warning: Irreversible.** | `FLUSHDB` |
| `DBSIZE`| Return the number of keys in the database. | `DBSIZE` |

### List Operations

| Command | Description | Syntax | 
|---|---|---|
| `LPUSH` | Prepend one or more values to a list. | `LPUSH <key> <value> [value ...]` |
| `RPUSH` | Append one or more values to a list. | `RPUSH <key> <value> [value ...]` |
| `LPOP` | Remove and get the first element in a list. | `LPOP <key>` |
| `RPOP` | Remove and get the last element in a list. | `RPOP <key>` |
| `LRANGE`| Get a range of elements from a list. | `LRANGE <key> <start> <stop>` |
| `LLEN` | Get the length of a list. | `LLEN <key>` |
| `LINDEX`| Get an element from a list by its index. | `LINDEX <key> <index>` |
| `LGET` | **(Custom)** Get all elements in a list. | `LGET <key>` |

### Set Operations

| Command | Description | Syntax | 
|---|---|---|
| `SADD` | Add one or more members to a set. | `SADD <key> <member> [member ...]` |
| `SREM` | Remove one or more members from a set. | `SREM <key> <member> [member ...]` |
| `SMEMBERS`| Get all the members in a set. | `SMEMBERS <key>` |
| `SISMEMBER`| Determine if a given value is a member of a set. | `SISMEMBER <key> <member>` |
| `SCARD` | Get the number of members in a set. | `SCARD <key>` |

### Hash Operations

Hashes are maps between string fields and string values.

| Command | Description | Syntax | 
|---|---|---|
| `HSET` | Set the string value of a hash field. | `HSET <key> <field> <value>` |
| `HGET` | Get the value of a hash field. | `HGET <key> <field>` |
| `HDEL` | Delete one or more hash fields. | `HDEL <key> <field> [field ...]` |
| `HGETALL`| Get all the fields and values in a hash. | `HGETALL <key>` |
| `HINCRBY`| Increment the integer value of a hash field by a given number. | `HINCRBY <key> <field> <increment>` |
| `HEXISTS`| Determine if a hash field exists. | `HEXISTS <key> <field>` |
| `HLEN` | Get the number of fields in a hash. | `HLEN <key>` |
| `HKEYS` | Get all the fields in a hash. | `HKEYS <key>` |
| `HVALS` | Get all the values in a hash. | `HVALS <key>` |
| `HMSET` | Set multiple hash fields to multiple values. | `HMSET <key> <field> <value> [field value ...]` |
| `HDELALL`| **(Custom)** Delete the entire hash. | `HDELALL <key>` |
| `HEXPIRE`| **(Custom)** Set a TTL on a hash key. | `HEXPIRE <key> <seconds>` |

### Sorted Set Operations

| Command | Description | Syntax | 
|---|---|---|
| `ZADD` | Add one or more members to a sorted set, or update its score. | `ZADD <key> <score> <member> [score member ...]` |
| `ZREM` | Remove one or more members from a sorted set. | `ZREM <key> <member> [member ...]` |
| `ZSCORE`| Get the score associated with the given member in a sorted set. | `ZSCORE <key> <member>` |
| `ZCARD` | Get the number of members in a sorted set. | `ZCARD <key>` |
| `ZRANGE`| Return a range of members in a sorted set, by index. | `ZRANGE <key> <start> <stop> [WITHSCORES]` |
| `ZREVRANGE`| Return a range of members in a sorted set, by index, ordered from high to low scores. | `ZREVRANGE <key> <start> <stop> [WITHSCORES]` |
| `ZGET` | **(Custom)** Get the score of a member or all members with scores. | `ZGET <key> [member]` |

### Expiration

| Command | Description | Syntax | 
|---|---|---|
| `EXPIRE`| Set a timeout on a key. | `EXPIRE <key> <seconds>` |
| `TTL` | Get the remaining time to live of a key. | `TTL <key>` |
| `PERSIST`| Remove the expiration from a key. | `PERSIST <key>` |

### Transactions

| Command | Description | Syntax | 
|---|---|---|
| `MULTI` | Mark the start of a transaction block. | `MULTI` |
| `EXEC` | Execute all commands queued in a transaction. | `EXEC` |
| `DISCARD`| Discard all commands issued after `MULTI`. | `DISCARD` |

### Persistence

| Command | Description | Syntax | 
|---|---|---|
| `SAVE` | **Synchronously** save the dataset to disk. **Blocks the server.** | `SAVE` |
| `BGSAVE`| **Asynchronously** save the dataset to disk in the background. | `BGSAVE` |
| `BGREWRITEAOF`| Asynchronously rewrite the append-only file. | `BGREWRITEAOF` |

### Server & Connection

| Command | Description | Syntax | 
|---|---|---|
| `PING` | Check the connection. Returns `PONG` or an optional message. | `PING [message]` |
| `AUTH` | Authenticate to the server. | `AUTH <password>` |
| `COMMAND`| A simple command that returns `OK`. | `COMMAND` |
| `COMMANDS`| **(Custom)** List all available commands. | `COMMANDS` |

### Monitoring & Information

| Command | Description | Syntax | 
|---|---|---|
| `INFO` | Get information and statistics about the server. | `INFO` |
| `MONITOR`| Listen for all requests received by the server in real-time. | `MONITOR` |

---

## 5. Persistence Explained

Go-Redis offers two mechanisms to persist your data on disk.

### AOF (Append-Only File)

The AOF logs every write operation received by the server. This data is replayed on startup to reconstruct the original dataset.

-   **Durability**: Offers better durability than RDB. You can configure how often data is synced to disk (`appendfsync`).
-   **`fsync` Modes**:
    -   `always`: Sync after every write. Slowest but safest.
    -   `everysec`: Sync once per second in the background. A good balance.
    -   `no`: Let the operating system handle syncing. Fastest but least safe.
-   **Compaction**: The `BGREWRITEAOF` command rebuilds the AOF to be as small as possible.

### RDB (Snapshot)

The RDB persistence performs point-in-time snapshots of your dataset at specified intervals.

-   **Performance**: Forking a background process for `BGSAVE` has minimal impact on the main server.
-   **Configuration**: You can configure `save` rules in `redis.conf` (e.g., `save 60 1000` saves if there are 1000 key changes in 60 seconds).
-   **Integrity**: RDB files are saved with a SHA-256 checksum to prevent corruption.

---

## 6. Memory Management

You can control memory usage with the `maxmemory` directive in `redis.conf`.

-   **`maxmemory <bytes>`**: Sets the maximum memory limit.
-   **`maxmemory-policy <policy>`**: Defines what happens when the limit is reached.
    -   `no-eviction`: (Default) Return an error on write commands.
    -   `allkeys-random`: Evict random keys to make space.

---

## 7. Limitations

Go-Redis is an educational project and intentionally omits certain advanced Redis features:

-   Single database only (no `SELECT` command).
-   No replication or clustering.
-   No Pub/Sub messaging.
-   No Lua scripting.
-   No `WATCH` command for optimistic locking in transactions.

---

For bug reports or support, please contact **Akash Maji** at `akashmaji@iisc.ac.in`.