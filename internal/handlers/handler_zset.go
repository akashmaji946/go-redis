/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_zset.go
*/
package handlers

import (
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// ZSetHandlers is the map of sorted set command names to their handler functions.
var ZSetHandlers = map[string]common.Handler{
	"ZADD":             Zadd,
	"ZREM":             Zrem,
	"ZSCORE":           Zscore,
	"ZCARD":            Zcard,
	"ZRANGE":           Zrange,
	"ZREVRANGE":        Zrevrange,
	"ZGET":             Zget,
	"ZINCRBY":          Zincrby,
	"ZRANK":            Zrank,
	"ZREVRANK":         Zrevrank,
	"ZCOUNT":           Zcount,
	"ZLEXCOUNT":        Zlexcount,
	"ZRANGEBYSCORE":    Zrangebyscore,
	"ZREVRANGEBYSCORE": Zrevrangebyscore,
	"ZRANGEBYLEX":      Zrangebylex,
	"ZREMRANGEBYRANK":  Zremrangebyrank,
	"ZREMRANGEBYSCORE": Zremrangebyscore,
	"ZREMRANGEBYLEX":   Zremrangebylex,
	"ZPOPMIN":          Zpopmin,
	"ZPOPMAX":          Zpopmax,
	"BZPOPMIN":         Bzpopmin,
	"BZPOPMAX":         Bzpopmax,
	"ZINTERSTORE":      Zinterstore,
	"ZUNIONSTORE":      Zunionstore,
	"ZMSCORE":          Zmscore,
	"ZSCAN":            Zscan,
	"ZRANDMEMBER":      Zrandmember,
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

// Zincrby handles the ZINCRBY command.
// Increments the score of member in the sorted set stored at key by increment.
//
// Syntax:
//
//	ZINCRBY <key> <increment> <member>
//
// Returns:
//
//	Bulk String: The new score of member.
func Zincrby(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zincrby' command")
	}

	key := args[0].Blk
	incrStr := args[1].Blk
	member := args[2].Blk

	incr, err := strconv.ParseFloat(incrStr, 64)
	if err != nil {
		return common.NewErrorValue("ERR value is not a valid float")
	}

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

	newScore := incr
	if existingScore, exists := item.ZSet[member]; exists {
		newScore += existingScore
	}
	item.ZSet[member] = newScore

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

	return common.NewBulkValue(strconv.FormatFloat(newScore, 'f', -1, 64))
}

// Zrank handles the ZRANK command.
// Returns the rank of member in the sorted set stored at key, with the scores ordered from low to high.
//
// Syntax:
//
//	ZRANK <key> <member>
//
// Returns:
//
//	Integer: The rank of member.
func Zrank(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return zrankGeneric(c, v, false)
}

// Zrevrank handles the ZREVRANK command.
// Returns the rank of member in the sorted set stored at key, with the scores ordered from high to low.
//
// Syntax:
//
//	ZREVRANK <key> <member>
//
// Returns:
//
//	Integer: The rank of member.
func Zrevrank(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return zrankGeneric(c, v, true)
}

// zrankGeneric is a helper function for ZRANK and ZREVRANK commands.
func zrankGeneric(c *common.Client, v *common.Value, reverse bool) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for command")
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

	if _, exists := item.ZSet[member]; !exists {
		return common.NewNullValue()
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

	for i, p := range pairs {
		if p.member == member {
			return common.NewIntegerValue(int64(i))
		}
	}

	return common.NewNullValue() // Should not reach here
}

// Zcount handles the ZCOUNT command.
// Returns the number of elements in the sorted set at key with a score between min and max.
//
// Syntax:
//
//	ZCOUNT <key> <min> <max>
//
// Returns:
//
//	Integer: The number of elements in the specified score range.
func Zcount(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zcount' command")
	}

	key := args[0].Blk
	minStr := args[1].Blk
	maxStr := args[2].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	count := int64(0)
	for _, score := range item.ZSet {
		minOk := checkScoreBound(score, minStr, true)
		maxOk := checkScoreBound(score, maxStr, false)
		if minOk && maxOk {
			count++
		}
	}

	return common.NewIntegerValue(count)
}

// parseScoreRange parses min and max score strings, handling inclusive/exclusive.
func parseScoreRange(minStr, maxStr string) (float64, float64, error) {
	min, err := parseScoreBound(minStr)
	if err != nil {
		return 0, 0, err
	}
	max, err := parseScoreBound(maxStr)
	if err != nil {
		return 0, 0, err
	}
	return min, max, nil
}

