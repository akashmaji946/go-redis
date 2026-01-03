package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
)

// Aof manages the Append-Only File (AOF) persistence mechanism.
// AOF logs every write operation to a file, allowing the database to be restored
// by replaying all commands on server startup.
//
// Fields:
//   - w: A Writer instance used to serialize and write commands to the AOF file
//   - f: The underlying file handle for the AOF file
//   - config: Reference to the server configuration containing AOF settings
//
// AOF Benefits:
//   - Provides better durability than RDB (every write is logged)
//   - Can recover up to 1 second of data loss (depending on fsync mode)
//   - Commands are logged in a human-readable format
//
// AOF Trade-offs:
//   - AOF files are typically larger than RDB snapshots
//   - Replay can be slower than loading an RDB snapshot
//   - Requires periodic rewriting (BGREWRITEAOF) to optimize file size
//
// Thread Safety:
//   - AOF operations should be synchronized if accessed from multiple goroutines
//   - The Writer and File handle are not thread-safe by themselves
type Aof struct {
	w      *Writer
	f      *os.File
	config *Config
}

// NewAof creates and initializes a new AOF instance.
// Opens or creates the AOF file based on configuration settings.
//
// Parameters:
//   - config: The server configuration containing AOF settings (dir, aofFn)
//
// Returns: A pointer to a new Aof instance with the file opened and ready for writing
//
// Behavior:
//   - Constructs the AOF file path by joining config.dir and config.aofFn
//   - Opens the file in append mode (os.O_APPEND) to preserve existing data
//   - Creates the file if it doesn't exist (os.O_CREATE)
//   - Opens in read-write mode (os.O_RDWR) to support both reading (Synchronize)
//     and writing (command logging)
//   - Sets file permissions to 0644 (readable by all, writable by owner)
//   - Wraps the file with a Writer for buffered, serialized output
//
// Error Handling:
//   - If file cannot be opened, prints error and returns Aof with nil file
//     (operations will fail gracefully)
//
// File Path:
//   - Constructed as: <config.dir>/<config.aofFn>
//   - Example: "./data/backup.aof"
//
// Note: The file is kept open for the lifetime of the Aof instance.
//
//	Close operations are typically handled by the OS when the process exits.
func NewAof(config *Config) *Aof {
	aof := Aof{
		config: config,
	}
	fp := path.Join(aof.config.dir, aof.config.aofFn)                  //filepath
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644) // append only + readwrite // perm = -rw-r--r--

	if err != nil {
		fmt.Println("Can't open this file path")
		return &aof
	}

	aof.w = NewWriter(f)
	aof.f = f

	return &aof
}

// Synchronize reads and replays all commands from the AOF file to restore the database state.
// This is called on server startup to rebuild the in-memory database from the AOF log.
//
// Behavior:
//   - Reads the AOF file sequentially, parsing each command as a RESP array
//   - Replays each command by calling the Set handler with a blank state
//   - Continues until EOF is reached (end of file)
//   - Logs the total number of records synchronized
//
// Process:
//  1. Reads each command from the AOF file using ReadArray()
//  2. Creates a blank AppState (no AOF, no RDB) to prevent recursive logging
//  3. Creates a dummy Client (not used during replay)
//  4. Executes the Set command to restore the key-value pair
//  5. Repeats until all commands are processed
//
// Error Handling:
//   - On EOF: Normal termination, all commands processed
//   - On other errors: Logs error and stops synchronization
//   - Partial recovery: Commands processed before error are applied
//
// Performance:
//   - Sequential file read (efficient for append-only files)
//   - Each command is parsed and executed individually
//   - Slower than RDB snapshot loading but provides complete recovery
//
// Use Cases:
//   - Server startup: Restore database from AOF
//   - After AOF rewrite: Verify rewritten file is valid
//
// Note: Only SET commands are expected in the AOF file during normal operation.
//
//	Other commands may cause unexpected behavior during replay.
func (aof *Aof) Synchronize(mem *Mem) {
	reader := bufio.NewReader(aof.f)
	total := 0
	for {
		v := Value{}
		err := v.ReadArray(reader)
		if err == io.EOF {
			break
		}
		if err != nil { // can't sync
			fmt.Println("Unexpected error while sync", err)
			break
		}

		config := Config{
			maxmemory:        mem.maxmemory,
			maxmemorySamples: mem.maxmemorySamples,
			eviction:         Eviction(mem.evictionPolicy),
		}
		blankState := NewAppState(&config) // new blank state with empty config
		blankClient := Client{}            // dummy

		Set(&blankClient, &v, blankState)

		total += 1
		// fmt.Println(v)
	}
	log.Printf("records synchronized: %d\n", total)

}

