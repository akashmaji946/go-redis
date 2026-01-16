/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handler_bitmap.go

Bitmap Implementation for go-redis
Bitmaps are not a separate data type in Redis, but rather a set of bit-oriented
operations defined on the String type. Since strings are binary safe blobs and
their maximum length is 512 MB, they are suitable to set up to 2^32 different bits.

Bit operations are divided into two groups:
1. Constant-time single bit operations (SETBIT, GETBIT)
2. Operations on groups of bits (BITCOUNT, BITOP, BITPOS, BITFIELD)
*/
package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// SetBit handles the SETBIT command.
// Sets or clears the bit at offset in the string value stored at key.
//
// Syntax:
//
//	SETBIT <key> <offset> <value>
//
// Parameters:
//   - key: The key of the string to modify
//   - offset: The bit offset (0-based, can be up to 2^32-1)
//   - value: The bit value to set (0 or 1)
//
// Returns:
//
//	Integer: The original bit value stored at offset (0 or 1)
//
// Behavior:
//   - If key does not exist, a new string value is created
//   - The string is grown to make sure it can hold a bit at offset
//   - When the string at key is grown, added bits are set to 0
//   - The offset argument is required to be >= 0 and < 2^32 (4294967296)
//   - The bit offset is calculated from left to right (MSB to LSB within each byte)
//
// Time Complexity: O(1)
//
// Memory: When setting a bit at a large offset, the string is automatically
// extended with zero bytes. Be careful with large offsets as they can cause
// significant memory allocation.
//
// Example:
//
//	SETBIT mykey 7 1      # Set bit at offset 7 to 1
//	SETBIT mykey 0 1      # Set bit at offset 0 to 1
//	GET mykey             # Returns "\x81" (binary: 10000001)
func SetBit(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'setbit' command")
	}

	key := args[0].Blk
	offsetStr := args[1].Blk
	bitValueStr := args[2].Blk

	// Parse offset
	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return common.NewErrorValue("ERR bit offset is not an integer")
	}

	// Validate offset range (max 2^32 - 1 = 4294967295)
	if offset > 4294967295 || offset < 0 {
		return common.NewErrorValue("ERR bit offset is out of range")
	}

	// Parse bit value
	bitValue, err := strconv.Atoi(bitValueStr)
	if err != nil || (bitValue != 0 && bitValue != 1) {
		return common.NewErrorValue("ERR bit is not an integer or out of range")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		if item.Type != common.STRING_TYPE && item.Type != "" {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.ApproxMemoryUsage(key)
	} else {
		item = common.NewStringItem("")
		database.DB.Store[key] = item
	}

	// Calculate byte index and bit position within the byte
	byteIndex := int(offset / 8)
	bitPosition := uint(7 - (offset % 8)) // MSB to LSB within byte

	// Extend the string if necessary
	currentBytes := []byte(item.Str)
	if byteIndex >= len(currentBytes) {
		// Extend with zero bytes
		extension := make([]byte, byteIndex-len(currentBytes)+1)
		currentBytes = append(currentBytes, extension...)
	}

	// Get the original bit value
	originalBit := int64(0)
	if (currentBytes[byteIndex] & (1 << bitPosition)) != 0 {
		originalBit = 1
	}

	// Set or clear the bit
	if bitValue == 1 {
		currentBytes[byteIndex] |= (1 << bitPosition)
	} else {
		currentBytes[byteIndex] &^= (1 << bitPosition)
	}

	item.Str = string(currentBytes)

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

	return common.NewIntegerValue(originalBit)
}