// parseScoreBound parses a score bound, handling +inf, -inf, and exclusive.
func parseScoreBound(s string) (float64, error) {
	if s == "+inf" {
		return math.Inf(1), nil
	}
	if s == "-inf" {
		return math.Inf(-1), nil
	}
	if strings.HasPrefix(s, "(") {
		val, err := strconv.ParseFloat(s[1:], 64)
		if err != nil {
			return 0, err
		}
		return val, nil // For exclusive, but since float, we handle in comparison
	}
	return strconv.ParseFloat(s, 64)
}

// checkScoreBound checks if score satisfies the bound.
func checkScoreBound(score float64, bound string, isMin bool) bool {
	if bound == "+inf" {
		return !isMin // for max, always true; for min, false
	}
	if bound == "-inf" {
		return isMin // for min, always true; for max, false
	}
	exclusive := strings.HasPrefix(bound, "(")
	valStr := bound
	if exclusive {
		valStr = bound[1:]
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return false
	}
	if isMin {
		if exclusive {
			return score > val
		}
		return score >= val
	} else {
		if exclusive {
			return score < val
		}
		return score <= val
	}
}

// Zlexcount handles the ZLEXCOUNT command.
// Returns the number of elements in the sorted set with a value between min and max.
//
// Syntax:
//
//	ZLEXCOUNT <key> <min> <max>
//
// Returns:
//
//	Integer: The number of elements in the specified score range.
func Zlexcount(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zlexcount' command")
	}

	key := args[0].Blk
	minStr := args[1].Blk
	maxStr := args[2].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Check if all scores are equal
	scoreSet := make(map[float64]bool)
	for _, s := range item.ZSet {
		scoreSet[s] = true
	}
	if len(scoreSet) != 1 {
		return common.NewErrorValue("ERR ZLEXCOUNT can only be used when all elements have the same score")
	}

	count := int64(0)
	for member := range item.ZSet {
		if checkLexBound(member, minStr, true) && checkLexBound(member, maxStr, false) {
			count++
		}
	}

	return common.NewIntegerValue(count)
}

// Zrangebyscore handles the ZRANGEBYSCORE command.
// Returns all the elements in the sorted set at key with a score between min and max.
//
// Syntax:
//
//	ZRANGEBYSCORE <key> <min> <max>
//
// Returns:
//
//	Array: List of elements in the specified score range.
func Zrangebyscore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return zrangebyscoreGeneric(c, v, false)
}

// Zrevrangebyscore handles the ZREVRANGEBYSCORE command.
// Returns all the elements in the sorted set at key with a score between max and min.
//
// Syntax:
//
//	ZREVRANGEBYSCORE <key> <max> <min>
//
// Returns:
//
//	Array: List of elements in the specified score range.
func Zrevrangebyscore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return zrangebyscoreGeneric(c, v, true)
}

// zrangebyscoreGeneric is a helper function for ZRANGEBYSCORE and ZREVRANGEBYSCORE.
func zrangebyscoreGeneric(c *common.Client, v *common.Value, reverse bool) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 {
		return common.NewErrorValue("ERR wrong number of arguments for command")
	}

	key := args[0].Blk
	minStr := args[1].Blk
	maxStr := args[2].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Extract and filter
	pairs := make([]zsetPair, 0)
	for m, s := range item.ZSet {
		minOk := checkScoreBound(s, minStr, true)
		maxOk := checkScoreBound(s, maxStr, false)
		if minOk && maxOk {
			pairs = append(pairs, zsetPair{m, s})
		}
	}

	// Sort
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

	result := make([]common.Value, 0, len(pairs))
	for _, p := range pairs {
		result = append(result, common.Value{Typ: common.BULK, Blk: p.member})
	}

	return common.NewArrayValue(result)
}

