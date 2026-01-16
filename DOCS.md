---
layout: default
title: Go-Redis-Server Documentation
---

![Go-Redis-Server Logo](images/go-redis-logo.png)

# Go-Redis-Server: The Complete Guide (v1.0)

Welcome to the complete developer and user documentation for **Go-Redis**, a lightweight, multi-threaded, Redis-compatible server implemented in Go.

This document provides a deep dive into the project's features, architecture, and usage. Whether you are a developer looking to understand the internals, or a user wanting to get started, this guide is for you.

---

## Table of Contents

1.  [**Overview & Features**](#1-overview--features)
    -   [Design Goals](#design-goals)
2.  [**Getting Started**](#2-getting-started)
    -   [Prerequisites](#prerequisites)
    -   [Building from Source](#building-from-source)
    -   [Configuration](#configuration)
    -   [Running the Server](#running-the-server)
    -   [Connecting with `redis-cli`](#connecting-with-redis-cli)
3.  [**Docker Deployment**](#3-docker-deployment)
    -   [Using the Pre-built Image](#using-the-pre-built-image)
    -   [Building a Custom Image](#building-a-custom-image)
4.  [**Command Reference**](#4-command-reference)
    -   [String Operations](#string-operations)
    -   [Key Management](#key-management)
    -   [List Operations](#list-operations)
    -   [Set Operations](#set-operations)
    -   [Hash Operations](#hash-operations)
    -   [Sorted Set Operations](#sorted-set-operations)
    -   [Expiration Commands](#expiration-commands)
    -   [Transactions](#transactions)
    -   [Persistence Commands](#persistence-commands)
    -   [Server & Connection](#server--connection)
    -   [Monitoring & Information](#monitoring--information)
5.  [**Internal Architecture**](#5-internal-architecture)
    -   [High-Level Diagram](#high-level-diagram)
    -   [Project Structure & Core Components](#project-structure--core-components)
    -   [Concurrency Model](#concurrency-model)
    -   [Command Execution Pipeline](#command-execution-pipeline)
    -   [Data Model](#data-model)
    -   [RESP Protocol Support](#resp-protocol-support)
6.  [**Core Subsystems Explained**](#6-core-subsystems-explained)
    -   [Persistence: AOF vs. RDB](#persistence-aof-vs-rdb)
    -   [Memory Management & Eviction](#memory-management--eviction)
7.  [**Limitations**](#7-limitations)
8.  [**Contact & Support**](#8-contact--support)

---

## 1. Overview & Features

Go-Redis is a Redis-compatible in-memory key-value store server written in Go. It is designed to be a learning tool for understanding how a database like Redis works under the hood, while also being a functional server for development and testing purposes.

-   **Broad Command Support**: Implements a rich subset of commands for Strings, Lists, Sets, Hashes, and Sorted Sets.
-   **Dual Persistence Model**: 
    -   **AOF (Append-Only File)**: Logs every write operation with configurable `fsync` modes for high durability.
    -   **RDB (Redis Database)**: Creates point-in-time snapshots for fast startups and backups.
    -   **Encrypted Storage**: Optional AES-GCM encryption for all persistence files and user credentials.
-   **Key Expiration**: Supports `EXPIRE`, `TTL`, and `PERSIST` with lazy (on-access) key removal.
-   **Atomic Transactions**: Group commands in `MULTI`/`EXEC` blocks with `WATCH`/`UNWATCH` for optimistic locking.
-   **User Management**: Multi-user support with role-based access control (Admin/User).
-   **Introspection & Monitoring**: 
    -   `INFO` provides a detailed look into server statistics.
    -   `MONITOR` streams live command processing for debugging.
-   **Memory Management**: Allows setting a `maxmemory` limit and an eviction policy.
-   **Pub/Sub Messaging**: Decoupled real-time communication between publishers and subscribers.
-   **RESP Compatible**: Fully compatible with the Redis Serialization Protocol (RESP), allowing `redis-cli` and other standard clients to connect seamlessly.
-   **Thread-Safe by Design**: Handles multiple concurrent clients safely using a single database protected by read-write locks.

### Design Goals

-   **Educational**: To provide a clear, readable, and well-documented codebase for those learning about database internals, concurrency in Go, and network programming.
-   **Redis-Compatible**: To work out-of-the-box with `redis-cli`.
-   **Correctness over Performance**: To prioritize a simple, correct, and deterministic implementation over complex performance optimizations.

---

## 2. Getting Started

### Prerequisites

-   **Go**: Version 1.24.4 or later.
-   **`redis-cli`**: The standard Redis command-line tool.
-   **OS**: Tested on Linux/Unix environments.

> **Note**: Before starting, ensure no other Redis instance is running on port `6379`. You can stop a default Redis service using `sudo systemctl stop redis-server`.

### Building from Source

Clone the repository and run the build command:
```bash
go build
```
This creates a `go-redis` executable in your project directory.

### Configuration

The server is configured using a `redis.conf` file. By default, it looks for `./config/redis.conf`.

**Example `redis.conf`:**
```conf
## how many databases?
databases 2

## encryption
encrypt no
nonce 1234567890

### store data where? directory
dir ./data

### which persistence technique? AOF vs RDB
## AOF parameters
appendonly yes
appendfilename aoffile 
appendfsync always
## RDB parameters
# save 900 1 #if in 900 sec, 1 key chnages, save db
# save 300 10 
save 1 1
dbfilename rdbfile

### auth
requirepass dsl

### memory and eviction management
## default is bytes (example usage: 256, 256kb, 256mb)
maxmemory 256mb
# policies: default is no-eviction
# no-eviction, allkeys-random, allkeys-lru, allkeys-lfu, 
# volatile-lru, volatile-lfu, volatile-random
maxmemory-policy allkeys-lru
maxmemory-samples 50
```

### Running the Server

The server can be started with default paths or custom ones.

**Syntax:**
```bash
./go-redis [config_file_path] [data_directory_path]
```

-   **With defaults:**
    ```bash
    ./go-redis
    ```
-   **With custom paths:**
    ```bash
    ./go-redis ./my.conf ./my-data
    ```

The server will log its startup process and listen on port `7379`.

### Connecting with `redis-cli`

Open a new terminal and connect:
```bash
redis-cli -p 7379
```
If you've set `requirepass`, authenticate your session:
```
127.0.0.1:6379> AUTH user-name your-secret-password
OK
```
You're all set to run commands!

---

## 3. Docker Deployment

### Using the Pre-built Image

The quickest way to run Go-Redis is with the official Docker image.

```bash
# 1. Pull the image from Docker Hub
docker pull akashmaji/go-redis:latest

# 2. Run the container, mounting a volume for persistent data (with default config)
docker run -d -p 7379:7379 \
  -v $(pwd)/.test:/app/data \
  -v $(pwd)/.test/redis.conf:/app/config/redis.conf:ro \
  --name go-redis \
  akashmaji/go-redis:latest

# 4. Connect from your host
redis-cli -p 7379
```

### Building a Custom Image

If you've modified the code, you can build your own image from the `Dockerfile`.

```bash
# 1. Build the image
docker build -t go-redis:latest .

# 2. Run the container
# This example mounts a custom config file and data directory
docker run -d -p 7379:7379 -p 7380:7380 \
  -v $(pwd)/.test:/app/data \
  -v $(pwd)/.test/redis.conf:/app/config/redis.conf:ro \
  --name go-redis \
  go-redis:latest

# 3.1 Connect from your host
redis-cli -p 7379
redis-cli -p 7380  --tls --insecure
# 3. Check logs
docker logs go-redis

# 4. Stop and remove the container
docker stop go-redis
docker rm go-redis

# 5. Optionally tag and push to Docker Hub
docker tag go-redis:latest akashmaji/go-redis:latest
docker push akashmaji/go-redis:latest
```

---

## 4. Command Reference

Below is a categorized list of all supported commands.

### String Operations

| Command | Description |
|---|---|
| `GET <key>` | Retrieve the string value stored at the specified key. Returns NULL if the key does not exist. Returns an error if the key holds a non-string data type. |
| `SET <key> <value>` | Set the string value of a key. If the key already exists, its value is overwritten regardless of its type. Any previous expiration is discarded. |
| `INCR <key>` | Increment the integer value stored at the specified key by one. If the key does not exist, it is initialized to 0 before performing the increment. |
| `DECR <key>` | Decrement the integer value stored at the specified key by one. If the key does not exist, it is initialized to 0 before performing the decrement. |
| `INCRBY <key> <increment>` | Increment the integer value stored at the specified key by the given increment amount. The increment can be negative to perform a decrement. |
| `DECRBY <key> <decrement>` | Decrement the integer value stored at the specified key by the given decrement amount. |
| `MGET <key> [key ...]` | Retrieve the values of multiple keys in a single operation. Returns an array of values in the same order as the requested keys. For keys that do not exist, NULL is returned. |
| `MSET <key> <value> [key value ...]` | Set multiple key-value pairs in a single atomic operation. If any keys already exist, their values are overwritten. |
| `STRLEN <key>` | Return the length of the string value stored at the specified key. Returns 0 if the key does not exist. |


### Key Management

| Command | Description |
|---|---|
| `DEL <key> [key ...]` | Delete one or more keys from the database. Keys that do not exist are silently ignored. Returns the number of keys that were actually removed. |
| `DELETE <key> [key ...]` | Alias for DEL. Delete one or more keys from the database. |
| `EXISTS <key> [key ...]` | Check if one or more keys exist in the database. Returns an integer count of how many of the specified keys exist. |
| `KEYS <pattern>` | Find all keys matching the specified glob-style pattern. Supports `*`, `?`, `[abc]`, and `[^abc]` patterns. |
| `RENAME <key> <newkey>` | Rename a key to a new name. Returns 1 if successful, 0 if the source key does not exist or destination already exists. |
| `TYPE <key>` | Return the data type of the value stored at the specified key. Returns STRING, LIST, SET, HASH, ZSET, HLL, or none. |
| `EXPIRE <key> <seconds>` | Set a timeout on the specified key. After the timeout expires, the key will be automatically deleted. |
| `TTL <key>` | Return the remaining time to live (in seconds) of a key. Returns -1 if no expiration, -2 if key does not exist. |
| `PERSIST <key>` | Remove the expiration timeout from a key, making it persistent (never expires). |


### List Operations

| Command | Description |
|---|---|
| `LPUSH <key> <value> [value ...]` | Insert one or more values at the head (left side) of a list. If the key does not exist, a new list is created. Returns the length of the list after the push. |
| `RPUSH <key> <value> [value ...]` | Append one or more values to the tail (right side) of a list. If the key does not exist, a new list is created. Returns the length of the list after the push. |
| `LPOP <key>` | Remove and return the first element (head) of a list. Returns NULL if the key does not exist. If the list becomes empty, the key is deleted. |
| `RPOP <key>` | Remove and return the last element (tail) of a list. Returns NULL if the key does not exist. If the list becomes empty, the key is deleted. |
| `LRANGE <key> <start> <stop>` | Retrieve a range of elements from a list. Indices are zero-based and inclusive. Negative indices count from the end. |
| `LLEN <key>` | Return the length (number of elements) of a list. Returns 0 if the key does not exist. |
| `LINDEX <key> <index>` | Retrieve the element at the specified index in a list. Negative indices count from the end (-1 is the last element). |
| `LGET <key>` | Retrieve all elements from a list. Equivalent to `LRANGE key 0 -1`. |


### Set Operations

| Command | Description |
|---|---|
| `SADD <key> <member> [member ...]` | Add one or more members to a set. If the key does not exist, a new set is created. Returns the number of members actually added (not counting duplicates). |
| `SREM <key> <member> [member ...]` | Remove one or more members from a set. Members that do not exist are silently ignored. Returns the number of members actually removed. |
| `SMEMBERS <key>` | Return all members of the set stored at the specified key. Returns an empty array if the key does not exist. |
| `SISMEMBER <key> <member>` | Determine if a given value is a member of the set. Returns 1 if the member exists, 0 otherwise. |
| `SCARD <key>` | Return the cardinality (number of members) of a set. Returns 0 if the key does not exist. |
| `SDIFF <key> [key ...]` | Return the members of the set resulting from the difference between the first set and all successive sets. |
| `SINTER <key> [key ...]` | Return the members of the set resulting from the intersection of all specified sets. |
| `SUNION <key> [key ...]` | Return the members of the set resulting from the union of all specified sets. |
| `SRANDMEMBER <key> [count]` | Return one or more random members from a set. With positive count, returns distinct members. With negative count, may include duplicates. |


### Hash Operations

| Command | Description |
|---|---|
| `HSET <key> <field> <value> [field value ...]` | Set one or more field-value pairs in a hash. If the key does not exist, a new hash is created. Returns the number of new fields added. |
| `HGET <key> <field>` | Retrieve the value associated with a specific field in a hash. Returns NULL if the key or field does not exist. |
| `HDEL <key> <field> [field ...]` | Delete one or more fields from a hash. Fields that do not exist are silently ignored. Returns the number of fields actually removed. |
| `HGETALL <key>` | Retrieve all fields and their values from a hash. Returns an array: [field1, value1, field2, value2, ...]. |
| `HINCRBY <key> <field> <increment>` | Increment the integer value of a hash field by the specified amount. If the field does not exist, it is initialized to 0. |
| `HEXISTS <key> <field>` | Check if a specific field exists within a hash. Returns 1 if exists, 0 otherwise. |
| `HLEN <key>` | Return the number of fields contained in a hash. Returns 0 if the key does not exist. |
| `HKEYS <key>` | Retrieve all field names from a hash. Returns an empty array if the key does not exist. |
| `HVALS <key>` | Retrieve all values from a hash. Returns an empty array if the key does not exist. |
| `HMGET <key> <field> [field ...]` | Retrieve the values associated with the specified fields in a hash. Returns NULL for fields that do not exist. |
| `HMSET <key> <field> <value> [field value ...]` | Set multiple field-value pairs in a hash. Deprecated in favor of HSET but kept for backward compatibility. |
| `HDELALL <key>` | Delete all fields from a hash, effectively clearing the entire hash. Returns the number of fields deleted. |
| `HEXPIRE <key> <field> <seconds>` | Set an expiration time on a specific field within a hash. Custom extension for fine-grained expiration control. |


### Sorted Set Operations

| Command | Description |
|---|---|
| `ZADD <key> <score> <member> [score member ...]` | Add one or more members with their scores to a sorted set. If a member already exists, its score is updated. Returns the number of new members added. |
| `ZREM <key> <member> [member ...]` | Remove one or more members from a sorted set. Members that do not exist are silently ignored. Returns the number of members actually removed. |
| `ZSCORE <key> <member>` | Return the score of a member in a sorted set. Returns NULL if the key or member does not exist. |
| `ZCARD <key>` | Return the cardinality (number of members) of a sorted set. Returns 0 if the key does not exist. |
| `ZRANGE <key> <start> <stop> [WITHSCORES]` | Return a range of members from a sorted set, ordered by score from lowest to highest. With WITHSCORES, includes scores in the result. |
| `ZREVRANGE <key> <start> <stop> [WITHSCORES]` | Return a range of members from a sorted set, ordered by score from highest to lowest (reverse order). |
| `ZGET <key> [member]` | Retrieve the score of a specific member, or all members with their scores from a sorted set. Custom convenience command. |


### HyperLogLog Operations

| Command | Description |
|---|---|
| `PFADD <key> <element> [element ...]` | Add one or more elements to a HyperLogLog probabilistic data structure. HyperLogLog provides approximate cardinality estimation using only ~12KB of memory. Returns 1 if at least one internal register was altered. |
| `PFCOUNT <key> [key ...]` | Return the approximated cardinality (number of unique elements) observed by the HyperLogLog(s). When called with multiple keys, returns the cardinality of the union. Standard error rate is approximately 0.81%. |
| `PFDEBUG <key>` | Return internal debugging information about a HyperLogLog including encoding type (sparse/dense), number of registers, and estimated cardinality. |
| `PFMERGE <destkey> <sourcekey> [sourcekey ...]` | Merge multiple HyperLogLog values into a single destination HyperLogLog. The merged result approximates the cardinality of the union of all sources. |


### Pub/Sub Operations

| Command | Description |
|---|---|
| `PUBLISH <channel> <message>` | Post a message to a channel for delivery to all subscribers. Returns the number of clients that received the message. |
| `PUB <channel> <message>` | Alias for PUBLISH. Post a message to a channel. |
| `SUBSCRIBE <channel> [channel ...]` | Subscribe the client to one or more channels for receiving published messages. Returns subscription confirmation for each channel. |
| `SUB <channel> [channel ...]` | Alias for SUBSCRIBE. Subscribe to one or more channels. |
| `UNSUBSCRIBE [channel ...]` | Unsubscribe the client from one or more channels. Returns unsubscription confirmation for each channel. |
| `UNSUB [channel ...]` | Alias for UNSUBSCRIBE. Unsubscribe from one or more channels. |
| `PSUBSCRIBE <pattern> [pattern ...]` | Subscribe to one or more channel patterns using glob-style matching (`*` and `?`). |
| `PSUB <pattern> [pattern ...]` | Alias for PSUBSCRIBE. Subscribe to channel patterns. |
| `PUNSUBSCRIBE [pattern ...]` | Unsubscribe from one or more channel patterns. |
| `PUNSUB [pattern ...]` | Alias for PUNSUBSCRIBE. Unsubscribe from channel patterns. |


### Transactions

| Command | Description |
|---|---|
| `MULTI` | Mark the start of a transaction block. After MULTI, all subsequent commands are queued instead of being executed immediately. |
| `EXEC` | Execute all commands that were queued after MULTI as a single atomic transaction. Returns an array containing the replies from each executed command. |
| `DISCARD` | Abort the current transaction by discarding all commands that were queued after MULTI. Returns 'Discarded' on success. |
| `WATCH <key> [key ...]` | Mark one or more keys to be watched for optimistic locking. If any watched keys are modified before EXEC, the transaction is aborted. |
| `UNWATCH` | Flush all previously watched keys for the current client connection. Automatically called after EXEC or DISCARD. |


### User Management

| Command | Description |
|---|---|
| `USERADD <admin_flag 1/0> <user> <password>` | Create a new user account on the server. The admin_flag specifies admin privileges (1 for admin, 0 for regular user). Password must be alphanumeric. Requires admin privileges. |
| `USERDEL <user>` | Delete a user account from the server. The specified user will be removed and will no longer be able to authenticate. Cannot delete the 'root' user. Requires admin privileges. |
| `PASSWD <user> <password>` | Change the password for a user account. Users can change their own password, and admin users can change any user's password. Password must be alphanumeric. |
| `USERS [username]` | List all usernames or show details for a specific user. Without arguments, returns an array of all usernames. With a username, returns detailed information including admin status. |
| `WHOAMI` | Display details of the currently authenticated user including username, client IP address, admin status, and full name. |


### Persistence Commands

| Command | Description |
|---|---|
| `SAVE` | Synchronously save the current database state to disk as an RDB snapshot file. This command blocks the server during the save operation. Requires admin privileges. |
| `BGSAVE` | Asynchronously save the current database state to disk as an RDB snapshot in the background. Returns 'OK' immediately while the save continues in the background. Requires admin privileges. |
| `BGREWRITEAOF` | Asynchronously rewrite the Append-Only File (AOF) in the background. Creates a new, optimized AOF file by reading the current dataset. Requires admin privileges. |


### Server & Connection

| Command | Description |
|---|---|
| `AUTH <user> <password>` | Authenticate to the server with the specified username and password. Required when the server is configured with 'requirepass' enabled. Returns 'OK' on success. |
| `ECHO <message>` | Returns the same message that was sent to the server. Useful for testing connectivity and verifying the server is properly receiving commands. |
| `PING [message]` | Test server connectivity and measure latency. Without arguments, returns 'PONG'. With a message argument, returns that message. Can be executed without authentication. |
| `COMMAND` | Returns 'OK' to indicate the server is ready to accept commands. Simple health check command. |
| `COMMANDS [pattern \| command_name]` | List available commands or get detailed help for a specific command. With '*' or no args, returns all command names. With a command name, returns detailed information. |
| `SELECT <db_index>` | Select the database with the specified zero-based numeric index for the current connection. Default is 16 databases (indices 0-15). |
| `SEL <db_index>` | Alias for SELECT. Select the database with the specified index. |
| `SIZE [db_index]` | Return the number of configured databases, or the number of keys in a specific database if an index is provided. |


### Monitoring & Information

| Command | Description |
|---|---|
| `INFO [key]` | Get server information and statistics, or per-key metadata. Without arguments, returns comprehensive server info (Server, Clients, Memory, Persistence, General). With a key argument, returns metadata for that specific key. |
| `MONITOR` | Enable real-time monitoring mode for the current client connection. All commands executed by other clients are streamed in real-time with timestamps and client information. |
| `DBSIZE` | Return the total number of keys currently stored in the selected database. Includes all key types (strings, lists, sets, hashes, sorted sets, HyperLogLogs). |
| `FLUSHDB` | Remove all keys from the currently selected database. This is a destructive operation that cannot be undone. Requires admin privileges. |
| `DROPDB` | Alias for FLUSHDB. Remove all keys from the currently selected database. Requires admin privileges. |
| `FLUSHALL` | Remove all keys from all databases on the server. This is a destructive operation that clears the entire server state. Requires admin privileges. Use with extreme caution. |


---

## 5. Internal Architecture

This section details the internal design of Go-Redis-Server for developers and contributors.

### High-Level Diagram
```
   Client (redis-cli)
           |
          TCP
           |
     RESP Parser  (client.go)
           |
   Command Dispatcher (handlers.go)
           |
+--------------------------+
|   In-Memory Database     | (database.go)
|   map[string]*Value      |
| (RWMutex Protection)     |
+--------------------------+
           |
+--------------------------+
|   Persistence Layer      |
| - AOF (aof.go)           |
| - RDB (rdb.go)           |
+--------------------------+
```

### Project Structure & Core Components
```bash
.
├── bin
├── cmd
│   ├── go-redis.service
│   ├── main.go
│   └── test.go
├── config
│   └── redis.conf
├── data
├── Dockerfile
├── go.mod
├── go-redis.code-workspace
├── go.sum
├── images
│   ├── go-redis-logo.png
│   └── go-redis.png
├── internal
│   ├── cluster
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
│   │   ├── handler_hyperloglog.go
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
├── run_clean.sh
├── run_client.sh
├── run_server.sh
```

### Concurrency Model

-   **One Goroutine Per Client**: The server spawns a new goroutine for each incoming connection, ensuring clients are handled in parallel.
-   **Centralized Data Store**: A single, shared database instance is used for all clients.
-   **Read/Write Locking**: Access to the database is synchronized using `sync.RWMutex`: 
    -   **Read operations** (`GET`, `TTL`, etc.) use a read lock (`RLock`), allowing multiple readers to proceed concurrently.
    -   **Write operations** (`SET`, `DEL`, etc.) use a write lock (`Lock`), ensuring exclusive access and data consistency.

### Command Execution Pipeline

1.  A client connection is accepted, and a new goroutine starts handling it.
2.  The client's request is read from the TCP socket and parsed as a RESP message.
3.  The command and its arguments are dispatched to the appropriate handler function.
4.  If authentication is enabled, the client's authenticated status is checked.
5.  The handler acquires the necessary lock (read or write) on the database.
6.  The command logic is executed (e.g., reading/writing a value).
7.  A RESP-formatted response is written back to the client.
8.  For write commands, the operation is appended to the AOF buffer if enabled.

### Data Model

Each key in the database maps to a `Value` struct, which contains:
- The stored data itself (e.g., a string, list, or hash).
- An optional expiration timestamp (as a `time.Time`).
- Metadata for future eviction policies (e.g., access frequency).

### RESP Protocol Support

Go-Redis supports all primary RESP data types, making it fully compatible with `redis-cli`:
- `+` Simple Strings
- `-` Errors
- `:` Integers
- `$` Bulk Strings
- `*` Arrays
- `$-1` Nulls

---

## 6. Core Subsystems Explained

### Persistence: AOF vs. RDB

| Feature | AOF (Append-Only File) | RDB (Snapshot) |
|---|---|---|
| **Strategy** | Logs every write command to a file. | Saves a point-in-time snapshot of the entire dataset. |
| **Pros** | - Higher durability. <br> - More granular (can lose at most 1s of data with `everysec`). | - Faster restarts (loads one big file). <br> - Compact file size. |
| **Cons** | - Larger file size. <br> - Slower restarts on large datasets. | - Less durable (can lose data since last snapshot). |
| **Use Case** | Maximum data safety. | Fast backups and disaster recovery. |

-   **AOF `fsync` Policies**: Controlled by `appendfsync` in `redis.conf`.
    -   `always`: Safest but slowest. `fsync()` on every write.
    -   `everysec`: Default. `fsync()` once per second. Good trade-off.
    -   `no`: Fastest. Lets the OS decide when to `fsync()`.
-   **RDB Triggers**: Controlled by `save` rules in `redis.conf` or manually via `SAVE`/`BGSAVE`.

### Memory Management & Eviction

-   **`maxmemory`**: This directive in `redis.conf` sets a hard limit on the memory Go-Redis can use.
-   **`maxmemory-policy`**: When the `maxmemory` limit is reached, this policy determines the eviction behavior.
    -   `no-eviction`: (Default) Blocks write commands that would exceed the limit, returning an error.
    -   `allkeys-random`: Randomly evicts keys to make space for new data.
    -   `allkeys-lru`: Evicts the least recently used keys.
    -   `allkeys-lfu`: Evicts the least frequently used keys.

---

## 7. Limitations

Go-Redis is an educational project and intentionally omits certain advanced Redis features:

-   No replication or clustering.
-   No Lua scripting.

---

## 8. Contact & Support

For bug reports, questions, or contributions, please contact:
-   **Author**: Akash Maji
-   **Email**: `akashmaji@iisc.ac.in`