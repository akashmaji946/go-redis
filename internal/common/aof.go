/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/aof.go
*/
package common

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
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
	W      *Writer
	F      *os.File
	Config *Config
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
//   - Constructs the AOF file path by joining config.Dir and config.AofFn
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
//   - Constructed as: <config.Dir>/<config.AofFn>
//   - Example: "./data/backup.aof"
//
// Note: The file is kept open for the lifetime of the Aof instance.
//
//	Close operations are typically handled by the OS when the process exits.
func NewAof(config *Config, dbID int) *Aof {
	aof := Aof{
		Config: config,
	}
	filename := fmt.Sprintf("%s%d.aof", config.AofFn, dbID)
	fp := path.Join(aof.Config.Dir, filename)                          //filepath
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644) // append only + readwrite // perm = -rw-r--r--

	if err != nil {
		fmt.Println("Can't open this file path")
		return &aof
	}

	var writer io.Writer = f
	if config.Encrypt {
		key := sha256.Sum256([]byte(config.Nonce))
		block, _ := aes.NewCipher(key[:])
		// Note: In production, use a unique IV stored in the file header
		iv := make([]byte, aes.BlockSize)
		stream := cipher.NewCTR(block, iv)
		writer = &cipher.StreamWriter{S: stream, W: f}
	}

	aof.W = NewWriter(writer)
	aof.F = f

	return &aof
}

// Synchronize reads and replays all commands from the AOF file to restore the database state.
// This is called on server startup to rebuild the in-memory database from the AOF log.
//
// Behavior:
//   - Reads the AOF file sequentially, parsing each command as a RESP array
//   - Replays each command using the appropriate handler (SET, HSET, HDEL, HMSET, HINCRBY, etc.)
//   - Continues until EOF is reached (end of file)
//   - Logs the total number of records synchronized
//
// Supported Commands:
//   - SET, GET, DEL, EXISTS, KEYS: String/key operations
//   - HSET, HGET, HDEL, HGETALL, HMSET, HINCRBY: Hash operations
//   - EXPIRE, TTL: Key expiration
//
// Process:
//  1. Reads each command from the AOF file using ReadArray()
//  2. Creates a blank AppState (no AOF, no RDB) to prevent recursive logging
//  3. Creates a dummy Client (not used during replay)
//  4. Looks up the command handler and executes it
//  5. Repeats until all commands are processed
//
// Error Handling:
//   - On EOF: Normal termination, all commands processed
//   - On other errors: Logs error and stops synchronization
//   - Partial recovery: Commands processed before error are applied
//   - Unknown commands: Logs warning and skips
//
// Performance:
//   - Sequential file read (efficient for append-only files)
//   - Each command is parsed and executed individually
//   - Slower than RDB snapshot loading but provides complete recovery
//
// Use Cases:
//   - Server startup: Restore database from AOF
//   - After AOF rewrite: Verify rewritten file is valid
func (aof *Aof) Synchronize(state *AppState, handler func(*Client, *Value, *AppState) *Value) {
	aof.F.Seek(0, 0)
	var r io.Reader = aof.F
	if state.Config.Encrypt {
		key := sha256.Sum256([]byte(state.Config.Nonce))
		block, _ := aes.NewCipher(key[:])
		iv := make([]byte, aes.BlockSize)
		stream := cipher.NewCTR(block, iv)
		r = &cipher.StreamReader{S: stream, R: aof.F}
	}
	reader := bufio.NewReader(r)
	total := 0

	// Disable AOF writing during synchronization to prevent recursive command logging
	originalAofEnabled := state.Config.AofEnabled
	state.Config.AofEnabled = false
	defer func() { state.Config.AofEnabled = originalAofEnabled }()

	// create a dummy client used during replay. Mark it authenticated so that
	// commands are executed even if the server requires authentication.
	dummyClient := &Client{Authenticated: true}

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

		if len(v.Arr) > 0 {
			handler(dummyClient, &v, state)
		}

		total += 1
	}
	log.Printf("records synchronized: %d\n", total)

}

