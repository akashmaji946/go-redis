/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_hash.go
*/
package main

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

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
func Hset(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 3 || len(args)%2 == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'hset' command")
	}

	key := args[0].blk
	DB.mu.Lock()
	defer DB.mu.Unlock()

	// Calculate old memory before modification
	var oldMemory int64 = 0
	var item *Item
	if existing, ok := DB.store[key]; ok {
		item = existing
		oldMemory = existing.approxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return NewErrorValue(err.Error())
		}
	} else {
		item = NewHashItem()
		DB.store[key] = item
	}

	count := int64(0)
	for i := 1; i < len(args); i += 2 {
		field := args[i].blk
		value := args[i+1].blk
		if _, exists := item.Hash[field]; !exists {
			count++
		}
		item.Hash[field] = NewHashFieldItem(value)
	}

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			fmt.Println("AOF write for HSET")
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(count)
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
//	NULL: If field or key does not exist
func Hget(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'hget' command")
	}

	key := args[0].blk
	field := args[1].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.Poll(key)
	if ok {
		if !item.IsHash() {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		fieldItem, exists := item.Hash[field]
		if exists {
			// Check if field is expired
			if fieldItem.IsExpired() {
				// Delete here
				fmt.Printf("Expired Key: %s:%s\n", key, field)
				delete(item.Hash, field)
				return NewNullValue()
			}
			// delete if expired
			deleted := DB.RemIfExpired(key, item, state)
			if deleted {
				fmt.Println("Expired Key: ", key)
				return NewNullValue()
			}
			return NewBulkValue(fieldItem.Str)
		}

		return NewNullValue()
	}

	if !ok {
		fmt.Println("Not Found: ", key)
		return NewNullValue()
	}

	return NewBulkValue(item.Str)

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
func Hdel(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'hdel' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Calculate old memory before modification
	oldMemory := item.approxMemoryUsage(key)

	count := int64(0)
	for i := 1; i < len(args); i++ {
		field := args[i].blk
		if _, exists := item.Hash[field]; exists {
			delete(item.Hash, field)
			count++
		}
	}

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
		if state.config.aofFsync == Always {
			fmt.Println("AOF write for HDEL")
			state.aof.w.Flush()
		}
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(count)
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
func Hgetall(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hgetall' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.Hash)*2)
	for field, fieldItem := range item.Hash {
		// Skip expired fields
		if fieldItem.IsExpired() {
			continue
		}
		result = append(result, Value{typ: BULK, blk: field})
		result = append(result, Value{typ: BULK, blk: fieldItem.Str})
	}

	return NewArrayValue(result)
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
func Hdelall(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hdelall' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Calculate old memory before clearing
	oldMemory := item.approxMemoryUsage(key)

	count := int64(len(item.Hash))
	item.Hash = make(map[string]*Item) // Clear the hash

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(count)
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
func Hincrby(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 3 {
		return NewErrorValue("ERR wrong number of arguments for 'hincrby' command")
	}

	key := args[0].blk
	field := args[1].blk
	incrStr := args[2].blk

	incr, err := ParseInt(incrStr)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}

	DB.mu.Lock()
	defer DB.mu.Unlock()

	var item *Item
	var oldMemory int64 = 0
	if existing, ok := DB.store[key]; ok {
		item = existing
		oldMemory = existing.approxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return NewErrorValue(err.Error())
		}
	} else {
		item = NewHashItem()
		DB.store[key] = item
	}

	var fieldItem *Item
	if existing, ok := item.Hash[field]; ok {
		fieldItem = existing
	} else {
		fieldItem = NewHashFieldItem("0")
		item.Hash[field] = fieldItem
	}

	// Check if field is expired
	if fieldItem.IsExpired() {
		fieldItem = NewHashFieldItem("0")
		item.Hash[field] = fieldItem
	}

	current := int64(0)
	if fieldItem.Str != "" {
		current, err = ParseInt(fieldItem.Str)
		if err != nil {
			return NewErrorValue("ERR hash value is not an integer")
		}
	}

	newVal := current + incr
	fieldItem.Str = fmt.Sprintf("%d", newVal)

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewIntegerValue(newVal)
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
func Hmset(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 3 || len(args)%2 == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'hmset' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	// Calculate old memory before modification
	var oldMemory int64 = 0
	var item *Item
	if existing, ok := DB.store[key]; ok {
		item = existing
		oldMemory = existing.approxMemoryUsage(key)
		if err := item.EnsureHash(); err != nil {
			return NewErrorValue(err.Error())
		}
	} else {
		item = NewHashItem()
		DB.store[key] = item
	}

	for i := 1; i < len(args); i += 2 {
		field := args[i].blk
		value := args[i+1].blk
		item.Hash[field] = NewHashFieldItem(value)
	}

	// Calculate new memory and update DB.mem
	newMemory := item.approxMemoryUsage(key)
	DB.mem -= oldMemory
	DB.mem += newMemory
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}
	log.Printf("memory = %d\n", DB.mem)

	if state.config.aofEnabled {
		state.aof.w.Write(v)
	}
	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	return NewStringValue("OK")
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
func Hexists(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'hexists' command")
	}

	key := args[0].blk
	field := args[1].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	_, exists := item.Hash[field]
	if exists {
		return NewIntegerValue(1)
	}

	return NewIntegerValue(0)
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
func Hlen(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hlen' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return NewIntegerValue(int64(len(item.Hash)))
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
func Hkeys(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hkeys' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.Hash))
	for field, fieldItem := range item.Hash {
		// Skip expired fields
		if !fieldItem.IsExpired() {
			result = append(result, Value{typ: BULK, blk: field})
		}
	}

	return NewArrayValue(result)
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
func Hvals(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'hvals' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]Value, 0, len(item.Hash))
	for _, fieldItem := range item.Hash {
		// Skip expired fields
		if !fieldItem.IsExpired() {
			result = append(result, Value{typ: BULK, blk: fieldItem.Str})
		}
	}

	return NewArrayValue(result)
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
func Hexpire(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 3 {
		return NewErrorValue("ERR wrong number of arguments for 'hexpire' command")
	}

	key := args[0].blk
	field := args[1].blk
	secondsStr := args[2].blk

	seconds, err := strconv.Atoi(secondsStr)
	if err != nil {
		return NewErrorValue("ERR invalid expiration time")
	}

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if !item.IsHash() {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	fieldItem, exists := item.Hash[field]
	if !exists {
		return NewIntegerValue(0)
	}

	// Set expiration on the field
	fieldItem.Exp = time.Now().Add(time.Second * time.Duration(seconds))

	return NewIntegerValue(1)
}
