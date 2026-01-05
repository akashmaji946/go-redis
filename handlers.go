package main

import (
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Success Message:
// +OK\r\n

// can run these even if authenticated=0
var safeCommands = []string{
	"COMMAND",
	"PING",
	"COMMANDS",
	"HELP",
	"AUTH",
}

// IsSafeCmd checks whether a command can be executed without authentication.
//
// Parameters:
//   - cmd: command name
//   - commands: list of safe commands
//
// Returns:
//
//	true if cmd is in commands, false otherwise
//
// Safe commands include:
//
//	COMMAND, AUTH
func IsSafeCmd(cmd string, commands []string) bool {
	for _, command := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}

// handle is the main command dispatcher.
//
// Responsibilities:
//  1. Extract command name from parsed Value
//  2. Lookup command handler in Handlers map
//  3. Enforce authentication rules (if requirepass is set)
//  4. Handle transaction queuing (if transaction is active)
//  5. Execute handler or queue command
//  6. Write response to client
//
// Transaction Support:
//   - If state.tx is not nil (transaction active):
//   - Commands (except MULTI, EXEC, DISCARD) are queued
//   - Returns "QUEUED" response instead of executing
//   - Commands are stored with their handler for later execution
//   - Transaction control commands (MULTI, EXEC, DISCARD) execute immediately
//
// Error cases:
//   - Unknown command → ERR no such command
//   - Authentication required but missing → NOAUTH error
//   - Transaction already running (for MULTI) → handled by Multi handler
//   - No transaction running (for EXEC/DISCARD) → handled by respective handlers
//
// Command Flow:
//  1. Parse command name from Value array
//  2. Check if command exists in Handlers map
//  3. Check authentication (if required)
//  4. Check transaction state:
//     - If transaction active: queue command (unless MULTI/EXEC/DISCARD)
//     - If no transaction: execute command immediately
//  5. Send response to client
func handle(client *Client, v *Value, state *AppState) {

	state.genStats.total_commands_executed += 1

	// the command is in the first entry of v.arr
	cmd := v.arr[0].blk
	handler, ok := Handlers[cmd]

	if !ok {
		log.Println("ERROR: no such command:", cmd)
		reply := NewErrorValue("ERR no such command")
		w := NewWriter(client.conn)
		w.Write(reply)
		w.Flush()
		return
	}

	// handle authentication: if password needed & not authenticated, then block running command
	if state.config.requirepass && !client.authenticated && !IsSafeCmd(cmd, safeCommands) {
		reply := NewErrorValue("NOAUTH client not authenticated, use AUTH <password>")
		w := NewWriter(client.conn)
		w.Write(reply)
		w.Flush()
		return
	}

	// handle transaction: if already running, then queue
	if state.tx != nil && cmd != "EXEC" && cmd != "DISCARD" && cmd != "MULTI" {
		txCmd := &TxCommand{
			value:   v,
			handler: handler,
		}
		state.tx.cmds = append(state.tx.cmds, txCmd)
		reply := NewStringValue("QUEUED")
		w := NewWriter(client.conn)
		w.Write(reply)
		w.Flush()
		return
	}

	reply := handler(client, v, state)
	w := NewWriter(client.conn)
	w.Write(reply)
	w.Flush()

	// for MONITOR handle will send to all monitors
	go func() {
		for _, mon := range state.monitors {
			if mon != *client {
				mon.writerMonitorLog(v, client)
			}
		}
	}()

}

type Handler func(*Client, *Value, *AppState) *Value

func init() {
	Handlers["COMMANDS"] = Commands
}

var Handlers = map[string]Handler{

	// health check
	"COMMAND": Command,
	"PING":    Ping,

	"GET":    Get,
	"SET":    Set,
	"INCR":   Incr,
	"DECR":   Decr,
	"INCRBY": IncrBy,
	"DECRBY": DecrBy,

	// List commands
	"LPUSH":  Lpush,
	"RPUSH":  Rpush,
	"LPOP":   Lpop,
	"RPOP":   Rpop,
	"LRANGE": Lrange,
	"LLEN":   Llen,
	"LINDEX": Lindex,
	"LGET":   Lget,

	// Set commands
	"SADD":      Sadd,
	"SREM":      Srem,
	"SMEMBERS":  Smembers,
	"SISMEMBER": Sismember,
	"SCARD":     Scard,

	"TYPE": Type,

	"DEL":    Del,
	"RENAME": Rename,

	"EXISTS": Exists,

	"KEYS": Keys,

	"SAVE":         Save,
	"BGSAVE":       BGSave,
	"BGREWRITEAOF": BGRewriteAOF,

	"FLUSHDB": FlushDB,
	"DBSIZE":  DBSize,

	"EXPIRE": Expire,
	"TTL":    Ttl,

	// Hash commands
	"HSET":    Hset,
	"HGET":    Hget,
	"HDEL":    Hdel,
	"HGETALL": Hgetall,
	"HDELALL": Hdelall,
	"HINCRBY": Hincrby,
	"HMSET":   Hmset,
	"HEXISTS": Hexists,
	"HLEN":    Hlen,
	"HKEYS":   Hkeys,
	"HVALS":   Hvals,
	"HEXPIRE": Hexpire,

	// authorize
	"AUTH": Auth,

	// transaction
	"MULTI":   Multi,
	"EXEC":    Exec,
	"DISCARD": Discard,

	"MONITOR": Monitor,
	"INFO":    Info,
}

// initial command just before connection
// COMMAND DOCS
// Command handles the COMMAND command.
// Utility command used for connection testing and protocol compliance.
//
// Syntax:
//   COMMAND
//
// Returns:
//   +OK\r\n
//
// Notes:
//   - Executable without authentication

func Command(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	return NewStringValue("OK")
}

// Commands handles the COMMANDS command.
// Lists all available commands in an array.
//
// Syntax:
//
//	COMMANDS
//
// Returns:
//
//	Array: List of all supported command names
func Commands(c *Client, v *Value, state *AppState) *Value {
	var cmds []string
	for k := range Handlers {
		cmds = append(cmds, k)
	}
	// sort.Strings(cmds)

	var arr []Value
	for _, cmd := range cmds {
		arr = append(arr, Value{typ: BULK, blk: cmd})
	}
	return NewArrayValue(arr)
}

func Ping(c *Client, v *Value, state *AppState) *Value {
	// cmd
	args := v.arr[1:]
	if len(args) != 0 {
		return NewStringValue("PONG " + args[0].blk)
	}
	return NewStringValue("PONG")
}

// Info handles the INFO command.
// Returns server information and statistics in a human-readable format.
// This command provides comprehensive details about the server's current state,
// including server metadata, client connections, memory usage, persistence status,
// and general statistics.
//
// Syntax:
//
//	INFO
//
// Parameters:
//   - None (takes no arguments)
//
// Returns:
//   - Bulk string: Formatted server information organized into categories
//   - Error: If arguments are provided (INFO takes no arguments)
//
// Information Categories:
//
//	Server:
//	  - redis_version: Server version (0.1)
//	  - author: Author information
//	  - process_id: Operating system process ID
//	  - tcp_port: Port the server is listening on (6379)
//	  - server_time: Current server time in Unix microseconds
//	  - server_uptime: Server uptime in seconds
//	  - server_path: Path to the server executable
//	  - config_path: Path to the configuration file
//
//	Clients:
//	  - clients: Number of currently connected clients
//
//	Memory:
//	  - used_memory: Current memory usage in bytes
//	  - used_memory_peak: Peak memory usage in bytes
//	  - total_memory: Total system memory in bytes
//	  - eviction_policy: Currently configured eviction policy
//
//	Persistence:
//	  - rdb_bgsave_running: Whether a background RDB save is in progress (true/false)
//	  - rdb_last_save_time: Unix timestamp of last RDB save
//	  - rdb_saves_count: Total number of RDB saves performed
//	  - aof_enabled: Whether AOF persistence is enabled (true/false)
//	  - aof_rewrite_running: Whether an AOF rewrite is in progress (true/false)
//	  - aof_last_rewrite_time: Unix timestamp of last AOF rewrite
//	  - rdb_rewrite_count: Total number of AOF rewrites performed
//
//	General:
//	  - total_connections_received: Total number of client connections since startup
//	  - total_commands_executed: Total number of commands executed
//	  - total_txn_executed: Total number of transactions (EXEC) executed
//	  - total_keys_expired: Total number of keys expired
//	  - total_keys_evicted: Total number of keys evicted due to memory limits
//
// Output Format:
//
//	Information is formatted with category headers (prefixed with #) and
//	key-value pairs with aligned formatting for readability.
//
// Example Output:
//
//	# Server
//	             redis_version  : 0.1
//	                     author : akashmaji(@iisc.ac.in)
//	                 process_id : 12345
//	                   tcp_port : 6379
//	                server_time : 1704067200000000
//	              server_uptime : 3600
//	                server_path : /path/to/go-redis
//	                config_path : ./redis.conf
//
//	# Clients
//	                     clients : 3
//
//	# Memory
//	                 used_memory : 1024 B
//	           used_memory_peak : 2048 B
//	                total_memory : 8589934592 B
//	           eviction_policy : allkeys-random
//
// Usage:
//
//	127.0.0.1:6379> INFO
//	# Server
//	redis_version  : 0.1
//	...
//
// Thread Safety:
//   - Reads from AppState which is protected by appropriate synchronization
//   - Safe to call from any client connection concurrently
//
// Note:
//   - Information is generated dynamically on each call
//   - Statistics are cumulative since server startup
//   - Memory information uses gopsutil library to get system memory
//
// Info handles the INFO command.
//
// Modes:
//   - INFO
//     Returns global server info (server, clients, memory, persistence, general)
//   - INFO <key>
//     Returns metadata for a specific key: type, length, ttl, and memory usage
func Info(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]

	// INFO (no args) -> global server info
	if len(args) == 0 {
		msg := state.redisInfo.Print(state)
		return &Value{typ: BULK, blk: msg}
	}

	// INFO <key> -> per-key info
	if len(args) == 1 {
		key := args[0].blk

		DB.mu.RLock()
		item, ok := DB.store[key]
		DB.mu.RUnlock()
		if !ok {
			return NewErrorValue("ERR key not found")
		}

		typ := item.Type
		if typ == "" {
			typ = STRING_TYPE
		}

		length := 0
		switch typ {
		case STRING_TYPE:
			length = 1
		case LIST_TYPE:
			length = len(item.List)
		case HASH_TYPE:
			length = len(item.Hash)
		case "set":
			length = len(item.ItemSet)
		}

		ttl := int64(-1)
		if item.Exp.Unix() != UNIX_TS_EPOCH {
			ttl = int64(time.Until(item.Exp).Seconds())
		}

		mem := item.approxMemoryUsage(key)

		msg := fmt.Sprintf(
			"type: %s\nlen: %d\nttl: %d\nmem: %d B\naccesses: %d\n",
			strings.ToUpper(typ), length, ttl, mem, item.AccessCount,
		)
		return NewBulkValue(msg)
	}

	// Anything else is invalid
	return NewErrorValue("ERR wrong number of arguments for 'info' command")
}

