package main

import "log"

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

// Transaction of commands per client connection
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
		return NewErrorValue("ERR invalid argument to MULTI")
	}
	// check if this client already has a tx running
	if c.inTx {
		log.Println("MULTI calls can not be nested")
		return NewErrorValue("ERR MULTI calls can not be nested")
	}

	c.inTx = true
	c.tx = NewTransaction()

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
		return NewErrorValue("ERR invalid argument to EXEC")
	}
	// check if some tx running
	if !c.inTx || c.tx == nil {
		log.Println("tx already NOT running")
		return NewErrorValue("ERR tx already NOT running")
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
		return NewErrorValue("ERR invalid argument to DISCARD")
	}
	// check if some tx running
	if !c.inTx || c.tx == nil {
		log.Println("tx already NOT running")
		return NewErrorValue("ERR tx already NOT running")
	}

	// discard without commiting
	c.inTx = false
	c.tx = nil
	unwatchClient(c)
	log.Println("tx discarded")

	return NewStringValue("Discarded")
}

// Watch handles the WATCH command.
func Watch(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) == 0 {
		return NewErrorValue("ERR wrong number of arguments for 'watch' command")
	}
	if c.inTx {
		return NewErrorValue("ERR WATCH inside MULTI is not allowed")
	}

	DB.watchersMu.Lock()
	defer DB.watchersMu.Unlock()

	for _, arg := range args {
		key := arg.blk
		c.watchedKeys = append(c.watchedKeys, key)
		DB.watchers[key] = append(DB.watchers[key], c)
	}

	return NewStringValue("OK")
}

// Unwatch handles the UNWATCH command.
func Unwatch(c *Client, v *Value, state *AppState) *Value {
	unwatchClient(c)
	return NewStringValue("OK")
}

// unwatchClient clears all watched keys for a client and resets the failure flag.
func unwatchClient(c *Client) {
	DB.watchersMu.Lock()
	defer DB.watchersMu.Unlock()

	for _, key := range c.watchedKeys {
		clients := DB.watchers[key]
		for i, client := range clients {
			if client == c {
				// Remove this client from the global watchers list for this key
				DB.watchers[key] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
		// Clean up the map entry if no one is watching the key anymore
		if len(DB.watchers[key]) == 0 {
			delete(DB.watchers, key)
		}
	}

	// Reset client state
	c.watchedKeys = nil
	c.txFailed = false
}

func ExecuteTransaction(client *Client, state *AppState) *Value {
	DB.txMu.Lock()
	defer DB.txMu.Unlock()

	// If a watched key was modified, fail the transaction
	if client.txFailed {
		client.inTx = false
		client.tx = nil
		unwatchClient(client)
		return NewNullValue()
	}

	state.genStats.total_txn_executed += 1

	replies := make([]Value, len(client.tx.cmds))
	for idx, txCmd := range client.tx.cmds {
		reply := txCmd.handler(client, txCmd.value, state)
		replies[idx] = *reply
	}

	// clear transaction
	client.inTx = false
	client.tx = nil
	unwatchClient(client)

	log.Println("tx executed")
	return &Value{
		typ: ARRAY,
		arr: replies,
	}
}
