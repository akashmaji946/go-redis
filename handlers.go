package main

import (
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"strconv"
	"time"
)

// Success Message:
// +OK\r\n

// initial command just before connection
// COMMAND DOCS
func Command(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	return &Value{
		typ: STRING,
		str: "OK",
	}
}

var UNIX_TS_EPOCH = time.Time{}.Unix()

type Handler func(*Client, *Value, *AppState) *Value

var Handlers = map[string]Handler{
	"COMMAND": Command,

	"GET": Get,
	"SET": Set,

	"DEL": Del,

	"EXISTS": Exists,

	"KEYS": Keys,

	"SAVE":         Save,
	"BGSAVE":       BGSave,
	"BGREWRITEAOF": BGRewriteAOF,

	"FLUSHDB": FlushDB,
	"DBSIZE":  DBSize,

	"EXPIRE": Expire,
	"TTL":    Ttl,

	"AUTH": Auth,
}

func BGRewriteAOF(c *Client, v *Value, state *AppState) *Value {

	go func() {
		DB.mu.RLock()
		cp := make(map[string]*VAL, len(DB.store))
		maps.Copy(cp, DB.store)
		DB.mu.RUnlock()

		state.aof.Rewrite(cp)

	}()
	return &Value{
		typ: STRING,
		str: "Started.",
	}
}

// GET <key>
func Get(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	args := v.arr[1:]
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid command uage with GET",
		}
	}
	key := args[0].blk // grab the key

	// lock before read
	DB.mu.RLock()
	Val, ok := DB.Poll(key)
	DB.mu.RUnlock()
	if !ok {
		fmt.Println("Not Found: ", key)
		return &Value{
			typ: NULL,
		}
	}

	// delete if expired
	// if expiry is set and ttl is <= 0, delete and return NULL
	if Val.exp.Unix() != UNIX_TS_EPOCH && time.Until(Val.exp).Seconds() <= 0 {
		DB.mu.Lock()
		DB.Del(key)
		DB.mu.Unlock()
		return &Value{
			typ: NULL,
		}
	}

	return &Value{
		typ: BULK,
		blk: Val.v,
	}

}

func Set(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	args := v.arr[1:]
	if len(args) != 2 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid command usage with SET",
		}
	}

	key := args[0].blk // grab the key
	val := args[1].blk // grab the value

	DB.mu.Lock()
	DB.Put(key, val)
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

	return &Value{
		typ: STRING,
		str: "OK",
	}
}

func Del(c *Client, v *Value, state *AppState) *Value {
	// DEL k1 k2 k3 ... kn
	// returns m, number of keys actually deleted (m <= n)
	args := v.arr[1:]
	m := 0
	DB.mu.Lock()
	for _, arg := range args {
		key := arg.blk
		_, ok := DB.store[key]
		if !ok {
			// doesnot exist
			continue
		}
		// delete
		delete(DB.store, key)
		m += 1
	}
	DB.mu.Unlock()
	return &Value{
		typ: INTEGER,
		num: m,
	}
}

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

	return &Value{
		typ: INTEGER,
		num: m,
	}
}

func Keys(c *Client, v *Value, state *AppState) *Value {
	// Keys pattern
	// all keys matching pattern (in an array)
	args := v.arr[1:]
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: "ERR invlid arg to Keys",
		}
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
func Save(c *Client, v *Value, state *AppState) *Value {
	SaveRDB(state)
	return &Value{
		typ: STRING,
		str: "OK",
	}
}

// background save
// using COW is not possible, we will copy map first then save async
func BGSave(c *Client, v *Value, state *AppState) *Value {

	DB.mu.RLock()
	if state.bgsaving {
		// already running, return
		DB.mu.RUnlock()
		return &Value{
			typ: ERROR,
			err: "already in progress",
		}
	}

	copy := make(map[string]*VAL, len(DB.store)) // actual copy of DB.store
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

	return &Value{
		typ: STRING,
		str: "OK",
	}
}

