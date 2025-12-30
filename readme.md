

# Go-Redis

A Redis-compatible in-memory key-value store server written in Go. This implementation supports core Redis commands, persistence mechanisms (AOF and RDB), authentication, and expiration.

## Features

- **Core Commands**: GET, SET, DEL, EXISTS, KEYS, DBSIZE, FLUSHDB
- **Persistence**: 
  - **AOF (Append-Only File)**: Logs every write operation
  - **RDB (Redis Database)**: Point-in-time snapshots
- **Expiration**: EXPIRE and TTL support for keys
- **Authentication**: Password-based authentication
- **Background Operations**: BGSAVE and BGREWRITEAOF
- **Thread-Safe**: Concurrent access with read-write locks
- **Redis Protocol**: Compatible with Redis RESP protocol

## Prerequirites
- redis-cli
- sudo systemctl status redis-server.service
- sudo systemctl stop redis-server.service
- redis-cli

## Building

```bash
go build
```

This will create an executable that you can run.

## Configuration

The server reads configuration from `redis.conf` file. Create a `redis.conf` file with the following options:

```conf
# Data directory
dir ./data

# AOF Configuration
appendonly yes                    # Enable AOF persistence
appendfilename backup.aof         # AOF filename
appendfsync always                # Sync mode: always, everysec, or no

# RDB Configuration
save 5 3                          # Save if 3 keys changed in 5 seconds
dbfilename backup.rdb              # RDB filename

# Authentication
requirepass dsl                   # Set password (optional)
```

### Configuration Options

- `dir`: Directory where persistence files are stored
- `appendonly`: Enable/disable AOF (yes/no)
- `appendfilename`: Name of the AOF file
- `appendfsync`: Sync mode for AOF
  - `always`: Sync after every write
  - `everysec`: Sync every second
  - `no`: Let OS decide when to sync
- `save <seconds> <keys>`: RDB snapshot trigger (save if N keys changed in M seconds)
- `dbfilename`: Name of the RDB file
- `requirepass`: Password for authentication (if set, clients must authenticate)

## Running

```bash
./go-redis
```

The server will start listening on port `6379` (default Redis port).

## Available Commands

### String Operations

#### GET
Get the value of a key.

```
GET <key>
```

**Example:**
```
GET name
```

#### SET
Set a key to hold a string value.

```
SET <key> <value>
```

**Example:**
```
SET name "John"
```

### Key Management

#### DEL
Delete one or more keys. Returns the number of keys deleted.

```
DEL <key1> [key2 ...]
```

**Example:**
```
DEL name age
```

#### EXISTS
Check if one or more keys exist. Returns the number of existing keys.

```
EXISTS <key1> [key2 ...]
```

**Example:**
```
EXISTS name age
```

#### KEYS
Find all keys matching a pattern.

```
KEYS <pattern>
```

**Example:**
```
KEYS *name*
KEYS user:*
```

#### DBSIZE
Return the number of keys in the database.

```
DBSIZE
```

#### FLUSHDB
Remove all keys from the current database.

```
FLUSHDB
```

### Expiration

#### EXPIRE
Set a timeout on a key. After the timeout, the key will be automatically deleted.

```
EXPIRE <key> <seconds>
```

**Returns:**
- `1` if timeout was set
- `0` if key doesn't exist

**Example:**
```
EXPIRE session:123 3600
```

#### TTL
Get the remaining time to live (TTL) of a key in seconds.

```
TTL <key>
```

**Returns:**
- Positive integer: TTL in seconds
- `-1`: Key exists but has no expiration
- `-2`: Key doesn't exist

**Example:**
```
TTL session:123
```

### Persistence

#### SAVE
Synchronously save the database to disk (blocks until complete).

```
SAVE
```

#### BGSAVE
Save the database to disk in the background (non-blocking).

```
BGSAVE
```

**Returns:**
- `OK` if save started
- Error if save already in progress

#### BGREWRITEAOF
Rewrite the AOF file in the background to remove duplicates and optimize size.

```
BGREWRITEAOF
```

### Authentication

#### AUTH
Authenticate with the server using a password.

```
AUTH <password>
```

**Example:**
```
AUTH dsl
```

**Note:** If `requirepass` is set in `redis.conf`, you must authenticate before running other commands.

### Utility

#### COMMAND
Returns OK (used for connection testing).

```
COMMAND
```

## Usage Examples

### Basic Operations

```bash
# Connect using redis-cli
redis-cli

# Set a key
127.0.0.1:6379> SET name "Alice"
OK

# Get a key
127.0.0.1:6379> GET name
"Alice"

# Set expiration
127.0.0.1:6379> EXPIRE name 60
(integer) 1

# Check TTL
127.0.0.1:6379> TTL name
(integer) 58

# Delete key
127.0.0.1:6379> DEL name
(integer) 1
```

### With Authentication

```bash
redis-cli

# Authenticate first
127.0.0.1:6379> AUTH dsl
OK

# Now you can use other commands
127.0.0.1:6379> SET key1 "value1"
OK
```

### Pattern Matching

```bash
127.0.0.1:6379> SET user:1:name "Alice"
OK
127.0.0.1:6379> SET user:2:name "Bob"
OK
127.0.0.1:6379> KEYS user:*
1) "user:1:name"
2) "user:2:name"
```

## Persistence

### AOF (Append-Only File)

AOF logs every write operation. On server restart, AOF is replayed to restore the database state.

**Configuration:**
```conf
appendonly yes
appendfilename backup.aof
appendfsync always
```

### RDB (Redis Database)

RDB creates point-in-time snapshots of the database.

**Configuration:**
```conf
save 5 3          # Save if 3 keys changed in 5 seconds
dbfilename backup.rdb
```

Both AOF and RDB can be enabled simultaneously for maximum durability.

## Protocol

The server implements the Redis Serialization Protocol (RESP), making it compatible with standard Redis clients like `redis-cli`.

## Architecture

- **Concurrent Access**: Uses read-write locks for thread-safe operations
- **Background Operations**: BGSAVE and BGREWRITEAOF run in separate goroutines
- **Automatic Expiration**: Expired keys are automatically deleted on access
- **Checksum Verification**: RDB saves include checksum verification for data integrity

## License

This is an educational project implementing Redis-like functionality in Go.

## Version

v0.1



