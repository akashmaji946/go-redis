/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_set.go
*/
package handlers

import (
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// SetHandlers is the map of set command names to their handler functions.
var SetHandlers = map[string]common.Handler{
	"SADD":        Sadd,
	"SREM":        Srem,
	"SMEMBERS":    Smembers,
	"SISMEMBER":   Sismember,
	"SCARD":       Scard,
	"SINTER":      Sinter,
	"SUNION":      Sunion,
	"SDIFF":       Sdiff,
	"SRANDMEMBER": Srandmember,
	"SPOP":        Spop,
	"SMOVE":       Smove,
	"SINTERSTORE": Sinterstore,
	"SUNIONSTORE": Sunionstore,
	"SDIFFSTORE":  Sdiffstore,
	"SMISMEMBER":  Smismember,
	"SSCAN":       Sscan,
}

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

	saveDBState(state, v)

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

	saveDBState(state, v)

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

// Sscan handles the SSCAN command.
// Iterates over the members of a set.
//
// Syntax:
//
//	SSCAN <key> <cursor> [MATCH pattern] [COUNT count]
//
// Returns:
//
//	Array: [new_cursor, [member1, member2, ...]]
func Sscan(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sscan' command")
	}

	key := args[0].Blk
	cursorStr := args[1].Blk
	cursor, err := strconv.Atoi(cursorStr)
	if err != nil || cursor < 0 {
		return common.NewErrorValue("ERR invalid cursor")
	}

	// Parse options
	match := ""
	count := 10
	i := 2
	for i < len(args) {
		opt := strings.ToUpper(args[i].Blk)
		switch opt {
		case "MATCH":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			match = args[i+1].Blk
			i += 2
		case "COUNT":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			c, err := strconv.Atoi(args[i+1].Blk)
			if err != nil || c <= 0 {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}
			count = c
			i += 2
		default:
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
				*common.NewBulkValue("0"),
				*common.NewArrayValue([]common.Value{}),
			},
		}
	}

	if item.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Collect all members and sort them for stable iteration
	members := make([]string, 0, len(item.ItemSet))
	for member := range item.ItemSet {
		members = append(members, member)
	}
	sort.Strings(members)

	var resultMembers []common.Value
	var nextCursor int

	if cursor >= len(members) {
		nextCursor = 0
	} else {
		end := cursor + count
		if end > len(members) {
			end = len(members)
		}

		for i := cursor; i < end; i++ {
			member := members[i]
			if match != "" {
				matched, _ := filepath.Match(match, member)
				if !matched {
					continue
				}
			}
			resultMembers = append(resultMembers, *common.NewBulkValue(member))
		}
		nextCursor = end
		if nextCursor >= len(members) {
			nextCursor = 0
		}
	}

	return &common.Value{
		Typ: common.ARRAY,
		Arr: []common.Value{
			*common.NewBulkValue(strconv.Itoa(nextCursor)),
			*common.NewArrayValue(resultMembers),
		},
	}
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

// Spop handles the SPOP command.
// Remove and return one or more random members from a set.
//
// Syntax:
//
//	SPOP <key> [count]
//
// Returns:
//
//	Bulk String: If no count is provided.
//	Array: If count is provided.
func Spop(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'spop' command")
	}

	key := args[0].Blk
	count := 1
	hasCount := false

	if len(args) > 1 {
		c, err := strconv.Atoi(args[1].Blk)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
		if c < 0 {
			return common.NewErrorValue("ERR value is out of range, must be positive")
		}
		count = c
		hasCount = true
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

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

	oldMemory := item.ApproxMemoryUsage(key)
	result := make([]common.Value, 0)

	// Collect members to remove
	membersToRemove := make([]string, 0)
	if count >= size {
		// Remove all
		for member := range item.ItemSet {
			membersToRemove = append(membersToRemove, member)
			result = append(result, common.Value{Typ: common.BULK, Blk: member})
		}
	} else {
		// Remove count random members
		members := make([]string, 0, size)
		for member := range item.ItemSet {
			members = append(members, member)
		}
		selected := make(map[int]bool)
		for len(membersToRemove) < count {
			idx := rand.Intn(size)
			if !selected[idx] {
				selected[idx] = true
				member := members[idx]
				membersToRemove = append(membersToRemove, member)
				result = append(result, common.Value{Typ: common.BULK, Blk: member})
			}
		}
	}

	// Remove from set
	for _, member := range membersToRemove {
		delete(item.ItemSet, member)
	}

	database.DB.Touch(key)
	if len(item.ItemSet) == 0 {
		delete(database.DB.Store, key)
		database.DB.Mem -= oldMemory
	} else {
		newMemory := item.ApproxMemoryUsage(key)
		database.DB.Mem += (newMemory - oldMemory)
	}

	saveDBState(state, v)

	if hasCount {
		return common.NewArrayValue(result)
	}
	if len(result) > 0 {
		return common.NewBulkValue(result[0].Blk)
	}
	return common.NewNullValue()
}

