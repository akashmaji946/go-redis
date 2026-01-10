/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_set.go
*/
package handlers

import (
	"math/rand"
	"strconv"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// Sadd handles the SADD command.
// Adds one or more members to a set.
//
// Syntax:
//
//	SADD <key> <member> [<member> ...]
//
// Returns:
//
//	Integer: The number of elements that were added to the set, not including all the elements already present into the set.
func Sadd(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sadd' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		if item.Type != "set" {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.ApproxMemoryUsage(key)
	} else {
		item = &common.Item{
			Type:    "set",
			ItemSet: make(map[string]bool),
		}
		database.DB.Store[key] = item
	}

	count := int64(0)
	for _, arg := range args[1:] {
		member := arg.Blk
		if !item.ItemSet[member] {
			item.ItemSet[member] = true
			count++
		}
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
		common.IncrRDBTrackers()
	}

	return common.NewIntegerValue(count)
}

// Srem handles the SREM command.
// Removes one or more members from a set.
//
// Syntax:
//
//	SREM <key> <member> [<member> ...]
//
// Returns:
//
//	Integer: The number of members that were removed from the set, not including non existing members.
func Srem(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'srem' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	oldMemory := item.ApproxMemoryUsage(key)
	count := int64(0)

	for _, arg := range args[1:] {
		member := arg.Blk
		if item.ItemSet[member] {
			delete(item.ItemSet, member)
			count++
		}
	}

	database.DB.Touch(key)
	if len(item.ItemSet) == 0 {
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
		common.IncrRDBTrackers()
	}

	return common.NewIntegerValue(count)
}

// Smembers handles the SMEMBERS command.
// Returns all the members of the set value stored at key.
//
// Syntax:
//
//	SMEMBERS <key>
//
// Returns:
//
//	Array: All elements of the set.
func Smembers(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'smembers' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if item.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0, len(item.ItemSet))
	for member := range item.ItemSet {
		result = append(result, common.Value{Typ: common.BULK, Blk: member})
	}

	return common.NewArrayValue(result)
}

// Sismember handles the SISMEMBER command.
// Returns if member is a member of the set stored at key.
//
// Syntax:
//
//	SISMEMBER <key> <member>
//
// Returns:
//
//	Integer: 1 if the element is a member of the set. 0 if the element is not a member of the set, or if key does not exist.
func Sismember(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sismember' command")
	}

	key := args[0].Blk
	member := args[1].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if item.ItemSet[member] {
		return common.NewIntegerValue(1)
	}

	return common.NewIntegerValue(0)
}

// Scard handles the SCARD command.
// Returns the set cardinality (number of elements) of the set stored at key.
//
// Syntax:
//
//	SCARD <key>
//
// Returns:
//
//	Integer: The cardinality (number of elements) of the set, or 0 if key does not exist.
func Scard(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'scard' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return common.NewIntegerValue(int64(len(item.ItemSet)))
}

