package common

// Handler is a function type that processes Redis commands.
// Each command has a corresponding handler function that implements its logic.
type Handler func(*Client, *Value, *AppState) *Value

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
	Cmds []*TxCommand
}

// NewTransaction creates and returns a new empty Transaction instance.
// Initializes a transaction with an empty command queue ready to accept commands.
//
// Returns: A pointer to a new Transaction with an empty command slice
//
// Example:
//
//	Tx := NewTransaction()
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
	Value   *Value
	Handler Handler
}
