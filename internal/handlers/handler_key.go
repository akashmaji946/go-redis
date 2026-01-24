/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_key.go
*/
package handlers

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// KeyHandlers is the map of key command names to their handler functions.
var KeyHandlers = map[string]common.Handler{
	"TYPE":        Type,
	"DELETE":      Del,
	"DEL":         Del,
	"RENAME":      Rename,
	"EXISTS":      Exists,
	"KEYS":        Keys,
	"EXPIRE":      Expire,
	"TTL":         Ttl,
	"PERSIST":     Persist,
	"EXPIREAT":    ExpireAt,
	"PEXPIRE":     PExpire,
	"PEXPIREAT":   PExpireAt,
	"PTTL":        PTtl,
	"EXPIRETIME":  ExpireTime,
	"PEXPIRETIME": PExpireTime,
	"COPY":        Copy,
	"RENAMENX":    RenameNx,
	"TOUCH":       Touch,
	"UNLINK":      Unlink,
	"RANDOMKEY":   RandomKey,
	"SORT":        Sort,
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

	saveDBState(state, v)

	return common.NewIntegerValue(1)
}

// ExpireAt handles the EXPIREAT command.
// Sets expiration time on a key to an absolute Unix timestamp (seconds).
//
// Syntax:
//
//	EXPIREAT <key> <timestamp>
//
// Returns:
//
//	1 if expiration set
//	0 if key does not exist
//
// Notes:
//   - Timestamp is in seconds since epoch
//   - If timestamp <= current time, key is deleted immediately
func ExpireAt(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR invalid args for EXPIREAT")
	}
	k := args[0].Blk
	tsStr := args[1].Blk
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return common.NewErrorValue("ERR invalid timestamp for EXPIREAT")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[k]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.IsExpired() {
		database.DB.Rem(k)
		return common.NewIntegerValue(0)
	}

	expTime := time.Unix(ts, 0)
	if expTime.Before(time.Now()) {
		database.DB.Rem(k)
		database.DB.Touch(k)
		if len(state.Config.Rdb) > 0 {
			database.DB.IncrTrackers()
		}
		return common.NewIntegerValue(0)
	}

	item.Exp = expTime
	database.DB.Touch(k)

	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(1)
}

// PExpire handles the PEXPIRE command.
// Sets expiration time on a key relative to now, in milliseconds.
//
// Syntax:
//
//	PEXPIRE <key> <milliseconds>
//
// Returns:
//
//	1 if expiration set
//	0 if key does not exist
func PExpire(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR invalid args for PEXPIRE")
	}
	k := args[0].Blk
	msStr := args[1].Blk
	ms, err := strconv.ParseInt(msStr, 10, 64)
	if err != nil {
		return common.NewErrorValue("ERR invalid milliseconds for PEXPIRE")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[k]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.IsExpired() {
		database.DB.Rem(k)
		return common.NewIntegerValue(0)
	}

	item.Exp = time.Now().Add(time.Duration(ms) * time.Millisecond)
	database.DB.Touch(k)

	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(1)
}

// PExpireAt handles the PEXPIREAT command.
// Sets expiration time on a key to an absolute Unix timestamp (milliseconds).
//
// Syntax:
//
//	PEXPIREAT <key> <timestamp_ms>
//
// Returns:
//
//	1 if expiration set
//	0 if key does not exist
func PExpireAt(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR invalid args for PEXPIREAT")
	}
	k := args[0].Blk
	tsStr := args[1].Blk
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return common.NewErrorValue("ERR invalid timestamp for PEXPIREAT")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[k]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.IsExpired() {
		database.DB.Rem(k)
		return common.NewIntegerValue(0)
	}

	expTime := time.Unix(0, ts*int64(time.Millisecond))
	if expTime.Before(time.Now()) {
		database.DB.Rem(k)
		database.DB.Touch(k)
		if len(state.Config.Rdb) > 0 {
			database.DB.IncrTrackers()
		}
		return common.NewIntegerValue(0)
	}

	item.Exp = expTime
	database.DB.Touch(k)

	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(1)
}

