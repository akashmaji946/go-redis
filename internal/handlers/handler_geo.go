/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_geo.go
*/
package handlers

import (
	"math"
	"strconv"
	"strings"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

const (
	// Geohash base32 alphabet (excluding a, i, l, o)
	base32 = "0123456789bcdefghjkmnpqrstuvwxyz"
	// Earth's radius in meters
	earthRadius = 6372797.560856
)

// GeoHandlers is the map of geospatial command names to their handler functions.
var GeoHandlers = map[string]common.Handler{
	"GEOADD":         GeoAdd,
	"GEOPOS":         GeoPos,
	"GEODIST":        GeoDist,
	"GEOHASH":        GeoHash,
	"GEORADIUS":      GeoRadius,
	"GEOSEARCH":      GeoSearch,
	"GEOSEARCHSTORE": GeoSearchStore,
}

// GeoAdd handles the GEOADD command.
// Adds geospatial items to a sorted set.
//
// Syntax:
//
//	GEOADD <key> <longitude> <latitude> <member> [<longitude> <latitude> <member> ...]
//
// Returns:
//
//	Integer: The number of elements added to the sorted set.
func GeoAdd(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 4 || len(args)%3 != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'geoadd' command")
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
	for i := 1; i < len(args); i += 3 {
		lngStr := args[i].Blk
		latStr := args[i+1].Blk
		member := args[i+2].Blk

		lng, err := strconv.ParseFloat(lngStr, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not a valid longitude")
		}
		lat, err := strconv.ParseFloat(latStr, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not a valid latitude")
		}

		if lng < -180 || lng > 180 || lat < -90 || lat > 90 {
			return common.NewErrorValue("ERR invalid longitude,latitude pair")
		}

		hash := encodeGeohash(lat, lng, 12)
		score := geohashToScore(hash)

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

	saveDBState(state, v)

	return common.NewIntegerValue(addedCount)
}

// GeoPos handles the GEOPOS command.
// Returns the positions (longitude,latitude) of all the specified members.
//
// Syntax:
//
//	GEOPOS <key> <member> [<member> ...]
//
// Returns:
//
//	Array: Array of positions, each position is an array of two elements: longitude and latitude.
func GeoPos(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'geopos' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		result := make([]common.Value, len(args)-1)
		for i := range result {
			result[i] = common.Value{Typ: common.NULL}
		}
		return common.NewArrayValue(result)
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0, len(args)-1)
	for _, arg := range args[1:] {
		member := arg.Blk
		score, exists := item.ZSet[member]
		if !exists {
			result = append(result, common.Value{Typ: common.NULL})
			continue
		}

		hash := scoreToGeohash(score, 12)
		lat, lng := decodeGeohash(hash)

		pos := []common.Value{
			{Typ: common.BULK, Blk: strconv.FormatFloat(lng, 'f', 6, 64)},
			{Typ: common.BULK, Blk: strconv.FormatFloat(lat, 'f', 6, 64)},
		}
		result = append(result, common.Value{Typ: common.ARRAY, Arr: pos})
	}

	return common.NewArrayValue(result)
}

// GeoDist handles the GEODIST command.
// Returns the distance between two members in the geospatial index.
//
// Syntax:
//
//	GEODIST <key> <member1> <member2> [m|km|ft|mi]
//
// Returns:
//
//	Bulk String: The distance as a double precision floating point number.
func GeoDist(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 || len(args) > 4 {
		return common.NewErrorValue("ERR wrong number of arguments for 'geodist' command")
	}

	key := args[0].Blk
	member1 := args[1].Blk
	member2 := args[2].Blk

	unit := "m"
	if len(args) == 4 {
		unit = strings.ToLower(args[3].Blk)
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewNullValue()
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	score1, exists1 := item.ZSet[member1]
	score2, exists2 := item.ZSet[member2]
	if !exists1 || !exists2 {
		return common.NewNullValue()
	}

	hash1 := scoreToGeohash(score1, 12)
	hash2 := scoreToGeohash(score2, 12)
	lat1, lng1 := decodeGeohash(hash1)
	lat2, lng2 := decodeGeohash(hash2)

	distance := haversineDistance(lat1, lng1, lat2, lng2)

	switch unit {
	case "km":
		distance /= 1000
	case "ft":
		distance *= 3.28084
	case "mi":
		distance *= 0.000621371
	}

	return common.NewBulkValue(strconv.FormatFloat(distance, 'f', 4, 64))
}

// GeoHash handles the GEOHASH command.
// Returns the geohash strings for the specified members.
//
// Syntax:
//
//	GEOHASH <key> <member> [<member> ...]
//
// Returns:
//
//	Array: Array of geohash strings.
func GeoHash(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'geohash' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		result := make([]common.Value, len(args)-1)
		for i := range result {
			result[i] = common.Value{Typ: common.NULL}
		}
		return common.NewArrayValue(result)
	}

	if item.Type != common.ZSET_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	result := make([]common.Value, 0, len(args)-1)
	for _, arg := range args[1:] {
		member := arg.Blk
		score, exists := item.ZSet[member]
		if !exists {
			result = append(result, common.Value{Typ: common.NULL})
			continue
		}

		hash := scoreToGeohash(score, 12)
		result = append(result, common.Value{Typ: common.BULK, Blk: hash})
	}

	return common.NewArrayValue(result)
}

// GeoRadius handles the GEORADIUS command.
// Query by radius (deprecated but useful).
//
// Syntax:
//
//	GEORADIUS <key> <longitude> <latitude> <radius> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC]
//
// Returns:
//
//	Array: Results depending on options.
func GeoRadius(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 5 {
		return common.NewErrorValue("ERR wrong number of arguments for 'georadius' command")
	}

	key := args[0].Blk
	lngStr := args[1].Blk
	latStr := args[2].Blk
	radiusStr := args[3].Blk
	unit := strings.ToLower(args[4].Blk)

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		return common.NewErrorValue("ERR value is not a valid longitude")
	}
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return common.NewErrorValue("ERR value is not a valid latitude")
	}
	radius, err := strconv.ParseFloat(radiusStr, 64)
	if err != nil {
		return common.NewErrorValue("ERR value is not a valid radius")
	}

	// Convert radius to meters
	switch unit {
	case "km":
		radius *= 1000
	case "ft":
		radius /= 3.28084
	case "mi":
		radius /= 0.000621371
	case "m":
		// already in meters
	default:
		return common.NewErrorValue("ERR invalid unit")
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

	// Parse options
	withCoord := false
	withDist := false
	withHash := false
	count := -1
	order := "asc"

	i := 5
	for i < len(args) {
		opt := strings.ToUpper(args[i].Blk)
		switch opt {
		case "WITHCOORD":
			withCoord = true
			i++
		case "WITHDIST":
			withDist = true
			i++
		case "WITHHASH":
			withHash = true
			i++
		case "COUNT":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			countStr := args[i+1].Blk
			c, err := strconv.Atoi(countStr)
			if err != nil {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}
			count = c
			i += 2
		case "ASC", "DESC":
			order = strings.ToLower(opt)
			i++
		default:
			return common.NewErrorValue("ERR syntax error")
		}
	}

	// Find members within radius
	type result struct {
		member string
		dist   float64
		hash   string
		lat    float64
		lng    float64
	}

	var results []result
	for member, score := range item.ZSet {
		hash := scoreToGeohash(score, 12)
		mLat, mLng := decodeGeohash(hash)
		dist := haversineDistance(lat, lng, mLat, mLng)
		if dist <= radius {
			results = append(results, result{
				member: member,
				dist:   dist,
				hash:   hash,
				lat:    mLat,
				lng:    mLng,
			})
		}
	}

	// Sort results
	if order == "asc" {
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[i].dist > results[j].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	} else {
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[i].dist < results[j].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	}

	// Limit count
	if count > 0 && len(results) > count {
		results = results[:count]
	}

	// Build response
	resp := make([]common.Value, 0, len(results))
	for _, r := range results {
		var itemArr []common.Value
		itemArr = append(itemArr, common.Value{Typ: common.BULK, Blk: r.member})

		if withDist {
			itemArr = append(itemArr, common.Value{Typ: common.BULK, Blk: strconv.FormatFloat(r.dist, 'f', 4, 64)})
		}
		if withHash {
			itemArr = append(itemArr, common.Value{Typ: common.BULK, Blk: r.hash})
		}
		if withCoord {
			coord := []common.Value{
				{Typ: common.BULK, Blk: strconv.FormatFloat(r.lng, 'f', 6, 64)},
				{Typ: common.BULK, Blk: strconv.FormatFloat(r.lat, 'f', 6, 64)},
			}
			itemArr = append(itemArr, common.Value{Typ: common.ARRAY, Arr: coord})
		}

		resp = append(resp, common.Value{Typ: common.ARRAY, Arr: itemArr})
	}

	return common.NewArrayValue(resp)
}