// Zrangebylex handles the ZRANGEBYLEX command.
// Returns the elements in the sorted set with a value between min and max.
//
// Syntax:
//
//	ZRANGEBYLEX <key> <min> <max>
//
// Returns:
//
//	Array: List of elements in the specified lexical range.
func Zrangebylex(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zrangebylex' command")
	}

	key := args[0].Blk
	minStr := args[1].Blk
	maxStr := args[2].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Check if all scores are equal
	scoreSet := make(map[float64]bool)
	for _, s := range item.ZSet {
		scoreSet[s] = true
	}
	if len(scoreSet) != 1 {
		return common.NewErrorValue("ERR ZRANGEBYLEX can only be used when all elements have the same score")
	}

	// Collect members in lex order
	members := make([]string, 0, len(item.ZSet))
	for m := range item.ZSet {
		members = append(members, m)
	}
	sort.Strings(members)

	result := make([]common.Value, 0)
	for _, m := range members {
		if checkLexBound(m, minStr, true) && checkLexBound(m, maxStr, false) {
			result = append(result, common.Value{Typ: common.BULK, Blk: m})
		}
	}

	return common.NewArrayValue(result)
}

// checkLexBound checks if member satisfies the lexical bound.
func checkLexBound(member, bound string, isMin bool) bool {
	if bound == "+" {
		return !isMin
	}
	if bound == "-" {
		return isMin
	}
	exclusive := strings.HasPrefix(bound, "(")
	val := bound
	if exclusive {
		val = bound[1:]
	}
	if isMin {
		if exclusive {
			return member > val
		}
		return member >= val
	} else {
		if exclusive {
			return member < val
		}
		return member <= val
	}
}

// Zremrangebyrank handles the ZREMRANGEBYRANK command.
// Removes all elements in the sorted set stored at key with rank between start and stop.
//
// Syntax:
//
//	ZREMRANGEBYRANK <key> <start> <stop>
//
// Returns:
//
//	Integer: The number of elements removed.
func Zremrangebyrank(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zremrangebyrank' command")
	}

	key := args[0].Blk
	startStr := args[1].Blk
	stopStr := args[2].Blk

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}
	stop, err := strconv.Atoi(stopStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}

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

	// Extract and sort
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
		return common.NewIntegerValue(0)
	}

	removed := int64(0)
	for i := start; i <= stop; i++ {
		delete(item.ZSet, pairs[i].member)
		removed++
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

	return common.NewIntegerValue(removed)
}

