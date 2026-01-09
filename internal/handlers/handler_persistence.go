/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler_persistence.go
*/
package handlers

import (
	"maps"
	"time"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
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
func Save(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	// database.DB.Mu.Lock()
	common.SaveRDB(state)
	// database.DB.Mu.Unlock()
	return common.NewStringValue("OK")
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
func BGSave(c *common.Client, v *common.Value, state *common.AppState) *common.Value {

	database.DB.Mu.RLock()
	if state.Bgsaving {
		// already running, return
		database.DB.Mu.RUnlock()
		return common.NewErrorValue("already in progress")
	}

	copy := make(map[string]*common.Item, len(database.DB.Store)) // actual copy of database.DB.Store
	maps.Copy(copy, database.DB.Store)
	state.Bgsaving = true
	state.DBCopy = copy // points to that

	database.DB.Mu.RUnlock()

	go func() {
		defer func() {
			state.Bgsaving = false
			state.DBCopy = nil
		}()

		common.SaveRDB(state)
	}()

	return common.NewStringValue("OK")
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
func BGRewriteAOF(c *common.Client, v *common.Value, state *common.AppState) *common.Value {

	go func() {
		state.Aofrewriting = true
		database.DB.Mu.RLock()
		cp := make(map[string]*common.Item, len(database.DB.Store))
		maps.Copy(cp, database.DB.Store)
		database.DB.Mu.RUnlock()
		state.Aof.Rewrite(cp)
		state.Aofrewriting = false

	}()

	// update the stats
	state.AofStats.AofLastRewriteTS = time.Now().Unix()
	state.AofStats.AofRewriteCount += 1

	return common.NewStringValue("Started.")
}
