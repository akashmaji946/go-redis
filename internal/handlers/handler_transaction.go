package handlers

import (
	"log"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
)

// Multi handles the MULTI command.
// Begins a transaction by creating a new transaction context for the client.
// All subsequent commands (except EXEC, DISCARD, and MULTI) will be queued
// until EXEC or DISCARD is called.
//
// Syntax:
//
//	MULTI
//
// Returns:
//
//	+Started\r\n on success
//	Error if transaction is already running
//
// Behavior:
//   - Creates a new Transaction instance and stores it in state.Tx
//   - Subsequent commands are queued instead of executed immediately
//   - Only one transaction can be active per client connection
//   - Commands return "QUEUED" response instead of actual results
//
// Transaction Flow:
//  1. MULTI - Start transaction (this command)
//  2. <commands> - Queue commands (GET, SET, etc.)
//  3. EXEC - Execute all queued commands atomically
//     OR
//  3. DISCARD - Abort transaction without executing
//
// Error Cases:
//   - Invalid arguments: Returns error if arguments provided
//   - Transaction already running: Returns error if Tx already exists
//
// Example:
//
//	127.0.0.1:6379> MULTI
//	OK
//	127.0.0.1:6379> SET key1 "value1"
//	QUEUED
//	127.0.0.1:6379> SET key2 "value2"
//	QUEUED
//	127.0.0.1:6379> EXEC
//	1) OK
//	2) OK
func Multi(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of MULTI")
		return common.NewErrorValue("ERR invalid argument to MULTI")
	}
	// check if this client already has a Tx running
	if c.InTx {
		log.Println("MULTI calls can not be nested")
		return common.NewErrorValue("ERR MULTI calls can not be nested")
	}

	c.InTx = true
	c.Tx = common.NewTransaction()

	log.Println("Tx started")
	return common.NewStringValue("Started")

}

// Exec handles the EXEC command.
// Executes all commands queued in the current transaction atomically.
// All queued commands are executed in order and their replies are returned
// as an array.
//
// Syntax:
//
//	EXEC
//
// Returns:
//
//	Array of replies: One reply per queued command, in order
//	Error if no transaction is running
//
// Behavior:
//   - Executes all commands in state.Tx.cmds sequentially
//   - Each command is executed with its stored handler and value
//   - All replies are collected and returned as an array
//   - Transaction is cleared after execution (state.Tx = nil)
//   - Commands are executed in the order they were queued
//
// Atomicity:
//   - All commands succeed or fail individually (no rollback on error)
//   - Commands are executed sequentially, not concurrently
//   - If a command fails, subsequent commands still execute
//
// Error Cases:
//   - Invalid arguments: Returns error if arguments provided
//   - No transaction running: Returns error if state.Tx is nil
//
// Example:
//
//	127.0.0.1:6379> MULTI
//	OK
//	127.0.0.1:6379> SET a "1"
//	QUEUED
//	127.0.0.1:6379> SET b "2"
//	QUEUED
//	127.0.0.1:6379> EXEC
//	1) OK
//	2) OK
//
// Note: Unlike Redis, this implementation does not support WATCH for
//
//	optimistic locking or rollback on conflicts.
func Exec(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of EXEC")
		return common.NewErrorValue("ERR invalid argument to EXEC")
	}
	// check if some Tx running
	if !c.InTx || c.Tx == nil {
		log.Println("Tx already NOT running")
		return common.NewErrorValue("ERR Tx already NOT running")
	}

	return ExecuteTransaction(c, state)
}

// Discard handles the DISCARD command.
// Aborts the current transaction by discarding all queued commands
// without executing them. The transaction context is cleared.
//
// Syntax:
//
//	DISCARD
//
// Returns:
//
//	+Discarded\r\n on success
//	Error if no transaction is running
//
// Behavior:
//   - Clears the transaction context (state.Tx = nil)
//   - All queued commands are discarded and never executed
//   - No changes are made to the database
//   - common.Client can start a new transaction with MULTI after discarding
//
// Use Cases:
//   - common.Client wants to abort a transaction without executing commands
//   - Error occurred during transaction building
//   - common.Client changed their mind about the transaction
//
// Error Cases:
//   - Invalid arguments: Returns error if arguments provided
//   - No transaction running: Returns error if state.Tx is nil
//
// Example:
//
//	127.0.0.1:6379> MULTI
//	OK
//	127.0.0.1:6379> SET key1 "value1"
//	QUEUED
//	127.0.0.1:6379> SET key2 "value2"
//	QUEUED
//	127.0.0.1:6379> DISCARD
//	OK
//	# All queued commands are discarded, no changes made
//
// Note: After DISCARD, the client must call MULTI again to start
//
//	a new transaction.
func Discard(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of DISCARD")
		return common.NewErrorValue("ERR invalid argument to DISCARD")
	}
	// check if some Tx running
	if !c.InTx || c.Tx == nil {
		log.Println("Tx already NOT running")
		return common.NewErrorValue("ERR Tx already NOT running")
	}

	// discard without commiting
	c.InTx = false
	c.Tx = nil
	unwatchClient(c)
	log.Println("Tx discarded")

	return common.NewStringValue("Discarded")
}

// Watch handles the WATCH command.
func Watch(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) == 0 {
		return common.NewErrorValue("ERR wrong number of arguments for 'watch' command")
	}
	if c.InTx {
		return common.NewErrorValue("ERR WATCH inside MULTI is not allowed")
	}

	database.DB.WatchersMu.Lock()
	defer database.DB.WatchersMu.Unlock()

	for _, arg := range args {
		key := arg.Blk
		c.WatchedKeys = append(c.WatchedKeys, key)
		database.DB.Watchers[key] = append(database.DB.Watchers[key], c)
	}

	return common.NewStringValue("OK")
}

// Unwatch handles the UNWATCH command.
func Unwatch(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	unwatchClient(c)
	return common.NewStringValue("OK")
}

// unwatchClient clears all watched keys for a client and resets the failure flag.
func unwatchClient(c *common.Client) {
	database.DB.WatchersMu.Lock()
	defer database.DB.WatchersMu.Unlock()

	for _, key := range c.WatchedKeys {
		clients := database.DB.Watchers[key]
		for i, client := range clients {
			if client == c {
				// Remove this client from the global watchers list for this key
				database.DB.Watchers[key] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
		// Clean up the map entry if no one is watching the key anymore
		if len(database.DB.Watchers[key]) == 0 {
			delete(database.DB.Watchers, key)
		}
	}

	// Reset client state
	c.WatchedKeys = nil
	c.TxFailed = false
}

func ExecuteTransaction(client *common.Client, state *common.AppState) *common.Value {
	database.DB.TxMu.Lock()
	defer database.DB.TxMu.Unlock()

	// If a watched key was modified, fail the transaction
	if client.TxFailed {
		client.InTx = false
		client.Tx = nil
		unwatchClient(client)
		return common.NewNullValue()
	}

	state.GenStats.TotalTxnExecuted += 1

	replies := make([]common.Value, len(client.Tx.Cmds))
	for idx, txCmd := range client.Tx.Cmds {
		reply := txCmd.Handler(client, txCmd.Value, state)
		replies[idx] = *reply
	}

	// clear transaction
	client.InTx = false
	client.Tx = nil
	unwatchClient(client)

	log.Println("Tx executed")
	return &common.Value{
		Typ: common.ARRAY,
		Arr: replies,
	}
}