// Monitor handles the MONITOR command.
// Enables real-time monitoring mode for the client, causing all commands executed
// by other clients to be streamed to this client's connection.
//
// Syntax:
//
//	MONITOR
//
// Parameters:
//   - None (takes no arguments)
//
// Returns:
//   - "+OK\r\n" on success
//   - Error if arguments are provided (MONITOR takes no arguments)
//
// Behavior:
//  1. Adds the current client to the server's monitoring list
//  2. Client remains in monitoring mode until connection is closed
//  3. All commands executed by other clients are streamed to this client
//  4. The monitoring client does not receive its own commands
//  5. Multiple clients can be in monitoring mode simultaneously
//
// Monitoring Format:
//
//	Each monitored command is sent as a RESP simple string with the format:
//	"<timestamp> [<client_ip>] \"<command>\" \"<arg1>\" \"<arg2>\" ... \"<argN>\"\r\n"
//
//	Where:
//	  - timestamp: Unix timestamp when the command was executed
//	  - client_ip: IP address and port of the client that executed the command
//	  - command: The command name (e.g., SET, GET, DEL)
//	  - arg1, arg2, ...: Command arguments in quoted format
//
// Example Output:
//
//	+1704067200 [127.0.0.1:54321] "SET" "key1" "value1"\r\n
//	+1704067201 [127.0.0.1:54322] "GET" "key1"\r\n
//	+1704067202 [127.0.0.1:54323] "DEL" "key1"\r\n
//
// Usage:
//
//	# In one terminal, connect and enable monitoring
//	127.0.0.1:6379> MONITOR
//	OK
//
//	# In another terminal, execute commands
//	127.0.0.1:6379> SET test "value"
//	OK
//
//	# Monitor terminal will show:
//	1704067200 [127.0.0.1:54321] "SET" "test" "value"
//
// Thread Safety:
//   - Adds client to shared monitors slice (protected by connection handling)
//   - Monitoring logs are sent asynchronously in a goroutine to avoid blocking
//   - Safe for multiple clients to enable monitoring concurrently
//
// Lifecycle:
//   - Client remains in monitoring mode for the lifetime of the connection
//   - Monitoring automatically stops when client disconnects
//   - Client is removed from monitoring list on connection close
//
// Performance:
//   - Monitoring adds minimal overhead (asynchronous logging)
//   - Each command execution triggers a goroutine to send logs to all monitors
//   - Does not block command execution
//
// Use Cases:
//   - Debugging: See all commands being executed in real-time
//   - Monitoring: Track server activity and usage patterns
//   - Development: Understand command flow and interactions
//
// Note:
//   - Use `redis-cli --raw` or similar to see the raw output properly formatted
//   - Monitoring can generate significant output on busy servers
//   - The client cannot disable monitoring without disconnecting
//   - Commands executed by the monitoring client itself are not logged to it
func Monitor(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of MONITOR")
		return NewErrorValue("ERR inavlid argument to MONITOR")
	}
	state.monitors = append(state.monitors, *c)
	return NewStringValue("OK")
}

