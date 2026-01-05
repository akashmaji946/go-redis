/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/database.go
*/
package main

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
	"unsafe"
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

// Database represents the main in-memory key-value store.
// It provides thread-safe operations for storing, retrieving, and deleting key-value pairs.
//
// Fields:
//   - store: A map that stores key-value pairs where keys are strings and values are VAL pointers
//   - mu: A read-write mutex (RWMutex) that allows multiple concurrent readers
//     or a single writer, ensuring thread-safe access to the store
//
// Thread Safety:
//   - All operations should be protected by appropriate locks (RLock for reads, Lock for writes)
//   - The RWMutex allows multiple goroutines to read simultaneously while ensuring
//     exclusive access for write operations
type Database struct {
	store   map[string]*Item
	mu      sync.RWMutex
	mem     int64
	mempeak int64
}

// NewDatabase creates and returns a new empty Database instance.
// Initializes an empty store map and a new read-write mutex.
//
// Returns: A pointer to a newly initialized Database with an empty store
//
// Example:
//
//	db := NewDatabase()
//	// db is ready to use with empty store
func NewDatabase() *Database {
	return &Database{
		store: map[string]*Item{},
		mu:    sync.RWMutex{},
	}
}

// Put stores a key-value pair in the database.
// Creates a new VAL entry with the provided value and no expiration time.
//
// Parameters:
//   - k: The key to store
//   - v: The string value to associate with the key
//
// Behavior:
//   - If the key already exists, it will be overwritten with the new value
//   - The new value will have no expiration time (zero time)
//   - The new item will have the current time as the last accessed time
//   - The new item will have the access count set to 0
//   - This method directly accesses DB.store without locking
//     (caller must ensure proper locking is in place)
//   - Memory eviction should be handled by the caller before calling Put
//
// Note: This is a low-level method. For thread-safe operations, ensure lock is held.
func (DB *Database) Put(k string, v string, state *AppState) (err error) {

	var item *Item
	// if already exists, decrease memory
	if oldItem, ok := DB.store[k]; ok {
		oldmemory := oldItem.approxMemoryUsage(k)
		DB.mem -= int64(oldmemory)
		// track peak memory
		item = oldItem
		item.Str = v
		item.Type = STRING_TYPE
	} else {
		item = NewStringItem(v)
	}

	// get memory
	memory := item.approxMemoryUsage(k)

	// increase memory
	DB.mem += int64(memory)
	// track peak memory
	if DB.mem > DB.mempeak {
		DB.mempeak = DB.mem
	}

	// put value
	DB.store[k] = item

	log.Printf("memory = %d\n", DB.mem)
	if DB.mem < 0 {
		panic("DB memory went negative!")
	}

	return nil
}

// Poll retrieves a value from the database by key.
// Returns the VAL structure associated with the key and a boolean indicating success.
//
// Parameters:
//   - k: The key to retrieve
//
// Returns:
//   - val: A pointer to the VAL structure if the key exists, or an empty VAL if not found
//   - ok: true if the key exists, false otherwise
//
// Behavior:
//   - Returns an empty VAL struct and false if the key doesn't exist
//   - Returns the actual VAL pointer and true if the key exists
//   - This method directly accesses DB.store without locking
//     (caller must ensure proper locking is in place)
//
// Note: This is a low-level method. For thread-safe operations, ensure
//
//	the caller holds the appropriate lock (read lock for reads)
//
// Example:
//
//	item, ok := DB.Poll("mykey")
//	if ok {
//	    // Use item.V for the value, item.Exp for expiration
//	}
func (DB *Database) Poll(k string) (item *Item, ok bool) {

	// get the item from the database
	item, ok = DB.store[k]
	if !ok {
		// not found
		return nil, false
	}

	// Check if expired, but don't delete here (would need write lock)
	// Return expired status and let caller handle deletion if needed
	if !item.IsExpired() {
		// update last accessed time and access count
		item.LastAccessed = time.Now()
		item.AccessCount++
	}

	// all good, return the item
	return item, true
}

