/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_hash.go
*/
package handlers

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// HashHandlers is the map of hash command names to their handler functions.
var HashHandlers = map[string]common.Handler{
	"HSET":         Hset,
	"HGET":         Hget,
	"HDEL":         Hdel,
	"HGETALL":      Hgetall,
	"HDELALL":      Hdelall,
	"HINCRBY":      Hincrby,
	"HMSET":        Hmset,
	"HMGET":        Hmget,
	"HEXISTS":      Hexists,
	"HLEN":         Hlen,
	"HKEYS":        Hkeys,
	"HVALS":        Hvals,
	"HEXPIRE":      Hexpire,
	"HSETNX":       Hsetnx,
	"HINCRBYFLOAT": Hincrbyfloat,
	"HSTRLEN":      Hstrlen,
	"HRANDFIELD":   Hrandfield,
}

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
func Hset(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 || len(args)%2 == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hset' command")
	}

	key := args[0].Blk
	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Calculate old memory before modification
	var oldMemory int64 = 0
	var item *common.Item
	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		oldMemory = existing.ApproxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return common.NewErrorValue(err.Error())
		}
	} else {
		item = common.NewHashItem()
		database.DB.Store[key] = item
	}

	count := int64(0)
	for i := 1; i < len(args); i += 2 {
		field := args[i].Blk
		value := args[i+1].Blk
		if _, exists := item.Hash[field]; !exists {
			count++
		}
		item.Hash[field] = common.NewHashFieldItem(value)
	}

	database.DB.Touch(key)
	// Calculate new memory and update database.DB.Mem
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem -= oldMemory
	database.DB.Mem += newMemory
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}
	logger.Warn("memory = %d\n", database.DB.Mem)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			fmt.Println("AOF write for HSET")
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(count)
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
//	common.NULL: If field or key does not exist
func Hget(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hget' command")
	}

	key := args[0].Blk
	field := args[1].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Poll(key)
	if ok {
		if !item.IsHash() {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		fieldItem, exists := item.Hash[field]
		if exists {
			// Check if field is expired
			if fieldItem.IsExpired() {
				// Delete here
				fmt.Printf("Expired Key: %s Field: %s\n", key, field)
				delete(item.Hash, field)
				return common.NewNullValue()
			}
			// delete if expired
			deleted := database.DB.RemIfExpired(key, item, state)
			if deleted {
				fmt.Println("Expired Key: ", key)
				return common.NewNullValue()
			}
			return common.NewBulkValue(fieldItem.Str)
		}

		return common.NewNullValue()
	}

	if !ok {
		fmt.Println("Not Found: ", key)
		return common.NewNullValue()
	}

	return common.NewBulkValue(item.Str)

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
func Hdel(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hdel' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Calculate old memory before modification
	oldMemory := item.ApproxMemoryUsage(key)

	count := int64(0)
	for i := 1; i < len(args); i++ {
		field := args[i].Blk
		if _, exists := item.Hash[field]; exists {
			delete(item.Hash, field)
			count++
		}
	}

	database.DB.Touch(key)
	// Calculate new memory and update database.DB.Mem
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem -= oldMemory
	database.DB.Mem += newMemory
	logger.Warn("memory = %d\n", database.DB.Mem)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			fmt.Println("AOF write for HDEL")
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(count)
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
func Hgetall(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hgetall' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0, len(item.Hash)*2)
	for field, fieldItem := range item.Hash {
		// Skip expired fields
		if fieldItem.IsExpired() {
			continue
		}
		result = append(result, common.Value{Typ: common.BULK, Blk: field})
		result = append(result, common.Value{Typ: common.BULK, Blk: fieldItem.Str})
	}

	return common.NewArrayValue(result)
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
func Hdelall(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hdelall' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Calculate old memory before clearing
	oldMemory := item.ApproxMemoryUsage(key)

	database.DB.Touch(key)
	count := int64(len(item.Hash))
	item.Hash = make(map[string]*common.Item) // Clear the hash

	// Calculate new memory and update database.DB.Mem
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem -= oldMemory
	database.DB.Mem += newMemory
	logger.Warn("memory = %d\n", database.DB.Mem)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(count)
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
func Hincrby(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hincrby' command")
	}

	key := args[0].Blk
	field := args[1].Blk
	incrStr := args[2].Blk

	incr, err := common.ParseInt(incrStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0
	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		oldMemory = existing.ApproxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return common.NewErrorValue(err.Error())
		}
	} else {
		item = common.NewHashItem()
		database.DB.Store[key] = item
	}

	var fieldItem *common.Item
	if existing, ok := item.Hash[field]; ok {
		fieldItem = existing
	} else {
		fieldItem = common.NewHashFieldItem("0")
		item.Hash[field] = fieldItem
	}

	// Check if field is expired
	if fieldItem.IsExpired() {
		fieldItem = common.NewHashFieldItem("0")
		item.Hash[field] = fieldItem
	}

	current := int64(0)
	if fieldItem.Str != "" {
		current, err = common.ParseInt(fieldItem.Str)
		if err != nil {
			return common.NewErrorValue("ERR hash value is not an integer")
		}
	}

	newVal := current + incr
	fieldItem.Str = fmt.Sprintf("%d", newVal)

	database.DB.Touch(key)
	// Calculate new memory and update database.DB.Mem
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem -= oldMemory
	database.DB.Mem += newMemory
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}
	logger.Warn("memory = %d\n", database.DB.Mem)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(newVal)
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
func Hmset(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 || len(args)%2 == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hmset' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Calculate old memory before modification
	var oldMemory int64 = 0
	var item *common.Item
	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		oldMemory = existing.ApproxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return common.NewErrorValue(err.Error())
		}
	} else {
		item = common.NewHashItem()
		database.DB.Store[key] = item
	}

	for i := 1; i < len(args); i += 2 {
		field := args[i].Blk
		value := args[i+1].Blk
		item.Hash[field] = common.NewHashFieldItem(value)
	}

	database.DB.Touch(key)
	// Calculate new memory and update database.DB.Mem
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem -= oldMemory
	database.DB.Mem += newMemory
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}
	logger.Warn("memory = %d\n", database.DB.Mem)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewStringValue("OK")
}

