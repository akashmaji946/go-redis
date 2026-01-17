/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_zset.go
*/
package handlers

import (
	"sort"
	"strconv"
	"strings"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// ZSetHandlers is the map of sorted set command names to their handler functions.
var ZSetHandlers = map[string]common.Handler{
	"ZADD":      Zadd,
	"ZREM":      Zrem,
	"ZSCORE":    Zscore,
	"ZCARD":     Zcard,
	"ZRANGE":    Zrange,
	"ZREVRANGE": Zrevrange,
	"ZGET":      Zget,
}

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
func Zadd(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 || len(args)%2 == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zadd' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		if item.Type != common.ZSET_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.ApproxMemoryUsage(key)
	} else {
		item = &common.Item{
			Type: common.ZSET_TYPE,
			ZSet: make(map[string]float64),
		}
		database.DB.Store[key] = item
	}

	addedCount := int64(0)
	for i := 1; i < len(args); i += 2 {
		scoreStr := args[i].Blk
		member := args[i+1].Blk
		score, err := strconv.ParseFloat(scoreStr, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not a valid float")
		}

		if _, exists := item.ZSet[member]; !exists {
			addedCount++
		}
		item.ZSet[member] = score
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
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(addedCount)
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
func Zrem(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zrem' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	oldMemory := item.ApproxMemoryUsage(key)
	removedCount := int64(0)

	for _, arg := range args[1:] {
		member := arg.Blk
		if _, exists := item.ZSet[member]; exists {
			delete(item.ZSet, member)
			removedCount++
		}
	}

	database.DB.Touch(key)
	if len(item.ZSet) == 0 {
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
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(removedCount)
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
func Zscore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zscore' command")
	}

	key := args[0].Blk
	member := args[1].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewNullValue()
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	score, exists := item.ZSet[member]
	if !exists {
		return common.NewNullValue()
	}

	return common.NewBulkValue(strconv.FormatFloat(score, 'f', -1, 64))
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
func Zcard(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zcard' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	return common.NewIntegerValue(int64(len(item.ZSet)))
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
func Zget(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 || len(args) > 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zget' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// ZGET <key> <member>
	if len(args) == 2 {
		member := args[1].Blk
		score, exists := item.ZSet[member]
		if !exists {
			return common.NewNullValue()
		}
		return &common.Value{
			Typ: common.BULK,
			Blk: strconv.FormatFloat(score, 'f', -1, 64),
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

	result := make([]common.Value, 0, len(pairs)*2)
	for _, p := range pairs {
		result = append(result, common.Value{Typ: common.BULK, Blk: p.member})
		result = append(result, common.Value{Typ: common.BULK, Blk: strconv.FormatFloat(p.score, 'f', -1, 64)})
	}

	return common.NewArrayValue(result)
}

// Zrange handles the ZRANGE command.
// Returns the specified range of elements in the sorted set stored at key.
//
// Syntax:
// ZRANGE <key> <start> <stop> [WITHSCORES]
func Zrange(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return zrangeGeneric(c, v, false)
}

// Zrevrange handles the ZREVRANGE command.
// Returns the specified range of elements in the sorted set stored at key, with scores ordered from high to low.
//
// Syntax:
// ZREVRANGE <key> <start> <stop> [WITHSCORES]
func Zrevrange(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return zrangeGeneric(c, v, true)
}

// zrangeGeneric is a helper function for ZRANGE and ZREVRANGE commands.
func zrangeGeneric(c *common.Client, v *common.Value, reverse bool) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 {
		return common.NewErrorValue("ERR wrong number of arguments for command")
	}

	key := args[0].Blk
	startStr := args[1].Blk
	stopStr := args[2].Blk

	withScores := false
	if len(args) > 3 {
		if strings.ToUpper(args[3].Blk) == "WITHSCORES" {
			withScores = true
		} else {
			return common.NewErrorValue("ERR syntax error")
		}
	}

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}
	stop, err := strconv.Atoi(stopStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
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
		return common.NewArrayValue([]common.Value{})
	}

	result := make([]common.Value, 0, (stop-start+1)*2)
	for i := start; i <= stop; i++ {
		result = append(result, common.Value{Typ: common.BULK, Blk: pairs[i].member})
		if withScores {
			result = append(result, common.Value{Typ: common.BULK, Blk: strconv.FormatFloat(pairs[i].score, 'f', -1, 64)})
		}
	}

	return common.NewArrayValue(result)
}
