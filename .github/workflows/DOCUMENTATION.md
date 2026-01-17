---
layout: default
title: Go-Redis-Server Documentation
---

# Go-Redis-Server: The Complete Guide (v1.0)

![Go-Redis-Server Logo](go-redis-logo.png)

Welcome to the complete developer and user documentation for **Go-Redis**, a lightweight, multi-threaded, Redis-compatible server implemented in Go.

This document provides a deep dive into the project's features, architecture, and usage. Whether you are a developer looking to understand the internals, or a user wanting to get started, this guide is for you.

---

## Table of Contents

1. [**Overview & Features**](#1-overview--features)
    - [Design Goals](#design-goals)
2. [**Getting Started**](#2-getting-started)
    - [Prerequisites](#prerequisites)
    - [Building from Source](#building-from-source)
    - [Configuration](#configuration)
    - [Running the Server](#running-the-server)
    - [Connecting with `redis-cli`](#connecting-with-redis-cli)
3. [**Docker Deployment**](#3-docker-deployment)
    - [Using the Pre-built Image](#using-the-pre-built-image)
    - [Building a Custom Image](#building-a-custom-image)
4. [**Command Reference**](#4-command-reference)
    - [String Operations](#string-operations)
    - [Key Management](#key-management)
    - [List Operations](#list-operations)
    - [Set Operations](#set-operations)
    - [Hash Operations](#hash-operations)
    - [Sorted Set Operations](#sorted-set-operations)
    - [HyperLogLog Operations](#hyperloglog-operations)
    - [Bitmap Operations](#bitmap-operations)
    - [Geospatial Operations](#geospatial-operations)
    - [Pub/Sub Operations](#pubsub-operations)
    - [Expiration Commands](#expiration-commands)
    - [Transactions](#transactions)
    - [Persistence Commands](#persistence-commands)
    - [Server & Connection](#server--connection)
    - [Monitoring & Information](#monitoring--information)
5. [**Internal Architecture**](#5-internal-architecture)
    - [High-Level Diagram](#high-level-diagram)
    - [Project Structure & Core Components](#project-structure--core-components)
    - [Concurrency Model](#concurrency-model)
    - [Command Execution Pipeline](#command-execution-pipeline)
    - [Data Model](#data-model)
    - [RESP Protocol Support](#resp-protocol-support)
6. [**Core Subsystems Explained**](#6-core-subsystems-explained)
    - [Persistence: AOF vs. RDB](#persistence-aof-vs-rdb)
    - [Memory Management & Eviction](#memory-management--eviction)
7. [**Limitations**](#7-limitations)
8. [**Contact & Support**](#8-contact--support)

---

## 1. Overview & Features

Go-Redis is a Redis-compatible in-memory key-value store server written in Go. It is designed to be a learning tool for understanding how a database like Redis works under the hood, while also being a functional server for development and testing purposes.

- **Broad Command Support**: Implements a rich subset of commands for Strings, Lists, Sets, Hashes, Sorted Sets, HyperLogLog, Bitmaps, and Geospatial data.
- **Dual Persistence Model**: 
  - **AOF (Append-Only File)**: Logs every write operation with configurable `fsync` modes for high durability.
    - **RDB (Redis Database)**: Creates point-in-time snapshots for fast startups and backups.
    - **Encrypted Storage**: Optional AES-GCM encryption for all persistence files and user credentials.
- **Key Expiration**: Supports `EXPIRE`, `TTL`, and `PERSIST` with lazy (on-access) key removal.
- **Atomic Transactions**: Group commands in `MULTI`/`EXEC` blocks with `WATCH`/`UNWATCH` for optimistic locking.
- **User Management**: Multi-user support with role-based access control (Admin/User).
- **Introspection & Monitoring**: 
  - `INFO` provides a detailed look into server statistics.
  - `MONITOR` streams live command processing for debugging.
- **Memory Management**: Allows setting a `maxmemory` limit and an eviction policy.
- **Pub/Sub Messaging**: Decoupled real-time communication between publishers and subscribers.
- **RESP Compatible**: Fully compatible with the Redis Serialization Protocol (RESP), allowing `redis-cli` and other standard clients to connect seamlessly.
- **Thread-Safe by Design**: Handles multiple concurrent clients safely using a single database protected by read-write locks.

### Design Goals

- **Educational**: To provide a clear, readable, and well-documented codebase for those learning about database internals, concurrency in Go, and network programming.
- **Redis-Compatible**: To work out-of-the-box with `redis-cli`.
- **Correctness over Performance**: To prioritize a simple, correct, and deterministic implementation over complex performance optimizations.

---

## 2. Getting Started

### Prerequisites

- **Go**: Version 1.24.4 or later.
- **`redis-cli`**: The standard Redis command-line tool.
- **OS**: Tested on Linux/Unix environments.

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

- **With defaults:**

```bash
./go-redis
```

- **With custom paths:**

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

```bash
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
| `APPEND <key> <value>` | Append the specified value to the string value stored at the specified key. If the key does not exist, it is created with the value as the initial content. Returns the length of the string after the append operation. |
| `DECR <key>` | Decrement the integer value stored at the specified key by one. If the key does not exist, it is initialized to 0 before performing the decrement operation, resulting in -1. Returns the new value after the decrement as an integer. Returns an error if the key exists but contains a value that cannot be parsed as an integer, or if the key holds a non-string data type. This operation is atomic and thread-safe. The command is recorded to AOF if persistence is enabled. |
| `DECRBY <key> <decrement>` | Decrement the integer value stored at the specified key by the given decrement amount. If the key does not exist, it is initialized to 0 before performing the decrement. The decrement value must be a valid integer. Returns the new value after the decrement as an integer. Returns an error if the decrement is not a valid integer, if the stored value cannot be parsed as an integer, or if the key holds a non-string data type. This operation is atomic and thread-safe. |
| `GET <key>` | Retrieve the string value stored at the specified key. Returns the value as a bulk string if the key exists and contains a string value. Returns NULL if the key does not exist. If the key exists but has expired, it is automatically deleted and NULL is returned. Returns an error if the key exists but holds a non-string data type (list, set, hash, sorted set, or HyperLogLog). This operation is thread-safe and uses a read lock. |
| `GETDEL <key>` | Get the value of a key and delete the key. Returns the value as a bulk string if the key exists, or NULL if the key does not exist. This is equivalent to GET followed by DEL, but performed atomically. This operation is thread-safe. |
| `GETEX <key> [EX seconds\|PX milliseconds\|EXAT unix-seconds\|PXAT unix-milliseconds\|PERSIST]` | Get the value of a key and optionally set its expiration time. Returns the value as a bulk string if the key exists, or NULL if the key does not exist. The optional expiration options allow setting expiration on the key after retrieval: EX (seconds), PX (milliseconds), EXAT (Unix timestamp in seconds), PXAT (Unix timestamp in milliseconds), or PERSIST to remove expiration. This operation is atomic and thread-safe. |
| `GETRANGE <key> <start> <end>` | Get a substring of the string value stored at the specified key. The substring is defined by the start and end offsets (inclusive). Negative offsets count from the end of the string. Returns the substring as a bulk string. Returns an empty string if the key does not exist or if the range is invalid. This operation is thread-safe and uses a read lock. |
| `GETSET <key> <value>` | Set the value of a key and return its old value. Returns the old value as a bulk string, or NULL if the key did not exist. The key is set to the new value regardless. This operation is atomic and thread-safe. The command is recorded to AOF if persistence is enabled. |
| `INCR <key>` | Increment the integer value stored at the specified key by one. If the key does not exist, it is initialized to 0 before performing the increment operation, resulting in 1. Returns the new value after the increment as an integer. Returns an error if the key exists but contains a value that cannot be parsed as an integer, or if the key holds a non-string data type. This operation is atomic and thread-safe. The command is recorded to AOF if persistence is enabled and tracked for RDB saving. |
| `INCRBY <key> <increment>` | Increment the integer value stored at the specified key by the given increment amount. If the key does not exist, it is initialized to 0 before performing the increment. The increment value must be a valid integer and can be negative to perform a decrement. Returns the new value after the increment as an integer. Returns an error if the increment is not a valid integer, if the stored value cannot be parsed as an integer, or if the key holds a non-string data type. This operation is atomic and thread-safe. |
| `INCRBYFLOAT <key> <increment>` | Increment the floating-point value stored at the specified key by the given increment amount. If the key does not exist, it is initialized to 0 before performing the increment. The increment can be a positive or negative floating-point number. Returns the new value after the increment as a bulk string. Returns an error if the increment is not a valid float, if the stored value cannot be parsed as a float, or if the key holds a non-string data type. This operation is atomic and thread-safe. |
| `MGET <key> [key ...]` | Retrieve the values of multiple keys in a single operation. Returns an array of values in the same order as the requested keys. For keys that do not exist, are expired, or hold non-string types, NULL is returned in their position. This command is more efficient than multiple GET calls when retrieving multiple values. At least one key must be specified. This operation is thread-safe and uses a read lock. All keys are read atomically within the same lock acquisition. |
| `MSET <key> <value> [key value ...]` | Set multiple key-value pairs in a single atomic operation. If any keys already exist, their values are overwritten. The number of arguments must be even (key-value pairs). Returns 'OK' on success. Returns an error if the number of arguments is not valid. Unlike individual SET commands, MSET is atomic - either all keys are set or none are (in case of error). The operation is recorded to AOF if persistence is enabled and tracked for RDB saving. This operation is thread-safe and uses a write lock for the entire operation. |
| `MSETNX <key> <value> [key value ...]` | Set multiple key-value pairs only if none of the keys exist. Returns 1 if all keys were set successfully, or 0 if any key already exists (in which case no keys are set). This command is atomic - either all keys are set or none are. The number of arguments must be even (key-value pairs). This operation is thread-safe and uses a write lock. |
| `PSETEX <key> <milliseconds> <value>` | Set the value of a key with an expiration time specified in milliseconds. If the key already exists, its value is overwritten and any previous expiration is discarded. Returns 'OK' on success. The expiration time must be a positive integer representing milliseconds. This operation is atomic and thread-safe. The command is recorded to AOF if persistence is enabled. |
| `SET <key> <value>` | Set the string value of a key. If the key already exists, its value is overwritten regardless of its type. Any previous expiration is discarded. Returns 'OK' on success. If maxmemory is configured and would be exceeded, the server attempts to evict keys according to the eviction policy before returning an error if eviction fails. The operation updates memory tracking and is recorded to AOF if persistence is enabled. Changes are tracked for automatic RDB saving. This operation is atomic and thread-safe. |
| `SETEX <key> <seconds> <value>` | Set the value of a key with an expiration time specified in seconds. If the key already exists, its value is overwritten and any previous expiration is discarded. Returns 'OK' on success. The expiration time must be a positive integer representing seconds. This operation is atomic and thread-safe. The command is recorded to AOF if persistence is enabled. |
| `SETNX <key> <value>` | Set the value of a key only if the key does not already exist. Returns 1 if the key was set successfully, or 0 if the key already exists. If the key is set, any previous expiration is discarded. This operation is atomic and thread-safe. The command is recorded to AOF if persistence is enabled. |
| `SETRANGE <key> <offset> <value>` | Overwrite part of a string value stored at the specified key, starting at the specified offset. If the offset is beyond the current string length, the string is padded with zero bytes. Returns the length of the string after the operation. The offset must be a non-negative integer. This operation is atomic and thread-safe. The command is recorded to AOF if persistence is enabled. |
| `STRLEN <key>` | Return the length of the string value stored at the specified key. Returns an integer representing the string length in bytes. Returns 0 if the key does not exist or has expired. Returns an error if the key exists but holds a non-string data type. This operation is O(1) as the string length is tracked internally. This operation is thread-safe and uses a read lock. |

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
| `BLMOVE <source> <destination> <LEFT\|RIGHT> <LEFT\|RIGHT> <timeout>` | Atomically move an element from one list to another, blocking if the source list is empty. The first LEFT/RIGHT specifies which side to pop from the source list, the second specifies which side to push to the destination list. If the source list is empty, wait up to timeout seconds for an element to become available. Returns the element that was moved as a bulk string, or NULL if timeout expires. Timeout of 0 means block indefinitely. Returns an error if either key exists but is not a list type. This operation is atomic and thread-safe. |
| `BLPOP <key> [key ...] <timeout>` | Remove and return the first element of the first non-empty list from the specified keys. If no lists contain elements, wait up to timeout seconds for an element to become available. Returns an array containing the key name and the popped element, or NULL if timeout expires. Timeout of 0 means block indefinitely. Returns an error if timeout is not a valid number. This operation is atomic and thread-safe. |
| `BRPOP <key> [key ...] <timeout>` | Remove and return the last element of the first non-empty list from the specified keys. If no lists contain elements, wait up to timeout seconds for an element to become available. Returns an array containing the key name and the popped element, or NULL if timeout expires. Timeout of 0 means block indefinitely. Returns an error if timeout is not a valid number. This operation is atomic and thread-safe. |
| `LGET <key>` | Retrieve all elements from a list stored at the specified key. This is a custom convenience command equivalent to 'LRANGE key 0 -1'. Returns an array containing all elements in the list from head to tail. Returns an empty array if the key does not exist. Returns an error if the key exists but is not a list type. This operation is thread-safe and uses a read lock. For large lists, consider using LRANGE with specific indices to retrieve subsets. |
| `LINDEX <key> <index>` | Retrieve the element at the specified index in a list stored at the specified key. The index is zero-based, where 0 is the first element (head) and -1 is the last element (tail). Negative indices count from the end of the list. Returns the element as a bulk string if found. Returns NULL if the key does not exist or if the index is out of range. Returns an error if the key exists but is not a list type. This operation is thread-safe and uses a read lock. |
| `LINSERT <key> <BEFORE\|AFTER> <pivot> <value>` | Insert a value into a list stored at the specified key either before or after the first occurrence of a pivot value. Returns the length of the list after the insert operation, or -1 if the pivot value was not found. If the key does not exist, no operation is performed and -1 is returned. Returns an error if the key exists but is not a list type. The BEFORE/AFTER keyword is case-sensitive. This operation is atomic and thread-safe. |
| `LLEN <key>` | Return the length (number of elements) of a list stored at the specified key. Returns an integer representing the list length. Returns 0 if the key does not exist (treating a non-existent key as an empty list). Returns an error if the key exists but is not a list type. This operation is O(1) as the length is tracked internally. This operation is thread-safe and uses a read lock. |
| `LMOVE <source> <destination> <LEFT\|RIGHT> <LEFT\|RIGHT>` | Atomically move an element from one list to another. The first LEFT/RIGHT specifies which side to pop from the source list, the second specifies which side to push to the destination list. Returns the element that was moved as a bulk string, or NULL if the source list is empty. Returns an error if either key exists but is not a list type. If the destination list does not exist, it is created. This operation is atomic and thread-safe. |
| `LPOP <key>` | Remove and return the first element (head) of a list stored at the specified key. Returns the removed element as a bulk string. Returns NULL if the key does not exist or the list is empty. Returns an error if the key exists but is not a list type. If the list becomes empty after the pop, the key is automatically deleted from the database. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |
| `LPOS <key> <element> [RANK rank] [COUNT count] [MAXLEN maxlen]` | Return the index of the first occurrence of element in a list stored at the specified key. Optional RANK specifies which occurrence to find (1 for first, 2 for second, etc., negative for counting from the end). Optional COUNT limits the number of matches returned. Optional MAXLEN limits the number of elements to scan. Returns an integer index, an array of indices, or NULL if not found. Returns an error if the key exists but is not a list type. This operation is thread-safe and uses a read lock. |
| `LPUSH <key> <value> [value ...]` | Insert one or more values at the head (left side) of a list stored at the specified key. If the key does not exist, a new list is created. Multiple values are inserted in left-to-right order, so 'LPUSH mylist a b c' results in 'c' being the first element. Returns an integer representing the length of the list after the push operations. Returns an error if the key exists but is not a list type. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |
| `LREM <key> <count> <value>` | Remove the first count occurrences of elements equal to value from a list stored at the specified key. If count is positive, removes elements moving from head to tail. If count is negative, removes elements moving from tail to head. If count is 0, removes all occurrences. Returns the number of elements removed. Returns 0 if the key does not exist. Returns an error if the key exists but is not a list type. If the list becomes empty after removal, the key is automatically deleted. This operation is atomic and thread-safe. |
| `LRANGE <key> <start> <stop>` | Retrieve a range of elements from a list stored at the specified key. The start and stop indices are zero-based and inclusive. Negative indices count from the end (-1 is the last element, -2 is second to last, etc.). Out-of-range indices are automatically clamped to valid values. Returns an array of elements in the specified range. Returns an empty array if the key does not exist, if start is greater than stop after normalization, or if the range is completely outside the list bounds. Returns an error if the key exists but is not a list type. This operation is thread-safe and uses a read lock. |
| `LSET <key> <index> <value>` | Set the value of an element at the specified index in a list stored at the specified key. The index is zero-based, where 0 is the first element (head) and -1 is the last element (tail). Negative indices count from the end of the list. Returns 'OK' on success. Returns an error if the key does not exist, if the index is out of range, or if the key exists but is not a list type. This operation is atomic and thread-safe. |
| `LTRIM <key> <start> <stop>` | Trim a list stored at the specified key so that it will contain only the specified range of elements. Both start and stop are zero-based indexes, inclusive. Negative indices count from the end of the list. If start is greater than stop or the range is completely outside the list bounds, the list becomes empty. Returns 'OK' on success. Returns an error if the key exists but is not a list type. If the trimmed list becomes empty, the key is automatically deleted. This operation is atomic and thread-safe. |
| `RPOP <key>` | Remove and return the last element (tail) of a list stored at the specified key. Returns the removed element as a bulk string. Returns NULL if the key does not exist or the list is empty. Returns an error if the key exists but is not a list type. If the list becomes empty after the pop, the key is automatically deleted from the database. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |
| `RPOPLPUSH <source> <destination>` | Atomically remove the last element from the source list and push it to the head of the destination list. Returns the element that was moved as a bulk string, or NULL if the source list is empty. If source and destination are the same, rotates the list. Returns an error if either key exists but is not a list type. If the destination list does not exist, it is created. This operation is atomic and thread-safe. |
| `RPUSH <key> <value> [value ...]` | Append one or more values to the tail (right side) of a list stored at the specified key. If the key does not exist, a new list is created. Multiple values are appended in left-to-right order, so 'RPUSH mylist a b c' results in 'c' being the last element. Returns an integer representing the length of the list after the push operations. Returns an error if the key exists but is not a list type. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |

### Set Operations

| Command | Description |
|---|---|
| `SADD <key> <member> [member ...]` | Add one or more members to a set stored at the specified key. If the key does not exist, a new set is created. Members that are already present in the set are ignored. Returns an integer representing the number of members that were actually added to the set (not counting members already present). Returns an error if the key exists but is not a set type. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |
| `SCARD <key>` | Return the cardinality (number of members) of a set stored at the specified key. Returns an integer representing the set size. Returns 0 if the key does not exist (treating a non-existent key as an empty set). Returns an error if the key exists but is not a set type. This operation is O(1) as the cardinality is tracked internally. This operation is thread-safe and uses a read lock. |
| `SDIFF <key> [key ...]` | Return the members of the set resulting from the difference between the first set and all successive sets. The difference consists of members that exist in the first set but not in any of the other sets. Returns an array of members in the resulting set. Returns an empty array if the first key does not exist. Returns an error if any key exists but is not a set type. At least one key must be specified. This operation is thread-safe and uses a read lock. |
| `SDIFFSTORE <destination> <key> [key ...]` | Compute the difference between the first set and all subsequent sets, storing the result in the destination key. The difference consists of members that exist in the first set but not in any of the other sets. The destination key is overwritten with the result. Returns the number of members in the resulting set. If the first key does not exist, the result is an empty set. Returns an error if any key exists but is not a set type. At least one source key must be specified. This operation is atomic and thread-safe. |
| `SINTER <key> [key ...]` | Return the members of the set resulting from the intersection of all specified sets. The intersection consists of members that exist in all of the specified sets. Returns an array of members in the resulting set. Returns an empty array if any key does not exist (since intersection with an empty set is empty). Returns an error if any key exists but is not a set type. The implementation optimizes by iterating over the smallest set first. This operation is thread-safe and uses a read lock. |
| `SINTERSTORE <destination> <key> [key ...]` | Compute the intersection of all specified sets and store the result in the destination key. The intersection consists of members that exist in all of the specified sets. The destination key is overwritten with the result. Returns the number of members in the resulting set. If any source key does not exist, the result is an empty set. Returns an error if any key exists but is not a set type. At least one source key must be specified. This operation is atomic and thread-safe. |
| `SISMEMBER <key> <member>` | Determine if a given value is a member of the set stored at the specified key. Returns 1 if the member exists in the set. Returns 0 if the member does not exist in the set, or if the key does not exist. Returns an error if the key exists but is not a set type. This operation is O(1) due to the hash-based set implementation. This operation is thread-safe and uses a read lock. |
| `SMEMBERS <key>` | Return all members of the set stored at the specified key. Returns an array containing all members of the set. Returns an empty array if the key does not exist. Returns an error if the key exists but is not a set type. The order of members in the result is not guaranteed and depends on the internal hash iteration order. This operation is thread-safe and uses a read lock. For large sets, consider using SSCAN for incremental iteration. |
| `SMISMEMBER <key> <member> [member ...]` | Check if multiple members are members of the set stored at the specified key. Returns an array of integers where each element is 1 if the corresponding member exists in the set, or 0 if it does not. If the key does not exist, all members are reported as not existing (0). Returns an error if the key exists but is not a set type. At least one member must be specified. This operation is thread-safe and uses a read lock. |
| `SMOVE <source> <destination> <member>` | Move a member from one set to another. Removes the member from the source set and adds it to the destination set. Returns 1 if the member was moved successfully, or 0 if the member does not exist in the source set. If the source and destination sets are the same, returns 0 without making changes. Returns an error if either key exists but is not a set type. If the destination set does not exist, it is created. If the source set becomes empty after the move, it is automatically deleted. This operation is atomic and thread-safe. |
| `SPOP <key> [count]` | Remove and return one or more random members from a set stored at the specified key. When called without count, returns a single random member as a bulk string, or NULL if the set is empty. When called with count, returns an array of up to count random members. If count is larger than the set size, returns all members. Returns NULL if the key does not exist. Returns an error if the key exists but is not a set type. If the set becomes empty after popping, the key is automatically deleted. This operation is atomic and thread-safe. |
| `SRANDMEMBER <key> [count]` | Return one or more random members from a set stored at the specified key. When called without count, returns a single random member as a bulk string, or NULL if the key does not exist. When called with a positive count, returns an array of up to count distinct random members. When called with a negative count, returns an array that may contain duplicates. Returns an empty array if the key does not exist and count is specified. Returns an error if the key exists but is not a set type. This operation is thread-safe and uses a read lock. |
| `SREM <key> <member> [member ...]` | Remove one or more members from a set stored at the specified key. Members that do not exist in the set are silently ignored. Returns an integer representing the number of members that were actually removed from the set. Returns 0 if the key does not exist. Returns an error if the key exists but is not a set type. If the set becomes empty after removal, the key is automatically deleted. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |
| `SUNION <key> [key ...]` | Return the members of the set resulting from the union of all specified sets. The union consists of all members that exist in at least one of the specified sets. Returns an array of members in the resulting set. Keys that do not exist are treated as empty sets. Returns an error if any key exists but is not a set type. At least one key must be specified. This operation is thread-safe and uses a read lock. |
| `SUNIONSTORE <destination> <key> [key ...]` | Compute the union of all specified sets and store the result in the destination key. The union consists of all members that exist in at least one of the specified sets. The destination key is overwritten with the result. Returns the number of members in the resulting set. Keys that do not exist are treated as empty sets. Returns an error if any key exists but is not a set type. At least one source key must be specified. This operation is atomic and thread-safe. |

### Hash Operations

| Command | Description |
|---|---|
| `HDEL <key> <field> [field ...]` | Delete one or more fields from a hash stored at the specified key. Fields that do not exist in the hash are silently ignored. Returns an integer representing the number of fields that were actually removed from the hash. Returns 0 if the key does not exist. Returns an error if the key exists but is not a hash type. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is thread-safe. |
| `HDELALL <key>` | Delete all fields from a hash stored at the specified key, effectively clearing the entire hash. Returns an integer representing the number of fields that were deleted. Returns 0 if the key does not exist or the hash is already empty. Returns an error if the key exists but is not a hash type. Unlike HDEL, this command removes all fields in a single operation. The operation updates memory tracking and is recorded to AOF if persistence is enabled. |
| `HEXISTS <key> <field>` | Check if a specific field exists within a hash stored at the specified key. Returns 1 if the hash contains the specified field, or 0 if the field does not exist or the key does not exist. Returns an error if the key exists but is not a hash type. This operation is thread-safe and uses a read lock. Note that expired fields (if field-level expiration is set via HEXPIRE) are still reported as existing until they are lazily deleted. |
| `HEXPIRE <key> <field> <seconds>` | Set an expiration time on a specific field within a hash. After the timeout expires, the field will be automatically deleted using lazy expiration (deleted on next access). The timeout is specified in seconds as a positive integer. Returns 1 if the expiration was set successfully, or 0 if the key or field does not exist. Returns an error if the key exists but is not a hash type. This is a custom extension not available in standard Redis, allowing fine-grained expiration control at the field level. |
| `HGET <key> <field>` | Retrieve the value associated with a specific field in a hash stored at the specified key. Returns the field value as a bulk string if both the key and field exist. Returns NULL if the key does not exist, if the field does not exist within the hash, or if the field has expired. Returns an error if the key exists but is not a hash type. Expired fields are lazily deleted when accessed. This operation is thread-safe. |
| `HGETALL <key>` | Retrieve all fields and their values from a hash stored at the specified key. Returns an array containing alternating field names and values: [field1, value1, field2, value2, ...]. Returns an empty array if the key does not exist. Returns an error if the key exists but is not a hash type. Expired fields (if field-level expiration is set) are automatically skipped and not included in the result. This operation is thread-safe and uses a read lock. |
| `HINCRBY <key> <field> <increment>` | Increment the integer value of a hash field by the specified amount. If the key does not exist, a new hash is created. If the field does not exist or has expired, it is initialized to 0 before the increment. The increment can be negative to perform a decrement. Returns the new value after the increment as an integer. Returns an error if the increment is not a valid integer, if the field value cannot be parsed as an integer, or if the key is not a hash type. This operation is atomic and thread-safe. |
| `HINCRBYFLOAT <key> <field> <increment>` | Increment the floating-point value of a hash field by the specified amount. If the key does not exist, a new hash is created. If the field does not exist or has expired, it is initialized to 0 before the increment. The increment can be a positive or negative floating-point number. Returns the new value after the increment as a bulk string. Returns an error if the increment is not a valid float, if the field value cannot be parsed as a float, or if the key is not a hash type. This operation is atomic and thread-safe. |
| `HKEYS <key>` | Retrieve all field names from a hash stored at the specified key. Returns an array containing all field names in the hash. Returns an empty array if the key does not exist. Returns an error if the key exists but is not a hash type. Expired fields (if field-level expiration is set) are automatically skipped and not included in the result. The order of fields in the result is not guaranteed. This operation is thread-safe and uses a read lock. |
| `HLEN <key>` | Return the number of fields contained in a hash stored at the specified key. Returns an integer representing the field count. Returns 0 if the key does not exist. Returns an error if the key exists but is not a hash type. Note that this count may include expired fields that have not yet been lazily deleted. This operation is thread-safe and uses a read lock. |
| `HMGET <key> <field> [field ...]` | Retrieve the values associated with the specified fields in a hash stored at the specified key. Returns an array of values in the same order as the requested fields. For fields that do not exist or have expired, NULL is returned in their position. If the key does not exist, an array of NULL values is returned. Returns an error if the key exists but is not a hash type. This command is more efficient than multiple HGET calls when retrieving multiple fields. This operation is thread-safe and uses a read lock. |
| `HMSET <key> <field> <value> [field value ...]` | Set multiple field-value pairs in a hash stored at the specified key. If the key does not exist, a new hash is created. If any specified fields already exist, their values are overwritten. This command is deprecated in favor of HSET which now supports multiple field-value pairs, but is kept for backward compatibility. Returns 'OK' on success. Returns an error if the number of arguments is not valid (must have an even number of field-value pairs after the key). This operation is atomic and thread-safe. |
| `HRANDFIELD <key> [count [WITHVALUES]]` | Return one or more random fields from a hash stored at the specified key. When called without count, returns a single random field name. When called with count, returns an array of up to count random field names. If count is negative, allows duplicates. If WITHVALUES is specified, returns field-value pairs. Returns an empty array if the key does not exist. Returns an error if the key exists but is not a hash type. Expired fields are automatically skipped. This operation is thread-safe and uses a read lock. |
| `HSET <key> <field> <value> [field value ...]` | Set one or more field-value pairs in a hash stored at the specified key. If the key does not exist, a new hash is created. If any specified fields already exist, their values are overwritten. Returns an integer representing the number of new fields that were added (not counting fields that were updated). Returns an error if the number of arguments is not valid (must have an odd number of arguments: key followed by field-value pairs). The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |
| `HSETNX <key> <field> <value>` | Set the value of a hash field only if the field does not already exist. Returns 1 if the field was set successfully, or 0 if the field already exists. If the key does not exist, a new hash is created. Returns an error if the key exists but is not a hash type. This operation is atomic and thread-safe. The command is recorded to AOF if persistence is enabled. |
| `HSTRLEN <key> <field>` | Return the length of the value associated with a hash field. Returns the string length in bytes of the field's value. Returns 0 if the key or field does not exist. Returns an error if the key exists but is not a hash type. This operation is thread-safe and uses a read lock. |
| `HVALS <key>` | Retrieve all values from a hash stored at the specified key. Returns an array containing all field values in the hash. Returns an empty array if the key does not exist. Returns an error if the key exists but is not a hash type. Expired fields (if field-level expiration is set) are automatically skipped and their values are not included in the result. The order of values in the result corresponds to the internal hash iteration order. This operation is thread-safe and uses a read lock. |

### Sorted Set Operations

| Command | Description |
|---|---|
| `BZPOPMAX <key> [key ...] <timeout>` | Remove and return the member with the highest score from the first non-empty sorted set from the specified keys. If no sorted sets contain members, wait up to timeout seconds for a member to become available. Returns an array containing the key name, member name, and score, or NULL if timeout expires. Timeout of 0 means block indefinitely. Returns an error if timeout is not a valid number or if any key exists but is not a sorted set type. This operation is atomic and thread-safe. |
| `BZPOPMIN <key> [key ...] <timeout>` | Remove and return the member with the lowest score from the first non-empty sorted set from the specified keys. If no sorted sets contain members, wait up to timeout seconds for a member to become available. Returns an array containing the key name, member name, and score, or NULL if timeout expires. Timeout of 0 means block indefinitely. Returns an error if timeout is not a valid number or if any key exists but is not a sorted set type. This operation is atomic and thread-safe. |
| `ZADD <key> <score> <member> [score member ...]` | Add one or more members with their scores to a sorted set stored at the specified key. If the key does not exist, a new sorted set is created. If a member already exists, its score is updated to the new value. Scores must be valid floating-point numbers. Returns an integer representing the number of new members added (not counting score updates). Returns an error if the key exists but is not a sorted set type, or if any score is not a valid float. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |
| `ZCARD <key>` | Return the cardinality (number of members) of a sorted set stored at the specified key. Returns an integer representing the sorted set size. Returns 0 if the key does not exist (treating a non-existent key as an empty sorted set). Returns an error if the key exists but is not a sorted set type. This operation is O(1) as the cardinality is tracked internally. This operation is thread-safe and uses a read lock. |
| `ZCOUNT <key> <min> <max>` | Return the number of members in a sorted set stored at the specified key with scores between min and max (inclusive). Min and max can be -inf and +inf to specify unbounded ranges. Returns an integer count. Returns 0 if the key does not exist. Returns an error if the key exists but is not a sorted set type. This operation is O(log N). This operation is thread-safe and uses a read lock. |
| `ZGET <key> [member]` | Retrieve the score of a specific member, or all members with their scores from a sorted set. When called with just a key, returns an array of all members and their scores sorted by score ascending: [member1, score1, member2, score2, ...]. When called with a member argument, returns just the score of that member as a bulk string, or NULL if the member does not exist. Returns an empty array if the key does not exist. Returns an error if the key exists but is not a sorted set type. This is a custom convenience command. This operation is thread-safe and uses a read lock. |
| `ZINCRBY <key> <increment> <member>` | Increment the score of a member in a sorted set stored at the specified key by the given increment amount. If the member does not exist, it is added with the increment as its score. If the key does not exist, a new sorted set is created. The increment can be a positive or negative floating-point number. Returns the new score of the member as a bulk string. Returns an error if the key exists but is not a sorted set type, or if the increment is not a valid float. This operation is atomic and thread-safe. |
| `ZINTERSTORE <destination> <numkeys> <key> [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM\|MIN\|MAX]` | Compute the intersection of the specified sorted sets and store the result in the destination key. The score of each member is computed using the specified weights and aggregate function. WEIGHTS assigns weights to each input set. AGGREGATE specifies how to combine scores (SUM, MIN, MAX). Returns the number of members in the resulting sorted set. Returns an error if any key exists but is not a sorted set type. The destination key is overwritten. This operation is atomic and thread-safe. |
| `ZLEXCOUNT <key> <min> <max>` | Return the number of members in a sorted set stored at the specified key with values between min and max in lexicographical order. All members must have the same score. Min and max should be specified with bracket notation: '[' for inclusive, '(' for exclusive. Returns an integer count. Returns 0 if the key does not exist. Returns an error if the key exists but is not a sorted set type, or if members have different scores. This operation is O(log N). This operation is thread-safe and uses a read lock. |
| `ZMSCORE <key> <member> [member ...]` | Return the scores of the specified members in a sorted set stored at the specified key. Returns an array of scores in the same order as the requested members. Returns NULL for members that do not exist. Returns an error if the key exists but is not a sorted set type. This operation is O(M) where M is the number of requested members. This operation is thread-safe and uses a read lock. |
| `ZPOPMAX <key> [count]` | Remove and return the members with the highest scores from a sorted set stored at the specified key. When called without count, returns the member with the highest score. When called with count, returns up to count members with the highest scores. Returns an array of member-score pairs. Returns an empty array if the key does not exist or the sorted set is empty. Returns an error if the key exists but is not a sorted set type. If the sorted set becomes empty, the key is automatically deleted. This operation is atomic and thread-safe. |
| `ZPOPMIN <key> [count]` | Remove and return the members with the lowest scores from a sorted set stored at the specified key. When called without count, returns the member with the lowest score. When called with count, returns up to count members with the lowest scores. Returns an array of member-score pairs. Returns an empty array if the key does not exist or the sorted set is empty. Returns an error if the key exists but is not a sorted set type. If the sorted set becomes empty, the key is automatically deleted. This operation is atomic and thread-safe. |
| `ZRANDMEMBER <key> [count]` | Return one or more random members from a sorted set stored at the specified key. When called without count, returns a single random member. When called with positive count, returns an array of distinct random members. When called with negative count, returns an array that may contain duplicates. Returns NULL if the key does not exist. Returns an error if the key exists but is not a sorted set type. This operation is O(N) where N is the number of members. This operation is thread-safe and uses a read lock. |
| `ZRANGE <key> <start> <stop> [WITHSCORES]` | Return a range of members from a sorted set stored at the specified key, ordered by score from lowest to highest. The start and stop indices are zero-based and inclusive. Negative indices count from the end (-1 is the last element). When WITHSCORES is specified, the result includes scores: [member1, score1, member2, score2, ...]. Returns an array of members (or members with scores). Returns an empty array if the key does not exist or the range is invalid. Returns an error if the key exists but is not a sorted set type. This operation is thread-safe and uses a read lock. |
| `ZRANGEBYLEX <key> <min> <max>` | Return all members in a sorted set stored at the specified key with values between min and max in lexicographical order. All members must have the same score. Min and max should be specified with bracket notation: '[' for inclusive, '(' for exclusive. Returns an array of members in lexicographical order. Returns an empty array if the key does not exist or no members match the range. Returns an error if the key exists but is not a sorted set type, or if members have different scores. This operation is O(log N + M) where M is the number of elements returned. This operation is thread-safe and uses a read lock. |
| `ZRANGEBYSCORE <key> <min> <max>` | Return all members in a sorted set stored at the specified key with scores between min and max (inclusive). Min and max can be -inf and +inf to specify unbounded ranges. Returns an array of members in score order. Returns an empty array if the key does not exist or no members match the range. Returns an error if the key exists but is not a sorted set type. This operation is O(log N + M) where M is the number of elements returned. This operation is thread-safe and uses a read lock. |
| `ZRANK <key> <member>` | Return the rank of a member in a sorted set stored at the specified key, with scores ordered from low to high. Returns the zero-based rank of the member. Returns NULL if the key does not exist or the member is not present in the sorted set. Returns an error if the key exists but is not a sorted set type. This operation is O(log N). This operation is thread-safe and uses a read lock. |
| `ZREM <key> <member> [member ...]` | Remove one or more members from a sorted set stored at the specified key. Members that do not exist in the sorted set are silently ignored. Returns an integer representing the number of members that were actually removed. Returns 0 if the key does not exist. Returns an error if the key exists but is not a sorted set type. If the sorted set becomes empty after removal, the key is automatically deleted. The operation updates memory tracking and is recorded to AOF if persistence is enabled. This operation is atomic and thread-safe. |
| `ZREMRANGEBYLEX <key> <min> <max>` | Remove all members in a sorted set stored at the specified key with values between min and max in lexicographical order. All members must have the same score. Min and max should be specified with bracket notation: '[' for inclusive, '(' for exclusive. Returns the number of members removed. Returns 0 if the key does not exist. Returns an error if the key exists but is not a sorted set type, or if members have different scores. If the sorted set becomes empty, the key is automatically deleted. This operation is atomic and thread-safe. |
| `ZREMRANGEBYRANK <key> <start> <stop>` | Remove all members in a sorted set stored at the specified key with ranks between start and stop (inclusive). Ranks are zero-based and ordered by score from low to high. Negative indices count from the end. Returns the number of members removed. Returns 0 if the key does not exist. Returns an error if the key exists but is not a sorted set type. If the sorted set becomes empty, the key is automatically deleted. This operation is atomic and thread-safe. |
| `ZREMRANGEBYSCORE <key> <min> <max>` | Remove all members in a sorted set stored at the specified key with scores between min and max (inclusive). Min and max can be -inf and +inf to specify unbounded ranges. Returns the number of members removed. Returns 0 if the key does not exist. Returns an error if the key exists but is not a sorted set type. If the sorted set becomes empty, the key is automatically deleted. This operation is atomic and thread-safe. |
| `ZREVRANGE <key> <start> <stop> [WITHSCORES]` | Return a range of members from a sorted set stored at the specified key, ordered by score from highest to lowest (reverse order). The start and stop indices are zero-based and inclusive. Negative indices count from the end (-1 is the last element in reverse order, i.e., the highest scored member). When WITHSCORES is specified, the result includes scores: [member1, score1, member2, score2, ...]. Returns an array of members (or members with scores). Returns an empty array if the key does not exist or the range is invalid. Returns an error if the key exists but is not a sorted set type. This operation is thread-safe and uses a read lock. |
| `ZREVRANGEBYSCORE <key> <max> <min>` | Return all members in a sorted set stored at the specified key with scores between max and min (inclusive), in reverse score order. Min and max can be -inf and +inf to specify unbounded ranges. Returns an array of members in reverse score order. Returns an empty array if the key does not exist or no members match the range. Returns an error if the key exists but is not a sorted set type. This operation is O(log N + M) where M is the number of elements returned. This operation is thread-safe and uses a read lock. |
| `ZREVRANK <key> <member>` | Return the rank of a member in a sorted set stored at the specified key, with scores ordered from high to low. Returns the zero-based rank of the member. Returns NULL if the key does not exist or the member is not present in the sorted set. Returns an error if the key exists but is not a sorted set type. This operation is O(log N). This operation is thread-safe and uses a read lock. |
| `ZSCAN <key> <cursor> [MATCH pattern] [COUNT count]` | Incrementally iterate over the members of a sorted set stored at the specified key. Returns a cursor for the next iteration and an array of member-score pairs. MATCH filters members by glob pattern. COUNT suggests the number of elements to return per iteration. Returns an error if the key exists but is not a sorted set type. This operation is thread-safe and uses a read lock. |
| `ZSCORE <key> <member>` | Return the score of a member in a sorted set stored at the specified key. Returns the score as a bulk string (floating-point number represented as string). Returns NULL if the key does not exist or the member is not present in the sorted set. Returns an error if the key exists but is not a sorted set type. This operation is O(1). This operation is thread-safe and uses a read lock. |
| `ZUNIONSTORE <destination> <numkeys> <key> [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM\|MIN\|MAX]` | Compute the union of the specified sorted sets and store the result in the destination key. The score of each member is computed using the specified weights and aggregate function. WEIGHTS assigns weights to each input set. AGGREGATE specifies how to combine scores (SUM, MIN, MAX). Returns the number of members in the resulting sorted set. Returns an error if any key exists but is not a sorted set type. The destination key is overwritten. This operation is atomic and thread-safe. |

### HyperLogLog Operations

| Command | Description |
|---|---|
| `PFADD <key> <element> [element ...]` | Add one or more elements to a HyperLogLog probabilistic data structure. HyperLogLog provides approximate cardinality estimation using only ~12KB of memory. Returns 1 if at least one internal register was altered. |
| `PFCOUNT <key> [key ...]` | Return the approximated cardinality (number of unique elements) observed by the HyperLogLog(s). When called with multiple keys, returns the cardinality of the union. Standard error rate is approximately 0.81%. |
| `PFDEBUG <key>` | Return internal debugging information about a HyperLogLog including encoding type (sparse/dense), number of registers, and estimated cardinality. |
| `PFMERGE <destkey> <sourcekey> [sourcekey ...]` | Merge multiple HyperLogLog values into a single destination HyperLogLog. The merged result approximates the cardinality of the union of all sources. |

### Bitmap Operations

| Command | Description |
|---|---|
| `SETBIT <key> <offset> <value>` | Set or clear the bit at offset in the string value stored at key. The offset must be >= 0 and < 2^32. The value must be 0 or 1. Returns the original bit value at offset. When the string is grown, added bits are set to 0. Time complexity: O(1). |
| `GETBIT <key> <offset>` | Return the bit value at offset in the string value stored at key. When offset is beyond the string length, the bit is assumed to be 0. If the key does not exist, it is treated as an empty string. Time complexity: O(1). |
| `BITCOUNT <key> [start end [BYTE\|BIT]]` | Count the number of set bits (population counting) in a string. By default, all bytes are examined. Optional start and end parameters specify a range (byte or bit indices). Negative values count from the end. The BYTE/BIT modifier (Redis 7.0+) specifies whether the range is in bytes or bits. Time complexity: O(N) where N is the number of bytes in the range. |
| `BITOP <operation> <destkey> <key> [key ...]` | Perform a bitwise operation between multiple keys and store the result in destkey. Operations: AND, OR, XOR, NOT. NOT requires exactly one source key. When strings have different lengths, shorter strings are zero-padded. Returns the size of the result string. Time complexity: O(N) where N is the size of the longest string. |
| `BITPOS <key> <bit> [start [end [BYTE\|BIT]]]` | Return the position of the first bit set to 1 or 0 in a string. The bit parameter must be 0 or 1. Optional start and end specify a range to search within (byte or bit indices). Returns -1 if the bit is not found. Time complexity: O(N) where N is the number of bytes in the range. |
| `BITFIELD <key> [GET <encoding> <offset>] [SET <encoding> <offset> <value>] [INCRBY <encoding> <offset> <increment>] [OVERFLOW <WRAP\|SAT\|FAIL>]` | Perform arbitrary bitfield integer operations on strings. Encoding format: i<bits> for signed, u<bits> for unsigned (e.g., i8, u16). Offset format: absolute number or #N (multiplied by encoding size). OVERFLOW controls behavior: WRAP (default, wrap around), SAT (saturate at min/max), FAIL (return nil on overflow). Returns an array with one element per operation. Time complexity: O(1) for each subcommand. |

### Geospatial Operations

| Command | Description |
|---|---|
| `GEOADD <key> <longitude> <latitude> <member> [<longitude> <latitude> <member> ...]` | Add one or more geospatial items (longitude, latitude, member name) to a sorted set. Longitude must be between -180 and 180 degrees. Latitude must be between -90 and 90 degrees. Internally stores items as geohash scores in a sorted set. Returns the number of elements added. Time complexity: O(log(N)) for each item added, where N is the number of elements in the sorted set. |
| `GEOPOS <key> <member> [<member> ...]` | Return the positions (longitude, latitude) of all specified members in the geospatial index. Returns an array of positions, where each position is an array of two elements: [longitude, latitude]. Returns NULL for members that do not exist. Time complexity: O(N) where N is the number of members requested. |
| `GEODIST <key> <member1> <member2> [m\|km\|ft\|mi]` | Return the distance between two members in the geospatial index. The optional unit parameter specifies the unit of measurement: m (meters, default), km (kilometers), ft (feet), mi (miles). Returns NULL if either member does not exist. Uses the Haversine formula for calculation. Time complexity: O(log(N)). |
| `GEOHASH <key> <member> [<member> ...]` | Return the geohash strings representing the positions of the specified members. Geohashes are base32-encoded strings that represent geographic coordinates. Returns an array of geohash strings. Returns NULL for members that do not exist. Time complexity: O(log(N)) for each member. |
| `GEORADIUS <key> <longitude> <latitude> <radius> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC\|DESC]` | Query members within a radius from a given longitude/latitude point. Unit can be m, km, ft, or mi. Options: WITHCOORD (include coordinates), WITHDIST (include distance), WITHHASH (include geohash), COUNT (limit results), ASC/DESC (sort by distance). Returns an array of matching members with optional metadata. Time complexity: O(N+log(M)) where N is the number of elements in the grid and M is the number of items inside the radius. |
| `GEOSEARCH <key> FROMMEMBER <member> \| FROMLONLAT <longitude> <latitude> BYRADIUS <radius> <unit> \| BYBOX <width> <height> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC\|DESC]` | Advanced geospatial search supporting both radius and box queries. Can search from a member's position (FROMMEMBER) or from coordinates (FROMLONLAT). Search area can be circular (BYRADIUS) or rectangular (BYBOX). Supports same options as GEORADIUS. Returns an array of matching members with optional metadata. Time complexity: O(N+log(M)) where N is the number of elements in the grid and M is the number of items in the search area. |
| `GEOSEARCHSTORE <destination> <source> FROMMEMBER <member> \| FROMLONLAT <longitude> <latitude> BYRADIUS <radius> <unit> \| BYBOX <width> <height> <unit> [COUNT <count>] [ASC\|DESC] [STOREDIST]` | Perform a GEOSEARCH query and store the results in a destination sorted set. Same syntax as GEOSEARCH but stores results instead of returning them. The STOREDIST option stores distances as scores instead of geohash scores. Returns the number of elements in the resulting sorted set. Useful for caching search results. Time complexity: O(N+log(M)) where N is the number of elements in the grid and M is the number of items in the search area. |

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
| `TIME` | Return the current server time, and uptime. |

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

```mermaid
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

- **One Goroutine Per Client**: The server spawns a new goroutine for each incoming connection, ensuring clients are handled in parallel.
- **Centralized Data Store**: A single, shared database instance is used for all clients.
- **Read/Write Locking**: Access to the database is synchronized using `sync.RWMutex`: 
  - **Read operations** (`GET`, `TTL`, etc.) use a read lock (`RLock`), allowing multiple readers to proceed concurrently.
  - **Write operations** (`SET`, `DEL`, etc.) use a write lock (`Lock`), ensuring exclusive access and data consistency.

### Command Execution Pipeline

1. A client connection is accepted, and a new goroutine starts handling it.
2. The client's request is read from the TCP socket and parsed as a RESP message.
3. The command and its arguments are dispatched to the appropriate handler function.
4. If authentication is enabled, the client's authenticated status is checked.
5. The handler acquires the necessary lock (read or write) on the database.
6. The command logic is executed (e.g., reading/writing a value).
7. A RESP-formatted response is written back to the client.
8. For write commands, the operation is appended to the AOF buffer if enabled.

### Data Model

Each key in the database maps to a `Value` struct, which contains:

- The stored data itself (e.g., a string, list, or hash).
- An optional expiration timestamp (as a `time.Time`).
- Metadata for future eviction policies (e.g., access frequency).

### RESP Protocol Support

Go-Redis supports all primary RESP data types, making it fully compatible with `redis-cli`:

- `+OK` Simple Strings
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

- **AOF `fsync` Policies**: Controlled by `appendfsync` in `redis.conf`.
  - `always`: Safest but slowest. `fsync()` on every write.
  - `everysec`: Default. `fsync()` once per second. Good trade-off.
  - `no`: Fastest. Lets the OS decide when to `fsync()`.
- **RDB Triggers**: Controlled by `save` rules in `redis.conf` or manually via `SAVE`/`BGSAVE`.

### Memory Management & Eviction

- **`maxmemory`**: This directive in `redis.conf` sets a hard limit on the memory Go-Redis can use.
- **`maxmemory-policy`**: When the `maxmemory` limit is reached, this policy determines the eviction behavior.

- `no-eviction`: (Default) Blocks write commands that would exceed the limit, returning an error.
- `allkeys-random`: Randomly evicts keys to make space for new data.
- `allkeys-lru`: Evicts the least recently used keys.
- `allkeys-lfu`: Evicts the least frequently used keys.

---

## 7. Limitations

Go-Redis is an educational project and intentionally omits certain advanced Redis features:

- No replication or clustering.
- No Lua scripting.

---

## 8. Contact & Support

For bug reports, questions, or contributions, please contact:

- **Author**: Akash Maji
- **Email**: `akashmaji@iisc.ac.in`