// Smove handles the SMOVE command.
// Move a member from one set to another.
//
// Syntax:
//
//	SMOVE <source> <destination> <member>
//
// Returns:
//
//	Integer: 1 if the element is moved. 0 if the element is not a member of source and no operation was performed.
func Smove(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'smove' command")
	}

	sourceKey := args[0].Blk
	destKey := args[1].Blk
	member := args[2].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	sourceItem, sourceOk := database.DB.Store[sourceKey]
	if sourceOk && sourceItem.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	destItem, destOk := database.DB.Store[destKey]
	if destOk && destItem.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if !sourceOk || !sourceItem.ItemSet[member] {
		return common.NewIntegerValue(0)
	}

	// Remove from source
	oldSourceMemory := sourceItem.ApproxMemoryUsage(sourceKey)
	delete(sourceItem.ItemSet, member)
	database.DB.Touch(sourceKey)
	if len(sourceItem.ItemSet) == 0 {
		delete(database.DB.Store, sourceKey)
		database.DB.Mem -= oldSourceMemory
	} else {
		newSourceMemory := sourceItem.ApproxMemoryUsage(sourceKey)
		database.DB.Mem += (newSourceMemory - oldSourceMemory)
	}

	// Add to destination
	var oldDestMemory int64 = 0
	if !destOk {
		destItem = &common.Item{
			Type:    "set",
			ItemSet: make(map[string]bool),
		}
		database.DB.Store[destKey] = destItem
	} else {
		oldDestMemory = destItem.ApproxMemoryUsage(destKey)
	}
	destItem.ItemSet[member] = true
	database.DB.Touch(destKey)
	newDestMemory := destItem.ApproxMemoryUsage(destKey)
	database.DB.Mem += (newDestMemory - oldDestMemory)

	saveDBState(state, v)

	return common.NewIntegerValue(1)
}

// Sinterstore handles the SINTERSTORE command.
// Store the members of the set resulting from the intersection of all the given sets.
//
// Syntax:
//
//	SINTERSTORE <destination> <key> [<key> ...]
//
// Returns:
//
//	Integer: The number of elements in the resulting set.
func Sinterstore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sinterstore' command")
	}

	destKey := args[0].Blk
	sourceKeys := args[1:]

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Check if destination exists and is not a set
	if existing, ok := database.DB.Store[destKey]; ok && existing.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Collect all source sets
	var sets []map[string]bool
	for _, arg := range sourceKeys {
		key := arg.Blk
		item, ok := database.DB.Store[key]
		if !ok {
			// If any key missing, intersection is empty
			sets = nil
			break
		}
		if item.Type != "set" {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		sets = append(sets, item.ItemSet)
	}

	var oldDestMemory int64 = 0
	destItem, destOk := database.DB.Store[destKey]
	if destOk {
		oldDestMemory = destItem.ApproxMemoryUsage(destKey)
	} else {
		destItem = &common.Item{
			Type:    "set",
			ItemSet: make(map[string]bool),
		}
		database.DB.Store[destKey] = destItem
	}

	// Clear destination set
	destItem.ItemSet = make(map[string]bool)

	if sets != nil && len(sets) > 0 {
		// Find smallest set
		minIndex := 0
		minLen := len(sets[0])
		for i, s := range sets {
			if len(s) < minLen {
				minLen = len(s)
				minIndex = i
			}
		}

		// Perform intersection
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
				destItem.ItemSet[member] = true
			}
		}
	}

	count := int64(len(destItem.ItemSet))
	database.DB.Touch(destKey)
	newDestMemory := destItem.ApproxMemoryUsage(destKey)
	database.DB.Mem += (newDestMemory - oldDestMemory)

	saveDBState(state, v)

	return common.NewIntegerValue(count)
}