// Multi handles the MULTI command.
// Begins a transaction by creating a new transaction context for the client.
// All subsequent commands (except EXEC, DISCARD, and MULTI) will be queued
// until EXEC or DISCARD is called.
//
// Syntax:
//
//	MULTI
//
// Returns:
//
//	+Started\r\n on success
//	Error if transaction is already running
//
// Behavior:
//   - Creates a new Transaction instance and stores it in state.tx
//   - Subsequent commands are queued instead of executed immediately
//   - Only one transaction can be active per client connection
//   - Commands return "QUEUED" response instead of actual results
//
// Transaction Flow:
//  1. MULTI - Start transaction (this command)
//  2. <commands> - Queue commands (GET, SET, etc.)
//  3. EXEC - Execute all queued commands atomically
//     OR
//  3. DISCARD - Abort transaction without executing
//
// Error Cases:
//   - Invalid arguments: Returns error if arguments provided
//   - Transaction already running: Returns error if tx already exists
//
// Example:
//
//	127.0.0.1:6379> MULTI
//	OK
//	127.0.0.1:6379> SET key1 "value1"
//	QUEUED
//	127.0.0.1:6379> SET key2 "value2"
//	QUEUED
//	127.0.0.1:6379> EXEC
//	1) OK
//	2) OK
func Multi(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of MULTI")
		return NewErrorValue("ERR inavlid argument to MULTI")
	}
	// check if some tx running, then don't run
	if state.tx != nil {
		log.Println("tx already running")
		return NewErrorValue("ERR tx already running")
	}

	state.tx = NewTransaction()

	log.Println("tx started")
	return NewStringValue("Started")

}

// Exec handles the EXEC command.
// Executes all commands queued in the current transaction atomically.
// All queued commands are executed in order and their replies are returned
// as an array.
//
// Syntax:
//
//	EXEC
//
// Returns:
//
//	Array of replies: One reply per queued command, in order
//	Error if no transaction is running
//
// Behavior:
//   - Executes all commands in state.tx.cmds sequentially
//   - Each command is executed with its stored handler and value
//   - All replies are collected and returned as an array
//   - Transaction is cleared after execution (state.tx = nil)
//   - Commands are executed in the order they were queued
//
// Atomicity:
//   - All commands succeed or fail individually (no rollback on error)
//   - Commands are executed sequentially, not concurrently
//   - If a command fails, subsequent commands still execute
//
// Error Cases:
//   - Invalid arguments: Returns error if arguments provided
//   - No transaction running: Returns error if state.tx is nil
//
// Example:
//
//	127.0.0.1:6379> MULTI
//	OK
//	127.0.0.1:6379> SET a "1"
//	QUEUED
//	127.0.0.1:6379> SET b "2"
//	QUEUED
//	127.0.0.1:6379> EXEC
//	1) OK
//	2) OK
//
// Note: Unlike Redis, this implementation does not support WATCH for
//
//	optimistic locking or rollback on conflicts.
func Exec(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of EXEC")
		return NewErrorValue("ERR inavlid argument to EXEC")
	}
	// check if some tx running
	if state.tx == nil {
		log.Println("tx already NOT running")
		return NewErrorValue("ERR tx already NOT running")
	}

	// commmit queued commands first
	replies := make([]Value, len(state.tx.cmds))
	for idx, txCmd := range state.tx.cmds {
		reply := txCmd.handler(c, txCmd.value, state)
		replies[idx] = *reply
	}

	state.tx = nil
	state.genStats.total_txn_executed += 1
	log.Println("tx executed")

	return &Value{
		typ: ARRAY,
		arr: replies,
	}
}

// Discard handles the DISCARD command.
// Aborts the current transaction by discarding all queued commands
// without executing them. The transaction context is cleared.
//
// Syntax:
//
//	DISCARD
//
// Returns:
//
//	+Discarded\r\n on success
//	Error if no transaction is running
//
// Behavior:
//   - Clears the transaction context (state.tx = nil)
//   - All queued commands are discarded and never executed
//   - No changes are made to the database
//   - Client can start a new transaction with MULTI after discarding
//
// Use Cases:
//   - Client wants to abort a transaction without executing commands
//   - Error occurred during transaction building
//   - Client changed their mind about the transaction
//
// Error Cases:
//   - Invalid arguments: Returns error if arguments provided
//   - No transaction running: Returns error if state.tx is nil
//
// Example:
//
//	127.0.0.1:6379> MULTI
//	OK
//	127.0.0.1:6379> SET key1 "value1"
//	QUEUED
//	127.0.0.1:6379> SET key2 "value2"
//	QUEUED
//	127.0.0.1:6379> DISCARD
//	OK
//	# All queued commands are discarded, no changes made
//
// Note: After DISCARD, the client must call MULTI again to start
//
//	a new transaction.
func Discard(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of DISCARD")
		return NewErrorValue("ERR inavlid argument to DISCARD")
	}
	// check if some tx running
	if state.tx == nil {
		log.Println("tx already NOT running")
		return NewErrorValue("ERR tx already NOT running")
	}

	// discard without commiting
	state.tx = nil
	log.Println("tx discarded")

	return NewStringValue("Discarded")
}

// BGRewriteAOF handles the BGREWRITEAOF command.
// Asynchronously rewrites the Append-Only File.
//
// Behavior:
//  1. Copies current DB state
//  2. Rewrites AOF with compact SET commands
//  3. Runs in background goroutine
//
// Returns:
//
//	+Started.\r\n
func BGRewriteAOF(c *Client, v *Value, state *AppState) *Value {

	go func() {
		state.aofrewriting = true
		DB.mu.RLock()
		cp := make(map[string]*Item, len(DB.store))
		maps.Copy(cp, DB.store)
		DB.mu.RUnlock()
		state.aof.Rewrite(cp)
		state.aofrewriting = false

	}()

	// update the stats
	state.aofStats.aof_last_rewrite_ts = time.Now().Unix()
	state.aofStats.aof_rewrite_count += 1

	return NewStringValue("Started.")
}