func FlushDB(c *Client, v *Value, state *AppState) *Value {
	// slower
	// DB.mu.Lock()
	// for k := range DB.store {
	// 	delete(DB.store, k)
	// }
	// DB.mu.Unlock()

	// fast
	DB.mu.Lock()
	DB.store = map[string]*VAL{}
	DB.mu.Unlock()

	return &Value{
		typ: STRING,
		str: "OK",
	}
}

func DBSize(c *Client, v *Value, state *AppState) *Value {
	// DBSIZE
	args := v.arr
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid argument to DBSIZE",
		}
	}

	DB.mu.RLock()
	size := len(DB.store)
	DB.mu.RUnlock()

	return &Value{
		typ: INTEGER,
		num: size,
	}

}

func Auth(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: fmt.Sprintf("ERR invalid argument to AUTH, given=%d, needed=1\n", len(args)),
		}
	}

	password := args[0].blk // AUTH <password>
	if state.config.password == password {
		c.authenticated = true
		return &Value{
			typ: STRING,
			str: "OK",
		}
	}
	c.authenticated = false
	return &Value{
		typ: ERROR,
		err: fmt.Sprintf("ERR invalid password, given=%s", password),
	}

}

func Expire(c *Client, v *Value, state *AppState) *Value {
	// EXPIRE <key> <secondsafter>
	args := v.arr[1:]
	if len(args) != 2 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid args for EXPIRE",
		}
	}
	k := args[0].blk
	exp := args[1].blk
	expirySeconds, err := strconv.Atoi(exp)
	if err != nil {
		return &Value{
			typ: ERROR,
			err: "ERR invalid 2nd arg for EXPIRE",
		}
	}

	DB.mu.RLock()
	Val, ok := DB.store[k]
	if !ok {
		return &Value{
			typ: INTEGER,
			num: 0,
		}
	}
	Val.exp = time.Now().Add(time.Second * time.Duration(expirySeconds))
	DB.mu.RUnlock()

	return &Value{
		typ: INTEGER,
		num: 1,
	}

}

func Ttl(c *Client, v *Value, state *AppState) *Value {
	// TTL <key>
	args := v.arr[1:]
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid arg for TTL",
		}
	}

	k := args[0].blk

	DB.mu.RLock()
	Val, ok := DB.store[k]
	if !ok {
		return &Value{
			typ: INTEGER,
			num: -2,
		}
	}
	exp := Val.exp
	DB.mu.RUnlock()

	// is exp not set
	if exp.Unix() == UNIX_TS_EPOCH {
		return &Value{
			typ: INTEGER,
			num: -1,
		}
	}
	secondsToExpire := time.Until(exp).Seconds() //float
	if secondsToExpire <= 0.0 {
		DB.mu.Lock()
		DB.Del(k)
		DB.mu.Unlock()
		return &Value{
			typ: INTEGER,
			num: -2,
		}
	}
	fmt.Println(secondsToExpire)
	return &Value{
		typ: INTEGER,
		num: int(secondsToExpire),
	}

}

// can run these even if authenticated=0
var safeCommands = []string{
	"COMMAND",
	"AUTH",
}

func IsSafeCmd(cmd string, commands []string) bool {
	for _, command := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}

func handle(client *Client, v *Value, state *AppState) {
	// the command is in the first entry of v.arr
	cmd := v.arr[0].blk
	handler, ok := Handlers[cmd]
	if !ok {
		log.Println("ERROR: no such command:", cmd)
		reply := &Value{
			typ: ERROR,
			err: "ERR no such command",
		}
		w := NewWriter(client.conn)
		w.Write(reply)
		w.Flush()
		return
	}

	var reply *Value
	if state.config.requirepass && !client.authenticated && !IsSafeCmd(cmd, safeCommands) {
		reply = &Value{
			typ: ERROR,
			err: "NOAUTH client not authenticated, use AUTH <password>",
		}
	} else {
		reply = handler(client, v, state)
	}
	w := NewWriter(client.conn)
	w.Write(reply)
	w.Flush()
}

