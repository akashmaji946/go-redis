/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/database/database.go
*/
package database

import (
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/akashmaji946/go-redis/internal/common"
)

// Database represents the main in-memory key-value store.
// It provides thread-safe operations for storing, retrieving, and deleting key-value pairs.
//
// Fields:
//   - store: A map that stores key-value pairs where keys are strings and values are common.Item pointers
//   - mu: A read-write mutex (RWMutex) that allows multiple concurrent readers
//     or a single writer, ensuring thread-safe access to the store
//
// Thread Safety:
//   - All operations should be protected by appropriate locks (RLock for reads, Lock for writes)
//   - The RWMutex allows multiple goroutines to read simultaneously while ensuring
//     exclusive access for write operations
type Database struct {
	Store      map[string]*common.Item
	Mu         sync.RWMutex
	TxMu       sync.RWMutex
	Watchers   map[string][]*common.Client // Key -> List of clients watching it
	WatchersMu sync.Mutex                  // Protects the watchers map
	Mem        int64
	Mempeak    int64
	ID         int
	Aof        *common.Aof
	Trackers   []*common.SnapshotTracker
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
func NewDatabase(id int) *Database {
	return &Database{
		Store:    map[string]*common.Item{},
		Mu:       sync.RWMutex{},
		TxMu:     sync.RWMutex{},
		Watchers: make(map[string][]*common.Client),
		ID:       id,
	}
}

// DBMu is the global mutex protecting access to the global DB instance.
// It should be used whenever switching or accessing the global DB variable.
var DBMu sync.Mutex

// DB is the global database instance used throughout the application.
// All database operations should use this shared instance to maintain
// consistency across the application.
var DB *Database

// DBS is the global slice of database instances.
// Each index corresponds to a separate logical database.
//
// This is initialized at server startup based on the configured number of databases.
// All database operations should use the appropriate DB instance from this slice.
var DBS []*Database

// InitDBS initializes the global database slice with n databases.
func InitDBS(n int, conf *common.Config, state *common.AppState) {
	DBS = make([]*Database, n)
	for i := 0; i < n; i++ {
		DBS[i] = NewDatabase(i)
		if conf.AofEnabled {
			DBS[i].Aof = common.NewAof(conf, i)
		}
		if len(conf.Rdb) > 0 {
			DBS[i].Trackers = common.InitRDBTrackers(conf, state, i, DBS[i].Snapshot)
		}
	}
	// Set the default DB to the first one
	DB = DBS[0]
}

// FlushAll clears all data from all logical databases.
func FlushAll(state *common.AppState) {
	DBMu.Lock()
	defer DBMu.Unlock()
	for _, db := range DBS {
		db.Mu.Lock()
		db.Store = make(map[string]*common.Item)
		db.Mem = 0
		db.TouchAll()
		db.Mu.Unlock()
	}
}

// Snapshot returns a point-in-time copy of the database store.
func (db *Database) Snapshot() map[string]*common.Item {
	db.Mu.RLock()
	defer db.Mu.RUnlock()
	copy := make(map[string]*common.Item, len(db.Store))
	for k, v := range db.Store {
		copy[k] = v
	}
	return copy
}

// IncrTrackers increments the change counter for all RDB trackers associated with this database.
func (db *Database) IncrTrackers() {
	for _, t := range db.Trackers {
		t.Incr()
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
func (DB *Database) Put(k string, v string, state *common.AppState) (err error) {

	var item *common.Item
	// if already exists, decrease memory
	if oldItem, ok := DB.Store[k]; ok {
		oldmemory := oldItem.ApproxMemoryUsage(k)
		DB.Mem -= int64(oldmemory)
		// track peak memory
		item = oldItem
		item.Str = v
		item.Type = common.STRING_TYPE
	} else {
		item = common.NewStringItem(v)
	}

	// get memory
	memory := item.ApproxMemoryUsage(k)

	// increase memory
	DB.Mem += int64(memory)
	// track peak memory
	if DB.Mem > DB.Mempeak {
		DB.Mempeak = DB.Mem
	}

	// put value
	DB.Store[k] = item

	// Notify watchers that the key has changed
	DB.Touch(k)

	// Increment RDB change tracker for automatic saving
	if len(state.Config.Rdb) > 0 {
		DB.IncrTrackers()
	}

	log.Printf("memory = %d\n", DB.Mem)
	if DB.Mem < 0 {
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
func (DB *Database) Poll(k string) (item *common.Item, ok bool) {

	// get the item from the database
	item, ok = DB.Store[k]
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
	if item, ok := DB.Store[k]; ok {
		mem := item.ApproxMemoryUsage(k) // also takes into account hashes
		DB.Mem -= int64(mem)
		if item.Type == common.HASH_TYPE && item.Hash != nil {
			item.Hash = nil // help GC
		}
		if item.Type == common.SET_TYPE && item.ItemSet != nil {
			item.ItemSet = nil // help GC
		}
		if item.Type == common.ZSET_TYPE && item.ZSet != nil {
			item.ZSet = nil // help GC
		}
		delete(DB.Store, k)

		// Notify watchers that the key has been deleted
		DB.Touch(k)
	}
	log.Printf("memory = %d\n", DB.Mem)
	if DB.Mem < 0 {
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
func (DB *Database) RemIfExpired(k string, item *common.Item, state *common.AppState) (deleted bool) {
	if item == nil {
		return false
	}
	if item.IsExpired() { // check if expired
		if _, exists := DB.Store[k]; exists {
			log.Println("Deleting expired key: ", k)
			DB.Rem(k)
			state.GenStats.TotalExpiredKeys += 1
			return true
		}
	}
	return false
}

// ActiveExpire periodically samples keys and removes expired ones.
// This prevents memory leaks from expired keys that are never accessed.
func (DB *Database) ActiveExpire(state *common.AppState) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		DB.Mu.Lock()
		// Sample up to 20 keys (Redis default behavior)
		iterationCount := 0
		for k, item := range DB.Store {
			DB.RemIfExpired(k, item, state)
			iterationCount++
			if iterationCount >= 20 {
				break
			}
		}
		DB.Mu.Unlock()
	}
}

// Touch marks all clients watching the given key as having a failed transaction.
// This is used for optimistic locking (WATCH/MULTI/EXEC).
func (DB *Database) Touch(key string) {
	DB.WatchersMu.Lock()
	defer DB.WatchersMu.Unlock()

	if clients, ok := DB.Watchers[key]; ok {
		for _, client := range clients {
			client.TxFailed = true
		}
		// Once a key is touched, we clear its watchers
		delete(DB.Watchers, key)
	}
}

// TouchAll marks all clients watching any key as having a failed transaction.
func (DB *Database) TouchAll() {
	DB.WatchersMu.Lock()
	defer DB.WatchersMu.Unlock()

	for _, clients := range DB.Watchers {
		for _, client := range clients {
			client.TxFailed = true
		}
	}
	DB.Watchers = make(map[string][]*common.Client)
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
//   - Stops when DB.Mem + requiredMemBytes < maxmemory
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
//   - MUST NOT be called while holding DB.Mu lock (will cause deadlock)
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
//	if DB.Mem + newMemory >= state.Config.Maxmemory {
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
func (db *Database) EvictKeys(state *common.AppState, requiredMemBytes int64) (count int, err error) {
	if state.Config.Eviction == common.NoEviction {
		return 0, errors.New("maxmemory reached : can't call eviction when policy is no-eviction")
	}

	samples := sampleKeysRandom(state)
	enoughMemFreed := func() bool {
		if DB.Mem+requiredMemBytes < state.Config.Maxmemory {
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

	switch state.Config.Eviction {
	case common.AllKeysRandom:
		// no sort, randomly delete till needed
		return EvictKeysFromSample(samples)
	case common.AllKeysLRU:

		// sort based by Least Recent Usage
		sort.Slice(samples, func(i, j int) bool {
			return samples[i].value.LastAccessed.After(samples[j].value.LastAccessed)
		})
		// delete lru ones
		return EvictKeysFromSample(samples)

	case common.AllKeysLFU:

		// sort based by Least Frequent Usage
		sort.Slice(samples, func(i, j int) bool {
			return samples[i].value.AccessCount < samples[j].value.AccessCount
		})
		// delete lfu ones
		return EvictKeysFromSample(samples)

	}
	return 0, nil
}