// Zremrangebyscore handles the ZREMRANGEBYSCORE command.
// Removes all elements in the sorted set stored at key with score between min and max.
//
// Syntax:
//
//	ZREMRANGEBYSCORE <key> <min> <max>
//
// Returns:
//
//	Integer: The number of elements removed.
func Zremrangebyscore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zremrangebyscore' command")
	}

	key := args[0].Blk
	minStr := args[1].Blk
	maxStr := args[2].Blk

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
	removed := int64(0)

	for m, s := range item.ZSet {
		minOk := checkScoreBound(s, minStr, true)
		maxOk := checkScoreBound(s, maxStr, false)
		if minOk && maxOk {
			delete(item.ZSet, m)
			removed++
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

	return common.NewIntegerValue(removed)
}

// Zremrangebylex handles the ZREMRANGEBYLEX command.
// Removes all elements in the sorted set stored at key between the lexical range specified by min and max.
//
// Syntax:
//
//	ZREMRANGEBYLEX <key> <min> <max>
//
// Returns:
//
//	Integer: The number of elements removed.
func Zremrangebylex(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zremrangebylex' command")
	}

	key := args[0].Blk
	minStr := args[1].Blk
	maxStr := args[2].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Check if all scores are equal
	scoreSet := make(map[float64]bool)
	for _, s := range item.ZSet {
		scoreSet[s] = true
	}
	if len(scoreSet) != 1 {
		return common.NewErrorValue("ERR ZREMRANGEBYLEX can only be used when all elements have the same score")
	}

	oldMemory := item.ApproxMemoryUsage(key)
	removed := int64(0)

	for m := range item.ZSet {
		if checkLexBound(m, minStr, true) && checkLexBound(m, maxStr, false) {
			delete(item.ZSet, m)
			removed++
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

	return common.NewIntegerValue(removed)
}

// Zpopmin handles the ZPOPMIN command.
// Removes and returns up to count members with the lowest scores in the sorted set stored at key.
//
// Syntax:
//
//	ZPOPMIN <key> [count]
//
// Returns:
//
//	Array: The removed members and their scores.
func Zpopmin(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return zpopGeneric(c, v, state, false)
}

// Zpopmax handles the ZPOPMAX command.
// Removes and returns up to count members with the highest scores in the sorted set stored at key.
//
// Syntax:
//
//	ZPOPMAX <key> [count]
//
// Returns:
//
//	Array: The removed members and their scores.
func Zpopmax(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return zpopGeneric(c, v, state, true)
}

// zpopGeneric is a helper function for ZPOPMIN and ZPOPMAX.
func zpopGeneric(c *common.Client, v *common.Value, state *common.AppState, reverse bool) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 || len(args) > 2 {
		return common.NewErrorValue("ERR wrong number of arguments for command")
	}

	key := args[0].Blk
	count := 1
	if len(args) == 2 {
		c, err := strconv.Atoi(args[1].Blk)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
		count = c
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewArrayValue([]common.Value{})
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	oldMemory := item.ApproxMemoryUsage(key)

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

	// Pop up to count
	popCount := count
	if popCount > len(pairs) {
		popCount = len(pairs)
	}

	result := make([]common.Value, 0, popCount*2)
	for i := 0; i < popCount; i++ {
		result = append(result, common.Value{Typ: common.BULK, Blk: pairs[i].member})
		result = append(result, common.Value{Typ: common.BULK, Blk: strconv.FormatFloat(pairs[i].score, 'f', -1, 64)})
		delete(item.ZSet, pairs[i].member)
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

	return common.NewArrayValue(result)
}

// Bzpopmin handles the BZPOPMIN command.
// Blocking version of ZPOPMIN. Since this is a simple implementation, it's non-blocking.
//
// Syntax:
//
//	BZPOPMIN <key> [key ...] <timeout>
//
// Returns:
//
//	Array: [key, member, score] or nil if timeout.
func Bzpopmin(c *common.Client, v *common.Value, _ *common.AppState) *common.Value {
	// For simplicity, implement as non-blocking
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'bzpopmin' command")
	}

	keys := args[:len(args)-1]
	timeoutStr := args[len(args)-1]

	// Ignore timeout for now
	_, err := strconv.ParseFloat(timeoutStr.Blk, 64)
	if err != nil {
		return common.NewErrorValue("ERR timeout is not a float or out of range")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	for _, keyVal := range keys {
		key := keyVal.Blk
		item, ok := database.DB.Store[key]
		if !ok {
			continue
		}
		if item.Type != common.ZSET_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		if len(item.ZSet) == 0 {
			continue
		}

		// Find min
		var minMember string
		minScore := math.Inf(1)
		for m, s := range item.ZSet {
			if s < minScore || (s == minScore && m < minMember) {
				minScore = s
				minMember = m
			}
		}

		delete(item.ZSet, minMember)

		database.DB.Touch(key)
		if len(item.ZSet) == 0 {
			delete(database.DB.Store, key)
		}

		// AOF and RDB for non-blocking version
		// Since it's non-blocking, perhaps no need, but for consistency
		// if state.Config.AofEnabled {
		//     state.Aof.W.Write(v)
		//     if state.Config.AofFsync == common.Always {
		//         state.Aof.W.Flush()
		//     }
		// }
		// if len(state.Config.Rdb) > 0 {
		//     database.DB.IncrTrackers()
		// }

		result := []common.Value{
			{Typ: common.BULK, Blk: key},
			{Typ: common.BULK, Blk: minMember},
			{Typ: common.BULK, Blk: strconv.FormatFloat(minScore, 'f', -1, 64)},
		}
		return common.NewArrayValue(result)
	}

	return common.NewNullValue()
}

// Bzpopmax handles the BZPOPMAX command.
// Blocking version of ZPOPMAX. Since this is a simple implementation, it's non-blocking.
//
// Syntax:
//
//	BZPOPMAX <key> [key ...] <timeout>
//
// Returns:
//
//	Array: [key, member, score] or nil if timeout.
func Bzpopmax(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// For simplicity, implement as non-blocking
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'bzpopmax' command")
	}

	keys := args[:len(args)-1]
	timeoutStr := args[len(args)-1]

	// Ignore timeout for now
	_, err := strconv.ParseFloat(timeoutStr.Blk, 64)
	if err != nil {
		return common.NewErrorValue("ERR timeout is not a float or out of range")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	for _, keyVal := range keys {
		key := keyVal.Blk
		item, ok := database.DB.Store[key]
		if !ok {
			continue
		}
		if item.Type != common.ZSET_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		if len(item.ZSet) == 0 {
			continue
		}

		// Find max
		var maxMember string
		maxScore := math.Inf(-1)
		for m, s := range item.ZSet {
			if s > maxScore || (s == maxScore && m > maxMember) {
				maxScore = s
				maxMember = m
			}
		}

		delete(item.ZSet, maxMember)

		database.DB.Touch(key)
		if len(item.ZSet) == 0 {
			delete(database.DB.Store, key)
		}

		// AOF and RDB for non-blocking version
		// if state.Config.AofEnabled {
		//     state.Aof.W.Write(v)
		//     if state.Config.AofFsync == common.Always {
		//         state.Aof.W.Flush()
		//     }
		// }
		// if len(state.Config.Rdb) > 0 {
		//     database.DB.IncrTrackers()
		// }

		result := []common.Value{
			{Typ: common.BULK, Blk: key},
			{Typ: common.BULK, Blk: maxMember},
			{Typ: common.BULK, Blk: strconv.FormatFloat(maxScore, 'f', -1, 64)},
		}
		return common.NewArrayValue(result)
	}

	return common.NewNullValue()
}

// Zmscore handles the ZMSCORE command.
// Returns the scores associated with the specified members in the sorted set stored at key.
//
// Syntax:
//
//	ZMSCORE <key> <member> [member ...]
//
// Returns:
//
//	Array: The scores of the members.
func Zmscore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zmscore' command")
	}

	key := args[0].Blk
	members := args[1:]

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		result := make([]common.Value, len(members))
		for i := range result {
			result[i] = common.Value{Typ: common.NULL}
		}
		return common.NewArrayValue(result)
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, len(members))
	for i, m := range members {
		score, exists := item.ZSet[m.Blk]
		if exists {
			result[i] = common.Value{Typ: common.BULK, Blk: strconv.FormatFloat(score, 'f', -1, 64)}
		} else {
			result[i] = common.Value{Typ: common.NULL}
		}
	}

	return common.NewArrayValue(result)
}