// GetBit handles the GETBIT command.
// Returns the bit value at offset in the string value stored at key.
//
// Syntax:
//
//	GETBIT <key> <offset>
//
// Parameters:
//   - key: The key of the string to read from
//   - offset: The bit offset (0-based)
//
// Returns:
//
//	Integer: The bit value stored at offset (0 or 1)
//
// Behavior:
//   - When offset is beyond the string length, the bit is assumed to be 0
//   - If key does not exist, it is interpreted as an empty string, so offset
//     is always out of range and the value is 0
//   - The bit offset is calculated from left to right (MSB to LSB within each byte)
//
// Time Complexity: O(1)
//
// Example:
//
//	SET mykey "\x42"      # Binary: 01000010
//	GETBIT mykey 0        # Returns 0
//	GETBIT mykey 1        # Returns 1
//	GETBIT mykey 6        # Returns 1
//	GETBIT mykey 100      # Returns 0 (beyond string length)
func GetBit(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'getbit' command")
	}

	key := args[0].Blk
	offsetStr := args[1].Blk

	// Parse offset
	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil || offset < 0 {
		return common.NewErrorValue("ERR bit offset is not an integer or out of range")
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.STRING_TYPE && item.Type != "" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	// Calculate byte index and bit position
	byteIndex := int(offset / 8)
	bitPosition := uint(7 - (offset % 8))

	currentBytes := []byte(item.Str)
	if byteIndex >= len(currentBytes) {
		return common.NewIntegerValue(0)
	}

	// Get the bit value
	if (currentBytes[byteIndex] & (1 << bitPosition)) != 0 {
		return common.NewIntegerValue(1)
	}

	return common.NewIntegerValue(0)
}

