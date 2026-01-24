/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_string.go
*/
package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// StringHandlers is the map of string command names to their handler functions.
var StringHandlers = map[string]common.Handler{
	"GET":         Get,
	"SET":         Set,
	"SETNX":       SetNX,
	"SETEX":       SetEX,
	"PSETEX":      PSetEX,
	"GETSET":      GetSet,
	"GETEX":       GetEX,
	"GETDEL":      GetDel,
	"APPEND":      Append,
	"GETRANGE":    GetRange,
	"SETRANGE":    SetRange,
	"INCR":        Incr,
	"DECR":        Decr,
	"INCRBY":      IncrBy,
	"DECRBY":      DecrBy,
	"INCRBYFLOAT": IncrByFloat,
	"MGET":        Mget,
	"MSET":        Mset,
	"MSETNX":      MSetNX,
	"STRLEN":      Strlen,
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
//   - common.NULL if key does not exist or expired
//
// Behavior:
//   - Automatically deletes expired keys
//   - Thread-safe (read lock)
func Get(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// cmd := v.Arr[0].Blk
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR invalid 'GET' command usage, expected 1 argument")
	}
	key := args[0].Blk // grab the key

	// get the item from the database
	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Poll(key)
	// delete if expired
	deleted := database.DB.RemIfExpired(key, item, state)
	if deleted {
		fmt.Println("[RUNTIME] Expired Key: ", key)
		return common.NewNullValue()
	}

	if !ok {
		fmt.Println("[RUNTIME] Key Not Found: ", key)
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
	oldMemory := int64(0)
	if existing, ok := database.DB.Store[key]; ok {
		oldItem = existing
		oldMemory = int64(oldItem.ApproxMemoryUsage(key))
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
	defer database.DB.Mu.Unlock()

	err := database.DB.Put(key, val, state)
	if err != nil {
		return common.NewErrorValue("ERR some error occured while PUT:" + err.Error())
	}
	database.DB.Touch(key)
	// record it for AOF
	saveDBState(state, v)

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

	saveDBState(state, v)
	return common.NewStringValue("OK")
}

// Strlen handles the STRLEN command.
// Returns the length of the string value stored at key.
//
// Syntax:
//
//	STRLEN <key>
//
// Returns:
//
//	Integer: Length of the string at key, or 0 if key does not exist
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

// APPEND - Append value to existing string
// If key does not exist, it is created with the value
// as the initial content.
// Returns the length of the string after the append operation.
// Syntax:
// APPEND <key> <value>
// Returns:
// Integer: Length of the string after append
func Append(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'append' command")
	}
	key := args[0].Blk
	value := args[1].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if ok && item.IsExpired() {
		database.DB.Rem(key)
		ok = false
	}

	if !ok {
		// create new
		newItem := common.NewStringItem(value)
		database.DB.Store[key] = newItem
		database.DB.Touch(key)
		mem := newItem.ApproxMemoryUsage(key)
		database.DB.Mem += mem
		if database.DB.Mem > database.DB.Mempeak {
			database.DB.Mempeak = database.DB.Mem
		}
		saveDBState(state, v)
		return common.NewIntegerValue(int64(len(value)))
	} else {
		if !item.IsString() {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		item.Str += value
		newLen := len(item.Str)
		oldMem := item.ApproxMemoryUsage(key)
		newMem := item.ApproxMemoryUsage(key)
		database.DB.Mem += (newMem - oldMem)
		if database.DB.Mem > database.DB.Mempeak {
			database.DB.Mempeak = database.DB.Mem
		}
		database.DB.Touch(key)
		saveDBState(state, v)
		return common.NewIntegerValue(int64(newLen))
	}
}

// GETRANGE - Get substring
// Gets a substring of the string stored at a key.
// Syntax:
// GETRANGE <key> <start> <end>
// Returns:
// Bulk String: Substring of the string stored at key with start and end offsets inclusive
// Example:
// mykey contains "hello world"
// GETRANGE mykey 1 4
// Returns:
// "ello"
func GetRange(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'getrange' command")
	}
	key := args[0].Blk
	startStr := args[1].Blk
	endStr := args[2].Blk
	start, err := common.ParseInt(startStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}
	end, err := common.ParseInt(endStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	database.DB.Mu.RLock()
	item, ok := database.DB.Poll(key)
	database.DB.Mu.RUnlock()

	if !ok || item.IsExpired() {
		return common.NewBulkValue("")
	}
	if !item.IsString() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	str := item.Str
	length := int64(len(str))
	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}
	if start < 0 {
		start = 0
	}
	if end >= length {
		end = length - 1
	}
	if start > end || start >= length {
		return common.NewBulkValue("")
	}
	substr := str[int(start) : int(end)+1]
	return common.NewBulkValue(substr)
}

// SetRange handles the SETRANGE command.
// Sets the substring of the string value stored at key,
// starting at the specified offset, to the given value.
// Syntax:
// SETRANGE <key> <offset> <value>
// Returns:
// Integer: Length of the string after the operation
func SetRange(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'setrange' command")
	}
	key := args[0].Blk
	offsetStr := args[1].Blk
	value := args[2].Blk
	offset, err := common.ParseInt(offsetStr)
	if err != nil || offset < 0 {
		return common.NewErrorValue("ERR offset is out of range")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if ok && item.IsExpired() {
		database.DB.Rem(key)
		ok = false
	}

	if !ok {
		if offset > 0 {
			str := make([]byte, int(offset)+len(value))
			copy(str[int(offset):], []byte(value))
			newItem := common.NewStringItem(string(str))
			database.DB.Store[key] = newItem
			database.DB.Touch(key)
			mem := newItem.ApproxMemoryUsage(key)
			database.DB.Mem += mem
			if database.DB.Mem > database.DB.Mempeak {
				database.DB.Mempeak = database.DB.Mem
			}
			saveDBState(state, v)
			return common.NewIntegerValue(int64(len(str)))
		} else {
			newItem := common.NewStringItem(value)
			database.DB.Store[key] = newItem
			database.DB.Touch(key)
			mem := newItem.ApproxMemoryUsage(key)
			database.DB.Mem += mem
			if database.DB.Mem > database.DB.Mempeak {
				database.DB.Mempeak = database.DB.Mem
			}
			saveDBState(state, v)
			return common.NewIntegerValue(int64(len(value)))
		}
	} else {
		if !item.IsString() {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		str := []byte(item.Str)
		valBytes := []byte(value)
		newLen := int(offset) + len(valBytes)
		if newLen > len(str) {
			newStr := make([]byte, newLen)
			copy(newStr, str)
			copy(newStr[int(offset):], valBytes)
			item.Str = string(newStr)
		} else {
			copy(str[int(offset):], valBytes)
			item.Str = string(str)
		}
		database.DB.Touch(key)
		oldMem := item.ApproxMemoryUsage(key)
		newMem := item.ApproxMemoryUsage(key)
		database.DB.Mem += (newMem - oldMem)
		if database.DB.Mem > database.DB.Mempeak {
			database.DB.Mempeak = database.DB.Mem
		}
		saveDBState(state, v)
		return common.NewIntegerValue(int64(len(item.Str)))
	}
}

// SetNX  handles the SETNX command.
// Sets the value of a key, only if the key does not already exist.
//
// Syntax:
//
//	SETNX <key> <value>
//
// Returns:
//
//	Integer: 1 if the key was set, 0 if the key was not set
func SetNX(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'setnx' command")
	}
	key := args[0].Blk
	val := args[1].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if ok && !item.IsExpired() {
		return common.NewIntegerValue(0)
	}
	if ok && item.IsExpired() {
		database.DB.Rem(key)
	}

	err := database.DB.Put(key, val, state)
	if err != nil {
		return common.NewErrorValue("ERR some error occured while PUT:" + err.Error())
	}
	saveDBState(state, v)

	return common.NewIntegerValue(1)
}

// SetEx handles the SETEX command.
// SETEX - Set with expiration (seconds)
// Sets the value of a key with an expiration time in seconds.
// Syntax:
// SETEX <key> <seconds> <value>
// Returns:
// Simple String: OK
func SetEX(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'setex' command")
	}
	key := args[0].Blk
	ttlStr := args[1].Blk
	val := args[2].Blk
	ttl, err := common.ParseInt(ttlStr)
	if err != nil || ttl < 0 {
		return common.NewErrorValue("ERR invalid expire time in setex")
	}

	database.DB.Mu.Lock()
	err = database.DB.Put(key, val, state)
	if err != nil {
		database.DB.Mu.Unlock()
		return common.NewErrorValue("ERR some error occured while PUT:" + err.Error())
	}
	database.DB.Store[key].Exp = time.Now().Add(time.Duration(ttl) * time.Second)
	database.DB.Touch(key)
	saveDBState(state, v)
	database.DB.Mu.Unlock()

	return common.NewStringValue("OK")
}

