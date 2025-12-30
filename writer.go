package main

import (
	"bufio"
	"fmt"
	"io"
)

// Writer provides functionality to serialize Value structures into RESP protocol format
// and write them to an underlying io.Writer (typically a network connection or file).
//
// The Writer wraps the underlying writer with a bufio.Writer for efficient buffered writes,
// which improves performance by reducing system calls.
//
// Fields:
//   - writer: The underlying io.Writer, which is wrapped in a bufio.Writer
//     for buffered output. Can be a network connection, file, or any io.Writer.
//
// Usage:
//   - Create with NewWriter() passing an io.Writer (e.g., network connection, file)
//   - Use Write() to serialize and write a Value
//   - Use Flush() to ensure all buffered data is written to the underlying writer
//
// Thread Safety:
//   - Not thread-safe. Each Writer should be used by a single goroutine,
//     or external synchronization should be provided if used concurrently.
type Writer struct {
	writer io.Writer
}

// NewWriter creates a new Writer instance that wraps the provided io.Writer
// with a buffered writer for efficient output.
//
// Parameters:
//   - w: The underlying io.Writer to write to (e.g., net.Conn, *os.File, bytes.Buffer)
//
// Returns: A pointer to a new Writer instance with buffered output enabled
//
// Behavior:
//   - Wraps the provided writer with bufio.NewWriter() for buffering
//   - Buffering reduces system calls and improves write performance
//   - The buffered writer must be flushed explicitly using Flush() when needed
//
// Example:
//
//	conn, _ := net.Dial("tcp", "localhost:6379")
//	writer := NewWriter(conn)
//	// Use writer to send RESP protocol messages
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer: bufio.NewWriter(w),
	}
}

// Deserialize converts a Value structure into its RESP protocol string representation.
// This is the serialization function that formats values according to the RESP specification.
//
// Parameters:
//   - v: The Value structure to serialize
//
// Returns: A string containing the RESP protocol representation of the value
//
// RESP Format Conversion:
//   - ARRAY:   "*<count>\r\n<element1><element2>..." (recursively serializes elements)
//   - STRING:  "+<data>\r\n"
//   - BULK:    "$<length>\r\n<data>\r\n"
//   - ERROR:   "-<error message>\r\n"
//   - NULL:    "$-1\r\n" (special case for null values)
//   - INTEGER: ":<number>\r\n"
//
// Behavior:
//   - Recursively serializes ARRAY elements by calling Deserialize on each element
//   - Uses the appropriate RESP format based on the Value's type
//   - Returns an empty string for invalid types (logs error but doesn't fail)
//
// Examples:
//
//	Value{typ: STRING, str: "OK"}           -> "+OK\r\n"
//	Value{typ: BULK, blk: "hello"}         -> "$5\r\nhello\r\n"
//	Value{typ: INTEGER, num: 42}           -> ":42\r\n"
//	Value{typ: ERROR, err: "ERR message"}  -> "-ERR message\r\n"
//	Value{typ: NULL}                        -> "$-1\r\n"
//
// Note: This method does not write to any output - it only converts to string format.
//
//	Use Write() to both serialize and write the value.
func (w *Writer) Deserialize(v *Value) (reply string) {

	switch v.typ {
	case ARRAY:
		reply = fmt.Sprintf("%s%d%s", v.typ, len(v.arr), EOD)
		for _, val := range v.arr {
			reply += w.Deserialize(&val)
		}
	case STRING:
		reply = fmt.Sprintf("%s%s%s", v.typ, v.str, EOD) //+TATABYE\r\n
	case BULK:
		reply = fmt.Sprintf("%s%d%s%s%s", v.typ, len(v.blk), EOD, v.blk, EOD) //$5\r\nAkash\r\n
	case ERROR:
		reply = fmt.Sprintf("%s%s%s", v.typ, v.err, EOD) //-ERR msg\r\n
	case NULL:
		reply = fmt.Sprintf("%s%d%s", "$", -1, EOD) //$-1\r\n
	case INTEGER:
		reply = fmt.Sprintf("%s%d%s", v.typ, v.num, EOD) //:12\r\n
	default:
		fmt.Errorf("invalid typ given to Deserialize")
		return reply
	}

	return reply
}

// Write serializes a Value structure and writes it to the underlying writer.
// This method combines serialization (via Deserialize) and writing in one operation.
//
// Parameters:
//   - v: The Value structure to serialize and write
//
// Behavior:
//   - Serializes the Value using Deserialize() to get the RESP protocol string
//   - Writes the serialized string to the buffered writer
//   - Does NOT automatically flush - data may remain in the buffer
//   - Call Flush() explicitly to ensure data is written to the underlying writer
//
// Buffering:
//   - Data is written to an internal buffer first
//   - The buffer is flushed when:
//   - Flush() is called explicitly
//   - The buffer becomes full (automatic flush)
//   - The underlying writer is closed
//
// Usage Pattern:
//
//	writer.Write(&value)  // Serialize and buffer
//	writer.Flush()         // Ensure data is sent (if needed immediately)
//
// Note: For AOF persistence, flushing behavior depends on appendfsync configuration:
//   - "always": Flush after each write
//   - "everysec": Flush every second (background goroutine)
//   - "no": Let OS decide when to flush
func (w *Writer) Write(v *Value) {
	reply := w.Deserialize(v)
	// write and flush reply
	w.writer.Write([]byte(reply))
	// w.Flush() // flush now depends on sync state
}

// Flush forces all buffered data to be written to the underlying writer.
// This ensures that any data written via Write() is actually sent to the destination
// (network connection, file, etc.) rather than remaining in the buffer.
//
// Behavior:
//   - Flushes the internal bufio.Writer buffer
//   - Ensures all previously written data is sent to the underlying io.Writer
//   - Blocks until all buffered data is written
//
// When to use:
//   - After critical writes that must be sent immediately
//   - Before closing a connection to ensure all data is sent
//   - When using AOF with "always" fsync mode (flush after each write)
//   - Periodically when using "everysec" mode (background goroutine handles this)
//
// Performance:
//   - Flushing too frequently can reduce performance (more system calls)
//   - Not flushing can cause data loss if the process crashes
//   - Balance between performance and durability based on use case
//
// Example:
//
//	writer.Write(&responseValue)
//	writer.Flush()  // Ensure response is sent to client immediately
//
// Note: The underlying writer must be a *bufio.Writer (which it is, created in NewWriter).
//
//	This method performs a type assertion to access the Flush() method.
func (w *Writer) Flush() {
	w.writer.(*bufio.Writer).Flush()
}