// PTtl handles the PTTL command.
// Returns remaining time-to-live for a key in milliseconds.
//
// Syntax:
//
//	PTTL <key>
//
// Returns:
//
//	>0  remaining milliseconds
//	-1  key exists but no expiration
//	-2  key does not exist or expired
func PTtl(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR invalid arg for PTTL")
	}

	k := args[0].Blk

	database.DB.Mu.RLock()
	item, ok := database.DB.Store[k]
	if !ok {
		database.DB.Mu.RUnlock()
		return common.NewIntegerValue(-2)
	}
	exp := item.Exp
	database.DB.Mu.RUnlock()

	if exp.IsZero() {
		return common.NewIntegerValue(-1)
	}

	expired := database.DB.RemIfExpired(k, item, state)
	if expired {
		return common.NewIntegerValue(-2)
	}

	msToExpire := time.Until(exp).Milliseconds()
	return common.NewIntegerValue(msToExpire)
}

// ExpireTime handles the EXPIRETIME command.
// Returns the absolute expiration time as Unix timestamp (seconds).
//
// Syntax:
//
//	EXPIRETIME <key>
//
// Returns:
//
//	Unix timestamp (seconds)
//	-1 if key exists but no expiration
//	-2 if key does not exist or expired
func ExpireTime(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR invalid arg for EXPIRETIME")
	}

	k := args[0].Blk

	database.DB.Mu.RLock()
	item, ok := database.DB.Store[k]
	if !ok {
		database.DB.Mu.RUnlock()
		return common.NewIntegerValue(-2)
	}
	exp := item.Exp
	database.DB.Mu.RUnlock()

	if exp.IsZero() {
		return common.NewIntegerValue(-1)
	}

	expired := database.DB.RemIfExpired(k, item, state)
	if expired {
		return common.NewIntegerValue(-2)
	}

	return common.NewIntegerValue(exp.Unix())
}

// PExpireTime handles the PEXPIRETIME command.
// Returns the absolute expiration time as Unix timestamp (milliseconds).
//
// Syntax:
//
//	PEXPIRETIME <key>
//
// Returns:
//
//	Unix timestamp (milliseconds)
//	-1 if key exists but no expiration
//	-2 if key does not exist or expired
func PExpireTime(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR invalid arg for PEXPIRETIME")
	}

	k := args[0].Blk

	database.DB.Mu.RLock()
	item, ok := database.DB.Store[k]
	if !ok {
		database.DB.Mu.RUnlock()
		return common.NewIntegerValue(-2)
	}
	exp := item.Exp
	database.DB.Mu.RUnlock()

	if exp.IsZero() {
		return common.NewIntegerValue(-1)
	}

	expired := database.DB.RemIfExpired(k, item, state)
	if expired {
		return common.NewIntegerValue(-2)
	}

	return common.NewIntegerValue(exp.UnixNano() / int64(time.Millisecond))
}

// Copy handles the COPY command.
// Copies a key to another name.
//
// Syntax:
//
//	COPY <src> <dst> [REPLACE]
//
// Returns:
//
//	1 on success
//	0 on failure (src doesn't exist or dst exists and no REPLACE)
func Copy(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 || len(args) > 3 {
		return common.NewErrorValue("ERR invalid args for COPY")
	}
	src := args[0].Blk
	dst := args[1].Blk
	replace := len(args) == 3 && strings.ToUpper(args[2].Blk) == "REPLACE"

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[src]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.IsExpired() {
		database.DB.Rem(src)
		return common.NewIntegerValue(0)
	}

	if _, exists := database.DB.Store[dst]; exists && !replace {
		return common.NewIntegerValue(0)
	}

	// Create a copy of the item
	newItem := *item // shallow copy
	newItem.LastAccessed = time.Now()
	newItem.AccessCount = 0

	// If dst exists, remove it first
	if _, exists := database.DB.Store[dst]; exists {
		oldMem := database.DB.Store[dst].ApproxMemoryUsage(dst)
		delete(database.DB.Store, dst)
		database.DB.Mem -= oldMem
	}

	database.DB.Store[dst] = &newItem
	newMem := newItem.ApproxMemoryUsage(dst)
	database.DB.Mem += newMem
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	database.DB.Touch(src)
	database.DB.Touch(dst)

	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(1)
}

