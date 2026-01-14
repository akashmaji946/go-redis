/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/client.go
*/
package common

import (
	"fmt"
	"log"
	"net"
	"time"
)

// Client represents a connected client session.
// Each client connection has its own Client instance that tracks connection-specific state.
//
// Fields:
//   - conn: The network connection to the client (used for reading commands and sending responses)
//   - authenticated: Whether this client has successfully authenticated
//     (required if requirepass is set in config)
//
// Authentication:
//   - Initially false for all new connections
//   - Set to true after successful AUTH command
//   - Checked before executing commands (if requirepass is enabled)
//   - Safe commands (COMMAND, AUTH) can be executed without authentication
//
// Lifecycle:
//   - Created when client connects (in handleOneConnection)
//   - Exists for the duration of the client connection
//   - Destroyed when connection closes (garbage collected)
//
// Thread Safety:
//   - Each Client is used by a single goroutine (one per connection)
//   - No synchronization needed for Client fields
type Client struct {
	Conn          net.Conn
	Authenticated bool

	// transaction
	InTx bool         // in a txn?
	Tx   *Transaction // txn context

	// optimistic locking
	WatchedKeys []string
	TxFailed    bool // set to true if a watched key is modified

	DatabaseID int
}

// NewClient creates a new Client instance for a network connection.
// Initializes a client with the given connection and sets authentication to false.
//
// Parameters:
//   - conn: The network connection associated with this client
//
// Returns: A pointer to a new Client instance with authenticated=false
//
// Usage:
//   - Called once per client connection in handleOneConnection()
//   - The Client instance is then used for all command handling for that connection
func NewClient(conn net.Conn) *Client {
	return &Client{
		Conn:          conn,
		Authenticated: false,
		InTx:          false,
		Tx:            nil,
		WatchedKeys:   make([]string, 0),
		TxFailed:      false,
		DatabaseID:    0,
	}
}

// WriterMonitorLog sends a formatted command log to a monitoring client.
// This function is used by the MONITOR command feature to stream all commands
// executed on the server to clients that have enabled monitoring.
//
// Parameters:
//   - value: The parsed command Value containing the command and arguments
//     that was executed on the server
//
// Behavior:
//  1. Extracts the client's local IP address from the connection
//  2. Formats a log message with:
//     - Unix timestamp of when the command was executed
//     - Client IP address that executed the command
//     - All command arguments in quoted format
//  3. Sends the formatted message to the monitoring client's connection
//  4. Flushes the writer to ensure immediate delivery
//
// Message Format:
//
//	"<timestamp> [<client_ip>] \"<arg1>\" \"<arg2>\" ... \"<argN>\"\r\n"
//
// Example Output:
//
//	"1704067200 [127.0.0.1:54321] \"SET\" \"key1\" \"value1\"\r\n"
//	"1704067201 [127.0.0.1:54322] \"GET\" \"key1\"\r\n"
//
// Usage:
//   - Called automatically for all registered monitoring clients when any
//     command is executed on the server
//   - Each monitoring client receives a copy of every command log
//   - The original command executor does not receive its own log
//
// Thread Safety:
//   - Called from a goroutine to avoid blocking the main command handler
//   - Each client connection is handled by a single goroutine
//   - Safe to call concurrently for different clients
//
// Note:
//   - This function is part of the MONITOR command implementation
//   - Clients must first execute MONITOR to receive these logs
//   - The function sends logs in RESP simple string format (+<message>\r\n)
//   - Logs are sent asynchronously to avoid blocking command execution
func (client *Client) WriterMonitorLog(value *Value, executingClient *Client) {

	executingClientIP := executingClient.Conn.RemoteAddr().String()
	clientIP := client.Conn.RemoteAddr().String()

	log.Printf("sending monitor log from=[%s] to=[%s]", executingClientIP, clientIP)

	msg := fmt.Sprintf("%d [%s --> %s] ", time.Now().Unix(), executingClientIP, clientIP)
	for _, v := range value.Arr {
		vmsg := fmt.Sprintf("\"%s\" ", v.Blk)
		msg += vmsg
	}
	w := NewWriter(client.Conn)
	w.Write(&Value{Typ: STRING, Str: msg})
	w.Flush()
}
