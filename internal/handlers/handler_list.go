/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_list.go
*/
package handlers

import (
	"strconv"
	"time"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// ListHandlers is the map of list command names to their handler functions.
var ListHandlers = map[string]common.Handler{
	"LPUSH":     Lpush,
	"RPUSH":     Rpush,
	"LPOP":      Lpop,
	"RPOP":      Rpop,
	"LRANGE":    Lrange,
	"LLEN":      Llen,
	"LINDEX":    Lindex,
	"LGET":      Lget,
	"LSET":      Lset,
	"LINSERT":   Linsert,
	"LREM":      Lrem,
	"LTRIM":     Ltrim,
	"RPOPLPUSH": RpopLpush,
	"LMOVE":     Lmove,
	"LPOS":      Lpos,
	"BLPOP":     Blpop,
	"BRPOP":     Brpop,
	"BLMOVE":    Blmove,
}

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

	saveDBState(state, v)

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

	saveDBState(state, v)

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

	saveDBState(state, v)

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

	saveDBState(state, v)

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

// Lset handles the LSET command.
// Sets the list element at index to value.
//
// Syntax:
//
//	LSET <key> <index> <value>
//
// Description:
//   - Sets the element at the specified index in the list stored at key.
//   - The index is zero-based, so 0 means the first element, 1 the second element, etc.
//   - Negative indices can be used to designate elements starting from the end of the list.
//   - For example, -1 is the last element, -2 is the penultimate element, and so forth.
//   - An error is returned if the index is out of range or if the key does not hold a list.
//
// Returns:
//
//	"OK" on success, or an error if the index is out of range.
//
// Example:
//
//	redis> RPUSH mylist "one"
//	(integer) 1
//	redis> RPUSH mylist "two"
//	(integer) 2
//	redis> RPUSH mylist "three"
//	(integer) 3
//	redis> LSET mylist 0 "four"
//	"OK"
//	redis> LRANGE mylist 0 -1
//	1) "four"
//	2) "two"
//	3) "three"
//	redis> LSET mylist -2 "five"
//	"OK"
//	redis> LRANGE mylist 0 -1
//	1) "four"
//	2) "five"
//	3) "three"
func Lset(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lset' command")
	}

	key := args[0].Blk
	indexStr := args[1].Blk
	value := args[2].Blk

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewErrorValue("ERR no such key")
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	listLen := len(item.List)
	if index < 0 {
		index = listLen + index
	}

	if index < 0 || index >= listLen {
		return common.NewErrorValue("ERR index out of range")
	}

	oldMemory := item.ApproxMemoryUsage(key)
	item.List[index] = value
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem += (newMemory - oldMemory)
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	database.DB.Touch(key)

	saveDBState(state, v)
	return common.NewStringValue("OK")
}