// Del removes a key-value pair from the database.
// Deletes the key from the store if it exists.
//
// Parameters:
//   - k: The key to delete
//
// Behavior:
//   - If the key exists, it is removed from the store
//   - If the key doesn't exist, the operation is a no-op (no error)
//   - This method directly accesses DB.store without locking
//     (caller must ensure proper locking is in place)
//
// Note: This is a low-level method. For thread-safe operations, ensure
//
//	the caller holds the appropriate lock (write lock for deletions)
//
// Example:
//
//	db.Rem("mykey")  // Removes "mykey" from the database
func (DB *Database) Rem(k string) {
	if item, ok := DB.store[k]; ok {
		mem := item.approxMemoryUsage(k) // also takes into account hashes
		DB.mem -= int64(mem)
		if item.Type == HASH_TYPE && item.Hash != nil {
			item.Hash = nil // help GC
		}
		delete(DB.store, k)
	}
	log.Printf("memory = %d\n", DB.mem)
	if DB.mem < 0 {
		panic("DB memory went negative!")
	}
}

// RemIfExpired removes a key-value pair from the database if it is expired.
// Deletes the key from the store if it exists and is expired.
//
// Parameters:
//   - k: The key to delete
//   - item: The item to delete
//
// Returns:
//   - true if the key was deleted, false otherwise
//
// Behavior:
//   - If the key exists and is expired, it is removed from the store
//   - If the key doesn't exist or is not expired, the operation is a no-op (no error)
//   - This method directly accesses DB.store without locking
//     (caller must ensure proper locking is in place)
//
// Note: This is a low-level method. For thread-safe operations, ensure
//
//	the caller holds the appropriate lock (write lock for deletions)
//
// Example:
//
//	db.RemIfExpired("mykey", item)
//	if deleted {
//	    // item is deleted
//	}
func (DB *Database) RemIfExpired(k string, item *Item, state *AppState) (deleted bool) {
	if item == nil {
		return false
	}
	if item.IsExpired() { // check if expired
		if _, exists := DB.store[k]; exists {
			fmt.Println("Deleting expired key: ", k)
			DB.Rem(k)
			state.genStats.total_expired_keys += 1
			return true
		}
	}
	return false
}

// DB is the global database instance used throughout the application.
// All database operations should use this shared instance to maintain
// consistency across the application.
//
// This is initialized at package load time using NewDatabase(),
// creating a single shared database that all handlers and operations access.
//
// Thread Safety:
//   - All operations on DB should be protected by DB.mu (read or write locks)
//   - Use DB.mu.RLock() for read operations (GET, EXISTS, KEYS, etc.)
//   - Use DB.mu.Lock() for write operations (SET, DEL, FLUSHDB, etc.)
var DB = NewDatabase()

// Transaction represents a transaction context that can queue multiple commands
// to be executed atomically. Commands added to a transaction are queued and
// can be executed together, ensuring atomicity and isolation.
//
// Fields:
//   - cmds: A slice of TxCommand structures representing queued commands
//     that will be executed as part of this transaction
//
// Usage:
//   - Create a transaction with NewTransaction()
//   - Add commands to the transaction
//   - Execute all commands atomically
//
// Thread Safety:
//   - Transactions should be used within a single goroutine
//   - Multiple transactions can be created concurrently, but each should
//     be managed by a single goroutine
//
// Note: This is a foundation for transaction support. Full implementation
//
//	would include MULTI, EXEC, DISCARD, and WATCH commands.
type Transaction struct {
	cmds []*TxCommand
}

// NewTransaction creates and returns a new empty Transaction instance.
// Initializes a transaction with an empty command queue ready to accept commands.
//
// Returns: A pointer to a new Transaction with an empty command slice
//
// Example:
//
//	tx := NewTransaction()
//	// Transaction is ready to queue commands
//
// Note: The transaction is initially empty and commands must be added
//
//	before execution.
func NewTransaction() *Transaction {
	return &Transaction{}
}