// GeoSearch handles the GEOSEARCH command.
// Search by radius/box.
//
// Syntax:
//
//	GEOSEARCH <key> FROMMEMBER <member> RADIUS <radius> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC]
//	GEOSEARCH <key> FROMLONLAT <longitude> <latitude> RADIUS <radius> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC]
//	GEOSEARCH <key> FROMMEMBER <member> BYBOX <width> <height> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC]
//	GEOSEARCH <key> FROMLONLAT <longitude> <latitude> BYBOX <width> <height> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC]
//
// Returns:
//
//	Array: Results depending on options.
func GeoSearch(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 7 {
		return common.NewErrorValue("ERR wrong number of arguments for 'geosearch' command")
	}

	key := args[0].Blk

	// Parse FROM
	var centerLat, centerLng float64
	var err error

	if strings.ToUpper(args[1].Blk) == "FROMMEMBER" {
		if len(args) < 3 {
			return common.NewErrorValue("ERR syntax error")
		}
		member := args[2].Blk

		database.DB.Mu.RLock()
		item, ok := database.DB.Store[key]
		database.DB.Mu.RUnlock()

		if !ok {
			return common.NewArrayValue([]common.Value{})
		}
		if item.Type != common.ZSET_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		score, exists := item.ZSet[member]
		if !exists {
			return common.NewArrayValue([]common.Value{})
		}

		hash := scoreToGeohash(score, 12)
		centerLat, centerLng = decodeGeohash(hash)
		args = args[3:]
	} else if strings.ToUpper(args[1].Blk) == "FROMLONLAT" {
		if len(args) < 4 {
			return common.NewErrorValue("ERR syntax error")
		}
		centerLng, err = strconv.ParseFloat(args[2].Blk, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not a valid longitude")
		}
		centerLat, err = strconv.ParseFloat(args[3].Blk, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not a valid latitude")
		}
		args = args[4:]
	} else {
		return common.NewErrorValue("ERR syntax error")
	}

	// Parse BY
	var radius float64
	var width, height float64
	var unit string
	isRadius := false

	if strings.ToUpper(args[0].Blk) == "RADIUS" {
		if len(args) < 3 {
			return common.NewErrorValue("ERR syntax error")
		}
		radius, err = strconv.ParseFloat(args[1].Blk, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not a valid radius")
		}
		unit = strings.ToLower(args[2].Blk)
		args = args[3:]
		isRadius = true
	} else if strings.ToUpper(args[0].Blk) == "BYBOX" {
		if len(args) < 4 {
			return common.NewErrorValue("ERR syntax error")
		}
		width, err = strconv.ParseFloat(args[1].Blk, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not a valid width")
		}
		height, err = strconv.ParseFloat(args[2].Blk, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not a valid height")
		}
		unit = strings.ToLower(args[3].Blk)
		args = args[4:]
	} else {
		return common.NewErrorValue("ERR syntax error")
	}

	// Convert to meters
	switch unit {
	case "km":
		radius *= 1000
		width *= 1000
		height *= 1000
	case "ft":
		radius /= 3.28084
		width /= 3.28084
		height /= 3.28084
	case "mi":
		radius /= 0.000621371
		width /= 0.000621371
		height /= 0.000621371
	case "m":
		// already in meters
	default:
		return common.NewErrorValue("ERR invalid unit")
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

	// Parse options
	withCoord := false
	withDist := false
	withHash := false
	count := -1
	order := "asc"

	i := 0
	for i < len(args) {
		opt := strings.ToUpper(args[i].Blk)
		switch opt {
		case "WITHCOORD":
			withCoord = true
			i++
		case "WITHDIST":
			withDist = true
			i++
		case "WITHHASH":
			withHash = true
			i++
		case "COUNT":
			if i+1 >= len(args) {
				return common.NewErrorValue("ERR syntax error")
			}
			countStr := args[i+1].Blk
			c, err := strconv.Atoi(countStr)
			if err != nil {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}
			count = c
			i += 2
		case "ASC", "DESC":
			order = strings.ToLower(opt)
			i++
		default:
			return common.NewErrorValue("ERR syntax error")
		}
	}

	// Find members within area
	type result struct {
		member string
		dist   float64
		hash   string
		lat    float64
		lng    float64
	}

	var results []result
	for member, score := range item.ZSet {
		hash := scoreToGeohash(score, 12)
		mLat, mLng := decodeGeohash(hash)

		if isRadius {
			dist := haversineDistance(centerLat, centerLng, mLat, mLng)
			if dist <= radius {
				results = append(results, result{
					member: member,
					dist:   dist,
					hash:   hash,
					lat:    mLat,
					lng:    mLng,
				})
			}
		} else {
			// Box search
			dLat := haversineDistance(centerLat, centerLng, mLat, centerLng)
			dLng := haversineDistance(centerLat, centerLng, centerLat, mLng)
			if dLat <= height/2 && dLng <= width/2 {
				dist := haversineDistance(centerLat, centerLng, mLat, mLng)
				results = append(results, result{
					member: member,
					dist:   dist,
					hash:   hash,
					lat:    mLat,
					lng:    mLng,
				})
			}
		}
	}

	// Sort results
	if order == "asc" {
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[i].dist > results[j].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	} else {
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[i].dist < results[j].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	}

	// Limit count
	if count > 0 && len(results) > count {
		results = results[:count]
	}

	// Build response
	resp := make([]common.Value, 0, len(results))
	for _, r := range results {
		var itemArr []common.Value
		itemArr = append(itemArr, common.Value{Typ: common.BULK, Blk: r.member})

		if withDist {
			itemArr = append(itemArr, common.Value{Typ: common.BULK, Blk: strconv.FormatFloat(r.dist, 'f', 4, 64)})
		}
		if withHash {
			itemArr = append(itemArr, common.Value{Typ: common.BULK, Blk: r.hash})
		}
		if withCoord {
			coord := []common.Value{
				{Typ: common.BULK, Blk: strconv.FormatFloat(r.lng, 'f', 6, 64)},
				{Typ: common.BULK, Blk: strconv.FormatFloat(r.lat, 'f', 6, 64)},
			}
			itemArr = append(itemArr, common.Value{Typ: common.ARRAY, Arr: coord})
		}

		resp = append(resp, common.Value{Typ: common.ARRAY, Arr: itemArr})
	}

	return common.NewArrayValue(resp)
}

// GeoSearchStore handles the GEOSEARCHSTORE command.
// Store search results.
//
// Syntax:
//
//	GEOSEARCHSTORE <destination> <source> FROMMEMBER <member> RADIUS <radius> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC] [STOREDIST]
//	GEOSEARCHSTORE <destination> <source> FROMLONLAT <longitude> <latitude> RADIUS <radius> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC] [STOREDIST]
//	GEOSEARCHSTORE <destination> <source> FROMMEMBER <member> BYBOX <width> <height> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC] [STOREDIST]
//	GEOSEARCHSTORE <destination> <source> FROMLONLAT <longitude> <latitude> BYBOX <width> <height> <unit> [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT <count>] [ASC|DESC] [STOREDIST]
//
// Returns:
//
//	Integer: The number of elements in the resulting sorted set.
func GeoSearchStore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 8 {
		return common.NewErrorValue("ERR wrong number of arguments for 'geosearchstore' command")
	}

	destKey := args[0].Blk
	srcKey := args[1].Blk

	// Remove dest and src from args for reuse of GeoSearch logic
	searchArgs := make([]common.Value, len(v.Arr))
	copy(searchArgs, v.Arr)
	searchArgs[1] = common.Value{Typ: common.BULK, Blk: srcKey} // replace dest with src for search
	searchV := &common.Value{Typ: common.ARRAY, Arr: searchArgs}

	// Perform search
	searchResult := GeoSearch(c, searchV, state)
	if searchResult.Typ == common.ERROR {
		return searchResult
	}

	// Store results in destination key
	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var destItem *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[destKey]; ok {
		destItem = existing
		if destItem.Type != common.ZSET_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = destItem.ApproxMemoryUsage(destKey)
	} else {
		destItem = &common.Item{
			Type: common.ZSET_TYPE,
			ZSet: make(map[string]float64),
		}
		database.DB.Store[destKey] = destItem
	}

	// Clear existing zset
	destItem.ZSet = make(map[string]float64)

	// Parse STOREDIST option
	storeDist := false
	for i, arg := range args {
		if strings.ToUpper(arg.Blk) == "STOREDIST" {
			storeDist = true
			// Remove STOREDIST from args for parsing
			copy(args[i:], args[i+1:])
			args = args[:len(args)-1]
			break
		}
	}

	count := int64(0)
	if searchResult.Typ == common.ARRAY {
		for _, item := range searchResult.Arr {
			if item.Typ == common.ARRAY && len(item.Arr) > 0 {
				member := item.Arr[0].Blk
				var score float64

				if storeDist && len(item.Arr) > 1 {
					// Store distance as score
					distStr := item.Arr[1].Blk
					score, _ = strconv.ParseFloat(distStr, 64)
				} else {
					// Store original geohash score
					database.DB.Mu.RLock()
					srcItem, ok := database.DB.Store[srcKey]
					database.DB.Mu.RUnlock()
					if ok && srcItem.Type == common.ZSET_TYPE {
						if origScore, exists := srcItem.ZSet[member]; exists {
							score = origScore
						}
					}
				}

				destItem.ZSet[member] = score
				count++
			}
		}
	}

	database.DB.Touch(destKey)
	newMemory := destItem.ApproxMemoryUsage(destKey)
	database.DB.Mem += (newMemory - oldMemory)
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	saveDBState(state, v)

	return common.NewIntegerValue(count)
}

