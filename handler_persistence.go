/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_persistence.go
*/
package main

import (
	"maps"
	"time"
)

// Save handles the SAVE command.
// Performs a synchronous RDB snapshot.
//
// Syntax:
//
//	SAVE
//
// Returns:
//
//	+OK\r\n
//
// Behavior:
//   - Blocks server during save
//   - Uses read lock, preventing writes
//
// Recommendation:
//
//	Use BGSAVE for non-blocking persistence
func Save(c *Client, v *Value, state *AppState) *Value {
	// DB.mu.Lock()
	SaveRDB(state)
	// DB.mu.Unlock()
	return NewStringValue("OK")
}

// BGSave handles the BGSAVE command.
// Performs an asynchronous RDB snapshot.
//
// Syntax:
//
//	BGSAVE
//
// Returns:
//
//	+OK\r\n on success
//	Error if a background save is already running
//
// Behavior:
//   - Copies DB state
//   - Saves in background goroutine
//   - Prevents concurrent background saves
func BGSave(c *Client, v *Value, state *AppState) *Value {

	DB.mu.RLock()
	if state.bgsaving {
		// already running, return
		DB.mu.RUnlock()
		return NewErrorValue("already in progress")
	}

	copy := make(map[string]*Item, len(DB.store)) // actual copy of DB.store
	maps.Copy(copy, DB.store)
	state.bgsaving = true
	state.DBCopy = copy // points to that

	DB.mu.RUnlock()

	go func() {
		defer func() {
			state.bgsaving = false
			state.DBCopy = nil
		}()

		SaveRDB(state)
	}()

	return NewStringValue("OK")
}

// BGRewriteAOF handles the BGREWRITEAOF command.
// Asynchronously rewrites the Append-Only File.
//
// Behavior:
//  1. Copies current DB state
//  2. Rewrites AOF with compact SET commands
//  3. Runs in background goroutine
//
// Returns:
//
//	+Started.\r\n
func BGRewriteAOF(c *Client, v *Value, state *AppState) *Value {

	go func() {
		state.aofrewriting = true
		DB.mu.RLock()
		cp := make(map[string]*Item, len(DB.store))
		maps.Copy(cp, DB.store)
		DB.mu.RUnlock()
		state.aof.Rewrite(cp)
		state.aofrewriting = false

	}()

	// update the stats
	state.aofStats.aof_last_rewrite_ts = time.Now().Unix()
	state.aofStats.aof_rewrite_count += 1

	return NewStringValue("Started.")
}
