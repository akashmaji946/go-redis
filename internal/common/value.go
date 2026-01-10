/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/common/value.go
*/
package common

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"
)

// ValueType represents the type of a RESP (Redis Serialization Protocol) value.
// Each type corresponds to a specific RESP protocol prefix character.
type ValueType string

// RESP protocol line ending constants used for parsing and serialization.
// These constants define the standard line termination used in the RESP protocol.
const (
	CARRIAGE_RETURN string = "\r"   // Carriage return character
	NEW_LINE        string = "\n"   // Newline character
	EOD             string = "\r\n" // End of data marker (carriage return + newline)
)

// RESP protocol value type constants.
// These constants define the prefix characters used in the RESP protocol
// to identify different value types in serialized format.
const (
	BULK   ValueType = "$" // Bulk string: $<length>\r\n<data>\r\n
	STRING ValueType = "+" // Simple string: +<data>\r\n
	ARRAY  ValueType = "*" // Array: *<count>\r\n<elements>...

	ERROR ValueType = "-" // Error: -<error message>\r\n
	NULL  ValueType = ""  // Null: $-1\r\n (represented as empty string)

	INTEGER ValueType = ":" // Integer: :<number>\r\n
)

// Item represents a value stored in the database along with its expiration time.
// This structure allows the database to support key expiration functionality and multiple data types.
//
// Fields:
//
//	-Type: The data type of the value (e.g., "string", "hash", "list", "set", "zset")
//
// -Str: The actual string value stored in the database
// -Int: Integer value stored in the database
// -Bool: Boolean value stored in the database
// -Float: Float value stored in the database
// -Hash: A map representing a hash data type, where each field is itself an Item (supports per-field expiration)
// -List: A slice representing a list data type (for future list support)
// -ItemSet: A map representing a set data type (for future set support)
// -ZSet: A map representing a sorted set data type (for future sorted set support)
//
//   - Exp: The expiration time for this key-value pair
//     If exp is the zero time (time.Time{}), the key has no expiration
//   - LastAccessed: The time when the key was last accessed
//   - AccessCount: The number of times the key was accessed
type Item struct {
	Type string // Data type: "string", "int", "bool", "float", "hash", "list", "set", "zset"

	Str   string  // String value
	Int   int64   // Integer value
	Bool  bool    // Boolean value
	Float float64 // Float value

	Hash    map[string]*Item   // Hash type: field -> Item (each field can have expiration)
	List    []string           // List type (future)
	ItemSet map[string]bool    // Set type (future)
	ZSet    map[string]float64 // Sorted set type (future)

	Exp          time.Time
	LastAccessed time.Time
	AccessCount  int
}

// Value represents a parsed RESP protocol value.
// This structure can hold different types of values depending on the Typ field.
// Only the relevant field should be populated based on the value type.
//
// Fields:
//   - Typ: The type of the value (BULK, STRING, ARRAY, ERROR, NULL, INTEGER)
//   - Blk: Used for BULK string values (the actual string data)
//   - str: Used for STRING values (simple string data)
//   - Arr: Used for ARRAY values (array of Value elements)
//   - err: Used for ERROR values (the error message)
//   - nul: Used for NULL values (typically empty)
//   - num: Used for INTEGER values (the numeric value)
//
// Usage:
//   - For BULK: Typ = BULK, Blk contains the string
//   - For STRING: Typ = STRING, str contains the string
//   - For ARRAY: Typ = ARRAY, Arr contains the array elements
//   - For ERROR: Typ = ERROR, err contains the error message
//   - For NULL: Typ = NULL
//   - For INTEGER: Typ = INTEGER, num contains the number
type Value struct {
	Typ ValueType

	Blk string
	Str string
	Arr []Value

	Err string
	Nul string

	Num int
}

// ReadLine reads a line from a buffered reader and removes the trailing "\r\n".
// This is a helper function for parsing RESP protocol messages which are
// line-oriented and terminated with "\r\n".
//
// Parameters:
//   - reader: A buffered reader to read from
//
// Returns:
//   - string: The line content without the trailing "\r\n"
//   - error: Any error encountered during reading
//
// Behavior:
//   - Reads until a newline character is encountered
//   - Removes the trailing "\r\n" from the line
//   - Returns an error if reading fails
//
// Example:
//
//	Input from reader: "*3\r\n"
//	Returns: "*3", nil
func ReadLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n') // line = *123\r\n
	if err != nil {
		return "", err
	}
	trimmedLine := strings.TrimSuffix(line, "\r\n") // line = *123
	return trimmedLine, nil
}