// TxCommand represents a single command queued within a transaction.
// This structure stores both the command Value and its handler function,
// allowing the transaction to execute the command later when the transaction
// is committed.
//
// Fields:
//   - value: The parsed command Value containing the command name and arguments
//     in RESP protocol format
//   - handler: The Handler function that will execute this command when the
//     transaction is executed (e.g., Get, Set, Del, etc.)
//
// Purpose:
//   - Allows commands to be queued without immediate execution
//   - Enables atomic execution of multiple commands together
//   - Maintains the relationship between command and its handler
//
// Usage:
//   - Created when commands are added to a transaction
//   - Executed when the transaction is committed (EXEC command)
//   - Discarded if the transaction is aborted (DISCARD command)
//
// Note: This is part of the transaction infrastructure. The handler
//
//	function is looked up from the Handlers map based on the command name.
type TxCommand struct {
	value   *Value
	handler Handler
}

// approxMemoryUsage calculates the approximate memory usage of an Item.
// Returns the size in bytes of the Item.
//
// Parameters:
//   - key: The key of the Item
//
// Returns:
//   - size: The approximate memory usage of the Item in bytes
//   - stringHeader: The size of the string header (16 bytes)
//   - expHeader: The size of the expiration time header (24 bytes)
//   - mapEntrySize: The size of the map entry (32 bytes)
//   - k: The key of the Item

func (item *Item) approxMemoryUsage(key string) int64 {
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

	// Item struct itself (headers + padding)
	size += int64(unsafe.Sizeof(*item))

	// String values inside Item (data only; headers counted in struct)
	size += int64(len(item.Str))
	size += int64(len(item.Type))

	// Hash map inside Item - now contains *Item pointers instead of strings
	if item.Type == HASH_TYPE && item.Hash != nil {
		// hmap header (pointer counted in struct, data here)
		size += int64(unsafe.Sizeof(item.Hash))

		for k, fieldItem := range item.Hash {
			if fieldItem == nil {
				continue
			}
			size += stringHeader + int64(len(k)) // field name
			size += pointerSize                  // pointer to field Item
			size += avgMapEntryOverhead          // map entry overhead

			// Recursively add size of the nested Item (field value)
			size += fieldItem.approxMemoryUsageNested()
		}
	}

	// Set type
	if item.Type == SET_TYPE {
		size += int64(unsafe.Sizeof(item.ItemSet))
		for k := range item.ItemSet {
			size += stringHeader + int64(len(k))
			size += 1 // bool value
			size += avgMapEntryOverhead
		}
	}

	return size
}

// approxMemoryUsageNested calculates memory usage for nested Items (hash fields)
// without double-counting the map entry overhead
func (item *Item) approxMemoryUsageNested() int64 {
	const pointerSize = 8

	var size int64

	// Item struct itself
	size += int64(unsafe.Sizeof(*item))

	// String values
	size += int64(len(item.Str))
	size += int64(len(item.Type))

	// Note: We don't recursively calculate nested hash fields to avoid deep nesting
	// If hash fields contain hashes, we only count the top level

	return size
}

// UNIX_TS_EPOCH is the Unix timestamp of the epoch time.
var UNIX_TS_EPOCH = time.Time{}.Unix()

// IsExpired checks if the item is expired.
// Returns true if the item is expired, false otherwise.
//
// Parameters:
//   - item: The item to check
//
// Returns:
//   - true if the item is expired, false otherwise
//
// Behavior:
//   - If the item has no expiration time, returns false
//   - If the item has an expiration time and it is in the past, returns true
//   - If the item has an expiration time and it is in the future, returns false
//   - This method directly accesses item.Exp without locking
//     (caller must ensure proper locking is in place)
//
// Note: This is a low-level method. For thread-safe operations, ensure
//
//	the caller holds the appropriate lock (read lock for reads)
func (item *Item) IsExpired() bool {
	return item.Exp.Unix() != UNIX_TS_EPOCH && time.Until(item.Exp).Seconds() <= 0
}

