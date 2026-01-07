/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_zset.go
*/
package main

import (
	"sort"
	"strconv"
	"strings"
)

type zsetPair struct {
	member string
	score  float64
}

// Zadd handles the ZADD command.
// Adds all the specified members with the specified scores to the sorted set stored at key.
//
// Syntax:
//
//	ZADD <key> <score> <member> [<score> <member> ...]
//
// Returns:
//
//	Integer: The number of elements added to the sorted sets, not including elements already existing for which the score was updated.
func Zadd(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 3 || len(args)%2 == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'zadd' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	var item *Item
	var oldMemory int64 = 0

	if existing, ok := DB.store[key]; ok {
		item = existing
		if item.Type != ZSET_TYPE {
			return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.approxMemoryUsage(key)
	} else {
		item = &Item{
			Type: ZSET_TYPE,
			ZSet: make(map[string]float64),
		}
		DB.store[key] = item
	}

	addedCount := int64(0)
	for i := 1; i < len(args); i += 2 {
		scoreStr := args[i].blk
		member := args[i+1].blk
		score, err := strconv.ParseFloat(scoreStr, 64)
		if err != nil {
			return NewErrorValue("ERR value is not a valid float")
		}

		if _, exists := item.ZSet[member]; !exists {
			addedCount++
		}
		item.ZSet[member] = score
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

	return NewIntegerValue(addedCount)
}

// Zrem handles the ZREM command.
// Removes the specified members from the sorted set stored at key.
//
// Syntax:
//
//	ZREM <key> <member> [<member> ...]
//
// Returns:
//
//	Integer: The number of members removed from the sorted set, not including non existing members.
func Zrem(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 2 {
		return NewErrorValue("ERR wrong number of arguments for 'zrem' command")
	}

	key := args[0].blk

	DB.mu.Lock()
	defer DB.mu.Unlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != ZSET_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	oldMemory := item.approxMemoryUsage(key)
	removedCount := int64(0)

	for _, arg := range args[1:] {
		member := arg.blk
		if _, exists := item.ZSet[member]; exists {
			delete(item.ZSet, member)
			removedCount++
		}
	}

	if len(item.ZSet) == 0 {
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

	return NewIntegerValue(removedCount)
}

// Zscore handles the ZSCORE command.
// Returns the score of member in the sorted set at key.
//
// Syntax:
//
//	ZSCORE <key> <member>
//
// Returns:
//
//	Bulk String: The score of member (a double precision floating point number), represented as string.
func Zscore(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'zscore' command")
	}

	key := args[0].blk
	member := args[1].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewNullValue()
	}

	if item.Type != ZSET_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	score, exists := item.ZSet[member]
	if !exists {
		return NewNullValue()
	}

	return NewBulkValue(strconv.FormatFloat(score, 'f', -1, 64))
}

// Zcard handles the ZCARD command.
// Returns the sorted set cardinality (number of elements) of the sorted set stored at key.
//
// Syntax:
//
//	ZCARD <key>
//
// Returns:
//
//	Integer: The cardinality (number of elements) of the sorted set, or 0 if key does not exist.
func Zcard(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue("ERR wrong number of arguments for 'zcard' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewIntegerValue(0)
	}

	if item.Type != ZSET_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return NewIntegerValue(int64(len(item.ZSet)))
}

func zrangeGeneric(c *Client, v *Value, reverse bool) *Value {
	args := v.arr[1:]
	if len(args) < 3 {
		return NewErrorValue("ERR wrong number of arguments for command")
	}

	key := args[0].blk
	startStr := args[1].blk
	stopStr := args[2].blk

	withScores := false
	if len(args) > 3 {
		if strings.ToUpper(args[3].blk) == "WITHSCORES" {
			withScores = true
		} else {
			return NewErrorValue("ERR syntax error")
		}
	}

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}
	stop, err := strconv.Atoi(stopStr)
	if err != nil {
		return NewErrorValue("ERR value is not an integer or out of range")
	}

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if item.Type != ZSET_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Extract and sort
	pairs := make([]zsetPair, 0, len(item.ZSet))
	for m, s := range item.ZSet {
		pairs = append(pairs, zsetPair{m, s})
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score == pairs[j].score {
			if reverse {
				return pairs[i].member > pairs[j].member
			}
			return pairs[i].member < pairs[j].member
		}
		if reverse {
			return pairs[i].score > pairs[j].score
		}
		return pairs[i].score < pairs[j].score
	})

	// Adjust indices
	l := len(pairs)
	if start < 0 {
		start = l + start
	}
	if stop < 0 {
		stop = l + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= l {
		stop = l - 1
	}

	if start > stop {
		return NewArrayValue([]Value{})
	}

	result := make([]Value, 0, (stop-start+1)*2)
	for i := start; i <= stop; i++ {
		result = append(result, Value{typ: BULK, blk: pairs[i].member})
		if withScores {
			result = append(result, Value{typ: BULK, blk: strconv.FormatFloat(pairs[i].score, 'f', -1, 64)})
		}
	}

	return NewArrayValue(result)
}

// Zrange handles the ZRANGE command.
// Returns the specified range of elements in the sorted set stored at key.
//
// Syntax:
//
//	ZRANGE <key> <start> <stop> [WITHSCORES]
func Zrange(c *Client, v *Value, state *AppState) *Value {
	return zrangeGeneric(c, v, false)
}

// Zrevrange handles the ZREVRANGE command.
// Returns the specified range of elements in the sorted set stored at key, with scores ordered from high to low.
//
// Syntax:
//
//	ZREVRANGE <key> <start> <stop> [WITHSCORES]
func Zrevrange(c *Client, v *Value, state *AppState) *Value {
	return zrangeGeneric(c, v, true)
}

// Zget handles the ZGET command.
// Custom command to get score of a member or all members with scores.
//
// Syntax:
//
//	ZGET <key> [<member>]
//
// Returns:
//
//	Array: [score] if member specified
//	Array: [member1, score1, member2, score2, ...] if no member specified
func Zget(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 1 || len(args) > 2 {
		return NewErrorValue("ERR wrong number of arguments for 'zget' command")
	}

	key := args[0].blk

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	item, ok := DB.store[key]
	if !ok {
		return NewArrayValue([]Value{})
	}

	if item.Type != ZSET_TYPE {
		return NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// ZGET <key> <member>
	if len(args) == 2 {
		member := args[1].blk
		score, exists := item.ZSet[member]
		if !exists {
			return NewNullValue()
		}
		return &Value{
			typ: BULK,
			blk: strconv.FormatFloat(score, 'f', -1, 64),
		}
	}

	// ZGET <key>
	pairs := make([]zsetPair, 0, len(item.ZSet))
	for m, s := range item.ZSet {
		pairs = append(pairs, zsetPair{m, s})
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score == pairs[j].score {
			return pairs[i].member < pairs[j].member
		}
		return pairs[i].score < pairs[j].score
	})

	result := make([]Value, 0, len(pairs)*2)
	for _, p := range pairs {
		result = append(result, Value{typ: BULK, blk: p.member})
		result = append(result, Value{typ: BULK, blk: strconv.FormatFloat(p.score, 'f', -1, 64)})
	}

	return NewArrayValue(result)
}