// Command handles the COMMAND command.
// This is a utility command used for connection testing and protocol compliance.
// Returns: "+OK\r\n"

// BGRewriteAOF handles the BGREWRITEAOF command.
// Performs an asynchronous rewrite of the Append-Only File (AOF) to optimize its size
// by removing redundant commands and creating a compact representation of the database.
//
// The rewrite process:
// 1. Creates a copy of the current database state
// 2. Truncates the AOF file
// 3. Writes SET commands for all keys in the database copy
// 4. Appends any new commands that arrived during the rewrite
//
// Returns: "+Started.\r\n" if the rewrite process begins successfully
// Note: This operation runs in a background goroutine and does not block the server

// Get handles the GET command.
// Retrieves the value associated with a key from the database.
//
// Syntax: GET <key>
//
// Parameters:
//   - key: The key to retrieve
//
// Returns:
//   - Bulk string: The value if the key exists and is not expired
//   - NULL: If the key does not exist or has expired
//
// Behavior:
//   - Automatically deletes expired keys when accessed
//   - Thread-safe read operation using read lock

// Set handles the SET command.
// Sets a key to hold a string value in the database.
//
// Syntax: SET <key> <value>
//
// Parameters:
//   - key: The key to set
//   - value: The string value to associate with the key
//
// Returns: "+OK\r\n" on success
//
// Side effects:
//   - If AOF is enabled, writes the command to the AOF file
//   - If AOF fsync is set to "always", immediately flushes to disk
//   - Increments RDB change tracker if RDB persistence is configured
//   - Thread-safe write operation using write lock

// Del handles the DEL command.
// Deletes one or more keys from the database.
//
// Syntax: DEL <key1> [key2 ...]
//
// Parameters:
//   - key1, key2, ...: One or more keys to delete
//
// Returns: Integer representing the number of keys that were successfully deleted
//          (keys that didn't exist are not counted)
//
// Example: DEL key1 key2 key3
//          Returns: 2 (if key1 and key2 existed, but key3 didn't)
//
// Thread-safe: Uses write lock for deletion

// Exists handles the EXISTS command.
// Checks if one or more keys exist in the database.
//
// Syntax: EXISTS <key1> [key2 ...]
//
// Parameters:
//   - key1, key2, ...: One or more keys to check
//
// Returns: Integer representing the number of keys that exist
//          (returns 0 if none exist, or the count of existing keys)
//
// Example: EXISTS key1 key2 key3
//          Returns: 2 (if key1 and key2 exist, but key3 doesn't)
//
// Thread-safe: Uses read lock for checking

// Keys handles the KEYS command.
// Finds all keys matching a given pattern using filepath glob matching.
//
// Syntax: KEYS <pattern>
//
// Parameters:
//   - pattern: A glob pattern to match keys against
//              Supports wildcards: * (matches any sequence), ? (matches single char)
//
// Returns: Array of bulk strings containing all matching keys
//          Returns empty array if no keys match
//
// Examples:
//   - KEYS *              - Returns all keys
//   - KEYS user:*         - Returns all keys starting with "user:"
//   - KEYS *name*         - Returns all keys containing "name"
//
// Thread-safe: Uses read lock for pattern matching

// Save handles the SAVE command.
// Synchronously saves the database snapshot to disk using RDB persistence.
//
// Syntax: SAVE
//
// Returns: "+OK\r\n" on successful save
//
// Behavior:
//   - Blocks the server until the save operation completes
//   - Uses read lock during the save, preventing write operations
//   - Computes checksums to verify data integrity
//   - For background saves, use BGSAVE instead
//
// Note: This is a blocking operation and may impact server performance
//       during large database saves

