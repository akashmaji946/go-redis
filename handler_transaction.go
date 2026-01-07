package main

import "log"

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
//   - Creates a new Transaction instance and stores it in state.tx
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
//   - Transaction already running: Returns error if tx already exists
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
func Multi(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of MULTI")
		return NewErrorValue("ERR inavlid argument to MULTI")
	}
	// check if some tx running, then don't run
	if state.tx != nil {
		log.Println("tx already running")
		return NewErrorValue("ERR tx already running")
	}

	state.tx = NewTransaction()

	log.Println("tx started")
	return NewStringValue("Started")

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
//   - Executes all commands in state.tx.cmds sequentially
//   - Each command is executed with its stored handler and value
//   - All replies are collected and returned as an array
//   - Transaction is cleared after execution (state.tx = nil)
//   - Commands are executed in the order they were queued
//
// Atomicity:
//   - All commands succeed or fail individually (no rollback on error)
//   - Commands are executed sequentially, not concurrently
//   - If a command fails, subsequent commands still execute
//
// Error Cases:
//   - Invalid arguments: Returns error if arguments provided
//   - No transaction running: Returns error if state.tx is nil
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
func Exec(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of EXEC")
		return NewErrorValue("ERR inavlid argument to EXEC")
	}
	// check if some tx running
	if state.tx == nil {
		log.Println("tx already NOT running")
		return NewErrorValue("ERR tx already NOT running")
	}

	// commmit queued commands first
	replies := make([]Value, len(state.tx.cmds))
	for idx, txCmd := range state.tx.cmds {
		reply := txCmd.handler(c, txCmd.value, state)
		replies[idx] = *reply
	}

	state.tx = nil
	state.genStats.total_txn_executed += 1
	log.Println("tx executed")

	return &Value{
		typ: ARRAY,
		arr: replies,
	}
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
//   - Clears the transaction context (state.tx = nil)
//   - All queued commands are discarded and never executed
//   - No changes are made to the database
//   - Client can start a new transaction with MULTI after discarding
//
// Use Cases:
//   - Client wants to abort a transaction without executing commands
//   - Error occurred during transaction building
//   - Client changed their mind about the transaction
//
// Error Cases:
//   - Invalid arguments: Returns error if arguments provided
//   - No transaction running: Returns error if state.tx is nil
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
func Discard(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 0 {
		log.Println("invalid use of DISCARD")
		return NewErrorValue("ERR inavlid argument to DISCARD")
	}
	// check if some tx running
	if state.tx == nil {
		log.Println("tx already NOT running")
		return NewErrorValue("ERR tx already NOT running")
	}

	// discard without commiting
	state.tx = nil
	log.Println("tx discarded")

	return NewStringValue("Discarded")
}