// Rewrite performs an AOF rewrite operation to optimize the file size.
// This is typically called by BGREWRITEAOF command to compact the AOF file
// by removing redundant commands and creating a minimal representation.
//
// Parameters:
//   - cp: A copy of the current database state (map[string]*Item)
//     This should be a snapshot taken before the rewrite begins
//
// Data Type Support:
//   - String: Written as SET key value
//   - Hash: Written as HSET key field value [field value ...]
//   - List: Written as LPUSH key value [value ...] (if List support added)
//   - Set: Written as SADD key member [member ...] (if Set support added)
//   - Zset: Written as ZADD key score member [score member ...] (if Zset support added)
//
// Behavior:
//  1. Redirects new writes to a buffer (commands arriving during rewrite)
//  2. Truncates the existing AOF file to start fresh
//  3. Writes appropriate commands for all keys in the database copy
//  4. Appends any commands that arrived during the rewrite (from buffer)
//  5. Restores normal AOF writing to the file
//
// Rewrite Process:
//
//   - Phase 1: Redirect writes to buffer
//
//   - Changes aof.w to write to a bytes.Buffer instead of the file
//
//   - Phase 2: Rewrite file with current state
//
//   - Truncates AOF file to zero length
//
//   - Seeks to beginning of file
//
//   - Writes appropriate commands for each data type
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
// Benefits:
//   - Removes redundant commands (only latest state per key)
//   - Reduces AOF file size significantly
//   - Improves replay performance
//   - Maintains data integrity (no commands lost)
//   - Supports all data types (string, hash, list, set, zset)
//
// Thread Safety:
//   - Should be called from a single goroutine (typically BGREWRITEAOF handler)
//   - Database copy should be taken with proper locking before calling
func (aof *Aof) Rewrite(cp map[string]*Item) {
	// future commands will go to buffer
	var b bytes.Buffer
	aof.W = NewWriter(&b) // writer to buffer

	// Truncate the file
	err := aof.F.Truncate(0)
	if err != nil {
		log.Println("ERR AOF Rewrite issue! Can't Truncate")
		return
	}
	_, err = aof.F.Seek(0, 0)
	if err != nil {
		log.Println("ERR AOF Rewrite issue! Can't Seek")
		return
	}

	// write all items with appropriate commands based on type
	fwriter := NewWriter(aof.F) // writer to file
	for k, item := range cp {
		if item == nil || item.IsExpired() {
			continue
		}

		key := Value{Typ: BULK, Blk: k}

		// Write command based on item type
		switch item.Type {
		case STRING_TYPE:
			// SET key value
			cmd := Value{Typ: BULK, Blk: "SET"}
			value := Value{Typ: BULK, Blk: item.Str}
			Arr := Value{Typ: ARRAY, Arr: []Value{cmd, key, value}}
			fwriter.Write(&Arr)

		case HASH_TYPE:
			// HSET key field value [field value ...]
			if len(item.Hash) > 0 {
				cmd := Value{Typ: BULK, Blk: "HSET"}
				Arr := []Value{cmd, key}
				for field, fieldItem := range item.Hash {
					// Skip expired fields
					if !fieldItem.IsExpired() {
						Arr = append(Arr, Value{Typ: BULK, Blk: field})
						Arr = append(Arr, Value{Typ: BULK, Blk: fieldItem.Str})
					}
				}
				// Only write if there are non-expired fields
				if len(Arr) > 2 {
					hsetCmd := Value{Typ: ARRAY, Arr: Arr}
					fwriter.Write(&hsetCmd)
				}
			}

		case LIST_TYPE:
			if len(item.List) > 0 {
				cmd := Value{Typ: BULK, Blk: "RPUSH"}
				Arr := []Value{cmd, key}
				for _, val := range item.List {
					Arr = append(Arr, Value{Typ: BULK, Blk: val})
				}
				rpushCmd := Value{Typ: ARRAY, Arr: Arr}
				fwriter.Write(&rpushCmd)
			}

		case SET_TYPE:
			if len(item.ItemSet) > 0 {
				cmd := Value{Typ: BULK, Blk: "SADD"}
				Arr := []Value{cmd, key}
				for member := range item.ItemSet {
					Arr = append(Arr, Value{Typ: BULK, Blk: member})
				}
				saddCmd := Value{Typ: ARRAY, Arr: Arr}
				fwriter.Write(&saddCmd)
			}

		case ZSET_TYPE:
			if len(item.ZSet) > 0 {
				cmd := Value{Typ: BULK, Blk: "ZADD"}
				Arr := []Value{cmd, key}
				for member, score := range item.ZSet {
					Arr = append(Arr, Value{
						Typ: BULK,
						Blk: strconv.FormatFloat(score, 'f', -1, 64),
					})
					Arr = append(Arr, Value{Typ: BULK, Blk: member})
				}
				zaddCmd := Value{Typ: ARRAY, Arr: Arr}
				fwriter.Write(&zaddCmd)
			}

		default:
			log.Printf("Warning: Unknown type %s for key %s in AOF Rewrite\n", item.Type, k)
		}
	}
	fwriter.Flush()
	log.Println("done BGREWRITE.")

	// if buffer b is not empty, write it as well
	if _, err := b.WriteTo(aof.F); err != nil {
		log.Println("ERR AOF Rewrite issue! Can't append buffered commands:", err)
		return
	} else if err := aof.F.Sync(); err != nil {
		log.Println("ERR AOF Rewrite issue! Can't sync after appending buffer:", err)
		return
	}

	// rewrite to file
	aof.W = NewWriter(aof.F)

}