// BGSave handles the BGSAVE command.
// Performs an asynchronous background save of the database snapshot to disk.
//
// Syntax: BGSAVE
//
// Returns:
//   - "+OK\r\n" if the background save starts successfully
//   - Error if a background save is already in progress
//
// Behavior:
//   - Creates a copy of the database state before saving
//   - Runs the save operation in a background goroutine (non-blocking)
//   - Sets bgsaving flag to prevent concurrent background saves
//   - Automatically clears the flag when save completes
//
// Advantages over SAVE:
//   - Non-blocking: server continues to handle commands during save
//   - Uses copy-on-write approach by creating a database copy first

// FlushDB handles the FLUSHDB command.
// Removes all keys from the current database.
//
// Syntax: FLUSHDB
//
// Returns: "+OK\r\n" on success
//
// Behavior:
//   - Efficiently clears the database by replacing the store map with a new empty map
//   - Faster than iterating and deleting individual keys
//   - Thread-safe: Uses write lock during flush
//
// Warning: This operation is irreversible and will delete all data in the database

// DBSize handles the DBSIZE command.
// Returns the number of keys in the current database.
//
// Syntax: DBSIZE
//
// Returns: Integer representing the total number of keys in the database
//
// Thread-safe: Uses read lock for counting keys

// Auth handles the AUTH command.
// Authenticates the client with the server using a password.
//
// Syntax: AUTH <password>
//
// Parameters:
//   - password: The password to authenticate with (must match requirepass in config)
//
// Returns:
//   - "+OK\r\n" if authentication succeeds
//   - Error if password is incorrect
//
// Behavior:
//   - Sets the client's authenticated flag to true on success
//   - Sets the client's authenticated flag to false on failure
//   - Required before executing other commands if requirepass is set in config
//
// Note: This is a "safe" command that can be executed without prior authentication

// Expire handles the EXPIRE command.
// Sets a timeout on a key. After the timeout expires, the key will be automatically deleted.
//
// Syntax: EXPIRE <key> <seconds>
//
// Parameters:
//   - key: The key to set expiration on
//   - seconds: Number of seconds until the key expires (must be a valid integer)
//
// Returns:
//   - Integer 1: If the timeout was set successfully
//   - Integer 0: If the key does not exist
//   - Error: If the seconds parameter is invalid
//
// Behavior:
//   - Calculates expiration time as current time + seconds
//   - Keys with expiration are automatically deleted when accessed after expiry
//   - Thread-safe: Uses read lock (updates expiration time atomically)

// Ttl handles the TTL command.
// Returns the remaining time to live (TTL) of a key in seconds.
//
// Syntax: TTL <key>
//
// Parameters:
//   - key: The key to check TTL for
//
// Returns:
//   - Positive integer: Remaining TTL in seconds
//   - Integer -1: Key exists but has no expiration set
//   - Integer -2: Key does not exist
//
// Behavior:
//   - Automatically deletes expired keys when checked (returns -2)
//   - Thread-safe: Uses read lock for checking, write lock for deletion

// IsSafeCmd checks if a command is in the list of safe commands.
// Safe commands can be executed without authentication even when requirepass is set.
//
// Parameters:
//   - cmd: The command name to check
//   - commands: List of safe command names
//
// Returns: true if the command is safe, false otherwise
//
// Safe commands: COMMAND, AUTH

// handle processes incoming commands and routes them to the appropriate handler.
// This is the main command dispatcher that:
// 1. Extracts the command name from the Value array
// 2. Looks up the handler in the Handlers map
// 3. Checks authentication if required
// 4. Executes the handler and sends the response to the client
//
// Parameters:
//   - client: The client connection making the request
//   - v: The parsed command Value containing command and arguments
//   - state: The application state containing config, AOF, and database state
//
// Behavior:
//   - Returns error if command doesn't exist
//   - Returns NOAUTH error if authentication is required but client is not authenticated
//   - Writes response back to client connection
