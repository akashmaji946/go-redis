/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_generic.go
*/
package handlers

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

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
func Info(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]

	// INFO (no args) -> global server info
	if len(args) == 0 {
		usedMem := database.DB.Mem
		usedMemPeak := database.DB.Mempeak
		msg := state.RedisInfo.Print(state, usedMem, usedMemPeak)
		return &common.Value{Typ: common.BULK, Blk: msg}
	}

	// INFO <key> -> per-key info
	if len(args) == 1 {
		key := args[0].Blk

		database.DB.Mu.RLock()
		item, ok := database.DB.Store[key]
		database.DB.Mu.RUnlock()
		if !ok {
			return common.NewErrorValue("ERR key not found")
		}

		typ := item.Type
		if typ == "" {
			typ = common.STRING_TYPE
		}

		length := 0
		switch typ {
		case common.STRING_TYPE:
			length = 1
		case common.LIST_TYPE:
			length = len(item.List)
		case common.HASH_TYPE:
			length = len(item.Hash)
		case "set":
			length = len(item.ItemSet)
		case common.ZSET_TYPE:
			length = len(item.ZSet)
		}

		ttl := int64(-1)
		if item.Exp.Unix() != common.UNIX_TS_EPOCH {
			ttl = int64(time.Until(item.Exp).Seconds())
		}

		mem := item.ApproxMemoryUsage(key)

		msg := fmt.Sprintf(
			"type: %s\nlen: %d\nttl: %d\nmem: %d B\naccesses: %d\n",
			strings.ToUpper(typ), length, ttl, mem, item.AccessCount,
		)
		return common.NewBulkValue(msg)
	}

	// Anything else is invalid
	return common.NewErrorValue("ERR wrong number of arguments for 'info' command")
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
func Monitor(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of MONITOR")
		return common.NewErrorValue("ERR invalid argument to MONITOR")
	}
	state.Monitors = append(state.Monitors, *c)
	return common.NewStringValue("OK")
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
func FlushDB(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// slower
	// database.DB.Mu.Lock()
	// for k := range database.DB.Store {
	// 	delete(database.DB.Store, k)
	// }
	// database.DB.Mu.Unlock()

	// fast
	database.DB.Mu.Lock()
	database.DB.Store = map[string]*common.Item{}
	database.DB.TouchAll()
	database.DB.Mu.Unlock()

	return common.NewStringValue("OK")
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
func DBSize(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// DBSIZE
	args := v.Arr
	if len(args) != 1 {
		return common.NewErrorValue("ERR invalid argument to DBSIZE")
	}

	database.DB.Mu.RLock()
	size := len(database.DB.Store)
	database.DB.Mu.RUnlock()

	return common.NewIntegerValue(int64(size))

}

// Select handles the SELECT command.
// Changes the selected database for the current connection.
//
// Syntax:
//
//	SELECT <db_index>
//
// Parameters:
//   - db_index: Integer index of the database to select (0-based)
//
// Returns:
//
//	+OK\r\n on success
//	Error if db_index is invalid
//
// Behavior:
//  1. Validates db_index is within range of available databases
//  2. Updates client's DatabaseID to the specified index
//  3. Subsequent commands from this client will operate on the selected database
func Select(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'select' command")
	}

	dbIndex, err := common.ParseInt(args[0].Blk)
	if err != nil || dbIndex < 0 || dbIndex >= int64(len(database.DBS)) {
		return common.NewErrorValue("ERR invalid DB index")
	}

	c.DatabaseID = int(dbIndex)
	return common.NewStringValue("OK")
}

// Size handles the SIZE command.
// Returns the number of databases configured in the server.
//
// Syntax:
//
//	SIZE [dbIndex]
//
// Returns:
//
//	Integer: The number of databases configured
func Size(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) > 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'size' command")
	}

	if len(args) == 1 {
		idx, err := common.ParseInt(args[0].Blk)
		if err != nil || idx < 0 || idx >= int64(len(database.DBS)) {
			return common.NewErrorValue("ERR invalid DB index")
		}
		dbSize := len(database.DBS[idx].Store)
		return common.NewIntegerValue(int64(dbSize))
	}
	numDBs := len(database.DBS)
	return common.NewIntegerValue(int64(numDBs))
}

// Echo handles the ECHO command.
// Returns the same message sent by the client.
//
// Syntax:
//
//	ECHO <message>
//
// Parameters:
//   - message: The message string to echo back
//
// Returns:
//
//	Bulk string: The same message sent by the client

func Echo(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'echo' command")
	}
	msg := args[0].Blk
	return common.NewBulkValue(msg)
}