// Get handles the GET command.
// Retrieves the value for a key.
//
// Syntax:
//
//	GET <key>
//
// Returns:
//   - Bulk string if key exists and not expired
//   - NULL if key does not exist or expired
//
// Behavior:
//   - Automatically deletes expired keys
//   - Thread-safe (read lock)
func Get(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR invalid command uage with GET")
	}
	key := args[0].blk // grab the key

	// get the item from the database
	DB.mu.RLock()
	item, ok := DB.Poll(key)
	// delete if expired
	deleted := DB.RemIfExpired(key, item, state)
	if deleted {
		fmt.Println("Expired Key: ", key)
		return NewNullValue()
	}
	DB.mu.RUnlock()

	if !ok {
		fmt.Println("Not Found: ", key)
		return NewNullValue()
	}

	return NewBulkValue(item.Str)

}

// Set handles the SET command.
// Sets a key to a string value.
//
// Syntax:
//
//	SET <key> <value>
//
// Returns:
//
//	+OK\r\n
//
// Side Effects:
//   - Appends command to AOF if enabled
//   - Flushes AOF if fsync=always
//   - Updates RDB change trackers
//
// Thread-safe:
//
//	Uses write lock
func Set(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR invalid command usage with SET")
	}

	key := args[0].blk // grab the key
	val := args[1].blk // grab the value

	DB.mu.Lock()
	// First check if key exists to get old memory usage
	var oldItem *Item
	if existing, ok := DB.store[key]; ok {
		oldItem = existing
	}
	DB.mu.Unlock()

	// Create new item and calculate memory (without lock)
	newItem := NewStringItem(val)
	newMemory := newItem.approxMemoryUsage(key)

	// Check if we need to evict (without holding lock)
	DB.mu.RLock()
	currentMem := DB.mem
	maxMem := state.config.maxmemory
	DB.mu.RUnlock()

	oldMemory := int64(0)
	if oldItem != nil {
		oldMemory = int64(oldItem.approxMemoryUsage(key))
	}

	// Calculate new total memory
	netNewMemory := newMemory - oldMemory

	if maxMem > 0 && currentMem+netNewMemory >= maxMem {
		// Need to evict - this acquires its own locks
		_, err := DB.EvictKeys(state, netNewMemory)
		if err != nil {
			return NewErrorValue("ERR maxmemory reached: " + err.Error())
		}
	}

	// Now acquire lock and actually put the item
	DB.mu.Lock()
	err := DB.Put(key, val, state)
	if err != nil {
		DB.mu.Unlock()
		return NewErrorValue("ERR some error occured while PUT:" + err.Error())
	}
	// record it for AOF
	if state.config.aofEnabled {
		state.aof.w.Write(v)

		if state.config.aofFsync == Always {
			fmt.Println("save AOF record on SET")
			state.aof.w.Flush()
		}

	}

	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	DB.mu.Unlock()

	return NewStringValue("OK")
}

// Incr handles the INCR command.
// Increments the integer value of a key by one.
//
// Syntax:
//
//	INCR <key>
//
// Returns:
//
//	Integer: The value of key after the increment
func Incr(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'incr' command")
	}
	return incrDecrBy(c, args[0].blk, 1, state, v)
}

// Decr handles the DECR command.
// Decrements the integer value of a key by one.
//
// Syntax:
//
//	DECR <key>
//
// Returns:
//
//	Integer: The value of key after the decrement
func Decr(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'decr' command")
	}
	return incrDecrBy(c, args[0].blk, -1, state, v)
}

// IncrBy handles the INCRBY command.
// Increments the integer value of a key by the given amount.
//
// Syntax:
//
//	INCRBY <key> <increment>
//
// Returns:
//
//	Integer: The value of key after the increment
func IncrBy(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'incrby' command")
	}
	incr, err := ParseInt(args[1].blk)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}
	return incrDecrBy(c, args[0].blk, incr, state, v)
}

// DecrBy handles the DECRBY command.
// Decrements the integer value of a key by the given amount.
//
// Syntax:
//
//	DECRBY <key> <decrement>
//
// Returns:
//
//	Integer: The value of key after the decrement
func DecrBy(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'decrby' command")
	}
	decr, err := ParseInt(args[1].blk)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}
	return incrDecrBy(c, args[0].blk, -decr, state, v)
}

func incrDecrBy(c *Client, key string, delta int64, state *AppState, v *Value) *Value {
	DB.mu.Lock()
	defer DB.mu.Unlock()

	var item *Item
	var oldMemory int64 = 0

	if existing, ok := DB.store[key]; ok {
		item = existing
		if item.IsExpired() {
			oldMemory = item.approxMemoryUsage(key)
			item = NewStringItem("0")
			DB.store[key] = item
		} else {
			if !item.IsString() {
				return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
			}
			oldMemory = item.approxMemoryUsage(key)
		}
	} else {
		item = NewStringItem("0")
		DB.store[key] = item
	}

	val, err := ParseInt(item.Str)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}

	newVal := val + delta
	item.Str = strconv.FormatInt(newVal, 10)

	newMemory := item.approxMemoryUsage(key)
	DB.mem += (newMemory - oldMemory)
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(newVal)
}

// Del handles the DEL command.
// Deletes one or more keys.
//
// Syntax:
//
//	DEL <key1> [key2 ...]
//
// Returns:
//
//	Integer count of keys deleted
//
// Notes:
//   - Non-existent keys are ignored
//   - Thread-safe (write lock)
func Del(c *Client, v *Value, state *AppState) *Value {
	// DEL k1 k2 k3 ... kn
	// returns m, number of keys actually deleted (m <= n)
	args := v.arr[1:]
	m := 0
	DB.mu.Lock()
	for _, arg := range args {
		key := arg.blk
		_, ok := DB.Poll(key)
		if !ok {
			// doesnot exist
			continue
		}
		// delete
		DB.Rem(key)
		m += 1
	}
	DB.mu.Unlock()
	return NewIntegerValue(int64(m))
}

// Exists handles the EXISTS command.
// Checks existence of keys.
//
// Syntax:
//
//	EXISTS <key1> [key2 ...]
//
// Returns:
//
//	Integer count of keys that exist
//
// Thread-safe:
//
//	Uses read lock
func Exists(c *Client, v *Value, state *AppState) *Value {
	// Exists k1 k2 k3 .. kn
	// m (m <= n)
	args := v.arr[1:]
	m := 0
	DB.mu.RLock()
	for _, arg := range args {
		_, ok := DB.store[arg.blk]
		if ok {
			m += 1
		}
	}
	DB.mu.RUnlock()

	return NewIntegerValue(int64(m))
}

