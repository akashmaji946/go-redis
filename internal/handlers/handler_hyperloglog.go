/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_hyperloglog.go

HyperLogLog Implementation for go-redis
Based on the HyperLogLog algorithm by Flajolet et al.

Key parameters:
- m = 2^14 = 16384 registers
- Each register stores max leading zeros (6 bits, values 0-63)
- Standard error: ~0.81%
*/
package handlers

import (
	"encoding/binary"
	"hash/fnv"
	"math"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

const (
	// HLL_P is the precision (number of bits for register index)
	HLL_P = 14
	// HLL_M is the number of registers (2^14 = 16384)
	HLL_M = 1 << HLL_P
	// HLL_ALPHA is the bias correction constant for m=16384
	HLL_ALPHA = 0.7213 / (1.0 + 1.079/float64(HLL_M))
	// HLL_SPARSE_THRESHOLD is when to convert from sparse to dense
	HLL_SPARSE_THRESHOLD = 1024
)

// hllHash computes a 64-bit hash of the input element
func hllHash(element string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(element))
	return h.Sum64()
}

// hllRho counts the position of the first 1-bit (leading zeros + 1)
// in the remaining bits after extracting the register index
func hllRho(hash uint64) uint8 {
	// We use the remaining 50 bits (64 - 14 = 50)
	// Shift left by HLL_P to get the remaining bits
	w := hash << HLL_P

	if w == 0 {
		// All zeros, return max value (50 + 1 = 51)
		return 51
	}

	// Count leading zeros + 1
	rho := uint8(1)
	for (w & (1 << 63)) == 0 {
		rho++
		w <<= 1
	}
	return rho
}

// hllEstimate computes the raw HyperLogLog estimate
func hllEstimate(registers []uint8) float64 {
	// Compute the harmonic mean of 2^(-M[i])
	sum := 0.0
	zeros := 0

	for _, val := range registers {
		sum += math.Pow(2.0, -float64(val))
		if val == 0 {
			zeros++
		}
	}

	// Raw estimate: E = alpha * m^2 * (sum of 2^(-M[i]))^(-1)
	estimate := HLL_ALPHA * float64(HLL_M) * float64(HLL_M) / sum

	// Small cardinality correction (Linear Counting)
	if estimate <= 2.5*float64(HLL_M) && zeros > 0 {
		// Use linear counting: E' = m * ln(m/V)
		estimate = float64(HLL_M) * math.Log(float64(HLL_M)/float64(zeros))
	}

	// Large cardinality correction
	// If E > (1/30) * 2^64, apply correction
	threshold := math.Pow(2, 64) / 30.0
	if estimate > threshold {
		estimate = -math.Pow(2, 64) * math.Log(1.0-estimate/math.Pow(2, 64))
	}

	return estimate
}

// hllMergeRegisters merges multiple register arrays by taking max of each position
func hllMergeRegisters(registerSets ...[]uint8) []uint8 {
	result := make([]uint8, HLL_M)

	for _, registers := range registerSets {
		for i := 0; i < HLL_M && i < len(registers); i++ {
			if registers[i] > result[i] {
				result[i] = registers[i]
			}
		}
	}

	return result
}

// sparseToRegisters converts sparse representation to dense registers
func sparseToRegisters(sparse map[uint16]uint8) []uint8 {
	registers := make([]uint8, HLL_M)
	for idx, val := range sparse {
		registers[idx] = val
	}
	return registers
}

// getHLLRegisters gets the registers from an item, handling both sparse and dense
func getHLLRegisters(item *common.Item) []uint8 {
	if item.HLLRegisters != nil {
		return item.HLLRegisters
	}
	if item.HLLSparse != nil {
		return sparseToRegisters(item.HLLSparse)
	}
	return make([]uint8, HLL_M)
}

// PfAdd handles the PFADD command.
// Adds elements to a HyperLogLog data structure.
//
// Syntax:
//
//	PFADD <key> <element> [<element> ...]
//
// Returns:
//
//	Integer: 1 if at least one register was altered, 0 otherwise
//
// Behavior:
//   - Creates HLL if key doesn't exist
//   - Uses sparse representation for small cardinalities
//   - Converts to dense when threshold exceeded
//   - Idempotent: adding same element twice has no effect
func PfAdd(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'pfadd' command")
	}

	key := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0
	var changed bool = false

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		if item.Type != common.HLL_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.ApproxMemoryUsage(key)
	} else {
		// Create new HLL with sparse representation
		item = &common.Item{
			Type:      common.HLL_TYPE,
			HLLSparse: make(map[uint16]uint8),
		}
		database.DB.Store[key] = item
	}

	// Process each element
	for i := 1; i < len(args); i++ {
		element := args[i].Blk
		hash := hllHash(element)

		// Extract register index (first 14 bits)
		index := uint16(hash & (HLL_M - 1))

		// Compute rho (leading zeros + 1)
		rho := hllRho(hash)

		// Update register if new value is larger
		if item.HLLSparse != nil {
			// Sparse mode
			if current, exists := item.HLLSparse[index]; !exists || rho > current {
				item.HLLSparse[index] = rho
				changed = true
			}

			// Check if we need to convert to dense
			if len(item.HLLSparse) > HLL_SPARSE_THRESHOLD {
				item.HLLRegisters = sparseToRegisters(item.HLLSparse)
				item.HLLSparse = nil
			}
		} else {
			// Dense mode
			if rho > item.HLLRegisters[index] {
				item.HLLRegisters[index] = rho
				changed = true
			}
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
		database.DB.IncrTrackers()
	}

	if changed {
		return common.NewIntegerValue(1)
	}
	return common.NewIntegerValue(0)
}