// BitCount handles the BITCOUNT command.
// Count the number of set bits (population counting) in a string.
//
// Syntax:
//
//	BITCOUNT <key> [start end [BYTE|BIT]]
//
// Parameters:
//   - key: The key of the string to count bits in
//   - start: (optional) Start position (default: 0)
//   - end: (optional) End position (default: -1, meaning last byte/bit)
//   - BYTE|BIT: (optional) Whether start/end are byte or bit indices (default: BYTE)
//
// Returns:
//
//	Integer: The number of bits set to 1
//
// Behavior:
//   - By default, all the bytes contained in the string are examined
//   - It is possible to specify the counting operation only in an interval
//   - Like for the GETRANGE command, start and end can contain negative values
//   - Non-existent keys are treated as empty strings, so the command will return 0
//   - The BYTE and BIT options (Redis 7.0+) specify whether the range is in bytes or bits
//
// Time Complexity: O(N) where N is the number of bytes in the range
//
// Example:
//
//	SET mykey "foobar"
//	BITCOUNT mykey              # Count all bits
//	BITCOUNT mykey 0 0          # Count bits in first byte only
//	BITCOUNT mykey 5 30 BIT     # Count bits from bit 5 to bit 30
func BitCount(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'bitcount' command")
	}

	key := args[0].Blk
	start := int64(0)
	end := int64(-1)
	useBitIndex := false

	// Parse optional start and end
	if len(args) >= 3 {
		var err error
		start, err = strconv.ParseInt(args[1].Blk, 10, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
		end, err = strconv.ParseInt(args[2].Blk, 10, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
	}

	// Parse optional BYTE|BIT modifier
	if len(args) >= 4 {
		modifier := strings.ToUpper(args[3].Blk)
		if modifier == "BIT" {
			useBitIndex = true
		} else if modifier != "BYTE" {
			return common.NewErrorValue("ERR syntax error")
		}
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		return common.NewIntegerValue(0)
	}

	if item.Type != common.STRING_TYPE && item.Type != "" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	data := []byte(item.Str)
	dataLen := int64(len(data))

	if dataLen == 0 {
		return common.NewIntegerValue(0)
	}

	var count int64

	if useBitIndex {
		// BIT mode: start and end are bit indices
		totalBits := dataLen * 8

		// Handle negative indices
		if start < 0 {
			start = totalBits + start
		}
		if end < 0 {
			end = totalBits + end
		}

		// Clamp to valid range
		if start < 0 {
			start = 0
		}
		if end >= totalBits {
			end = totalBits - 1
		}
		if start > end {
			return common.NewIntegerValue(0)
		}

		// Count bits in the range
		for bitIdx := start; bitIdx <= end; bitIdx++ {
			byteIdx := bitIdx / 8
			bitPos := uint(7 - (bitIdx % 8))
			if (data[byteIdx] & (1 << bitPos)) != 0 {
				count++
			}
		}
	} else {
		// BYTE mode: start and end are byte indices
		// Handle negative indices
		if start < 0 {
			start = dataLen + start
		}
		if end < 0 {
			end = dataLen + end
		}

		// Clamp to valid range
		if start < 0 {
			start = 0
		}
		if end >= dataLen {
			end = dataLen - 1
		}
		if start > end {
			return common.NewIntegerValue(0)
		}

		// Count bits in the byte range
		for i := start; i <= end; i++ {
			count += int64(popcount(data[i]))
		}
	}

	return common.NewIntegerValue(count)
}

// popcount returns the number of set bits in a byte (population count)
func popcount(b byte) int {
	count := 0
	for b != 0 {
		count += int(b & 1)
		b >>= 1
	}
	return count
}

// BitOp handles the BITOP command.
// Perform a bitwise operation between multiple keys and store the result in the destination key.
//
// Syntax:
//
//	BITOP <operation> <destkey> <key> [key ...]
//
// Parameters:
//   - operation: The bitwise operation to perform (AND, OR, XOR, NOT)
//   - destkey: The key to store the result in
//   - key: One or more source keys (NOT operation takes exactly one source key)
//
// Returns:
//
//	Integer: The size of the string stored in the destination key (equal to the
//	         size of the longest input string)
//
// Behavior:
//   - AND: Performs bitwise AND between all source strings
//   - OR: Performs bitwise OR between all source strings
//   - XOR: Performs bitwise XOR between all source strings
//   - NOT: Performs bitwise NOT on a single source string
//   - When strings have different lengths, shorter strings are treated as if
//     they were zero-padded up to the length of the longest string
//   - Non-existent keys are treated as empty strings
//
// Time Complexity: O(N) where N is the size of the longest string
//
// Example:
//
//	SET key1 "foof"
//	SET key2 "foof"
//	BITOP AND destkey key1 key2    # destkey = "foof"
//	BITOP OR destkey key1 key2     # destkey = "foof"
//	BITOP XOR destkey key1 key2    # destkey = "\x00\x00\x00\x00"
//	BITOP NOT destkey key1         # destkey = bitwise NOT of key1
func BitOp(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 3 {
		return common.NewErrorValue("ERR wrong number of arguments for 'bitop' command")
	}

	operation := strings.ToUpper(args[0].Blk)
	destKey := args[1].Blk
	sourceKeys := args[2:]

	// Validate operation
	validOps := map[string]bool{"AND": true, "OR": true, "XOR": true, "NOT": true}
	if !validOps[operation] {
		return common.NewErrorValue("ERR syntax error")
	}

	// NOT operation requires exactly one source key
	if operation == "NOT" && len(sourceKeys) != 1 {
		return common.NewErrorValue("ERR BITOP NOT requires one and only one key")
	}

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	// Collect source strings and find max length
	var sources [][]byte
	maxLen := 0

	for _, keyArg := range sourceKeys {
		key := keyArg.Blk
		item, ok := database.DB.Store[key]
		if !ok {
			sources = append(sources, []byte{})
			continue
		}
		if item.Type != common.STRING_TYPE && item.Type != "" {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		data := []byte(item.Str)
		sources = append(sources, data)
		if len(data) > maxLen {
			maxLen = len(data)
		}
	}

	// If all sources are empty, result is empty
	if maxLen == 0 {
		// Store empty string
		var oldMemory int64 = 0
		if existing, ok := database.DB.Store[destKey]; ok {
			oldMemory = existing.ApproxMemoryUsage(destKey)
		}
		item := common.NewStringItem("")
		database.DB.Store[destKey] = item
		database.DB.Touch(destKey)
		newMemory := item.ApproxMemoryUsage(destKey)
		database.DB.Mem += (newMemory - oldMemory)
		return common.NewIntegerValue(0)
	}

	// Perform the operation
	result := make([]byte, maxLen)

	switch operation {
	case "AND":
		// Initialize with all 1s
		for i := range result {
			result[i] = 0xFF
		}
		for _, src := range sources {
			for i := 0; i < maxLen; i++ {
				if i < len(src) {
					result[i] &= src[i]
				} else {
					result[i] &= 0 // Zero-pad
				}
			}
		}

	case "OR":
		for _, src := range sources {
			for i := 0; i < len(src); i++ {
				result[i] |= src[i]
			}
		}

	case "XOR":
		for _, src := range sources {
			for i := 0; i < len(src); i++ {
				result[i] ^= src[i]
			}
		}

	case "NOT":
		src := sources[0]
		for i := 0; i < maxLen; i++ {
			if i < len(src) {
				result[i] = ^src[i]
			} else {
				result[i] = 0xFF // NOT of 0 is all 1s
			}
		}
	}

	// Store the result
	var oldMemory int64 = 0
	if existing, ok := database.DB.Store[destKey]; ok {
		oldMemory = existing.ApproxMemoryUsage(destKey)
	}

	item := common.NewStringItem(string(result))
	database.DB.Store[destKey] = item

	database.DB.Touch(destKey)
	newMemory := item.ApproxMemoryUsage(destKey)
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

	return common.NewIntegerValue(int64(maxLen))
}

// BitPos handles the BITPOS command.
// Return the position of the first bit set to 1 or 0 in a string.
//
// Syntax:
//
//	BITPOS <key> <bit> [start [end [BYTE|BIT]]]
//
// Parameters:
//   - key: The key of the string to search in
//   - bit: The bit value to search for (0 or 1)
//   - start: (optional) Start position (default: 0)
//   - end: (optional) End position (default: -1, meaning last byte/bit)
//   - BYTE|BIT: (optional) Whether start/end are byte or bit indices (default: BYTE)
//
// Returns:
//
//	Integer: The position of the first bit set to the specified value, or -1 if not found
//
// Behavior:
//   - The position is returned as an absolute bit position from the start of the string
//   - By default, all the bytes contained in the string are examined
//   - The start and end arguments specify a range to search within
//   - Negative values for start and end are interpreted as offsets from the end
//   - When searching for bit 0 in an empty string or non-existent key, -1 is returned
//   - When searching for bit 1 in an empty string or non-existent key, -1 is returned
//
// Time Complexity: O(N) where N is the number of bytes in the range
//
// Example:
//
//	SET mykey "\xff\xf0\x00"    # Binary: 11111111 11110000 00000000
//	BITPOS mykey 0              # Returns 12 (first 0 bit)
//	BITPOS mykey 1              # Returns 0 (first 1 bit)
//	BITPOS mykey 0 2            # Returns 16 (first 0 in byte 2)
func BitPos(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'bitpos' command")
	}

	key := args[0].Blk
	bitStr := args[1].Blk

	// Parse bit value
	bit, err := strconv.Atoi(bitStr)
	if err != nil || (bit != 0 && bit != 1) {
		return common.NewErrorValue("ERR bit is not an integer or out of range")
	}

	start := int64(0)
	end := int64(-1)
	useBitIndex := false
	hasEnd := false

	// Parse optional start
	if len(args) >= 3 {
		start, err = strconv.ParseInt(args[2].Blk, 10, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
	}

	// Parse optional end
	if len(args) >= 4 {
		end, err = strconv.ParseInt(args[3].Blk, 10, 64)
		if err != nil {
			return common.NewErrorValue("ERR value is not an integer or out of range")
		}
		hasEnd = true
	}

	// Parse optional BYTE|BIT modifier
	if len(args) >= 5 {
		modifier := strings.ToUpper(args[4].Blk)
		if modifier == "BIT" {
			useBitIndex = true
		} else if modifier != "BYTE" {
			return common.NewErrorValue("ERR syntax error")
		}
	}

	database.DB.Mu.RLock()
	defer database.DB.Mu.RUnlock()

	item, ok := database.DB.Store[key]
	if !ok {
		// Non-existent key
		if bit == 1 {
			return common.NewIntegerValue(-1)
		}
		return common.NewIntegerValue(-1)
	}

	if item.Type != common.STRING_TYPE && item.Type != "" {
		return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	data := []byte(item.Str)
	dataLen := int64(len(data))

	if dataLen == 0 {
		return common.NewIntegerValue(-1)
	}

	if useBitIndex {
		// BIT mode
		totalBits := dataLen * 8

		// Handle negative indices
		if start < 0 {
			start = totalBits + start
		}
		if end < 0 {
			end = totalBits + end
		}

		// Clamp to valid range
		if start < 0 {
			start = 0
		}
		if end >= totalBits {
			end = totalBits - 1
		}
		if start > end {
			return common.NewIntegerValue(-1)
		}

		// Search for the bit
		for bitIdx := start; bitIdx <= end; bitIdx++ {
			byteIdx := bitIdx / 8
			bitPos := uint(7 - (bitIdx % 8))
			currentBit := 0
			if (data[byteIdx] & (1 << bitPos)) != 0 {
				currentBit = 1
			}
			if currentBit == bit {
				return common.NewIntegerValue(bitIdx)
			}
		}
	} else {
		// BYTE mode
		// Handle negative indices
		if start < 0 {
			start = dataLen + start
		}
		if end < 0 {
			end = dataLen + end
		}

		// Clamp to valid range
		if start < 0 {
			start = 0
		}
		if end >= dataLen {
			end = dataLen - 1
		}
		if start > end {
			return common.NewIntegerValue(-1)
		}

		// Search for the bit in the byte range
		for byteIdx := start; byteIdx <= end; byteIdx++ {
			b := data[byteIdx]
			for bitPos := 7; bitPos >= 0; bitPos-- {
				currentBit := 0
				if (b & (1 << uint(bitPos))) != 0 {
					currentBit = 1
				}
				if currentBit == bit {
					return common.NewIntegerValue(byteIdx*8 + int64(7-bitPos))
				}
			}
		}

		// Special case: searching for 0 without explicit end
		// If we didn't find a 0 in the specified range, but no end was given,
		// the first 0 is at the position right after the string
		if bit == 0 && !hasEnd {
			return common.NewIntegerValue((end + 1) * 8)
		}
	}

	return common.NewIntegerValue(-1)
}

// BitField handles the BITFIELD command.
// Perform arbitrary bitfield integer operations on strings.
//
// Syntax:
//
//	BITFIELD <key> [GET <encoding> <offset>] [SET <encoding> <offset> <value>]
//	              [INCRBY <encoding> <offset> <increment>] [OVERFLOW <WRAP|SAT|FAIL>]
//
// Parameters:
//   - key: The key of the string to operate on
//   - GET: Get the specified bitfield
//   - SET: Set the specified bitfield and return its old value
//   - INCRBY: Increment/decrement the specified bitfield and return the new value
//   - OVERFLOW: Control overflow behavior for subsequent INCRBY operations
//
// Encoding Format:
//   - i<bits>: Signed integer with specified number of bits (e.g., i8, i16, i32)
//   - u<bits>: Unsigned integer with specified number of bits (e.g., u8, u16, u32)
//   - Maximum bits: 64 for signed, 63 for unsigned
//
// Offset Format:
//   - <number>: Absolute bit offset
//   - #<number>: Offset multiplied by encoding size (e.g., #0, #1, #2)
//
// Overflow Behaviors:
//   - WRAP: Wrap around on overflow (default)
//   - SAT: Saturate to min/max value on overflow
//   - FAIL: Return nil and don't perform the operation on overflow
//
// Returns:
//
//	Array: One element for each GET/SET/INCRBY operation performed
//
// Time Complexity: O(1) for each subcommand
//
// Example:
//
//	BITFIELD mykey SET u8 0 200 GET u8 0
//	BITFIELD mykey INCRBY u8 0 1 OVERFLOW SAT INCRBY u8 0 100
func BitField(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'bitfield' command")
	}

	key := args[0].Blk
	subcommands := args[1:]

	database.DB.Mu.Lock()
	defer database.DB.Mu.Unlock()

	var item *common.Item
	var oldMemory int64 = 0

	if existing, ok := database.DB.Store[key]; ok {
		item = existing
		if item.Type != common.STRING_TYPE && item.Type != "" {
			return common.NewErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}
		oldMemory = item.ApproxMemoryUsage(key)
	} else {
		item = common.NewStringItem("")
		database.DB.Store[key] = item
	}

	results := []common.Value{}
	overflow := "WRAP" // Default overflow behavior
	modified := false

	i := 0
	for i < len(subcommands) {
		cmd := strings.ToUpper(subcommands[i].Blk)

		switch cmd {
		case "GET":
			if i+2 >= len(subcommands) {
				return common.NewErrorValue("ERR syntax error")
			}
			encoding := subcommands[i+1].Blk
			offsetStr := subcommands[i+2].Blk
			i += 3

			signed, bits, err := parseEncoding(encoding)
			if err != nil {
				return common.NewErrorValue(err.Error())
			}

			offset, err := parseOffset(offsetStr, bits)
			if err != nil {
				return common.NewErrorValue(err.Error())
			}

			value := getBitfield([]byte(item.Str), offset, bits, signed)
			results = append(results, *common.NewIntegerValue(value))

		case "SET":
			if i+3 >= len(subcommands) {
				return common.NewErrorValue("ERR syntax error")
			}
			encoding := subcommands[i+1].Blk
			offsetStr := subcommands[i+2].Blk
			valueStr := subcommands[i+3].Blk
			i += 4

			signed, bits, err := parseEncoding(encoding)
			if err != nil {
				return common.NewErrorValue(err.Error())
			}

			offset, err := parseOffset(offsetStr, bits)
			if err != nil {
				return common.NewErrorValue(err.Error())
			}

			newValue, err := strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}

			// Get old value
			data := []byte(item.Str)
			oldValue := getBitfield(data, offset, bits, signed)

			// Set new value
			data = setBitfield(data, offset, bits, newValue)
			item.Str = string(data)
			modified = true

			results = append(results, *common.NewIntegerValue(oldValue))

		case "INCRBY":
			if i+3 >= len(subcommands) {
				return common.NewErrorValue("ERR syntax error")
			}
			encoding := subcommands[i+1].Blk
			offsetStr := subcommands[i+2].Blk
			incrStr := subcommands[i+3].Blk
			i += 4

			signed, bits, err := parseEncoding(encoding)
			if err != nil {
				return common.NewErrorValue(err.Error())
			}

			offset, err := parseOffset(offsetStr, bits)
			if err != nil {
				return common.NewErrorValue(err.Error())
			}

			increment, err := strconv.ParseInt(incrStr, 10, 64)
			if err != nil {
				return common.NewErrorValue("ERR value is not an integer or out of range")
			}

			// Get current value
			data := []byte(item.Str)
			currentValue := getBitfield(data, offset, bits, signed)

			// Calculate new value with overflow handling
			newValue, overflowed := handleOverflow(currentValue, increment, bits, signed, overflow)

			if overflow == "FAIL" && overflowed {
				results = append(results, common.Value{Typ: common.NULL})
			} else {
				data = setBitfield(data, offset, bits, newValue)
				item.Str = string(data)
				modified = true
				results = append(results, *common.NewIntegerValue(newValue))
			}

		case "OVERFLOW":
			if i+1 >= len(subcommands) {
				return common.NewErrorValue("ERR syntax error")
			}
			overflowType := strings.ToUpper(subcommands[i+1].Blk)
			i += 2

			if overflowType != "WRAP" && overflowType != "SAT" && overflowType != "FAIL" {
				return common.NewErrorValue("ERR Invalid OVERFLOW type (should be one of WRAP, SAT, FAIL)")
			}
			overflow = overflowType

		default:
			return common.NewErrorValue(fmt.Sprintf("ERR Unknown BITFIELD subcommand '%s'", cmd))
		}
	}

	if modified {
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
	}

	return common.NewArrayValue(results)
}

// parseEncoding parses a bitfield encoding string (e.g., "u8", "i16")
// Returns: signed (bool), bits (int), error
func parseEncoding(encoding string) (bool, int, error) {
	if len(encoding) < 2 {
		return false, 0, fmt.Errorf("ERR Invalid bitfield type. Use something like i16 u8. Note that u64 is not supported but i64 is")
	}

	signed := encoding[0] == 'i'
	if encoding[0] != 'i' && encoding[0] != 'u' {
		return false, 0, fmt.Errorf("ERR Invalid bitfield type. Use something like i16 u8. Note that u64 is not supported but i64 is")
	}

	bits, err := strconv.Atoi(encoding[1:])
	if err != nil || bits < 1 || bits > 64 {
		return false, 0, fmt.Errorf("ERR Invalid bitfield type. Use something like i16 u8. Note that u64 is not supported but i64 is")
	}

	// Unsigned integers can't be 64 bits (would overflow int64)
	if !signed && bits > 63 {
		return false, 0, fmt.Errorf("ERR Invalid bitfield type. Use something like i16 u8. Note that u64 is not supported but i64 is")
	}

	return signed, bits, nil
}

// parseOffset parses a bitfield offset string
// Supports absolute offsets and #N syntax (multiplied by encoding size)
func parseOffset(offsetStr string, bits int) (int64, error) {
	if len(offsetStr) == 0 {
		return 0, fmt.Errorf("ERR bit offset is not an integer or out of range")
	}

	if offsetStr[0] == '#' {
		// Multiplied offset
		multiplier, err := strconv.ParseInt(offsetStr[1:], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("ERR bit offset is not an integer or out of range")
		}
		return multiplier * int64(bits), nil
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("ERR bit offset is not an integer or out of range")
	}
	return offset, nil
}

// getBitfield reads a bitfield value from a byte slice
func getBitfield(data []byte, offset int64, bits int, signed bool) int64 {
	// Ensure we have enough bytes
	requiredBytes := int((offset + int64(bits) + 7) / 8)
	if len(data) < requiredBytes {
		// Extend with zeros for reading
		extended := make([]byte, requiredBytes)
		copy(extended, data)
		data = extended
	}

	var value int64 = 0

	// Read bits one by one
	for i := 0; i < bits; i++ {
		bitOffset := offset + int64(i)
		byteIdx := bitOffset / 8
		bitPos := uint(7 - (bitOffset % 8))

		if byteIdx < int64(len(data)) && (data[byteIdx]&(1<<bitPos)) != 0 {
			value |= (1 << uint(bits-1-i))
		}
	}

	// Handle sign extension for signed integers
	if signed && bits < 64 {
		// Check if the sign bit is set
		signBit := int64(1) << uint(bits-1)
		if (value & signBit) != 0 {
			// Sign extend
			mask := int64(-1) << uint(bits)
			value |= mask
		}
	}

	return value
}

// setBitfield writes a bitfield value to a byte slice
func setBitfield(data []byte, offset int64, bits int, value int64) []byte {
	// Ensure we have enough bytes
	requiredBytes := int((offset + int64(bits) + 7) / 8)
	if len(data) < requiredBytes {
		extended := make([]byte, requiredBytes)
		copy(extended, data)
		data = extended
	}

	// Write bits one by one
	for i := 0; i < bits; i++ {
		bitOffset := offset + int64(i)
		byteIdx := bitOffset / 8
		bitPos := uint(7 - (bitOffset % 8))

		// Get the bit from value (MSB first)
		valueBit := (value >> uint(bits-1-i)) & 1

		if valueBit == 1 {
			data[byteIdx] |= (1 << bitPos)
		} else {
			data[byteIdx] &^= (1 << bitPos)
		}
	}

	return data
}

// handleOverflow handles overflow for INCRBY operations
// Returns the new value and whether overflow occurred
func handleOverflow(current, increment int64, bits int, signed bool, overflowType string) (int64, bool) {
	newValue := current + increment

	var minVal, maxVal int64
	if signed {
		minVal = -(1 << uint(bits-1))
		maxVal = (1 << uint(bits-1)) - 1
	} else {
		minVal = 0
		maxVal = (1 << uint(bits)) - 1
	}

	overflowed := newValue < minVal || newValue > maxVal

	switch overflowType {
	case "WRAP":
		// Wrap around
		if signed {
			// For signed, wrap within the range
			rangeSize := int64(1) << uint(bits)
			newValue = ((newValue-minVal)%rangeSize+rangeSize)%rangeSize + minVal
		} else {
			// For unsigned, simple modulo
			rangeSize := int64(1) << uint(bits)
			newValue = ((newValue % rangeSize) + rangeSize) % rangeSize
		}
	case "SAT":
		// Saturate at min/max
		if newValue < minVal {
			newValue = minVal
		} else if newValue > maxVal {
			newValue = maxVal
		}
	case "FAIL":
		// Return original value if overflow (caller handles nil)
		if overflowed {
			return current, true
		}
	}

	return newValue, overflowed
}