// Zrandmember handles the ZRANDMEMBER command.
// Returns a random element from the sorted set stored at key.
//
// Syntax:
//
//	ZRANDMEMBER <key> [count]
//
// Returns:
//
//	Bulk String or Array: Random member(s).
func Zrandmember(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 || len(args) > 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zrandmember' command")
	}

	key := args[0].Blk
	count := 1
	if len(args) == 2 {
		c, err := strconv.Atoi(args[1].Blk)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
		count = c
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		if count > 0 {
			return common.NewArrayValue([]common.Value{})
		}
		return common.NewNullValue()
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if len(item.ZSet) == 0 {
		if count > 0 {
			return common.NewArrayValue([]common.Value{})
		}
		return common.NewNullValue()
	}

	members := make([]string, 0, len(item.ZSet))
	for m := range item.ZSet {
		members = append(members, m)
	}

	if count > 0 {
		if count > len(members) {
			count = len(members)
		}
		rand.Shuffle(len(members), func(i, j int) {
			members[i], members[j] = members[j], members[i]
		})
		result := make([]common.Value, count)
		for i := 0; i < count; i++ {
			result[i] = common.Value{Typ: common.BULK, Blk: members[i]}
		}
		return common.NewArrayValue(result)
	} else {
		// Allow duplicates
		count = -count
		result := make([]common.Value, count)
		for i := 0; i < count; i++ {
			idx := rand.Intn(len(members))
			result[i] = common.Value{Typ: common.BULK, Blk: members[idx]}
		}
		return common.NewArrayValue(result)
	}
}