// Rewrite performs an AOF rewrite operation to optimize the file size.
// This is typically called by BGREWRITEAOF command to compact the AOF file
// by removing redundant commands and creating a minimal representation.
//
// Parameters:
//   - cp: A copy of the current database state (map of key-value pairs)
//     This should be a snapshot taken before the rewrite begins
//
// Behavior:
//  1. Redirects new writes to a buffer (commands arriving during rewrite)
//  2. Truncates the existing AOF file to start fresh
//  3. Writes SET commands for all keys in the database copy
//  4. Appends any commands that arrived during the rewrite (from buffer)
//  5. Restores normal AOF writing to the file
//
// Rewrite Process:
//
//   - Phase 1: Redirect writes to buffer
//
//   - Changes aof.w to write to a bytes.Buffer instead of the file
//
//   - New SET commands during rewrite are buffered
//
//   - Phase 2: Rewrite file with current state
//
//   - Truncates AOF file to zero length
//
//   - Seeks to beginning of file
//
//   - Writes SET commands for each key-value pair in the copy
//
//   - Flushes to ensure data is written
//
//   - Phase 3: Append buffered commands
//
//   - Writes any commands that arrived during rewrite to the file
//
//   - Syncs to disk to ensure durability
//
//   - Phase 4: Restore normal operation
//
//   - Changes aof.w back to writing to the file
//
//   - Future commands are written directly to the file
//
// Benefits:
//   - Removes redundant SET commands (only latest value per key)
//   - Reduces AOF file size significantly
//   - Improves replay performance
//   - Maintains data integrity (no commands lost)
//
// Thread Safety:
//   - Should be called from a single goroutine (typically BGREWRITEAOF handler)
//   - Database copy should be taken with proper locking before calling
//
// Error Handling:
//   - Truncate errors: Logs and returns (rewrite fails, file unchanged)
//   - Seek errors: Logs and returns (rewrite fails)
//   - Write errors: Logs and returns (partial rewrite may have occurred)
//   - Sync errors: Logs and returns (data may not be on disk)
//
// Example:
//
//	// In BGREWRITEAOF handler:
//	DB.mu.RLock()
//	cp := make(map[string]*VAL, len(DB.store))
//	maps.Copy(cp, DB.store)
//	DB.mu.RUnlock()
//	state.aof.Rewrite(cp)
//
// Note: This operation is typically performed in the background to avoid
//
//	blocking the server. The file is rewritten atomically (old file
//	is replaced only after successful rewrite).
func (aof *Aof) Rewrite(cp map[string]*Item) {
	// future SET commands will go to to buffer
	var b bytes.Buffer
	aof.w = NewWriter(&b) // writer to buffer

	// we have copy of DB in cp, so remoev file
	err := aof.f.Truncate(0)
	if err != nil {
		log.Println("ERR AOF Rewrite issue! Can't Truncate")
		return
	}
	_, err = aof.f.Seek(0, 0)
	if err != nil {
		log.Println("ERR AOF Rewrite issue! Can't Seek")
		return
	}

	// write all k, v as SET k, v into truncated file(no duplicates!)
	fwriter := NewWriter(aof.f) // writer to file
	for k, v := range cp {
		cmd := Value{typ: BULK, blk: "SET"}
		key := Value{typ: BULK, blk: k}       // string
		value := Value{typ: BULK, blk: v.Str} // actual string

		arr := Value{typ: ARRAY, arr: []Value{cmd, key, value}}
		fwriter.Write(&arr)
	}
	fwriter.Flush()
	log.Println("done BGREWRITE.")

	// if buffer b is not empty, write it as well
	if _, err := b.WriteTo(aof.f); err != nil {
		log.Println("ERR AOF Rewrite issue! Can't append buffered commands:", err)
		return
	} else if err := aof.f.Sync(); err != nil {
		log.Println("ERR AOF Rewrite issue! Can't sync after appending buffer:", err)
		return
	}

	// if b.Len() > 0 {
	// 	// Flush the writer to ensure all buffered data is in the bytes.Buffer
	// 	aof.w.Flush()

	// 	// Append the buffered commands to the file
	// 	_, err = aof.f.Write(b.Bytes())
	// 	if err != nil {
	// 		log.Println("ERR AOF Rewrite issue! Can't append buffered commands:", err)
	// 		return
	// 	}

	// 	// Sync to ensure data is written to disk
	// 	if err := aof.f.Sync(); err != nil {
	// 		log.Println("ERR AOF Rewrite issue! Can't sync after appending buffer:", err)
	// 		return
	// 	}
	// }

	// rewrite to file
	aof.w = NewWriter(aof.f)

}