// Keys handles the KEYS command.
// Finds keys matching a glob pattern.
//
// Syntax:
//
//	KEYS <pattern>
//
// Pattern rules:
//   - matches any sequence
//     ?  matches single character
//
// Returns:
//
//	Array of matching keys
//
// Thread-safe:
//
//	Uses read lock
func Keys(c *Client, v *Value, state *AppState) *Value {
	// Keys pattern
	// all keys matching pattern (in an array)
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR invlid arg to Keys")
	}

	pattern := args[0].blk // string representing the pattern e.g. "*name*" matches name, names, firstname, lastname

	DB.mu.RLock()
	var matches []string
	for key, _ := range DB.store {
		matched, err := filepath.Match(pattern, key)
		if err != nil {
			fmt.Printf("error matching for keys: (key=%s, pattern=%s)\nError: %s\n", key, pattern, err)
			continue
		}
		if matched {
			matches = append(matches, key)
		}
	}
	DB.mu.RUnlock()

	reply := Value{
		typ: ARRAY,
	}
	for _, key := range matches {
		reply.arr = append(reply.arr, Value{typ: BULK, blk: key})
	}
	return &reply
}

// saves with mutex read lock => inefficent, and no other command can be run to write
// Save handles the SAVE command.
// Performs a synchronous RDB snapshot.
//
// Syntax:
//
//	SAVE
//
// Returns:
//
//	+OK\r\n
//
// Behavior:
//   - Blocks server during save
//   - Uses read lock, preventing writes
//
// Recommendation:
//
//	Use BGSAVE for non-blocking persistence
func Save(c *Client, v *Value, state *AppState) *Value {
	// DB.mu.Lock()
	SaveRDB(state)
	// DB.mu.Unlock()
	return NewStringValue("OK")
}

// background save
// using COW is not possible, we will copy map first then save async
// BGSave handles the BGSAVE command.
// Performs an asynchronous RDB snapshot.
//
// Syntax:
//
//	BGSAVE
//
// Returns:
//
//	+OK\r\n on success
//	Error if a background save is already running
//
// Behavior:
//   - Copies DB state
//   - Saves in background goroutine
//   - Prevents concurrent background saves
func BGSave(c *Client, v *Value, state *AppState) *Value {

	DB.mu.RLock()
	if state.bgsaving {
		// already running, return
		DB.mu.RUnlock()
		return NewErrorValue("already in progress")
	}

	copy := make(map[string]*Item, len(DB.store)) // actual copy of DB.store
	maps.Copy(copy, DB.store)
	state.bgsaving = true
	state.DBCopy = copy // points to that

	DB.mu.RUnlock()

	go func() {
		defer func() {
			state.bgsaving = false
			state.DBCopy = nil
		}()

		SaveRDB(state)
	}()

	return NewStringValue("OK")
}

// FlushDB handles the FLUSHDB command.
// Deletes all keys in the database.
//
// Syntax:
//
//	FLUSHDB
//
// Returns:
//
//	+OK\r\n
//
// Implementation:
//   - Replaces store map for efficiency
//   - Thread-safe (write lock)
//
// Warning:
//
//	This operation is irreversible
func FlushDB(c *Client, v *Value, state *AppState) *Value {
	// slower
	// DB.mu.Lock()
	// for k := range DB.store {
	// 	delete(DB.store, k)
	// }
	// DB.mu.Unlock()

	// fast
	DB.mu.Lock()
	DB.store = map[string]*Item{}
	DB.mu.Unlock()

	return NewStringValue("OK")
}

// DBSize handles the DBSIZE command.
// Returns number of keys.
//
// Syntax:
//
//	DBSIZE
//
// Returns:
//
//	Integer key count
//
// Thread-safe:
//
//	Uses read lock
func DBSize(c *Client, v *Value, state *AppState) *Value {
	// DBSIZE
	args := v.arr
	if len(args) != 1 {
		return NewErrorValue("ERR invalid argument to DBSIZE")
	}

	DB.mu.RLock()
	size := len(DB.store)
	DB.mu.RUnlock()

	return NewIntegerValue(int64(size))

}

// Auth handles the AUTH command.
// Authenticates a client.
//
// Syntax:
//
//	AUTH <password>
//
// Returns:
//
//	+OK\r\n if successful
//	Error if password is incorrect
//
// Behavior:
//   - Sets client.authenticated flag
//   - Safe command (no prior auth required)
func Auth(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue(fmt.Sprintf("ERR invalid argument to AUTH, given=%d, needed=1\n", len(args)))
	}

	password := args[0].blk // AUTH <password>
	if state.config.password == password {
		c.authenticated = true
		return NewStringValue("OK")
	}
	c.authenticated = false
	return NewErrorValue(fmt.Sprintf("ERR invalid password, given=%s", password))

}

// Expire handles the EXPIRE command.
// Sets expiration time on a key.
//
// Syntax:
//
//	EXPIRE <key> <seconds>
//
// Returns:
//
//	1 if expiration set
//	0 if key does not exist
//
// Notes:
//   - Expiration stored as absolute timestamp
//   - Lazy deletion on access
func Expire(c *Client, v *Value, state *AppState) *Value {
	// EXPIRE <key> <secondsafter>
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR invalid args for EXPIRE")
	}
	k := args[0].blk
	exp := args[1].blk
	expirySeconds, err := strconv.Atoi(exp)
	if err != nil {
		return NewErrorValue("ERR invalid 2nd arg for EXPIRE")
	}

	DB.mu.RLock()
	Val, ok := DB.store[k]
	if !ok {
		return NewIntegerValue(0)
	}
	Val.Exp = time.Now().Add(time.Second * time.Duration(expirySeconds))
	DB.mu.RUnlock()

	return NewIntegerValue(1)

}

// Ttl handles the TTL command.
// Returns remaining time-to-live for a key.
//
// Syntax:
//
//	TTL <key>
//
// Returns:
//
//	>0  remaining seconds
//	-1  key exists but no expiration
//	-2  key does not exist or expired
//
// Behavior:
//   - Deletes key if expired
func Ttl(c *Client, v *Value, state *AppState) *Value {
	// TTL <key>
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR invalid arg for TTL")
	}

	k := args[0].blk

	DB.mu.RLock()
	item, ok := DB.store[k]
	if !ok {
		fmt.Println("TTL: key not found ", k)
		DB.mu.RUnlock()
		return NewIntegerValue(-2)
	}
	exp := item.Exp
	DB.mu.RUnlock()

	// is exp not set
	if exp.Unix() == UNIX_TS_EPOCH {
		return NewIntegerValue(-1)
	}

	expired := DB.RemIfExpired(k, item, state)
	if expired {
		return NewIntegerValue(-2)
	}

	secondsToExpire := time.Until(exp).Seconds() //float
	// fmt.Println(secondsToExpire)
	return NewIntegerValue(int64(secondsToExpire))

}