// PfCount handles the PFCOUNT command.
// Returns the approximated cardinality of the set(s) observed by the HyperLogLog(s).
//
// Syntax:
//
//	PFCOUNT <key> [<key> ...]
//
// Returns:
//
//	Integer: Approximated number of unique elements
//
// Behavior:
//   - Single key: returns cardinality estimate
//   - Multiple keys: returns cardinality of union (virtual merge)
//   - Standard error: ~0.81%
//   - Uses linear counting for small cardinalities
//   - Uses large cardinality correction for very large sets
func PfCount(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'pfcount' command")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	var registerSets [][]uint8

	for _, arg := range args {
		key := arg.Blk
		item, ok := database.DB.Store[key]

		if !ok {
			// Non-existent key contributes empty registers
			continue
		}

		if item.Type != common.HLL_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		registerSets = append(registerSets, getHLLRegisters(item))
	}

	if len(registerSets) == 0 {
		return common.NewIntegerValue(0)
	}

	// Merge all register sets (virtual merge for multiple keys)
	var registers []uint8
	if len(registerSets) == 1 {
		registers = registerSets[0]
	} else {
		registers = hllMergeRegisters(registerSets...)
	}

	// Compute estimate
	estimate := hllEstimate(registers)

	return common.NewIntegerValue(int64(math.Round(estimate)))
}

// PfMerge handles the PFMERGE command.
// Merges multiple HyperLogLog values into a single one.
//
// Syntax:
//
//	PFMERGE <destkey> <sourcekey> [<sourcekey> ...]
//
// Returns:
//
//	Simple String: OK
//
// Behavior:
//   - Creates destination HLL if it doesn't exist
//   - Merges by taking max of each register position
//   - Preserves cardinality estimate of union
//   - Result is always in dense format if any source is dense
func PfMerge(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'pfmerge' command")
	}

	destKey := args[0].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var registerSets [][]uint8
	var oldMemory int64 = 0

	// Check if destination exists and get its memory
	if existing, ok := database.DB.Store[destKey]; ok {
		if existing.Type != common.HLL_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = existing.ApproxMemoryUsage(destKey)
	}

	// Collect all source registers
	for i := 1; i < len(args); i++ {
		sourceKey := args[i].Blk
		item, ok := database.DB.Store[sourceKey]

		if !ok {
			// Non-existent key contributes empty registers
			continue
		}

		if item.Type != common.HLL_TYPE {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		registerSets = append(registerSets, getHLLRegisters(item))
	}

	// Merge all registers
	var mergedRegisters []uint8
	if len(registerSets) == 0 {
		mergedRegisters = make([]uint8, HLL_M)
	} else if len(registerSets) == 1 {
		// Copy the single source
		mergedRegisters = make([]uint8, HLL_M)
		copy(mergedRegisters, registerSets[0])
	} else {
		mergedRegisters = hllMergeRegisters(registerSets...)
	}

	// Create or update destination
	destItem := &common.Item{
		Type:         common.HLL_TYPE,
		HLLRegisters: mergedRegisters,
		HLLSparse:    nil, // Merged result is always dense
	}
	database.DB.Store[destKey] = destItem

	database.DB.Touch(destKey)
	newMemory := destItem.ApproxMemoryUsage(destKey)
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

	return common.NewStringValue("OK")
}

// PfDebug handles the PFDEBUG command (for testing/debugging).
// Returns internal HLL information.
//
// Syntax:
//
//	PFDEBUG <key>
//
// Returns:
//
//	Array: [encoding, registers_used, estimated_cardinality]
func PfDebug(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'pfdebug' command")
	}

	key := args[0].Blk

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewNullValue()
	}

	if item.Type != common.HLL_TYPE {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	encoding := "dense"
	registersUsed := 0

	if item.HLLSparse != nil {
		encoding = "sparse"
		registersUsed = len(item.HLLSparse)
	} else if item.HLLRegisters != nil {
		for _, v := range item.HLLRegisters {
			if v > 0 {
				registersUsed++
			}
		}
	}

	registers := getHLLRegisters(item)
	estimate := hllEstimate(registers)

	result := []common.Value{
		{Typ: common.BULK, Blk: "encoding"},
		{Typ: common.BULK, Blk: encoding},
		{Typ: common.BULK, Blk: "registers_used"},
		*common.NewIntegerValue(int64(registersUsed)),
		{Typ: common.BULK, Blk: "estimated_cardinality"},
		*common.NewIntegerValue(int64(math.Round(estimate))),
	}

	return common.NewArrayValue(result)
}