// Linsert handles the LINSERT command.
// Inserts value in the list stored at key either before or after the reference value pivot.
//
// Syntax:
//
//	LINSERT <key> <BEFORE|AFTER> <pivot> <value>
//
// Description:
//   - Inserts value in the list stored at key either before or after the reference value pivot.
//   - When key does not exist, it is considered an empty list and no operation is performed.
//   - An error is returned when key exists but does not hold a list value.
//
// Returns:
//
//	Integer: The length of the list after the insert operation, or -1 when the value pivot was not found.
//
// Example:
//
//	redis> RPUSH mylist "Hello"
//	(integer) 1
//	redis> RPUSH mylist "World"
//	(integer) 2
//	redis> LINSERT mylist BEFORE "World" "There"
//	(integer) 3
//	redis> LRANGE mylist 0 -1
//	1) "Hello"
//	2) "There"
//	3) "World"
func Linsert(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 4 {
		return common.NewErrorValue("ERR wrong number of arguments for 'linsert' command")
	}

	key := args[0].Blk
	where := args[1].Blk
	pivot := args[2].Blk
	value := args[3].Blk

	if where != "BEFORE" && where != "AFTER" {
		return common.NewErrorValue("ERR syntax error")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(-1)
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Find pivot
	pivotIdx := -1
	for i, val := range item.List {
		if val == pivot {
			pivotIdx = i
			break
		}
	}

	if pivotIdx == -1 {
		return common.NewIntegerValue(-1)
	}

	oldMemory := item.ApproxMemoryUsage(key)

	// Insert
	if where == "BEFORE" {
		item.List = append(item.List[:pivotIdx], append([]string{value}, item.List[pivotIdx:]...)...)
	} else {
		item.List = append(item.List[:pivotIdx+1], append([]string{value}, item.List[pivotIdx+1:]...)...)
	}

	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem += (newMemory - oldMemory)
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	database.DB.Touch(key)

	saveDBState(state, v)

	return common.NewIntegerValue(int64(len(item.List)))
}

// Lrem handles the LREM command.
// Removes the first count occurrences of elements equal to value from the list stored at key.
//
// Syntax:
//
//	LREM <key> <count> <value>
//
// Description:
//   - Removes the first count occurrences of elements equal to value from the list stored at key.
//   - The count argument influences the operation in the following ways:
//   - count > 0: Remove elements equal to value moving from head to tail.
//   - count < 0: Remove elements equal to value moving from tail to head.
//   - count = 0: Remove all elements equal to value.
//   - For example, LREM list -2 "hello" will remove the last two occurrences of "hello" in the list.
//   - Note that non-existing keys are treated like empty lists, so when key does not exist, the command returns 0.
//
// Returns:
//
//	Integer: The number of removed elements.
//
// Example:
//
//	redis> RPUSH mylist "hello"
//	(integer) 1
//	redis> RPUSH mylist "hello"
//	(integer) 2
//	redis> RPUSH mylist "foo"
//	(integer) 3
//	redis> RPUSH mylist "hello"
//	(integer) 4
//	redis> LREM mylist -2 "hello"
//	(integer) 2
//	redis> LRANGE mylist 0 -1
//	1) "hello"
//	2) "foo"
func Lrem(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lrem' command")
	}

	key := args[0].Blk
	countStr := args[1].Blk
	value := args[2].Blk

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	oldMemory := item.ApproxMemoryUsage(key)

	removed := 0
	if count == 0 {
		// Remove all
		newList := []string{}
		for _, val := range item.List {
			if val != value {
				newList = append(newList, val)
			} else {
				removed++
			}
		}
		item.List = newList
	} else if count > 0 {
		// Remove from head
		newList := []string{}
		for _, val := range item.List {
			if val == value && removed < count {
				removed++
			} else {
				newList = append(newList, val)
			}
		}
		item.List = newList
	} else {
		// Remove from tail, count is negative
		count = -count
		newList := []string{}
		for i := len(item.List) - 1; i >= 0; i-- {
			val := item.List[i]
			if val == value && removed < count {
				removed++
			} else {
				newList = append([]string{val}, newList...)
			}
		}
		item.List = newList
	}

	if len(item.List) == 0 {
		delete(database.DB.Store, key)
		database.DB.Mem -= oldMemory
	} else {
		newMemory := item.ApproxMemoryUsage(key)
		database.DB.Mem += (newMemory - oldMemory)
	}

	database.DB.Touch(key)

	saveDBState(state, v)

	return common.NewIntegerValue(int64(removed))
}

// Ltrim handles the LTRIM command.
// Trims an existing list so that it will contain only the specified range of elements.
//
// Syntax:
//
//	LTRIM <key> <start> <stop>
//
// Description:
//   - Trims an existing list so that it will contain only the specified range of elements specified.
//   - Both start and stop are zero-based indexes, where 0 is the first element of the list (the head), 1 the next element and so forth.
//   - For example: LTRIM foobar 0 2 will modify the list stored at foobar so that only the first three elements of the list will remain.
//   - start and end can also be negative numbers indicating offsets from the end of the list, where -1 is the last element of the list, -2 the penultimate element and so forth.
//   - Out of range indexes will not produce an error: if start is larger than the end of the list, or start > stop, the result will be an empty list (which causes key to be removed).
//   - If stop is larger than the end of the list, Redis will treat it like the last element of the list.
//
// Returns:
//
//	"OK" on success.
//
// Example:
//
//	redis> RPUSH mylist "one"
//	(integer) 1
//	redis> RPUSH mylist "two"
//	(integer) 2
//	redis> RPUSH mylist "three"
//	(integer) 3
//	redis> LTRIM mylist 1 -1
//	"OK"
//	redis> LRANGE mylist 0 -1
//	1) "two"
//	2) "three"
func Ltrim(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'ltrim' command")
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

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewStringValue("OK")
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
		// Empty list
		delete(database.DB.Store, key)
	} else {
		oldMemory := item.ApproxMemoryUsage(key)
		item.List = item.List[start : stop+1]
		newMemory := item.ApproxMemoryUsage(key)
		database.DB.Mem += (newMemory - oldMemory)
		database.DB.Touch(key)
	}

	saveDBState(state, v)

	return common.NewStringValue("OK")
}