// Hmget handles the HMGET command.
// Returns the values associated with the specified fields in the hash stored at key.
//
// Syntax:
//
//	HMGET <key> <field> [<field> ...]
//
// Returns:
//
//	Array: List of values associated with the given fields, in the same order as they are requested.
func Hmget(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hmget' command")
	}

	key := args[0].Blk
	fields := args[1:]

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		result := make([]common.Value, len(fields))
		for i := range fields {
			result[i] = common.Value{Typ: common.NULL}
		}
		return common.NewArrayValue(result)
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0, len(fields))
	for _, fieldArg := range fields {
		field := fieldArg.Blk
		if fieldItem, exists := item.Hash[field]; exists && !fieldItem.IsExpired() {
			result = append(result, common.Value{Typ: common.BULK, Blk: fieldItem.Str})
		} else {
			result = append(result, common.Value{Typ: common.NULL})
		}
	}

	return common.NewArrayValue(result)
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
func Hexists(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hexists' command")
	}

	key := args[0].Blk
	field := args[1].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	_, exists := item.Hash[field]
	if exists {
		return common.NewIntegerValue(1)
	}

	return common.NewIntegerValue(0)
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
func Hlen(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hlen' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return common.NewIntegerValue(int64(len(item.Hash)))
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
func Hkeys(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hkeys' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0, len(item.Hash))
	for field, fieldItem := range item.Hash {
		// Skip expired fields
		if !fieldItem.IsExpired() {
			result = append(result, common.Value{Typ: common.BULK, Blk: field})
		}
	}

	return common.NewArrayValue(result)
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
func Hvals(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hvals' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0, len(item.Hash))
	for _, fieldItem := range item.Hash {
		// Skip expired fields
		if !fieldItem.IsExpired() {
			result = append(result, common.Value{Typ: common.BULK, Blk: fieldItem.Str})
		}
	}

	return common.NewArrayValue(result)
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
func Hexpire(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hexpire' command")
	}

	key := args[0].Blk
	field := args[1].Blk
	secondsStr := args[2].Blk

	seconds, err := strconv.Atoi(secondsStr)
	if err != nil {
		return common.NewErrorValue("ERR invalid expiration time")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	fieldItem, exists := item.Hash[field]
	if !exists {
		return common.NewIntegerValue(0)
	}

	// Set expiration on the field
	fieldItem.Exp = time.Now().Add(time.Second * time.Duration(seconds))
	database.DB.Touch(key)

	return common.NewIntegerValue(1)
}

// Hincrbyfloat handles the HINCRBYFLOAT command.
// Increments the float value of a hash field by the given amount.
//
// Syntax:
//
//	HINCRBYFLOAT <key> <field> <increment>
//
// Returns:
//
//	Bulk string: The new value after increment as a string
//
// Behavior:
//   - Creates hash and field if they don't exist (initialized to 0)
//   - Returns error if field value is not a valid float
//   - Returns the new value as a string
func Hincrbyfloat(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hincrbyfloat' command")
	}

	key := args[0].Blk
	field := args[1].Blk
	incrStr := args[2].Blk

	incr, err := common.ParseFloat(incrStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not a valid float")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0
	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		oldMemory = existing.ApproxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return common.NewErrorValue(err.Error())
		}
	} else {
		item = common.NewHashItem()
		database.DB.Store[key] = item
	}

	var fieldItem *common.Item
	if existing, ok := item.Hash[field]; ok {
		fieldItem = existing
	} else {
		fieldItem = common.NewHashFieldItem("0")
		item.Hash[field] = fieldItem
	}

	// Check if field is expired
	if fieldItem.IsExpired() {
		fieldItem = common.NewHashFieldItem("0")
		item.Hash[field] = fieldItem
	}

	current := float64(0)
	if fieldItem.Str != "" {
		current, err = common.ParseFloat(fieldItem.Str)
		if err != nil {
			return common.NewErrorValue("ERR hash value is not a float")
		}
	}

	newVal := current + incr
	fieldItem.Str = fmt.Sprintf("%g", newVal)

	database.DB.Touch(key)
	// Calculate new memory and update database.DB.Mem
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem -= oldMemory
	database.DB.Mem += newMemory
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}
	logger.Warn("memory = %d\n", database.DB.Mem)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			fmt.Println("AOF write for HINCRBYFLOAT")
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewBulkValue(fieldItem.Str)
}