// EvictKeys removes keys from the database to free up memory when the maximum
// memory limit is reached. This function implements the eviction policy configured
// in the server settings to automatically free space for new keys.
//
// Parameters:
//   - state: The application state containing eviction policy configuration
//   - requiredMemBytes: The minimum number of bytes that must be freed to make
//     room for the new operation
//
// Returns:
//   - error: nil on success, or an error if eviction fails or is not allowed
//
// Behavior:
//   - Checks if eviction is enabled (returns error if policy is NoEviction)
//   - Samples keys from the database based on the eviction policy
//   - Iteratively removes keys until enough memory is freed
//   - Stops when DB.mem + requiredMemBytes < maxmemory
//
// Supported Eviction Policies:
//   - AllKeysRandom: Randomly selects and removes keys from all keys in the database
//     until sufficient memory is freed. Uses random sampling for efficiency.
//   - AllKeysLRU: Not yet implemented (returns error)
//   - AllKeysLFU: Not yet implemented (returns error)
//   - Volatile* policies: Not yet implemented
//
// Eviction Process:
//  1. Samples keys using sampleKeysRandom() based on maxmemory-samples config
//  2. For each sampled key, removes it using DB.Rem()
//  3. Checks after each removal if enough memory has been freed
//  4. Continues until sufficient memory is available or all samples are processed
//  5. Returns error if unable to free enough memory
//
// Thread Safety:
//   - This function acquires its own locks internally
//   - MUST NOT be called while holding DB.mu lock (will cause deadlock)
//   - Should be called before acquiring write locks for Put operations
//
// Error Cases:
//   - NoEviction policy: Returns error immediately (eviction disabled)
//   - Insufficient memory freed: Returns error if all samples processed but
//     still not enough memory available
//   - Unimplemented policies: Returns error for LRU/LFU policies (not yet supported)
//
// Example Usage:
//
//	// Check memory before Put operation
//	if DB.mem + newMemory >= state.config.maxmemory {
//	    err := DB.EvictKeys(state, newMemory)
//	    if err != nil {
//	        return err  // Handle eviction failure
//	    }
//	}
//	// Now safe to proceed with Put
//
// Note:
//   - Eviction is a best-effort operation and may not always free exactly
//     the required amount due to sampling limitations
//   - The function uses random sampling for performance (not all keys are examined)
//   - Memory calculation includes key size, value size, and metadata overhead
func (db *Database) EvictKeys(state *AppState, requiredMemBytes int64) (count int, err error) {
	if state.config.eviction == NoEviction {
		return 0, errors.New("maxmemory reached : can't call eviction when policy is no-eviction")
	}

	samples := sampleKeysRandom(state)
	enoughMemFreed := func() bool {
		if DB.mem+requiredMemBytes < state.config.maxmemory {
			return true
		}
		return false
	}

	EvictKeysFromSample := func(samples []Sample) (int, error) {
		// iterate till needed
		count := 0
		for _, sample := range samples {
			if enoughMemFreed() {
				break
			}
			log.Printf("evicting key=%s\n", sample.key)
			DB.Rem(sample.key)
			count += 1
		}
		if !enoughMemFreed() {
			return count, errors.New("maxmemory reached : can't free enough memory")
		}
		return count, nil
	}

	switch state.config.eviction {
	case AllKeysRandom:
		// no sort, randomly delete till needed
		return EvictKeysFromSample(samples)
	case AllKeysLRU:

		// sort based by Least Recent Usage
		sort.Slice(samples, func(i, j int) bool {
			return samples[i].value.LastAccessed.After(samples[j].value.LastAccessed)
		})
		// delete lru ones
		return EvictKeysFromSample(samples)

	case AllKeysLFU:

		// sort based by Least Frequent Usage
		sort.Slice(samples, func(i, j int) bool {
			return samples[i].value.AccessCount < samples[j].value.AccessCount
		})
		// delete lfu ones
		return EvictKeysFromSample(samples)

	}
	return 0, nil
}