// Zscan handles the ZSCAN command.
// Iterates over the elements of the sorted set.
//
// Syntax:
//
//	ZSCAN <key> <cursor> [MATCH pattern] [COUNT count]
//
// Returns:
//
//	Array: [cursor, [member1, score1, ...]]
func Zscan(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zscan' command")
	}

	key := args[0].Blk
	cursorStr := args[1].Blk

	cursor, err := strconv.Atoi(cursorStr)
	if err != nil {
		return common.NewErrorValue("ERR invalid cursor")
	}

	// Parse options
	match := ""
	count := 10
	i := 2
	for i < len(args) {
		if strings.ToUpper(args[i].Blk) == "MATCH" && i+1 < len(args) {
			match = args[i+1].Blk
			i += 2
		} else if strings.ToUpper(args[i].Blk) == "COUNT" && i+1 < len(args) {
			c, err := strconv.Atoi(args[i+1].Blk)
			if err != nil {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}
			count = c
			i += 2
		} else {
			return common.NewErrorValue("ERR syntax error")
		}
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return &common.Value{
			Typ: common.ARRAY,
			Arr: []common.Value{
				{Typ: common.BULK, Blk: "0"},
				{Typ: common.ARRAY, Arr: []common.Value{}},
			},
		}
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Get all pairs
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

	// Start from cursor
	start := cursor
	if start >= len(pairs) {
		return &common.Value{
			Typ: common.ARRAY,
			Arr: []common.Value{
				{Typ: common.BULK, Blk: "0"},
				{Typ: common.ARRAY, Arr: []common.Value{}},
			},
		}
	}

	end := start + count
	if end > len(pairs) {
		end = len(pairs)
	}

	result := []common.Value{}
	for i := start; i < end; i++ {
		if match != "" {
			// Simple match, no glob for now
			if !strings.Contains(pairs[i].member, match) {
				continue
			}
		}
		result = append(result, common.Value{Typ: common.BULK, Blk: pairs[i].member})
		result = append(result, common.Value{Typ: common.BULK, Blk: strconv.FormatFloat(pairs[i].score, 'f', -1, 64)})
	}

	nextCursor := "0"
	if end < len(pairs) {
		nextCursor = strconv.Itoa(end)
	}

	return &common.Value{
		Typ: common.ARRAY,
		Arr: []common.Value{
			{Typ: common.BULK, Blk: nextCursor},
			{Typ: common.ARRAY, Arr: result},
		},
	}
}

// Zinterstore handles the ZINTERSTORE command.
// Computes the intersection of the specified sorted sets and stores the result in destination.
//
// Syntax:
//
//	ZINTERSTORE <destination> <numkeys> <key> [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
//
// Returns:
//
//	Integer: The number of elements in the resulting sorted set.
func Zinterstore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zinterstore' command")
	}

	dest := args[0].Blk
	numkeysStr := args[1].Blk

	numkeys, err := strconv.Atoi(numkeysStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}
	if numkeys < 1 {
		return common.NewErrorValue("ERR at least 1 input key is needed for ZINTERSTORE")
	}

	// Parse keys
	if len(args) < 2+numkeys {
		return common.NewErrorValue("ERR wrong number of keys")
	}
	keys := args[2 : 2+numkeys]
	remaining := args[2+numkeys:]

	// Parse WEIGHTS
	weights := make([]float64, numkeys)
	for i := range weights {
		weights[i] = 1.0 // default
	}
	aggregate := "SUM" // default

	i := 0
	for i < len(remaining) {
		if strings.ToUpper(remaining[i].Blk) == "WEIGHTS" {
			if i+numkeys >= len(remaining) {
				return common.NewErrorValue("ERR syntax error")
			}
			for j := 0; j < numkeys; j++ {
				w, err := strconv.ParseFloat(remaining[i+1+j].Blk, 64)
				if err != nil {
					return common.NewErrorValue("ERR weight value is not a float")
				}
				weights[j] = w
			}
			i += 1 + numkeys
		} else if strings.ToUpper(remaining[i].Blk) == "AGGREGATE" {
			if i+1 >= len(remaining) {
				return common.NewErrorValue("ERR syntax error")
			}
			agg := strings.ToUpper(remaining[i+1].Blk)
			if agg != "SUM" && agg != "MIN" && agg != "MAX" {
				return common.NewErrorValue("ERR aggregate value must be SUM, MIN or MAX")
			}
			aggregate = agg
			i += 2
		} else {
			return common.NewErrorValue("ERR syntax error")
		}
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Collect all zsets
	zsets := make([]map[string]float64, 0, numkeys)
	for _, keyVal := range keys {
		key := keyVal.Blk
		item, ok := database.DB.Store[key]
		if !ok || item.Type != common.ZSET_TYPE {
			zsets = append(zsets, make(map[string]float64))
		} else {
			zsets = append(zsets, item.ZSet)
		}
	}

	// Find intersection
	result := make(map[string]float64)
	for member := range zsets[0] {
		allPresent := true
		scores := make([]float64, 0, numkeys)
		for i, zset := range zsets {
			if s, exists := zset[member]; exists {
				scores = append(scores, weights[i]*s)
			} else {
				allPresent = false
				break
			}
		}
		if allPresent {
			var combined float64
			switch aggregate {
			case "SUM":
				for _, s := range scores {
					combined += s
				}
			case "MIN":
				combined = math.Inf(1)
				for _, s := range scores {
					if s < combined {
						combined = s
					}
				}
			case "MAX":
				combined = math.Inf(-1)
				for _, s := range scores {
					if s > combined {
						combined = s
					}
				}
			}
			result[member] = combined
		}
	}

	// Store result
	var destItem *common.Item
	if existing, ok := database.DB.Store[dest]; ok {
		destItem = existing
		if destItem.Type != common.ZSET_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		destItem.ZSet = result
	} else {
		destItem = &common.Item{
			Type: common.ZSET_TYPE,
			ZSet: result,
		}
		database.DB.Store[dest] = destItem
	}

	database.DB.Touch(dest)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(int64(len(result)))
}