// PSetEX handles the PSETEX command.
// PSETEX - Set with expiration (milliseconds)
// Set with expiration (milliseconds)
// Sets the value of a key with an expiration time in milliseconds.
// Syntax:
// PSETEX <key> <milliseconds> <value>
// Returns:
// Simple String: OK
func PSetEX(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'psetex' command")
	}
	key := args[0].Blk
	ttlStr := args[1].Blk
	val := args[2].Blk
	ttl, err := common.ParseInt(ttlStr)
	if err != nil || ttl < 0 {
		return common.NewErrorValue("ERR invalid expire time in psetex")
	}

	database.DB.Mu.Lock()
	err = database.DB.Put(key, val, state)
	if err != nil {
		database.DB.Mu.Unlock()
		return common.NewErrorValue("ERR some error occured while PUT:" + err.Error())
	}
	database.DB.Store[key].Exp = time.Now().Add(time.Duration(ttl) * time.Millisecond)
	database.DB.Touch(key)
	saveDBState(state, v)
	database.DB.Mu.Unlock()

	return common.NewStringValue("OK")
}

// GetSet handles the GETSET command.
// GETSET - Set new value and return old
// Sets the value of a key, and returns its old value.
// Syntax:
// GETSET <key> <value>
// Returns:
// Bulk String: Old value of the key, or null if key did not exist
func GetSet(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'getset' command")
	}
	key := args[0].Blk
	val := args[1].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	var oldVal *common.Value
	if ok && !item.IsExpired() {
		if !item.IsString() {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldVal = common.NewBulkValue(item.Str)
	} else {
		oldVal = common.NewNullValue()
		if ok && item.IsExpired() {
			database.DB.Rem(key)
		}
	}

	err := database.DB.Put(key, val, state)
	if err != nil {
		return common.NewErrorValue("ERR some error occured while PUT:" + err.Error())
	}

	saveDBState(state, v)

	return oldVal
}

