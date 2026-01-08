/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_key.go
*/
package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

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
		DB.Touch(key)
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
	expirySeconds, err := ParseInt(exp)
	if err != nil {
		return NewErrorValue("ERR invalid 2nd arg for EXPIRE")
	}

	DB.mu.RLock()
	Val, ok := DB.store[k]
	if !ok {
		return NewIntegerValue(0)
	}
	Val.Exp = time.Now().Add(time.Second * time.Duration(expirySeconds))
	DB.Touch(k)
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

// Persist handles the PERSIST command.
// Remove the existing timeout on key.
//
// Syntax:
//
//	PERSIST <key>
//
// Returns:
//
//	Integer: 1 if timeout was removed.
//	Integer: 0 if key does not exist or does not have an associated timeout.
func Persist(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'persist' command")
	}
	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.IsExpired() {
		DB.Rem(key)
		return NewIntegerValue(0)
	}

	if item.Exp.IsZero() {
		return NewIntegerValue(0)
	}

	item.Exp = time.Time{} // Clear expiration
	DB.Touch(key)
	return NewIntegerValue(1)
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

	DB.Touch(oldKey)
	DB.Touch(newKey)

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