// ============================================================================
// HASH COMMAND HANDLERS
// ============================================================================

// Hset handles the HSET command.
// Sets one or more field-value pairs in a hash.
//
// Syntax:
//
//	HSET <key> <field> <value> [<field> <value> ...]
//
// Returns:
//
//	Integer: Number of fields added (not updated)
//
// Behavior:
//   - Creates hash if it doesn't exist
//   - Updates existing fields
//   - Returns count of new fields added
func Hset(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 3 || len(args)%2 == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'hset' command")
	}

	key := args[0].blk
	DB.mu.Lock()
	defer DB.mu.Unlock()

	// Calculate old memory before modification
	var oldMemory int64 = 0
	var item *Item
	if existing, ok := DB.store[key]; ok {
		item = existing
		oldMemory = existing.approxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return NewErrorValue(err.Error())
		}
	} else {
		item = NewHashItem()
		DB.store[key] = item
	}

	count := int64(0)
	for i := 1; i < len(args); i += 2 {
		field := args[i].blk
		value := args[i+1].blk
		if _, exists := item.Hash[field]; !exists {
			count++
		}
		item.Hash[field] = NewHashFieldItem(value)
	}

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			fmt.Println("AOF write for HSET")
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(count)
}

// Hget handles the HGET command.
// Gets the value of a hash field.
//
// Syntax:
//
//	HGET <key> <field>
//
// Returns:
//
//	Bulk string: Field value
//	NULL: If field or key does not exist
func Hget(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'hget' command")
	}

	key := args[0].blk
	field := args[1].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.Poll(key)
	if ok {
		if !item.IsHash() {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		fieldItem, exists := item.Hash[field]
		if exists {
			// Check if field is expired
			if fieldItem.IsExpired() {
				// Delete here
				fmt.Printf("Expired Key: %s:%s\n", key, field)
				delete(item.Hash, field)
				return NewNullValue()
			}
			// delete if expired
			deleted := DB.RemIfExpired(key, item, state)
			if deleted {
				fmt.Println("Expired Key: ", key)
				return NewNullValue()
			}
			return NewBulkValue(fieldItem.Str)
		}

		return NewNullValue()
	}

	if !ok {
		fmt.Println("Not Found: ", key)
		return NewNullValue()
	}

	return NewBulkValue(item.Str)

}

// Hdel handles the HDEL command.
// Deletes one or more hash fields.
//
// Syntax:
//
//	HDEL <key> <field> [<field> ...]
//
// Returns:
//
//	Integer: Number of fields deleted
func Hdel(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'hdel' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Calculate old memory before modification
	oldMemory := item.approxMemoryUsage(key)

	count := int64(0)
	for i := 1; i < len(args); i++ {
		field := args[i].blk
		if _, exists := item.Hash[field]; exists {
			delete(item.Hash, field)
			count++
		}
	}

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			fmt.Println("AOF write for HDEL")
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(count)
}

// Hgetall handles the HGETALL command.
// Returns all field-value pairs in a hash.
//
// Syntax:
//
//	HGETALL <key>
//
// Returns:
//
//	Array: [field1, value1, field2, value2, ...]
func Hgetall(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hgetall' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.Hash)*2)
	for field, fieldItem := range item.Hash {
		// Skip expired fields
		if fieldItem.IsExpired() {
			continue
		}
		result = append(result, Value{typ: BULK, blk: field})
		result = append(result, Value{typ: BULK, blk: fieldItem.Str})
	}

	return NewArrayValue(result)
}

// ============================================================================
// SET COMMAND HANDLERS
// ============================================================================

// Sadd handles the SADD command.
// Adds one or more members to a set.
//
// Syntax:
//
//	SADD <key> <member> [<member> ...]
//
// Returns:
//
//	Integer: The number of elements that were added to the set, not including all the elements already present into the set.
func Sadd(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'sadd' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	var item *Item
	var oldMemory int64 = 0

	if existing, ok := DB.store[key]; ok {
		item = existing
		if item.Type != "set" {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.approxMemoryUsage(key)
	} else {
		item = &Item{
			Type:    "set",
			ItemSet: make(map[string]bool),
		}
		DB.store[key] = item
	}

	count := int64(0)
	for _, arg := range args[1:] {
		member := arg.blk
		if !item.ItemSet[member] {
			item.ItemSet[member] = true
			count++
		}
	}

	newMemory := item.approxMemoryUsage(key)
	DB.mem += (newMemory - oldMemory)
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(count)
}

// Srem handles the SREM command.
// Removes one or more members from a set.
//
// Syntax:
//
//	SREM <key> <member> [<member> ...]
//
// Returns:
//
//	Integer: The number of members that were removed from the set, not including non existing members.
func Srem(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'srem' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	oldMemory := item.approxMemoryUsage(key)
	count := int64(0)

	for _, arg := range args[1:] {
		member := arg.blk
		if item.ItemSet[member] {
			delete(item.ItemSet, member)
			count++
		}
	}

	if len(item.ItemSet) == 0 {
		delete(DB.store, key)
		DB.mem -= oldMemory
	} else {
		newMemory := item.approxMemoryUsage(key)
		DB.mem += (newMemory - oldMemory)
	}

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(count)
}

// Smembers handles the SMEMBERS command.
// Returns all the members of the set value stored at key.
//
// Syntax:
//
//	SMEMBERS <key>
//
// Returns:
//
//	Array: All elements of the set.
func Smembers(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'smembers' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.ItemSet))
	for member := range item.ItemSet {
		result = append(result, Value{typ: BULK, blk: member})
	}

	return NewArrayValue(result)
}

// Sismember handles the SISMEMBER command.
// Returns if member is a member of the set stored at key.
//
// Syntax:
//
//	SISMEMBER <key> <member>
//
// Returns:
//
//	Integer: 1 if the element is a member of the set. 0 if the element is not a member of the set, or if key does not exist.
func Sismember(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'sismember' command")
	}

	key := args[0].blk
	member := args[1].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if item.ItemSet[member] {
		return NewIntegerValue(1)
	}

	return NewIntegerValue(0)
}

// Scard handles the SCARD command.
// Returns the set cardinality (number of elements) of the set stored at key.
//
// Syntax:
//
//	SCARD <key>
//
// Returns:
//
//	Integer: The cardinality (number of elements) of the set, or 0 if key does not exist.
func Scard(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'scard' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return NewIntegerValue(int64(len(item.ItemSet)))
}