// RpopLpush handles the RPOPLPUSH command.
// Atomically returns and removes the last element (tail) of the list stored at source,
// and pushes the element at the first element (head) of the list stored at destination.
//
// Syntax:
//
//	RPOPLPUSH <source> <destination>
//
// Description:
//   - Atomically returns and removes the last element (tail) of the list stored at source,
//     and pushes the element at the first element (head) of the list stored at destination.
//   - For example: consider source holding the list a,b,c, and destination holding the list x,y,z.
//     Executing RPOPLPUSH results in source holding a,b and destination holding c,x,y,z.
//   - If source does not exist, the value nil is returned and no operation is performed.
//   - If source and destination are the same, it will rotate the list.
//
// Returns:
//
//	Bulk string: The element being popped and pushed.
//
// Example:
//
//	redis> RPUSH mylist "one"
//	(integer) 1
//	redis> RPUSH mylist "two"
//	(integer) 2
//	redis> RPUSH mylist "three"
//	(integer) 3
//	redis> RPOPLPUSH mylist myotherlist
//	"three"
//	redis> LRANGE mylist 0 -1
//	1) "one"
//	2) "two"
//	redis> LRANGE myotherlist 0 -1
//	1) "three"
func RpopLpush(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'rpoplpush' command")
	}

	source := args[0].Blk
	destination := args[1].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	sourceItem, ok := database.DB.Store[source]
	if !ok {
		return common.NewNullValue()
	}

	if sourceItem.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if len(sourceItem.List) == 0 {
		return common.NewNullValue()
	}

	// Pop from source
	val := sourceItem.List[len(sourceItem.List)-1]
	sourceItem.List = sourceItem.List[:len(sourceItem.List)-1]

	// Push to destination
	destItem, ok := database.DB.Store[destination]
	if !ok {
		destItem = &common.Item{
			Type: common.LIST_TYPE,
			List: []string{},
		}
		database.DB.Store[destination] = destItem
	} else if destItem.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	destItem.List = append([]string{val}, destItem.List...)

	// Update memory
	sourceMemory := sourceItem.ApproxMemoryUsage(source)
	if len(sourceItem.List) == 0 {
		delete(database.DB.Store, source)
		database.DB.Mem -= sourceMemory
	} else {
		newSourceMemory := sourceItem.ApproxMemoryUsage(source)
		database.DB.Mem += (newSourceMemory - sourceMemory)
	}

	destMemory := destItem.ApproxMemoryUsage(destination)
	newDestMemory := destItem.ApproxMemoryUsage(destination)
	database.DB.Mem += (newDestMemory - destMemory)

	database.DB.Touch(source)
	database.DB.Touch(destination)

	saveDBState(state, v)

	return common.NewBulkValue(val)
}

// Lmove handles the LMOVE command.
// Atomically moves element from one list to another.
//
// Syntax:
//
//	LMOVE <source> <destination> <LEFT|RIGHT> <LEFT|RIGHT>
//
// Description:
//   - Atomically moves element from one list to another.
//   - The first LEFT|RIGHT specifies from which side to pop the element from the source list.
//   - The second LEFT|RIGHT specifies to which side to push the element to the destination list.
//   - For example, LMOVE source destination LEFT RIGHT pops the first element from the source list
//     and pushes it to the right of the destination list.
//
// Returns:
//
//	Bulk string: The element being moved, or nil if source is empty.
//
// Example:
//
//	redis> RPUSH mylist "one"
//	(integer) 1
//	redis> RPUSH mylist "two"
//	(integer) 2
//	redis> RPUSH mylist "three"
//	(integer) 3
//	redis> LMOVE mylist myotherlist RIGHT LEFT
//	"three"
//	redis> LRANGE mylist 0 -1
//	1) "one"
//	2) "two"
//	redis> LRANGE myotherlist 0 -1
//	1) "three"
func Lmove(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 4 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lmove' command")
	}

	source := args[0].Blk
	destination := args[1].Blk
	srcWhere := args[2].Blk
	destWhere := args[3].Blk

	if (srcWhere != "LEFT" && srcWhere != "RIGHT") || (destWhere != "LEFT" && destWhere != "RIGHT") {
		return common.NewErrorValue("ERR syntax error")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	sourceItem, ok := database.DB.Store[source]
	if !ok {
		return common.NewNullValue()
	}

	if sourceItem.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if len(sourceItem.List) == 0 {
		return common.NewNullValue()
	}

	// Pop from source
	var val string
	if srcWhere == "LEFT" {
		val = sourceItem.List[0]
		sourceItem.List = sourceItem.List[1:]
	} else {
		val = sourceItem.List[len(sourceItem.List)-1]
		sourceItem.List = sourceItem.List[:len(sourceItem.List)-1]
	}

	// Push to destination
	destItem, ok := database.DB.Store[destination]
	if !ok {
		destItem = &common.Item{
			Type: common.LIST_TYPE,
			List: []string{},
		}
		database.DB.Store[destination] = destItem
	} else if destItem.Type != common.LIST_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if destWhere == "LEFT" {
		destItem.List = append([]string{val}, destItem.List...)
	} else {
		destItem.List = append(destItem.List, val)
	}

	// Update memory
	sourceMemory := sourceItem.ApproxMemoryUsage(source)
	if len(sourceItem.List) == 0 {
		delete(database.DB.Store, source)
		database.DB.Mem -= sourceMemory
	} else {
		newSourceMemory := sourceItem.ApproxMemoryUsage(source)
		database.DB.Mem += (newSourceMemory - sourceMemory)
	}

	destMemory := destItem.ApproxMemoryUsage(destination)
	newDestMemory := destItem.ApproxMemoryUsage(destination)
	database.DB.Mem += (newDestMemory - destMemory)

	database.DB.Touch(source)
	database.DB.Touch(destination)

	saveDBState(state, v)

	return common.NewBulkValue(val)
}