// Hsetnx handles the HSETNX command.
// Sets the value of a hash field only if the field does not exist.
//
// Syntax:
//
//	HSETNX <key> <field> <value>
//
// Returns:
//
//	Integer: 1 if field was set, 0 if field already exists
//
// Behavior:
//   - Creates hash if it doesn't exist
//   - Does not overwrite existing fields
func Hsetnx(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hsetnx' command")
	}

	key := args[0].Blk
	field := args[1].Blk
	value := args[2].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Calculate old memory before modification
	var oldMemory int64 = 0
	var item *common.Item
	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		oldMemory = existing.ApproxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return common.NewErrorValue(err.Error())
		}
	} else {
		item = common.NewHashItem()
		database.DB.Store[key] = item
	}

	if _, exists := item.Hash[field]; exists {
		return common.NewIntegerValue(0)
	}

	item.Hash[field] = common.NewHashFieldItem(value)

	database.DB.Touch(key)
	// Calculate new memory and update database.DB.Mem
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem -= oldMemory
	database.DB.Mem += newMemory
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}
	logger.Warn("memory = %d\n", database.DB.Mem)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			fmt.Println("AOF write for HSETNX")
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(1)
}

// Hstrlen handles the HSTRLEN command.
// Returns the length of the value associated with a hash field.
//
// Syntax:
//
//	HSTRLEN <key> <field>
//
// Returns:
//
//	Integer: Length of the field's value, 0 if field doesn't exist
func Hstrlen(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hstrlen' command")
	}

	key := args[0].Blk
	field := args[1].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	fieldItem, exists := item.Hash[field]
	if !exists {
		return common.NewIntegerValue(0)
	}

	return common.NewIntegerValue(int64(len(fieldItem.Str)))
}

// Hrandfield handles the HRANDFIELD command.
// Returns one or more random fields from the hash value stored at key.
//
// Syntax:
//
//	HRANDFIELD <key> [count [WITHVALUES]]
//
// Returns:
//
//	Array: List of random fields (and values if WITHVALUES is specified)
//
// Behavior:
//   - If count is positive, returns up to count random fields
//   - If count is negative, allows duplicates
//   - WITHVALUES returns field-value pairs
//   - If key doesn't exist, returns empty array
func Hrandfield(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 || len(args) > 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'hrandfield' command")
	}

	key := args[0].Blk
	count := 1
	withValues := false

	if len(args) >= 2 {
		countStr := args[1].Blk
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
	}

	if len(args) == 3 {
		if strings.ToUpper(args[2].Blk) == "WITHVALUES" {
			withValues = true
		} else {
			return common.NewErrorValue("ERR syntax error")
		}
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if !item.IsHash() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Collect non-expired fields
	fields := make([]string, 0, len(item.Hash))
	for field, fieldItem := range item.Hash {
		if !fieldItem.IsExpired() {
			fields = append(fields, field)
		}
	}

	if len(fields) == 0 {
		return common.NewArrayValue([]common.Value{})
	}

	result := make([]common.Value, 0)
	absCount := count
	if count < 0 {
		absCount = -count
	}

	for i := 0; i < absCount; i++ {
		idx := rand.Intn(len(fields))
		field := fields[idx]
		fieldItem := item.Hash[field]

		result = append(result, common.Value{Typ: common.BULK, Blk: field})
		if withValues {
			result = append(result, common.Value{Typ: common.BULK, Blk: fieldItem.Str})
		}

		// If count is negative, allow duplicates; else remove selected field
		if count > 0 {
			fields = append(fields[:idx], fields[idx+1:]...)
			if len(fields) == 0 {
				break
			}
		}
	}

	return common.NewArrayValue(result)
}
