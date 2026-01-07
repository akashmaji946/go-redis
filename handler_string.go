/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_string.go
*/
package main

import (
	"fmt"
	"strconv"
)

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

// Mget handles the MGET command.
// Returns the values of all specified keys.
//
// Syntax:
//
//	MGET <key> [<key> ...]
//
// Returns:
//
//	Array: List of values at the specified keys.
func Mget(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 1 {
		return NewErrorValue("ERR wrong number of arguments for 'mget' command")
	}

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	result := make([]Value, 0, len(args))

	for _, arg := range args {
		key := arg.blk
		item, ok := DB.Poll(key)

		if !ok || item.IsExpired() {
			result = append(result, Value{typ: NULL})
			continue
		}

		if item.Type != STRING_TYPE {
			result = append(result, Value{typ: NULL})
			continue
		}

		result = append(result, Value{typ: BULK, blk: item.Str})
	}

	return NewArrayValue(result)
}

// Mset handles the MSET command.
// Sets multiple keys to multiple values.
//
// Syntax:
//
//	MSET <key> <value> [<key> <value> ...]
//
// Returns:
//
//	Simple String: OK
func Mset(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) == 0 || len(args)%2 != 0 {
		return NewErrorValue("ERR wrong number of arguments for 'mset' command")
	}

	DB.mu.Lock()
	defer DB.mu.Unlock()

	for i := 0; i < len(args); i += 2 {
		key := args[i].blk
		val := args[i+1].blk
		DB.Put(key, val, state)
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

	return NewStringValue("OK")
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