// RenameNx handles the RENAMENX command.
// Renames a key only if the destination does not exist.
//
// Syntax:
//
//	RENAMENX <oldkey> <newkey>
//
// Returns:
//
//	1 if renamed
//	0 otherwise
func RenameNx(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'renamenx' command")
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

	// Check if target exists
	if _, ok := database.DB.Store[newKey]; ok {
		return common.NewIntegerValue(0)
	}

	// Move logic
	oldMem := item.ApproxMemoryUsage(oldKey)
	delete(database.DB.Store, oldKey)
	database.DB.Mem -= oldMem

	database.DB.Touch(oldKey)
	database.DB.Touch(newKey)

	database.DB.Store[newKey] = item
	newMem := item.ApproxMemoryUsage(newKey)
	database.DB.Mem += newMem
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	saveDBState(state, v)

	return common.NewIntegerValue(1)
}

// Touch handles the TOUCH command.
// Updates the last access time for keys.
//
// Syntax:
//
//	TOUCH <key> [key ...]
//
// Returns:
//
//	Number of keys touched
func Touch(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'touch' command")
	}

	database.DB.Mu.Lock()
	count := 0
	for _, arg := range args {
		key := arg.Blk
		if _, ok := database.DB.Store[key]; ok {
			database.DB.Touch(key)
			count++
		}
	}
	database.DB.Mu.Unlock()

	return common.NewIntegerValue(int64(count))
}

// Unlink handles the UNLINK command.
// Asynchronously deletes keys (but implemented synchronously here).
//
// Syntax:
//
//	UNLINK <key> [key ...]
//
// Returns:
//
//	Number of keys unlinked
func Unlink(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// Same as DEL for now
	return Del(c, v, state)
}

// RandomKey handles the RANDOMKEY command.
// Returns a random key from the database.
//
// Syntax:
//
//	RANDOMKEY
//
// Returns:
//
//	A random key, or nil if DB is empty
func RandomKey(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'randomkey' command")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	if len(database.DB.Store) == 0 {
		return common.NewNullValue()
	}

	// Get all keys
	keys := make([]string, 0, len(database.DB.Store))
	for k := range database.DB.Store {
		keys = append(keys, k)
	}

	randomKey := keys[rand.Intn(len(keys))]
	return common.NewBulkValue(randomKey)
}

// Sort handles the SORT command.
// Sorts a collection (list, set, zset).
//
// Syntax:
//
//	SORT <key> [ALPHA]
//
// Returns:
//
//	Array of sorted elements
func Sort(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 || len(args) > 2 {
		return common.NewErrorValue("ERR invalid args for SORT")
	}
	key := args[0].Blk
	alpha := len(args) == 2 && strings.ToUpper(args[1].Blk) == "ALPHA"

	database.DB.Mu.RLock()
	item, ok := database.DB.Store[key]
	if !ok {
		database.DB.Mu.RUnlock()
		return common.NewErrorValue("ERR no such key")
	}

	if item.IsExpired() {
		database.DB.Mu.RUnlock()
		database.DB.RemIfExpired(key, item, state)
		return common.NewErrorValue("ERR no such key")
	}

	var elements []string

	switch item.Type {
	case "list":
		elements = make([]string, len(item.List))
		copy(elements, item.List)
	case "set":
		elements = make([]string, 0, len(item.ItemSet))
		for k := range item.ItemSet {
			elements = append(elements, k)
		}
	case "zset":
		elements = make([]string, 0, len(item.ZSet))
		for k := range item.ZSet {
			elements = append(elements, k)
		}
	default:
		database.DB.Mu.RUnlock()
		return common.NewErrorValue("ERR WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	database.DB.Mu.RUnlock()

	if alpha {
		sort.Strings(elements)
	} else {
		// Numeric sort
		sort.Slice(elements, func(i, j int) bool {
			a, errA := strconv.ParseFloat(elements[i], 64)
			b, errB := strconv.ParseFloat(elements[j], 64)
			if errA != nil || errB != nil {
				// If not numeric, sort lexicographically
				return elements[i] < elements[j]
			}
			return a < b
		})
	}

	reply := common.Value{
		Typ: common.ARRAY,
	}
	for _, elem := range elements {
		reply.Arr = append(reply.Arr, common.Value{Typ: common.BULK, Blk: elem})
	}
	return &reply
}
