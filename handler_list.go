/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_list.go
*/
package main

import (
	"strconv"
)

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

	DB.Touch(key)
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

	DB.Touch(key)
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

	DB.Touch(key)
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

	DB.Touch(key)
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