// Lpos handles the LPOS command.
// Returns the index of matching elements inside a list.
//
// Syntax:
//
//	LPOS <key> <element> [RANK rank] [COUNT count] [MAXLEN maxlen]
//
// Description:
//   - Returns the index of the first occurrence of element in the list stored at key.
//   - The optional RANK argument specifies the "rank" of the first element to return, in case there are multiple matches.
//   - RANK 1 returns the first match, RANK 2 the second match, etc.
//   - A negative RANK returns matches starting from the end of the list.
//   - The optional COUNT argument limits the number of matches returned.
//   - The optional MAXLEN argument specifies the maximum number of list elements to scan.
//
// Returns:
//
//	Integer or array: The index(es) of the matching element(s), or nil if not found.
//
// Example:
//
//	redis> RPUSH mylist "a" "b" "c" "d" "e" "f"
//	(integer) 6
//	redis> LPOS mylist "c"
//	(integer) 2
//	redis> LPOS mylist "f" RANK 2
//	(integer) 5
func Lpos(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'lpos' command")
	}

	key := args[0].Blk
	element := args[1].Blk

	rank := 1
	count := 1
	maxlen := -1

	i := 2
	for i < len(args) {
		arg := args[i].Blk
		switch arg {
		case "RANK":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			rankStr := args[i+1].Blk
			r, err := strconv.Atoi(rankStr)
			if err != nil || r == 0 {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}
			rank = r
			i += 2
		case "COUNT":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			countStr := args[i+1].Blk
			c, err := strconv.Atoi(countStr)
			if err != nil || c < 0 {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}
			count = c
			i += 2
		case "MAXLEN":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			maxlenStr := args[i+1].Blk
			m, err := strconv.Atoi(maxlenStr)
			if err != nil || m <= 0 {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}
			maxlen = m
			i += 2
		default:
			return common.NewErrorValue("ERR syntax error")
		}
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

	listLen := len(item.List)
	if maxlen > 0 && maxlen < listLen {
		listLen = maxlen
	}

	indices := []int{}
	found := 0

	if rank > 0 {
		for idx := 0; idx < listLen; idx++ {
			if item.List[idx] == element {
				found++
				if found == rank {
					if count == 1 {
						return common.NewIntegerValue(int64(idx))
					}
					indices = append(indices, idx)
					break
				}
			}
		}
	} else {
		rank = -rank
		for idx := listLen - 1; idx >= 0; idx-- {
			if item.List[idx] == element {
				found++
				if found == rank {
					if count == 1 {
						return common.NewIntegerValue(int64(idx))
					}
					indices = append(indices, idx)
					break
				}
			}
		}
	}

	if count > 1 {
		// For COUNT > 1, find all matches
		indices = []int{}
		matches := 0
		if rank > 0 {
			for idx := 0; idx < listLen && matches < count; idx++ {
				if item.List[idx] == element {
					indices = append(indices, idx)
					matches++
				}
			}
		} else {
			for idx := listLen - 1; idx >= 0 && matches < count; idx-- {
				if item.List[idx] == element {
					indices = append([]int{idx}, indices...)
					matches++
				}
			}
		}
		if len(indices) == 0 {
			return common.NewNullValue()
		}
		result := make([]common.Value, len(indices))
		for i, idx := range indices {
			result[i] = common.Value{Typ: common.INTEGER, Num: idx}
		}
		return common.NewArrayValue(result)
	}

	if len(indices) == 0 {
		return common.NewNullValue()
	}

	return common.NewIntegerValue(int64(indices[0]))
}