// Sinter handles the SINTER command.
// Returns the members of the set resulting from the intersection of all the given sets.
//
// Syntax:
//
//	SINTER <key> [<key> ...]
//
// Returns:
//
//	Array: List of members.
func Sinter(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sinter' command")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	// 1. Collect all sets
	var sets []map[string]bool
	for _, arg := range args {
		key := arg.Blk
		item, ok := database.DB.Store[key]

		// If any key is missing, the intersection is empty
		if !ok {
			return common.NewArrayValue([]common.Value{})
		}

		if item.Type != "set" {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		sets = append(sets, item.ItemSet)
	}

	if len(sets) == 0 {
		return common.NewArrayValue([]common.Value{})
	}

	// 2. Optimization: Find the smallest set to iterate over
	minIndex := 0
	minLen := len(sets[0])
	for i, s := range sets {
		if len(s) < minLen {
			minLen = len(s)
			minIndex = i
		}
	}

	// 3. Perform Intersection
	result := make([]common.Value, 0)
	for member := range sets[minIndex] {
		presentInAll := true
		for i, s := range sets {
			if i == minIndex {
				continue
			}
			if !s[member] {
				presentInAll = false
				break
			}
		}
		if presentInAll {
			result = append(result, common.Value{Typ: common.BULK, Blk: member})
		}
	}

	return common.NewArrayValue(result)
}

// Sunion handles the SUNION command.
// Returns the members of the set resulting from the union of all the given sets.
//
// Syntax:
//
//	SUNION <key> [<key> ...]
//
// Returns:
//
//	Array: List of members.
func Sunion(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sunion' command")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	unionMap := make(map[string]bool)

	for _, arg := range args {
		key := arg.Blk
		item, ok := database.DB.Store[key]
		if !ok {
			continue
		}
		if item.Type != "set" {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		for member := range item.ItemSet {
			unionMap[member] = true
		}
	}

	result := make([]common.Value, 0, len(unionMap))
	for member := range unionMap {
		result = append(result, common.Value{Typ: common.BULK, Blk: member})
	}

	return common.NewArrayValue(result)
}

// Sdiff handles the SDIFF command.
// Returns the members of the set resulting from the difference between the first set and all the successive sets.
//
// Syntax:
//
//	SDIFF <key> [<key> ...]
//
// Returns:
//
//	Array: List of members.
func Sdiff(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sdiff' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}
	if item.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0)

	for member := range item.ItemSet {
		presentInOthers := false
		for _, arg := range args[1:] {
			otherKey := arg.Blk
			otherItem, ok := database.DB.Store[otherKey]
			if ok {
				if otherItem.Type != "set" {
					return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
				}
				if otherItem.ItemSet[member] {
					presentInOthers = true
					break
				}
			}
		}
		if !presentInOthers {
			result = append(result, common.Value{Typ: common.BULK, Blk: member})
		}
	}

	return common.NewArrayValue(result)
}

// Srandmember handles the SRANDMEMBER command.
// Get one or multiple random members from a set.
//
// Syntax:
//
//	SRANDMEMBER <key> [count]
//
// Returns:
//
//	Bulk String: If no count is provided.
//	Array: If count is provided.
func Srandmember(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'srandmember' command")
	}

	key := args[0].Blk
	count := 1
	hasCount := false

	if len(args) > 1 {
		c, err := strconv.Atoi(args[1].Blk)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
		count = c
		hasCount = true
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		if hasCount {
			return common.NewArrayValue([]common.Value{})
		}
		return common.NewNullValue()
	}
	if item.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	size := len(item.ItemSet)
	if size == 0 {
		if hasCount {
			return common.NewArrayValue([]common.Value{})
		}
		return common.NewNullValue()
	}

	// Case 1: common.No count argument -> return Bulk String
	if !hasCount {
		// Map iteration is random in Go
		for member := range item.ItemSet {
			return common.NewBulkValue(member)
		}
	}

	// Case 2: Count argument -> return Array
	result := make([]common.Value, 0)

	// If count is positive, return distinct elements
	if count > 0 {
		if count >= size {
			// Return all elements
			for member := range item.ItemSet {
				result = append(result, common.Value{Typ: common.BULK, Blk: member})
			}
		} else {
			// Return count distinct elements
			i := 0
			for member := range item.ItemSet {
				result = append(result, common.Value{Typ: common.BULK, Blk: member})
				i++
				if i == count {
					break
				}
			}
		}
	} else {
		// If count is negative, return non-distinct elements (allow duplicates)
		count = -count
		members := make([]string, 0, size)
		for member := range item.ItemSet {
			members = append(members, member)
		}

		for i := 0; i < count; i++ {
			idx := rand.Intn(size)
			result = append(result, common.Value{Typ: common.BULK, Blk: members[idx]})
		}
	}

	return common.NewArrayValue(result)
}
