/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_set.go
*/
package main

import (
	"math/rand"
	"strconv"
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
func Sadd(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'sadd' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	var item *Item
	var oldMemory int64 = 0

	if existing, ok := DB.store[key]; ok {
		item = existing
		if item.Type != "set" {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.approxMemoryUsage(key)
	} else {
		item = &Item{
			Type:    "set",
			ItemSet: make(map[string]bool),
		}
		DB.store[key] = item
	}

	count := int64(0)
	for _, arg := range args[1:] {
		member := arg.blk
		if !item.ItemSet[member] {
			item.ItemSet[member] = true
			count++
		}
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

	return NewIntegerValue(count)
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
func Srem(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'srem' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	oldMemory := item.approxMemoryUsage(key)
	count := int64(0)

	for _, arg := range args[1:] {
		member := arg.blk
		if item.ItemSet[member] {
			delete(item.ItemSet, member)
			count++
		}
	}

	DB.Touch(key)
	if len(item.ItemSet) == 0 {
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

	return NewIntegerValue(count)
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
func Smembers(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'smembers' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.ItemSet))
	for member := range item.ItemSet {
		result = append(result, Value{typ: BULK, blk: member})
	}

	return NewArrayValue(result)
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
func Sismember(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'sismember' command")
	}

	key := args[0].blk
	member := args[1].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if item.ItemSet[member] {
		return NewIntegerValue(1)
	}

	return NewIntegerValue(0)
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
func Scard(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'scard' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return NewIntegerValue(int64(len(item.ItemSet)))
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
func Sinter(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'sinter' command")
	}

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	// 1. Collect all sets
	var sets []map[string]bool
	for _, arg := range args {
		key := arg.blk
		item, ok := DB.store[key]

		// If any key is missing, the intersection is empty
		if !ok {
			return NewArrayValue([]Value{})
		}

		if item.Type != "set" {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		sets = append(sets, item.ItemSet)
	}

	if len(sets) == 0 {
		return NewArrayValue([]Value{})
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
	result := make([]Value, 0)
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
			result = append(result, Value{typ: BULK, blk: member})
		}
	}

	return NewArrayValue(result)
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
func Sunion(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'sunion' command")
	}

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	unionMap := make(map[string]bool)

	for _, arg := range args {
		key := arg.blk
		item, ok := DB.store[key]
		if !ok {
			continue
		}
		if item.Type != "set" {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		for member := range item.ItemSet {
			unionMap[member] = true
		}
	}

	result := make([]Value, 0, len(unionMap))
	for member := range unionMap {
		result = append(result, Value{typ: BULK, blk: member})
	}

	return NewArrayValue(result)
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
func Sdiff(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'sdiff' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}
	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0)

	for member := range item.ItemSet {
		presentInOthers := false
		for _, arg := range args[1:] {
			otherKey := arg.blk
			otherItem, ok := DB.store[otherKey]
			if ok {
				if otherItem.Type != "set" {
					return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
				}
				if otherItem.ItemSet[member] {
					presentInOthers = true
					break
				}
			}
		}
		if !presentInOthers {
			result = append(result, Value{typ: BULK, blk: member})
		}
	}

	return NewArrayValue(result)
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
func Srandmember(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'srandmember' command")
	}

	key := args[0].blk
	count := 1
	hasCount := false

	if len(args) > 1 {
		c, err := strconv.Atoi(args[1].blk)
		if err != nil {
			return NewErrorValue("ERR value is not an integer or out of range")
		}
		count = c
		hasCount = true
	}

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		if hasCount {
			return NewArrayValue([]Value{})
		}
		return NewNullValue()
	}
	if item.Type != "set" {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	size := len(item.ItemSet)
	if size == 0 {
		if hasCount {
			return NewArrayValue([]Value{})
		}
		return NewNullValue()
	}

	// Case 1: No count argument -> return Bulk String
	if !hasCount {
		// Map iteration is random in Go
		for member := range item.ItemSet {
			return NewBulkValue(member)
		}
	}

	// Case 2: Count argument -> return Array
	result := make([]Value, 0)

	// If count is positive, return distinct elements
	if count > 0 {
		if count >= size {
			// Return all elements
			for member := range item.ItemSet {
				result = append(result, Value{typ: BULK, blk: member})
			}
		} else {
			// Return count distinct elements
			i := 0
			for member := range item.ItemSet {
				result = append(result, Value{typ: BULK, blk: member})
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
			result = append(result, Value{typ: BULK, blk: members[idx]})
		}
	}

	return NewArrayValue(result)
}