// encodeGeohash encodes latitude and longitude into a geohash string
func encodeGeohash(lat, lng float64, precision int) string {
	if precision <= 0 {
		precision = 12
	}
	if precision > 12 {
		precision = 12
	}

	minLat, maxLat := -90.0, 90.0
	minLng, maxLng := -180.0, 180.0

	var hash strings.Builder
	bits := 0
	bit := 0
	ch := 0

	for hash.Len() < precision {
		if bit%2 == 0 {
			// Even bit: longitude
			mid := (minLng + maxLng) / 2
			if lng > mid {
				ch |= (1 << (4 - bits))
				minLng = mid
			} else {
				maxLng = mid
			}
		} else {
			// Odd bit: latitude
			mid := (minLat + maxLat) / 2
			if lat > mid {
				ch |= (1 << (4 - bits))
				minLat = mid
			} else {
				maxLat = mid
			}
		}

		bits++
		if bits == 5 {
			hash.WriteByte(base32[ch])
			bits = 0
			ch = 0
		}
		bit++
	}

	return hash.String()
}

// decodeGeohash decodes a geohash string into latitude and longitude
func decodeGeohash(hash string) (float64, float64) {
	minLat, maxLat := -90.0, 90.0
	minLng, maxLng := -180.0, 180.0

	even := true
	for _, ch := range hash {
		cd := strings.IndexByte(base32, byte(ch))
		if cd == -1 {
			break
		}

		for i := 4; i >= 0; i-- {
			bit := (cd >> uint(i)) & 1
			if even {
				// Longitude
				mid := (minLng + maxLng) / 2
				if bit == 1 {
					minLng = mid
				} else {
					maxLng = mid
				}
			} else {
				// Latitude
				mid := (minLat + maxLat) / 2
				if bit == 1 {
					minLat = mid
				} else {
					maxLat = mid
				}
			}
			even = !even
		}
	}

	lat := (minLat + maxLat) / 2
	lng := (minLng + maxLng) / 2
	return lat, lng
}

// geohashToScore converts geohash to a score for zset
func geohashToScore(hash string) float64 {
	// Convert geohash to a numerical score
	// Use the interleaved bits as a 64-bit integer, then to float64
	score := 0.0
	for i, ch := range hash {
		cd := strings.IndexByte(base32, byte(ch))
		if cd == -1 {
			break
		}
		score += float64(cd) * math.Pow(32, float64(len(hash)-1-i))
	}
	return score
}

// scoreToGeohash converts score back to geohash (approximate)
func scoreToGeohash(score float64, precision int) string {
	if precision <= 0 {
		precision = 12
	}
	hash := ""
	val := score
	for i := 0; i < precision; i++ {
		idx := int(val) % 32
		hash = string(base32[idx]) + hash
		val /= 32
	}
	return hash
}

// haversineDistance calculates distance between two points in meters
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
