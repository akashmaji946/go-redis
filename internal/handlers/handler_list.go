/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_list.go
*/
package handlers

import (
	"strconv"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
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
func Lpush(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lpush' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		if item.Type != common.LIST_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.ApproxMemoryUsage(key)
	} else {
		item = &common.Item{
			Type: common.LIST_TYPE,
			List: []string{},
		}
		database.DB.Store[key] = item
	}

	// Push values (prepend)
	// LPUSH k v1 v2 => v2, v1, ...
	for _, arg := range args[1:] {
		item.List = append([]string{arg.Blk}, item.List...)
	}

	database.DB.Touch(key)
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem += (newMemory - oldMemory)
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(int64(len(item.List)))
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
func Rpush(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'rpush' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		if item.Type != common.LIST_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.ApproxMemoryUsage(key)
	} else {
		item = &common.Item{
			Type: common.LIST_TYPE,
			List: []string{},
		}
		database.DB.Store[key] = item
	}

	// Push values (append)
	for _, arg := range args[1:] {
		item.List = append(item.List, arg.Blk)
	}

	database.DB.Touch(key)
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem += (newMemory - oldMemory)
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(int64(len(item.List)))
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
//	Bulk String: The value of the first element, or common.NULL if key does not exist.
func Lpop(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lpop' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewNullValue()
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if len(item.List) == 0 {
		return common.NewNullValue()
	}

	oldMemory := item.ApproxMemoryUsage(key)

	// Pop first
	val := item.List[0]
	item.List = item.List[1:]

	database.DB.Touch(key)
	// If empty, remove key
	if len(item.List) == 0 {
		delete(database.DB.Store, key)
		database.DB.Mem -= oldMemory
	} else {
		newMemory := item.ApproxMemoryUsage(key)
		database.DB.Mem += (newMemory - oldMemory)
	}

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewBulkValue(val)
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
//	Bulk String: The value of the last element, or common.NULL if key does not exist.
func Rpop(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'rpop' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewNullValue()
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if len(item.List) == 0 {
		return common.NewNullValue()
	}

	oldMemory := item.ApproxMemoryUsage(key)

	// Pop last
	lastIdx := len(item.List) - 1
	val := item.List[lastIdx]
	item.List = item.List[:lastIdx]

	database.DB.Touch(key)
	// If empty, remove key
	if len(item.List) == 0 {
		delete(database.DB.Store, key)
		database.DB.Mem -= oldMemory
	} else {
		newMemory := item.ApproxMemoryUsage(key)
		database.DB.Mem += (newMemory - oldMemory)
	}

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewBulkValue(val)
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
func Lrange(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lrange' command")
	}

	key := args[0].Blk
	startStr := args[1].Blk
	stopStr := args[2].Blk

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}
	stop, err := strconv.Atoi(stopStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
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
		return common.NewArrayValue([]common.Value{})
	}

	// Extract range
	result := make([]common.Value, 0, stop-start+1)
	for i := start; i <= stop; i++ {
		result = append(result, common.Value{Typ: common.BULK, Blk: item.List[i]})
	}

	return common.NewArrayValue(result)
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
func Llen(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'llen' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return common.NewIntegerValue(int64(len(item.List)))
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
//	Bulk String: The requested element, or common.NULL if index is out of range.
func Lindex(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lindex' command")
	}

	key := args[0].Blk
	indexStr := args[1].Blk

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewNullValue()
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Handle negative index
	if index < 0 {
		index = len(item.List) + index
	}

	if index < 0 || index >= len(item.List) {
		return common.NewNullValue()
	}

	return common.NewBulkValue(item.List[index])
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
func Lget(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lget' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0, len(item.List))
	for _, val := range item.List {
		result = append(result, common.Value{Typ: common.BULK, Blk: val})
	}

	return common.NewArrayValue(result)
}
