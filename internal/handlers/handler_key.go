/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_key.go
*/
package handlers

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// KeyHandlers is the map of key command names to their handler functions.
var KeyHandlers = map[string]common.Handler{
	"TYPE":    Type,
	"DELETE":  Del,
	"DEL":     Del,
	"RENAME":  Rename,
	"EXISTS":  Exists,
	"KEYS":    Keys,
	"EXPIRE":  Expire,
	"TTL":     Ttl,
	"PERSIST": Persist,
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
func Del(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// DEL k1 k2 k3 ... kn
	// returns m, number of keys actually deleted (m <= n)
	args := v.Arr[1:]
	m := 0
	database.DB.Mu.Lock()
	for _, arg := range args {
		key := arg.Blk
		_, ok := database.DB.Poll(key)
		if !ok {
			// doesnot exist
			continue
		}
		// delete
		database.DB.Rem(key)
		database.DB.Touch(key)
		m += 1
	}
	database.DB.Mu.Unlock()

	// Signal changes for automatic RDB saving
	if m > 0 && len(state.Config.Rdb) > 0 {
		for i := 0; i < m; i++ {
			database.DB.IncrTrackers()
		}
	}
	return common.NewIntegerValue(int64(m))
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
func Exists(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// Exists k1 k2 k3 .. kn
	// m (m <= n)
	args := v.Arr[1:]
	m := 0
	database.DB.Mu.RLock()
	for _, arg := range args {
		_, ok := database.DB.Store[arg.Blk]
		if ok {
			m += 1
		}
	}
	database.DB.Mu.RUnlock()

	return common.NewIntegerValue(int64(m))
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
func Keys(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// Keys pattern
	// all keys matching pattern (in an array)
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR invlid arg to Keys")
	}

	pattern := args[0].Blk // string representing the pattern e.g. "*name*" matches name, names, firstname, lastname

	database.DB.Mu.RLock()
	var matches []string
	for key, _ := range database.DB.Store {
		matched, err := filepath.Match(pattern, key)
		if err != nil {
			fmt.Printf("error matching for keys: (key=%s, pattern=%s)\nError: %s\n", key, pattern, err)
			continue
		}
		if matched {
			matches = append(matches, key)
		}
	}
	database.DB.Mu.RUnlock()

	reply := common.Value{
		Typ: common.ARRAY,
	}
	for _, key := range matches {
		reply.Arr = append(reply.Arr, common.Value{Typ: common.BULK, Blk: key})
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
//	Simple String: type of key (e.g., common.STRING, LIST, HASH), or "none" if key does not exist.
func Type(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'type' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewStringValue("none")
	}

	return common.NewStringValue(strings.ToUpper(item.Type))
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
func Expire(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// EXPIRE <key> <secondsafter>
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR invalid args for EXPIRE")
	}
	k := args[0].Blk
	exp := args[1].Blk
	expirySeconds, err := common.ParseInt(exp)
	if err != nil {
		return common.NewErrorValue("ERR invalid 2nd arg for EXPIRE")
	}

	database.DB.Mu.RLock()
	Val, ok := database.DB.Store[k]
	if !ok {
		return common.NewIntegerValue(0)
	}
	Val.Exp = time.Now().Add(time.Second * time.Duration(expirySeconds))
	database.DB.Touch(k)
	database.DB.Mu.RUnlock()

	// Signal change for automatic RDB saving
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(1)

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
func Ttl(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// TTL <key>
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR invalid arg for TTL")
	}

	k := args[0].Blk

	database.DB.Mu.RLock()
	item, ok := database.DB.Store[k]
	if !ok {
		fmt.Println("TTL: key not found ", k)
		database.DB.Mu.RUnlock()
		return common.NewIntegerValue(-2)
	}
	exp := item.Exp
	database.DB.Mu.RUnlock()

	// is exp not set
	if exp.Unix() == common.UNIX_TS_EPOCH {
		return common.NewIntegerValue(-1)
	}

	expired := database.DB.RemIfExpired(k, item, state)
	if expired {
		return common.NewIntegerValue(-2)
	}

	secondsToExpire := time.Until(exp).Seconds() //float
	// fmt.Println(secondsToExpire)
	return common.NewIntegerValue(int64(secondsToExpire))

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
func Persist(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'persist' command")
	}
	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.IsExpired() {
		database.DB.Rem(key)
		return common.NewIntegerValue(0)
	}

	if item.Exp.IsZero() {
		return common.NewIntegerValue(0)
	}

	item.Exp = time.Time{} // Clear expiration
	database.DB.Touch(key)

	// Signal change for automatic RDB saving
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}
	return common.NewIntegerValue(1)
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
func Rename(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'rename' command")
	}

	oldKey := args[0].Blk
	newKey := args[1].Blk

	if oldKey == newKey {
		return common.NewIntegerValue(1)
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Check if source exists
	item, ok := database.DB.Store[oldKey]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.IsExpired() {
		database.DB.Rem(oldKey)
		return common.NewIntegerValue(0)
	}

	// If target exists, don't remove it
	if _, ok := database.DB.Store[newKey]; ok {
		return common.NewIntegerValue(0)
	}

	// Move logic: manually handle memory to avoid database.DB.Rem clearing hash data
	// 1. Calculate old memory usage
	oldMem := item.ApproxMemoryUsage(oldKey)

	// 2. Remove from old key (delete directly to preserve item content)
	delete(database.DB.Store, oldKey)
	database.DB.Mem -= oldMem

	database.DB.Touch(oldKey)
	database.DB.Touch(newKey)

	// 3. Add to new key
	database.DB.Store[newKey] = item
	newMem := item.ApproxMemoryUsage(newKey)
	database.DB.Mem += newMem

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

	return common.NewIntegerValue(1)
}
