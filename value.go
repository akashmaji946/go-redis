package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
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

// Value represents a parsed RESP protocol value.
// This structure can hold different types of values depending on the typ field.
// Only the relevant field should be populated based on the value type.
//
// Fields:
//   - typ: The type of the value (BULK, STRING, ARRAY, ERROR, NULL, INTEGER)
//   - blk: Used for BULK string values (the actual string data)
//   - str: Used for STRING values (simple string data)
//   - arr: Used for ARRAY values (array of Value elements)
//   - err: Used for ERROR values (the error message)
//   - nul: Used for NULL values (typically empty)
//   - num: Used for INTEGER values (the numeric value)
//
// Usage:
//   - For BULK: typ = BULK, blk contains the string
//   - For STRING: typ = STRING, str contains the string
//   - For ARRAY: typ = ARRAY, arr contains the array elements
//   - For ERROR: typ = ERROR, err contains the error message
//   - For NULL: typ = NULL
//   - For INTEGER: typ = INTEGER, num contains the number
type Value struct {
	typ ValueType

	blk string
	str string
	arr []Value

	err string
	nul string

	num int
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
//   - Value: A Value struct with typ=BULK and blk containing the string data
//     Returns NULL value on error
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
//	Returns: Value{typ: BULK, blk: "hello"}
func (v *Value) ReadBulk(reader *bufio.Reader) Value {

	line, err := ReadLine(reader) // line = $1123
	if err != nil {
		fmt.Println("error in ReadBulk:", err)
		return Value{
			typ: NULL,
		}
	}
	if line[0] != '$' {
		fmt.Println("error in ReadBulk:", err)
		return Value{
			typ: NULL,
		}
	}

	n, err := strconv.Atoi(string(line[1:])) // "1123"
	if err != nil {
		log.Fatal("can't convert to get bulk length")
	}

	bulkDataBuffer := make([]byte, n+2) // data + \r\n

	// read till filling the buffer
	_, err = io.ReadFull(reader, bulkDataBuffer)
	if err != nil {
		fmt.Println("error in ReadBulk:", err)
		return Value{
			typ: NULL,
		}
	}
	bulkData := string(bulkDataBuffer[:n]) // data without \r\n

	return Value{
		typ: BULK,
		blk: bulkData,
	}

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
//   - Appends each parsed element to v.arr
//   - Sets v.typ to ARRAY implicitly (through the array being populated)
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
//	Parses into: Value{typ: ARRAY, arr: [
//	  Value{typ: BULK, blk: "GET"},
//	  Value{typ: BULK, blk: "name"}
//	]}
//
// Note: This method is typically used to parse Redis commands from clients,
//
//	where the first element is the command name and subsequent elements are arguments.
func (v *Value) ReadArray(r io.Reader) error {

	reader := bufio.NewReader(r)
	line, err := ReadLine(reader) // line = *123
	if err != nil {
		fmt.Println("error in ReadArray:", err)
		return err
	}
	if line[0] != '*' {
		fmt.Println("error in ReadArray:", err)
		return fmt.Errorf("invalid input by user")
	}

	arrLen, err := strconv.Atoi(line[1:]) // pass "123"
	if err != nil {
		log.Println("can't convert to get array length")
		return err
	}

	for range arrLen {
		bulk := v.ReadBulk(reader)
		v.arr = append(v.arr, bulk)
	}
	return nil
}