// hllSerialize serializes HLL registers to bytes for RDB persistence
func HLLSerialize(item *common.Item) []byte {
	registers := getHLLRegisters(item)

	// Format: [1 byte: encoding] [2 bytes: count] [data...]
	// For dense: data is all 16384 registers
	// For sparse: data is pairs of (index, value)

	if item.HLLSparse != nil {
		// Sparse encoding
		buf := make([]byte, 3+len(item.HLLSparse)*3)
		buf[0] = 0 // sparse marker
		binary.LittleEndian.PutUint16(buf[1:3], uint16(len(item.HLLSparse)))

		offset := 3
		for idx, val := range item.HLLSparse {
			binary.LittleEndian.PutUint16(buf[offset:offset+2], idx)
			buf[offset+2] = val
			offset += 3
		}
		return buf
	}

	// Dense encoding
	buf := make([]byte, 1+HLL_M)
	buf[0] = 1 // dense marker
	copy(buf[1:], registers)
	return buf
}

// HLLDeserialize deserializes bytes back to HLL item
func HLLDeserialize(data []byte) *common.Item {
	if len(data) < 1 {
		return nil
	}

	item := &common.Item{
		Type: common.HLL_TYPE,
	}

	if data[0] == 0 {
		// Sparse encoding
		if len(data) < 3 {
			return nil
		}
		count := binary.LittleEndian.Uint16(data[1:3])
		item.HLLSparse = make(map[uint16]uint8, count)

		offset := 3
		for i := uint16(0); i < count && offset+2 < len(data); i++ {
			idx := binary.LittleEndian.Uint16(data[offset : offset+2])
			val := data[offset+2]
			item.HLLSparse[idx] = val
			offset += 3
		}
	} else {
		// Dense encoding
		item.HLLRegisters = make([]uint8, HLL_M)
		copy(item.HLLRegisters, data[1:])
	}

	return item
}

// HLLRestore handles the _HLLRESTORE command (internal command for AOF replay).
// Restores a HyperLogLog from its serialized state.
//
// Syntax:
//
//	_HLLRESTORE <key> <format> <data>
//
// Parameters:
//   - key: The key to restore
//   - format: "dense" or "sparse"
//   - data: Serialized register data
//
// Returns:
//
//	Simple String: OK
func HLLRestore(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 {
		return common.NewErrorValue("ERR wrong number of arguments for '_hllrestore' command")
	}

	key := args[0].Blk
	format := args[1].Blk
	data := args[2].Blk

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item

	if format == "dense" {
		// Restore dense format
		item = &common.Item{
			Type:         common.HLL_TYPE,
			HLLRegisters: make([]uint8, HLL_M),
		}
		for i := 0; i < len(data) && i < HLL_M; i++ {
			item.HLLRegisters[i] = uint8(data[i]) - 32 // Remove offset
		}
	} else if format == "sparse" {
		// Restore sparse format
		item = &common.Item{
			Type:      common.HLL_TYPE,
			HLLSparse: make(map[uint16]uint8),
		}
		// Parse "idx:val,idx:val,..." format
		pairs := splitString(data, ',')
		for _, pair := range pairs {
			if pair == "" {
				continue
			}
			parts := splitString(pair, ':')
			if len(parts) != 2 {
				continue
			}
			idx, err1 := parseUint16(parts[0])
			val, err2 := parseUint8(parts[1])
			if err1 == nil && err2 == nil {
				item.HLLSparse[idx] = val
			}
		}
	} else {
		return common.NewErrorValue("ERR invalid format for '_hllrestore' command")
	}

	database.DB.Store[key] = item
	database.DB.Touch(key)

	// Update memory tracking
	newMemory := item.ApproxMemoryUsage(key)
	database.DB.Mem += newMemory
	if database.DB.Mem > database.DB.Mempeak {
		database.DB.Mempeak = database.DB.Mem
	}

	return common.NewStringValue("OK")
}

// Helper functions for parsing
func splitString(s string, sep rune) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func parseUint16(s string) (uint16, error) {
	val, err := common.ParseInt(s)
	if err != nil {
		return 0, err
	}
	return uint16(val), nil
}

func parseUint8(s string) (uint8, error) {
	val, err := common.ParseInt(s)
	if err != nil {
		return 0, err
	}
	return uint8(val), nil
}