// ReadBulk reads a bulk string value from a buffered reader.
// Parses a RESP bulk string in the format: $<length>\r\n<data>\r\n
//
// Parameters:
//   - reader: A buffered reader to read from
//
// Returns:
//   - Value: A Value struct with Typ=BULK and Blk containing the string data
//     Returns NULL value on error
//   - Error: an error
//
// Behavior:
//   - Reads the length prefix (e.g., "$5" means 5 bytes)
//   - Reads exactly <length> bytes of data plus the trailing "\r\n"
//   - Extracts the actual data without the trailing "\r\n"
//   - Returns NULL value if parsing fails or format is invalid
//
// RESP Format:
//
//	$<length>\r\n
//	<data>\r\n
//
// Example:
//
//	Input: "$5\r\nhello\r\n"
//	Returns: Value{Typ: BULK, Blk: "hello"}
func (v *Value) ReadBulk(reader *bufio.Reader) (Value, error) {

	line, err := ReadLine(reader) // line = $1123
	if err != nil {
		log.Println("error in ReadBulk:", err)
		return Value{
			Typ: NULL,
		}, err
	}
	if line[0] != '$' {
		err := fmt.Errorf("must have $ with readbulk")
		log.Println("error in ReadBulk:", err)
		return Value{
			Typ: NULL,
		}, err
	}

	n, err := strconv.Atoi(string(line[1:])) // "1123"
	if err != nil {
		log.Fatal("can't convert to get bulk length")
	}

	bulkDataBuffer := make([]byte, n+2) // data + \r\n

	// read till filling the buffer
	_, err = io.ReadFull(reader, bulkDataBuffer)
	if err != nil {
		log.Println("error in ReadBulk:", err)
		return Value{
			Typ: NULL,
		}, err
	}
	bulkData := string(bulkDataBuffer[:n]) // data without \r\n

	return Value{
		Typ: BULK,
		Blk: bulkData,
	}, nil

}

// ReadArray reads an array value from an io.Reader.
// Parses a RESP array in the format: *<count>\r\n<element1><element2>...
// where each element is a bulk string.
//
// Parameters:
//   - r: An io.Reader to read from (typically a network connection)
//
// Returns:
//   - error: nil on success, error if parsing fails
//
// Behavior:
//   - Reads the array length prefix (e.g., "*3" means 3 elements)
//   - Reads each element as a bulk string using ReadBulk
//   - Appends each parsed element to v.Arr
//   - Sets v.Typ to ARRAY implicitly (through the array being populated)
//
// RESP Format:
//
//	*<count>\r\n
//	$<len1>\r\n<data1>\r\n
//	$<len2>\r\n<data2>\r\n
//	...
//
// Example:
//
//	Input: "*2\r\n$3\r\nGET\r\n$4\r\nname\r\n"
//	Parses into: Value{Typ: ARRAY, Arr: [
//	  Value{Typ: BULK, Blk: "GET"},
//	  Value{Typ: BULK, Blk: "name"}
//	]}
//
// Note: This method is typically used to parse Redis commands from clients,
//
//	where the first element is the command name and subsequent elements are arguments.
func (v *Value) ReadArray(reader *bufio.Reader) error {

	// reader := bufio.NewReader(r) // creates problem with aof file sync
	line, err := ReadLine(reader) // line = *123
	if err != nil {
		log.Println("error in ReadArray:", err)
		return err
	}
	if line[0] != '*' {
		log.Println("error in ReadArray:", err)
		return fmt.Errorf("invalid input by user")
	}

	arrLen, err := strconv.Atoi(line[1:]) // pass "123"
	if err != nil {
		log.Println("can't convert to get array length")
		return err
	}

	for range arrLen {
		bulk, err := v.ReadBulk(reader)
		if err != nil {
			log.Printf("can't proceed with readbulk")
			break
		}
		v.Arr = append(v.Arr, bulk)
	}
	return nil
}

// IsExpired checks if the item is expired.
// Returns true if the item is expired, false otherwise.
func (item *Item) IsExpired() bool {
	zeroTime := time.Time{}
	return item.Exp != zeroTime && time.Until(item.Exp).Seconds() <= 0
}

// ApproxMemoryUsage calculates approximate memory usage of an Item
func (item *Item) ApproxMemoryUsage(key string) int64 {
	const (
		stringHeader        = 16 // Go string header: pointer + length
		pointerSize         = 8  // *Item pointer on 64-bit arch
		avgMapEntryOverhead = 18 // amortized per-entry overhead in Go maps
	)

	var size int64

	// map[string]*Item entry
	size += stringHeader + int64(len(key)) // key string
	size += pointerSize                    // pointer to Item
	size += avgMapEntryOverhead            // map bucket overhead

	// Item struct itself
	size += int64(56) // approximate size of Item struct

	// String values inside Item
	size += int64(len(item.Str))
	size += int64(len(item.Type))

	// Hash map inside Item
	if item.Type == HASH_TYPE && item.Hash != nil {
		for k, fieldItem := range item.Hash {
			if fieldItem == nil {
				continue
			}
			size += stringHeader + int64(len(k)) // field name
			size += pointerSize                  // pointer to field Item
			size += avgMapEntryOverhead          // map entry overhead
			size += int64(len(fieldItem.Str))    // field value
		}
	}

	// Set type
	if item.Type == SET_TYPE {
		for k := range item.ItemSet {
			size += stringHeader + int64(len(k))
			size += 1 // bool value
			size += avgMapEntryOverhead
		}
	}

	// ZSet type
	if item.Type == ZSET_TYPE {
		for k := range item.ZSet {
			size += stringHeader + int64(len(k))
			size += 8 // float64 value
			size += avgMapEntryOverhead
		}
	}

	return size
}
