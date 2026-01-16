/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_string.go
*/
package handlers

import (
	"fmt"
	"strconv"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
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
//   - common.NULL if key does not exist or expired
//
// Behavior:
//   - Automatically deletes expired keys
//   - Thread-safe (read lock)
func Get(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// cmd := v.Arr[0].Blk
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR invalid command usage with GET")
	}
	key := args[0].Blk // grab the key

	// get the item from the database
	database.DB.Mu.RLock()
	item, ok := database.DB.Poll(key)
	// delete if expired
	deleted := database.DB.RemIfExpired(key, item, state)
	if deleted {
		fmt.Println("Expired Key: ", key)
		return common.NewNullValue()
	}
	database.DB.Mu.RUnlock()

	if !ok {
		fmt.Println("Not Found: ", key)
		return common.NewNullValue()
	}

	return common.NewBulkValue(item.Str)

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
func Set(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// cmd := v.Arr[0].Blk
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR invalid command usage with SET")
	}

	key := args[0].Blk // grab the key
	val := args[1].Blk // grab the value

	database.DB.Mu.Lock()
	// First check if key exists to get old memory usage
	var oldItem *common.Item
	if existing, ok := database.DB.Store[key]; ok {
		oldItem = existing
	}
	database.DB.Mu.Unlock()

	// Create new item and calculate memory (without lock)
	newItem := common.NewStringItem(val)
	newMemory := newItem.ApproxMemoryUsage(key)

	// Check if we need to evict (without holding lock)
	database.DB.Mu.RLock()
	currentMem := database.DB.Mem
	maxMem := state.Config.Maxmemory
	database.DB.Mu.RUnlock()

	oldMemory := int64(0)
	if oldItem != nil {
		oldMemory = int64(oldItem.ApproxMemoryUsage(key))
	}

	// Calculate new total memory
	netNewMemory := newMemory - oldMemory

	if maxMem > 0 && currentMem+netNewMemory >= maxMem {
		// Need to evict - this acquires its own locks
		_, err := database.DB.EvictKeys(state, netNewMemory)
		if err != nil {
			return common.NewErrorValue("ERR maxmemory reached: " + err.Error())
		}
	}

	// Now acquire lock and actually put the item
	database.DB.Mu.Lock()
	err := database.DB.Put(key, val, state)
	if err != nil {
		database.DB.Mu.Unlock()
		return common.NewErrorValue("ERR some error occured while PUT:" + err.Error())
	}
	database.DB.Touch(key)
	// record it for AOF
	if state.Config.AofEnabled {
		state.Aof.W.Write(v)

		if state.Config.AofFsync == common.Always {
			logger.Info("save AOF record on SET\n")
			state.Aof.W.Flush()
		}

	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	database.DB.Mu.Unlock()

	return common.NewStringValue("OK")
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
func Incr(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'incr' command")
	}
	return incrDecrBy(c, args[0].Blk, 1, state, v)
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
func Decr(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'decr' command")
	}
	return incrDecrBy(c, args[0].Blk, -1, state, v)
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
func IncrBy(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'incrby' command")
	}
	incr, err := common.ParseInt(args[1].Blk)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}
	return incrDecrBy(c, args[0].Blk, incr, state, v)
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
func DecrBy(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'decrby' command")
	}
	decr, err := common.ParseInt(args[1].Blk)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}
	return incrDecrBy(c, args[0].Blk, -decr, state, v)
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
func Mget(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'mget' command")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	result := make([]common.Value, 0, len(args))

	for _, arg := range args {
		key := arg.Blk
		item, ok := database.DB.Poll(key)

		if !ok || item.IsExpired() {
			result = append(result, common.Value{Typ: common.NULL})
			continue
		}

		if item.Type != common.STRING_TYPE {
			result = append(result, common.Value{Typ: common.NULL})
			continue
		}

		result = append(result, common.Value{Typ: common.BULK, Blk: item.Str})
	}

	return common.NewArrayValue(result)
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
func Mset(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 || len(args)%2 != 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'mset' command")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	for i := 0; i < len(args); i += 2 {
		key := args[i].Blk
		val := args[i+1].Blk
		database.DB.Put(key, val, state)
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
	return common.NewStringValue("OK")
}

// Strlen handles the STRLEN command.
// Returns the length of the string value stored at key.
//
// Syntax:
//
//	STRLEN <key>
func Strlen(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'strlen' command")
	}
	key := args[0].Blk

	database.DB.Mu.RLock()
	item, ok := database.DB.Poll(key)
	database.DB.Mu.RUnlock()

	if !ok || item.IsExpired() {
		return common.NewIntegerValue(0)
	}
	if !item.IsString() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	return common.NewIntegerValue(int64(len(item.Str)))
}

func incrDecrBy(c *common.Client, key string, delta int64, state *common.AppState, v *common.Value) *common.Value {
	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		if item.IsExpired() {
			oldMemory = item.ApproxMemoryUsage(key)
			item = common.NewStringItem("0")
			database.DB.Store[key] = item
		} else {
			if !item.IsString() {
				return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
			}
			oldMemory = item.ApproxMemoryUsage(key)
		}
	} else {
		item = common.NewStringItem("0")
		database.DB.Store[key] = item
	}

	val, err := common.ParseInt(item.Str)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	newVal := val + delta
	item.Str = strconv.FormatInt(newVal, 10)

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

	return common.NewIntegerValue(newVal)
}
