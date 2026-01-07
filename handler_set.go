/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_set.go
*/
package main

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