// GETEX - Get with expiration options
// Supports EX, PX, EXAT, PXAT, PERSIST options
// to set expiration on the key after retrieval
// of its value.
// If no expiration option is provided, simply
// returns the value without modifying expiration.
// EX - seconds
// PX - milliseconds
// EXAT - unix timestamp in seconds
// PXAT - unix timestamp in milliseconds
// PERSIST - remove expiration
// Syntax:
// GETEX <key> [EX seconds|PX milliseconds|EXAT unix-seconds|PXAT unix-milliseconds|PERSIST]
// Returns:
// Bulk String: Value of the key, or null if key did not exist
func GetEX(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'getex' command")
	}
	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok || item.IsExpired() {
		if ok && item.IsExpired() {
			database.DB.Rem(key)
		}
		return common.NewNullValue()
	}
	if !item.IsString() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	val := common.NewBulkValue(item.Str)

	if len(args) == 1 {
		return val
	}

	i := 1
	for i < len(args) {
		opt := args[i].Blk
		switch opt {
		case "EX":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			secondsStr := args[i+1].Blk
			seconds, err := common.ParseInt(secondsStr)
			if err != nil || seconds < 0 {
				return common.NewErrorValue("ERR invalid expire time in getex")
			}
			item.Exp = time.Now().Add(time.Duration(seconds) * time.Second)
			i += 2
		case "PX":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			msStr := args[i+1].Blk
			ms, err := common.ParseInt(msStr)
			if err != nil || ms < 0 {
				return common.NewErrorValue("ERR invalid expire time in getex")
			}
			item.Exp = time.Now().Add(time.Duration(ms) * time.Millisecond)
			i += 2
		case "EXAT":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			unixStr := args[i+1].Blk
			unixSec, err := common.ParseInt(unixStr)
			if err != nil || unixSec < 0 {
				return common.NewErrorValue("ERR invalid expire time in getex")
			}
			item.Exp = time.Unix(unixSec, 0)
			i += 2
		case "PXAT":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			unixMsStr := args[i+1].Blk
			unixMs, err := common.ParseInt(unixMsStr)
			if err != nil || unixMs < 0 {
				return common.NewErrorValue("ERR invalid expire time in getex")
			}
			sec := unixMs / 1000
			nsec := (unixMs % 1000) * 1000000
			item.Exp = time.Unix(sec, nsec)
			i += 2
		case "PERSIST":
			item.Exp = time.Time{}
			i += 1
		default:
			return common.NewErrorValue("ERR syntax error")
		}
	}

	database.DB.Touch(key)

	return val
}

