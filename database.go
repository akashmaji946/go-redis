package main

import (
	"sync"
	"time"
)

// VAL represents a value stored in the database along with its expiration time.
// This structure allows the database to support key expiration functionality.
//
// Fields:
//   - v: The actual string value stored in the database
//   - exp: The expiration time for this key-value pair
//     If exp is the zero time (time.Time{}), the key has no expiration
type VAL struct {
	v   string
	exp time.Time
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
	store map[string]*VAL
	mu    sync.RWMutex
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
		store: map[string]*VAL{},
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
//   - This method directly accesses DB.store without locking
//     (caller must ensure proper locking is in place)
//
// Note: This is a low-level method. For thread-safe operations, ensure
//
//	the caller holds the appropriate lock (write lock for writes)
func (db *Database) Put(k string, v string) {
	DB.store[k] = &VAL{v: v}
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
//	val, ok := db.Poll("mykey")
//	if ok {
//	    // Use val.v for the value, val.exp for expiration
//	}
func (db *Database) Poll(k string) (val *VAL, ok bool) {
	Val, ok := DB.store[k]
	if ok != true {
		return &VAL{}, ok
	}
	return Val, ok
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
//	db.Del("mykey")  // Removes "mykey" from the database
func (db *Database) Del(k string) {
	delete(DB.store, k)
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