// Zunionstore handles the ZUNIONSTORE command.
// Computes the union of the specified sorted sets and stores the result in destination.
//
// Syntax:
//
//	ZUNIONSTORE <destination> <numkeys> <key> [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
//
// Returns:
//
//	Integer: The number of elements in the resulting sorted set.
func Zunionstore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'zunionstore' command")
	}

	dest := args[0].Blk
	numkeysStr := args[1].Blk

	numkeys, err := strconv.Atoi(numkeysStr)
	if err != nil {
		return common.NewErrorValue("ERR value is not an integer or out of range")
	}
	if numkeys < 1 {
		return common.NewErrorValue("ERR at least 1 input key is needed for ZUNIONSTORE")
	}

	// Parse keys
	if len(args) < 2+numkeys {
		return common.NewErrorValue("ERR wrong number of keys")
	}
	keys := args[2 : 2+numkeys]
	remaining := args[2+numkeys:]

	// Parse WEIGHTS
	weights := make([]float64, numkeys)
	for i := range weights {
		weights[i] = 1.0 // default
	}
	aggregate := "SUM" // default

	i := 0
	for i < len(remaining) {
		if strings.ToUpper(remaining[i].Blk) == "WEIGHTS" {
			if i+numkeys >= len(remaining) {
				return common.NewErrorValue("ERR syntax error")
			}
			for j := 0; j < numkeys; j++ {
				w, err := strconv.ParseFloat(remaining[i+1+j].Blk, 64)
				if err != nil {
					return common.NewErrorValue("ERR weight value is not a float")
				}
				weights[j] = w
			}
			i += 1 + numkeys
		} else if strings.ToUpper(remaining[i].Blk) == "AGGREGATE" {
			if i+1 >= len(remaining) {
				return common.NewErrorValue("ERR syntax error")
			}
			agg := strings.ToUpper(remaining[i+1].Blk)
			if agg != "SUM" && agg != "MIN" && agg != "MAX" {
				return common.NewErrorValue("ERR aggregate value must be SUM, MIN or MAX")
			}
			aggregate = agg
			i += 2
		} else {
			return common.NewErrorValue("ERR syntax error")
		}
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Collect all zsets
	zsets := make([]map[string]float64, 0, numkeys)
	for _, keyVal := range keys {
		key := keyVal.Blk
		item, ok := database.DB.Store[key]
		if !ok || item.Type != common.ZSET_TYPE {
			zsets = append(zsets, make(map[string]float64))
		} else {
			zsets = append(zsets, item.ZSet)
		}
	}

	// Find union
	result := make(map[string]float64)
	memberSet := make(map[string]bool)
	for _, zset := range zsets {
		for member := range zset {
			memberSet[member] = true
		}
	}

	for member := range memberSet {
		scores := make([]float64, 0, numkeys)
		for i, zset := range zsets {
			if s, exists := zset[member]; exists {
				scores = append(scores, weights[i]*s)
			}
		}
		if len(scores) > 0 {
			var combined float64
			switch aggregate {
			case "SUM":
				for _, s := range scores {
					combined += s
				}
			case "MIN":
				combined = math.Inf(1)
				for _, s := range scores {
					if s < combined {
						combined = s
					}
				}
			case "MAX":
				combined = math.Inf(-1)
				for _, s := range scores {
					if s > combined {
						combined = s
					}
				}
			}
			result[member] = combined
		}
	}

	// Store result
	var destItem *common.Item
	if existing, ok := database.DB.Store[dest]; ok {
		destItem = existing
		if destItem.Type != common.ZSET_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		destItem.ZSet = result
	} else {
		destItem = &common.Item{
			Type: common.ZSET_TYPE,
			ZSet: result,
		}
		database.DB.Store[dest] = destItem
	}

	database.DB.Touch(dest)

	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}

	return common.NewIntegerValue(int64(len(result)))
}