// Rename handles the RENAME command.
// Renames a key.
//
// Syntax:
//
//	RENAME <key> <newkey>
//
// Returns:
//
//	1 if key was renamed
//	0 if key does not exist
func Rename(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'rename' command")
	}

	oldKey := args[0].blk
	newKey := args[1].blk

	if oldKey == newKey {
		return NewIntegerValue(1)
	}

	DB.mu.Lock()
	defer DB.mu.Unlock()

	// Check if source exists
	item, ok := DB.store[oldKey]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.IsExpired() {
		DB.Rem(oldKey)
		return NewIntegerValue(0)
	}

	// If target exists, don't remove it
	if _, ok := DB.store[newKey]; ok {
		return NewIntegerValue(0)
	}

	// Move logic: manually handle memory to avoid DB.Rem clearing hash data
	// 1. Calculate old memory usage
	oldMem := item.approxMemoryUsage(oldKey)

	// 2. Remove from old key (delete directly to preserve item content)
	delete(DB.store, oldKey)
	DB.mem -= oldMem

	// 3. Add to new key
	DB.store[newKey] = item
	newMem := item.approxMemoryUsage(newKey)
	DB.mem += newMem

	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(1)
}

// Hincrby handles the HINCRBY command.
// Increments the integer value of a hash field by the given amount.
//
// Syntax:
//
//	HINCRBY <key> <field> <increment>
//
// Returns:
//
//	Integer: The new value after increment
//
// Behavior:
//   - Creates hash and field if they don't exist (initialized to 0)
//   - Returns error if field value is not a valid integer
func Hincrby(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 3 {
		return NewErrorValue("ERR wrong number of arguments for 'hincrby' command")
	}

	key := args[0].blk
	field := args[1].blk
	incrStr := args[2].blk

	incr, err := ParseInt(incrStr)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}

	DB.mu.Lock()
	defer DB.mu.Unlock()

	var item *Item
	var oldMemory int64 = 0
	if existing, ok := DB.store[key]; ok {
		item = existing
		oldMemory = existing.approxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return NewErrorValue(err.Error())
		}
	} else {
		item = NewHashItem()
		DB.store[key] = item
	}

	var fieldItem *Item
	if existing, ok := item.Hash[field]; ok {
		fieldItem = existing
	} else {
		fieldItem = NewHashFieldItem("0")
		item.Hash[field] = fieldItem
	}

	// Check if field is expired
	if fieldItem.IsExpired() {
		fieldItem = NewHashFieldItem("0")
		item.Hash[field] = fieldItem
	}

	current := int64(0)
	if fieldItem.Str != "" {
		current, err = ParseInt(fieldItem.Str)
		if err != nil {
			return NewErrorValue("ERR hash value is not an integer")
		}
	}

	newVal := current + incr
	fieldItem.Str = fmt.Sprintf("%d", newVal)

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(newVal)
}

// Hmset handles the HMSET command.
// Sets multiple field-value pairs in a hash.
// (Deprecated in favor of HSET, but kept for compatibility)
//
// Syntax:
//
//	HMSET <key> <field> <value> [<field> <value> ...]
//
// Returns:
//
//	String: OK
func Hmset(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 3 || len(args)%2 == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'hmset' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	// Calculate old memory before modification
	var oldMemory int64 = 0
	var item *Item
	if existing, ok := DB.store[key]; ok {
		item = existing
		oldMemory = existing.approxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return NewErrorValue(err.Error())
		}
	} else {
		item = NewHashItem()
		DB.store[key] = item
	}

	for i := 1; i < len(args); i += 2 {
		field := args[i].blk
		value := args[i+1].blk
		item.Hash[field] = NewHashFieldItem(value)
	}

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewStringValue("OK")
}

// Hexists handles the HEXISTS command.
// Checks if a hash field exists.
//
// Syntax:
//
//	HEXISTS <key> <field>
//
// Returns:
//
//	Integer: 1 if field exists, 0 otherwise
func Hexists(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'hexists' command")
	}

	key := args[0].blk
	field := args[1].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	_, exists := item.Hash[field]
	if exists {
		return NewIntegerValue(1)
	}

	return NewIntegerValue(0)
}

// Hlen handles the HLEN command.
// Returns the number of fields in a hash.
//
// Syntax:
//
//	HLEN <key>
//
// Returns:
//
//	Integer: Number of fields in the hash
func Hlen(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hlen' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return NewIntegerValue(int64(len(item.Hash)))
}

// Hkeys handles the HKEYS command.
// Returns all field names in a hash.
//
// Syntax:
//
//	HKEYS <key>
//
// Returns:
//
//	Array: List of field names
func Hkeys(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hkeys' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.Hash))
	for field, fieldItem := range item.Hash {
		// Skip expired fields
		if !fieldItem.IsExpired() {
			result = append(result, Value{typ: BULK, blk: field})
		}
	}

	return NewArrayValue(result)
}

// Hvals handles the HVALS command.
// Returns all values in a hash.
//
// Syntax:
//
//	HVALS <key>
//
// Returns:
//
//	Array: List of values
func Hvals(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hvals' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.Hash))
	for _, fieldItem := range item.Hash {
		// Skip expired fields
		if !fieldItem.IsExpired() {
			result = append(result, Value{typ: BULK, blk: fieldItem.Str})
		}
	}

	return NewArrayValue(result)
}

// Hdelall handles the HDELALL command.
// Removes all field-value pairs from a hash, effectively clearing it.
//
// Syntax:
//
//	HDELALL <key>
//
// Returns:
//
//	Integer: Number of fields that were deleted
//	0: If hash doesn't exist or is already empty
//
// Behavior:
//   - Deletes all fields from the hash
//   - Removes the hash key itself if all fields are deleted
//   - Returns the count of fields deleted
func Hdelall(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hdelall' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Calculate old memory before clearing
	oldMemory := item.approxMemoryUsage(key)

	count := int64(len(item.Hash))
	item.Hash = make(map[string]*Item) // Clear the hash

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(count)
}

// Hexpire handles the HEXPIRE command.
// Sets expiration time on a specific hash field.
//
// Syntax:
//
//	HEXPIRE <key> <field> <seconds>
//
// Returns:
//
//	Integer: 1 if expiration set, 0 if field doesn't exist
//
// Behavior:
//   - Sets the expiration time for a specific field in the hash
//   - The field must exist in the hash
//   - After the expiration time, the field will be treated as expired
//   - Expired fields are lazily deleted on access
//
// Example:
//
//	HEXPIRE myhash field1 10   // field1 expires after 10 seconds
func Hexpire(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 3 {
		return NewErrorValue("ERR wrong number of arguments for 'hexpire' command")
	}

	key := args[0].blk
	field := args[1].blk
	secondsStr := args[2].blk

	seconds, err := strconv.Atoi(secondsStr)
	if err != nil {
		return NewErrorValue("ERR invalid expiration time")
	}

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	fieldItem, exists := item.Hash[field]
	if !exists {
		return NewIntegerValue(0)
	}

	// Set expiration on the field
	fieldItem.Exp = time.Now().Add(time.Second * time.Duration(seconds))

	return NewIntegerValue(1)
}