// Blpop handles the BLPOP command.
// Removes and returns the first element of the list stored at key.
// If the list is empty, it blocks until an element is available or timeout expires.
//
// Syntax:
//
//	BLPOP <key> [<key> ...] <timeout>
//
// Description:
//   - BLPOP is a blocking list pop primitive.
//   - It is the blocking version of LPOP because it blocks the connection when there are no elements to pop from any of the given lists.
//   - An element is popped from the head of the first list that is non-empty, with the given keys being checked in the order that they are given.
//   - A timeout of 0 means to block forever.
//
// Returns:
//
//	Array: A two-element array containing the key name and the popped element, or nil if timeout.
//
// Example:
//
//	redis> DEL mylist
//	(integer) 1
//	redis> RPUSH mylist "one"
//	(integer) 1
//	redis> BLPOP mylist 0
//	1) "mylist"
//	2) "one"
func Blpop(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'blpop' command")
	}

	timeoutStr := args[len(args)-1].Blk
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	keys := make([]string, len(args)-1)
	for i := 0; i < len(args)-1; i++ {
		keys[i] = args[i].Blk
	}

	start := time.Now()
	for {
		for _, key := range keys {
			database.DB.Mu.Lock()
			item, ok := database.DB.Store[key]
			if ok && item.Type == common.LIST_TYPE && len(item.List) > 0 {
				// Pop first
				val := item.List[0]
				item.List = item.List[1:]

				oldMemory := item.ApproxMemoryUsage(key)
				if len(item.List) == 0 {
					delete(database.DB.Store, key)
					database.DB.Mem -= oldMemory
				} else {
					newMemory := item.ApproxMemoryUsage(key)
					database.DB.Mem += (newMemory - oldMemory)
				}

				database.DB.Touch(key)

				saveDBState(state, v)

				database.DB.Mu.Unlock()

				result := []common.Value{
					{Typ: common.BULK, Blk: key},
					{Typ: common.BULK, Blk: val},
				}
				return common.NewArrayValue(result)
			}
			database.DB.Mu.Unlock()
		}

		// Check timeout
		if timeout > 0 && time.Since(start).Seconds() >= float64(timeout) {
			return common.NewNullValue()
		}

		// Sleep for 100ms before checking again
		time.Sleep(100 * time.Millisecond)
	}
}

// Brpop handles the BRPOP command.
// Removes and returns the last element of the list stored at key.
// If the list is empty, it blocks until an element is available or timeout expires.
//
// Syntax:
//
//	BRPOP <key> [<key> ...] <timeout>
//
// Description:
//   - BRPOP is a blocking list pop primitive.
//   - It is the blocking version of RPOP because it blocks the connection when there are no elements to pop from any of the given lists.
//   - An element is popped from the tail of the first list that is non-empty, with the given keys being checked in the order that they are given.
//   - A timeout of 0 means to block forever.
//
// Returns:
//
//	Array: A two-element array containing the key name and the popped element, or nil if timeout.
//
// Example:
//
//	redis> DEL mylist
//	(integer) 1
//	redis> RPUSH mylist "one"
//	(integer) 1
//	redis> BRPOP mylist 0
//	1) "mylist"
//	2) "one"
func Brpop(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'brpop' command")
	}

	timeoutStr := args[len(args)-1].Blk
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	keys := make([]string, len(args)-1)
	for i := 0; i < len(args)-1; i++ {
		keys[i] = args[i].Blk
	}

	start := time.Now()
	for {
		for _, key := range keys {
			database.DB.Mu.Lock()
			item, ok := database.DB.Store[key]
			if ok && item.Type == common.LIST_TYPE && len(item.List) > 0 {
				// Pop last
				lastIdx := len(item.List) - 1
				val := item.List[lastIdx]
				item.List = item.List[:lastIdx]

				oldMemory := item.ApproxMemoryUsage(key)
				if len(item.List) == 0 {
					delete(database.DB.Store, key)
					database.DB.Mem -= oldMemory
				} else {
					newMemory := item.ApproxMemoryUsage(key)
					database.DB.Mem += (newMemory - oldMemory)
				}

				database.DB.Touch(key)
				saveDBState(state, v)

				database.DB.Mu.Unlock()

				result := []common.Value{
					{Typ: common.BULK, Blk: key},
					{Typ: common.BULK, Blk: val},
				}
				return common.NewArrayValue(result)
			}
			database.DB.Mu.Unlock()
		}

		// Check timeout
		if timeout > 0 && time.Since(start).Seconds() >= float64(timeout) {
			return common.NewNullValue()
		}

		// Sleep for 100ms before checking again
		time.Sleep(100 * time.Millisecond)
	}
}

