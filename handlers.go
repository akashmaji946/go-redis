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

// BGRewriteAOF handles the BGREWRITEAOF command.
// Asynchronously rewrites the Append-Only File.
//
// Behavior:
//   1. Copies current DB state
//   2. Rewrites AOF with compact SET commands
//   3. Runs in background goroutine
//
// Returns:
//   +Started.\r\n

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

	return &Value{
		typ: INTEGER,
		num: m,
	}
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
// Save handles the SAVE command.
// Performs a synchronous RDB snapshot.
//
// Syntax:
//   SAVE
//
// Returns:
//   +OK\r\n
//
// Behavior:
//   - Blocks server during save
//   - Uses read lock, preventing writes
//
// Recommendation:
//   Use BGSAVE for non-blocking persistence

func Save(c *Client, v *Value, state *AppState) *Value {
	SaveRDB(state)
	return &Value{
		typ: STRING,
		str: "OK",
	}
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
	DB.store = map[string]*VAL{}
	DB.mu.Unlock()

	return &Value{
		typ: STRING,
		str: "OK",
	}
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
//  2. Lookup command handler
//  3. Enforce authentication rules
//  4. Execute handler
//  5. Write response to client
//
// Error cases:
//   - Unknown command → ERR no such command
//   - Authentication required but missing → NOAUTH error
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