// ============================================================================
// LIST COMMAND HANDLERS
// ============================================================================

// Lpush handles the LPUSH command.
// Prepends one or more values to a list.
//
// Syntax:
//
//	LPUSH <key> <value> [<value> ...]
//
// Returns:
//
//	Integer: The length of the list after the push operations.
func Lpush(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'lpush' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	var item *Item
	var oldMemory int64 = 0

	if existing, ok := DB.store[key]; ok {
		item = existing
		if item.Type != LIST_TYPE {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.approxMemoryUsage(key)
	} else {
		item = &Item{
			Type: LIST_TYPE,
			List: []string{},
		}
		DB.store[key] = item
	}

	// Push values (prepend)
	// LPUSH k v1 v2 => v2, v1, ...
	for _, arg := range args[1:] {
		item.List = append([]string{arg.blk}, item.List...)
	}

	newMemory := item.approxMemoryUsage(key)
	DB.mem += (newMemory - oldMemory)
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(int64(len(item.List)))
}

// Rpush handles the RPUSH command.
// Appends one or more values to a list.
//
// Syntax:
//
//	RPUSH <key> <value> [<value> ...]
//
// Returns:
//
//	Integer: The length of the list after the push operations.
func Rpush(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'rpush' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	var item *Item
	var oldMemory int64 = 0

	if existing, ok := DB.store[key]; ok {
		item = existing
		if item.Type != LIST_TYPE {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.approxMemoryUsage(key)
	} else {
		item = &Item{
			Type: LIST_TYPE,
			List: []string{},
		}
		DB.store[key] = item
	}

	// Push values (append)
	for _, arg := range args[1:] {
		item.List = append(item.List, arg.blk)
	}

	newMemory := item.approxMemoryUsage(key)
	DB.mem += (newMemory - oldMemory)
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(int64(len(item.List)))
}

// Lpop handles the LPOP command.
// Removes and returns the first element of the list.
//
// Syntax:
//
//	LPOP <key>
//
// Returns:
//
//	Bulk String: The value of the first element, or NULL if key does not exist.
func Lpop(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'lpop' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewNullValue()
	}

	if item.Type != LIST_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if len(item.List) == 0 {
		return NewNullValue()
	}

	oldMemory := item.approxMemoryUsage(key)

	// Pop first
	val := item.List[0]
	item.List = item.List[1:]

	// If empty, remove key
	if len(item.List) == 0 {
		delete(DB.store, key)
		DB.mem -= oldMemory
	} else {
		newMemory := item.approxMemoryUsage(key)
		DB.mem += (newMemory - oldMemory)
	}

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewBulkValue(val)
}

// Rpop handles the RPOP command.
// Removes and returns the last element of the list.
//
// Syntax:
//
//	RPOP <key>
//
// Returns:
//
//	Bulk String: The value of the last element, or NULL if key does not exist.
func Rpop(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'rpop' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewNullValue()
	}

	if item.Type != LIST_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if len(item.List) == 0 {
		return NewNullValue()
	}

	oldMemory := item.approxMemoryUsage(key)

	// Pop last
	lastIdx := len(item.List) - 1
	val := item.List[lastIdx]
	item.List = item.List[:lastIdx]

	// If empty, remove key
	if len(item.List) == 0 {
		delete(DB.store, key)
		DB.mem -= oldMemory
	} else {
		newMemory := item.approxMemoryUsage(key)
		DB.mem += (newMemory - oldMemory)
	}

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewBulkValue(val)
}

// Lrange handles the LRANGE command.
// Returns the specified elements of the list stored at key.
//
// Syntax:
//
//	LRANGE <key> <start> <stop>
//
// Returns:
//
//	Array: List of elements in the specified range.
func Lrange(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 3 {
		return NewErrorValue("ERR wrong number of arguments for 'lrange' command")
	}

	key := args[0].blk
	startStr := args[1].blk
	stopStr := args[2].blk

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}
	stop, err := strconv.Atoi(stopStr)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if item.Type != LIST_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listLen := len(item.List)

	// Handle negative indices
	if start < 0 {
		start = listLen + start
	}
	if stop < 0 {
		stop = listLen + stop
	}

	// Clamp indices
	if start < 0 {
		start = 0
	}
	if stop >= listLen {
		stop = listLen - 1
	}

	if start > stop {
		return NewArrayValue([]Value{})
	}

	// Extract range
	result := make([]Value, 0, stop-start+1)
	for i := start; i <= stop; i++ {
		result = append(result, Value{typ: BULK, blk: item.List[i]})
	}

	return NewArrayValue(result)
}

// Llen handles the LLEN command.
// Returns the length of the list stored at key.
//
// Syntax:
//
//	LLEN <key>
//
// Returns:
//
//	Integer: The length of the list, or 0 if key does not exist.
func Llen(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'llen' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != LIST_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return NewIntegerValue(int64(len(item.List)))
}

// Lindex handles the LINDEX command.
// Returns the element at index index in the list stored at key.
//
// Syntax:
//
//	LINDEX <key> <index>
//
// Returns:
//
//	Bulk String: The requested element, or NULL if index is out of range.
func Lindex(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'lindex' command")
	}

	key := args[0].blk
	indexStr := args[1].blk

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewNullValue()
	}

	if item.Type != LIST_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Handle negative index
	if index < 0 {
		index = len(item.List) + index
	}

	if index < 0 || index >= len(item.List) {
		return NewNullValue()
	}

	return NewBulkValue(item.List[index])
}

// Type handles the TYPE command.
// Returns the string representation of the type of the value stored at key.
//
// Syntax:
//
//	TYPE <key>
//
// Returns:
//
//	Simple String: type of key (e.g., STRING, LIST, HASH), or "none" if key does not exist.
func Type(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'type' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewStringValue("none")
	}

	return NewStringValue(strings.ToUpper(item.Type))
}

// Lget handles the LGET command.
// Returns all elements of the list stored at key.
// This is a custom command equivalent to LRANGE <key> 0 -1.
//
// Syntax:
//
//	LGET <key>
//
// Returns:
//
//	Array: List of all elements in the list.
func Lget(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'lget' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if item.Type != LIST_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.List))
	for _, val := range item.List {
		result = append(result, Value{typ: BULK, blk: val})
	}

	return NewArrayValue(result)
}