// Blmove handles the BLMOVE command.
// Atomically moves element from one list to another, blocking if source is empty.
//
// Syntax:
//
//	BLMOVE <source> <destination> <LEFT|RIGHT> <LEFT|RIGHT> <timeout>
//
// Description:
//   - BLMOVE is the blocking variant of LMOVE.
//   - When source contains elements, this command behaves exactly like LMOVE.
//   - When source is empty, Redis will block the connection until another client pushes to it or until timeout is reached.
//   - A timeout of 0 means to block forever.
//
// Returns:
//
//	Bulk string: The element being moved, or nil if timeout.
//
// Example:
//
//	redis> DEL mylist
//	(integer) 1
//	redis> RPUSH mylist "one"
//	(integer) 1
//	redis> BLMOVE mylist myotherlist RIGHT LEFT 0
//	"one"
//	redis> LRANGE mylist 0 -1
//	(empty list or set)
//	redis> LRANGE myotherlist 0 -1
//	1) "one"
func Blmove(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 5 {
		return common.NewErrorValue("ERR wrong number of arguments for 'blmove' command")
	}

	source := args[0].Blk
	destination := args[1].Blk
	srcWhere := args[2].Blk
	destWhere := args[3].Blk
	timeoutStr := args[4].Blk

	if (srcWhere != "LEFT" && srcWhere != "RIGHT") || (destWhere != "LEFT" && destWhere != "RIGHT") {
		return common.NewErrorValue("ERR syntax error")
	}

	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	start := time.Now()
	for {
		database.DB.Mu.Lock()
		sourceItem, ok := database.DB.Store[source]
		if ok && sourceItem.Type == common.LIST_TYPE && len(sourceItem.List) > 0 {
			// Pop from source
			var val string
			if srcWhere == "LEFT" {
				val = sourceItem.List[0]
				sourceItem.List = sourceItem.List[1:]
			} else {
				val = sourceItem.List[len(sourceItem.List)-1]
				sourceItem.List = sourceItem.List[:len(sourceItem.List)-1]
			}

			// Push to destination
			destItem, ok := database.DB.Store[destination]
			if !ok {
				destItem = &common.Item{
					Type: common.LIST_TYPE,
					List: []string{},
				}
				database.DB.Store[destination] = destItem
			} else if destItem.Type != common.LIST_TYPE {
				database.DB.Mu.Unlock()
				return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
			}

			if destWhere == "LEFT" {
				destItem.List = append([]string{val}, destItem.List...)
			} else {
				destItem.List = append(destItem.List, val)
			}

			// Update memory
			sourceMemory := sourceItem.ApproxMemoryUsage(source)
			if len(sourceItem.List) == 0 {
				delete(database.DB.Store, source)
				database.DB.Mem -= sourceMemory
			} else {
				newSourceMemory := sourceItem.ApproxMemoryUsage(source)
				database.DB.Mem += (newSourceMemory - sourceMemory)
			}

			destMemory := destItem.ApproxMemoryUsage(destination)
			newDestMemory := destItem.ApproxMemoryUsage(destination)
			database.DB.Mem += (newDestMemory - destMemory)

			database.DB.Touch(source)
			database.DB.Touch(destination)

			saveDBState(state, v)

			database.DB.Mu.Unlock()
			return common.NewBulkValue(val)
		}
		database.DB.Mu.Unlock()

		// Check timeout
		if timeout > 0 && time.Since(start).Seconds() >= float64(timeout) {
			return common.NewNullValue()
		}

		// Sleep for 100ms before checking again
		time.Sleep(100 * time.Millisecond)
	}
}