// Sunionstore handles the SUNIONSTORE command.
// Store the members of the set resulting from the union of all the given sets.
//
// Syntax:
//
//	SUNIONSTORE <destination> <key> [<key> ...]
//
// Returns:
//
//	Integer: The number of elements in the resulting set.
func Sunionstore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sunionstore' command")
	}

	destKey := args[0].Blk
	sourceKeys := args[1:]

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Check if destination exists and is not a set
	if existing, ok := database.DB.Store[destKey]; ok && existing.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	var oldDestMemory int64 = 0
	destItem, destOk := database.DB.Store[destKey]
	if destOk {
		oldDestMemory = destItem.ApproxMemoryUsage(destKey)
	} else {
		destItem = &common.Item{
			Type:    "set",
			ItemSet: make(map[string]bool),
		}
		database.DB.Store[destKey] = destItem
	}

	// Clear destination set
	destItem.ItemSet = make(map[string]bool)

	// Perform union
	for _, arg := range sourceKeys {
		key := arg.Blk
		item, ok := database.DB.Store[key]
		if !ok {
			continue
		}
		if item.Type != "set" {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		for member := range item.ItemSet {
			destItem.ItemSet[member] = true
		}
	}

	count := int64(len(destItem.ItemSet))
	database.DB.Touch(destKey)
	newDestMemory := destItem.ApproxMemoryUsage(destKey)
	database.DB.Mem += (newDestMemory - oldDestMemory)

	saveDBState(state, v)

	return common.NewIntegerValue(count)
}

// Sdiffstore handles the SDIFFSTORE command.
// Store the members of the set resulting from the difference between the first set and all the successive sets.
//
// Syntax:
//
//	SDIFFSTORE <destination> <key> [<key> ...]
//
// Returns:
//
//	Integer: The number of elements in the resulting set.
func Sdiffstore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'sdiffstore' command")
	}

	destKey := args[0].Blk
	sourceKeys := args[1:]

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Check if destination exists and is not a set
	if existing, ok := database.DB.Store[destKey]; ok && existing.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	var oldDestMemory int64 = 0
	destItem, destOk := database.DB.Store[destKey]
	if destOk {
		oldDestMemory = destItem.ApproxMemoryUsage(destKey)
	} else {
		destItem = &common.Item{
			Type:    "set",
			ItemSet: make(map[string]bool),
		}
		database.DB.Store[destKey] = destItem
	}

	// Clear destination set
	destItem.ItemSet = make(map[string]bool)

	// Get first set
	firstKey := sourceKeys[0].Blk
	firstItem, ok := database.DB.Store[firstKey]
	if !ok {
		// If first key doesn't exist, result is empty
		count := int64(0)
		database.DB.Touch(destKey)
		newDestMemory := destItem.ApproxMemoryUsage(destKey)
		database.DB.Mem += (newDestMemory - oldDestMemory)
		return common.NewIntegerValue(count)
	}
	if firstItem.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Perform difference
	for member := range firstItem.ItemSet {
		presentInOthers := false
		for _, arg := range sourceKeys[1:] {
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
			destItem.ItemSet[member] = true
		}
	}

	count := int64(len(destItem.ItemSet))
	database.DB.Touch(destKey)
	newDestMemory := destItem.ApproxMemoryUsage(destKey)
	database.DB.Mem += (newDestMemory - oldDestMemory)

	saveDBState(state, v)

	return common.NewIntegerValue(count)
}

// Smismember handles the SMISMEMBER command.
// Check if multiple members are members of a set.
//
// Syntax:
//
//	SMISMEMBER <key> <member> [<member> ...]
//
// Returns:
//
//	Array: List of integers, 1 if the element is a member of the set, 0 if not.
func Smismember(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'smismember' command")
	}

	key := args[0].Blk
	members := args[1:]

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		// If key doesn't exist, all members are 0
		result := make([]common.Value, len(members))
		for i := range result {
			result[i] = common.Value{Typ: common.INTEGER, Num: 0}
		}
		return common.NewArrayValue(result)
	}
	if item.Type != "set" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, len(members))
	for i, arg := range members {
		member := arg.Blk
		if item.ItemSet[member] {
			result[i] = common.Value{Typ: common.INTEGER, Num: 1}
		} else {
			result[i] = common.Value{Typ: common.INTEGER, Num: 0}
		}
	}

	return common.NewArrayValue(result)
}