// GetDel handles the GETDEL command.
// GETDEL - Get and delete
// Retrieves the value of a key and deletes the key.
// Syntax:
// GETDEL <key>
// Returns:
// Bulk String: Value of the key, or null if key did not exist
func GetDel(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'getdel' command")
	}
	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok || item.IsExpired() {
		if ok && item.IsExpired() {
			database.DB.Rem(key)
		}
		return common.NewNullValue()
	}
	if !item.IsString() {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	val := common.NewBulkValue(item.Str)
	database.DB.Rem(key)

	return val
}

// IncrByFloat handles the INCRBYFLOAT command.
// INCRBYFLOAT - Increment by float
// Increments the float value of a key by the given amount.
// Syntax:
// INCRBYFLOAT <key> <increment>
// Returns:
// Bulk String: The value of key after the increment
func IncrByFloat(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'incrbyfloat' command")
	}
	key := args[0].Blk
	incrStr := args[1].Blk
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

	val, err := common.ParseFloat(item.Str)
	if err != nil {
		return common.NewErrorValue("ERR value is not a valid float")
	}

	newVal := val + incr
	item.Str = strconv.FormatFloat(newVal, 'f', -1, 64)

	database.DB.Touch(key)
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem += (newMemory - oldMemory)
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	saveDBState(state, v)

	return common.NewBulkValue(item.Str)
}

// MSetNX handles the MSETNX command.
// MSETNX - Set multiple if none exist
// Sets multiple keys to multiple values, only if none of the keys exist.
// Syntax:
// MSETNX <key> <value> [<key> <value> ...]
// Returns:
// Integer: 1 if all keys were set, 0 if no keys were set
func MSetNX(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 || len(args)%2 != 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'msetnx' command")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	for i := 0; i < len(args); i += 2 {
		key := args[i].Blk
		item, ok := database.DB.Store[key]
		if ok && !item.IsExpired() {
			return common.NewIntegerValue(0)
		}
	}

	for i := 0; i < len(args); i += 2 {
		key := args[i].Blk
		val := args[i+1].Blk
		database.DB.Put(key, val, state)
	}

	saveDBState(state, v)

	return common.NewIntegerValue(1)
}

// Helper function to handle INCRBY and DECRBY logic
func incrDecrBy(c *common.Client, key string, delta int64, state *common.AppState, v *common.Value) *common.Value {
	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		oldMemory = item.ApproxMemoryUsage(key)
		if item.IsExpired() {
			item = common.NewStringItem("0")
			database.DB.Store[key] = item
		} else {
			if !item.IsString() {
				return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
			}
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

	saveDBState(state, v)

	return common.NewIntegerValue(newVal)
}
