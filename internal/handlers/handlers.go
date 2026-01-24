/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/internal/handlers/handlers.go
*/
package handlers

import (
	"fmt"

	"github.com/akashmaji946/go-redis/internal/database"

	"github.com/akashmaji946/go-redis/internal/common"
)

var logger = common.NewLogger()

// Handlers is the main map of command names to their handler functions.
var Handlers = make(map[string]common.Handler)

func init() {
	Handlers["COMMANDS"] = Commands

	// merge all handler maps
	for k, v := range StringHandlers {
		Handlers[k] = v
	}
	for k, v := range ListHandlers {
		Handlers[k] = v
	}
	for k, v := range SetHandlers {
		Handlers[k] = v
	}
	for k, v := range ZSetHandlers {
		Handlers[k] = v
	}
	for k, v := range KeyHandlers {
		Handlers[k] = v
	}
	for k, v := range HashHandlers {
		Handlers[k] = v
	}
	for k, v := range ConnectionHandlers {
		Handlers[k] = v
	}
	for k, v := range TransactionHandlers {
		Handlers[k] = v
	}
	for k, v := range GenericHandlers {
		Handlers[k] = v
	}
	for k, v := range PubSubHandlers {
		Handlers[k] = v
	}
	for k, v := range PersistenceHandlers {
		Handlers[k] = v
	}
	for k, v := range BitmapHandlers {
		Handlers[k] = v
	}
	for k, v := range GeoHandlers {
		Handlers[k] = v
	}
	for k, v := range HyperLogLogHandlers {
		Handlers[k] = v
	}

}

// can run these even if authenticated=0
var safeCommands = []string{
	"COMMAND",
	"PING",
	"COMMANDS",
	"HELP",
	"AUTH",
	"PASSWD",
	"WHOAMI",
}

// sensitiveCommands is a set of commands that need root user
var sensitiveCommands = map[string]bool{
	"FLUSHDB":  true,
	"DROPDB":   true,
	"FLUSHALL": true,

	"USERADD": true,
	"USERDEL": true,
	"USERS":   true,

	"BGREWRITEAOF": true,
	"BGSAVE":       true,
	"SAVE":         true,
}

// Handler is a function type that processes Redis commands.
// Each command has a corresponding handler function that implements its logic.
type Handler func(*common.Client, *common.Value, *common.AppState) *common.Value

// handle is the main command dispatcher.
//
// Responsibilities:
//  1. Extract command name from parsed common.Value
//  2. Lookup command handler in Handlers map
//  3. Enforce authentication rules (if requirepass is set)
//  4. Handle transaction queuing (if transaction is active)
//  5. Execute handler or queue command
//  6. Write response to client
//
// Transaction Support:
//   - If state.tx is not nil (transaction active):
//   - Commands (except MULTI, EXEC, DISCARD) are queued
//   - Returns "QUEUED" response instead of executing
//   - Commands are stored with their handler for later execution
//   - Transaction control commands (MULTI, EXEC, DISCARD) execute immediately
//
// Error cases:
//   - Unknown command → ERR no such command
//   - Authentication required but missing → NOAUTH error
//   - Transaction already running (for MULTI) → handled by Multi handler
//   - common.No transaction running (for EXEC/DISCARD) → handled by respective handlers
//
// Command Flow:
//  1. Parse command name from common.Value array
//  2. Check if command exists in Handlers map
//  3. Check authentication (if required)
//  4. Check transaction state:
//     - If transaction active: queue command (unless MULTI/EXEC/DISCARD)
//     - If no transaction: execute command immediately
//  5. Send response to client
func Handle(client *common.Client, v *common.Value, state *common.AppState) {

	state.GenStats.TotalCommandsExecuted += 1

	// the command is in the first entry of v.Arr
	cmd := v.Arr[0].Blk

	handler, ok := Handlers[cmd]

	if !ok {
		logger.Warn("ERROR: no such command: '%s'\n", cmd)
		msg := fmt.Sprintf("ERR no such command '%s', use COMMANDS for help", cmd)
		reply := common.NewErrorValue(msg)
		if client != nil && client.Conn != nil {
			w := common.NewWriter(client.Conn)
			w.Write(reply)
			w.Flush()
		}
		return
	}

	// handle authentication: if password needed & not Authenticated, then block running command
	if state.Config.Requirepass && !client.Authenticated && !isSafeCmd(cmd, safeCommands) {
		reply := common.NewErrorValue("NOAUTH client not Authenticated, use AUTH <password>")
		if client != nil && client.Conn != nil {
			w := common.NewWriter(client.Conn)
			w.Write(reply)
			w.Flush()
		}
		return
	}

	// Check for admin permissions on sensitive commands
	if sensitiveCommands[cmd] {
		if client.User == nil || !client.User.Admin {
			reply := common.NewErrorValue("ERR only admins can run this command")
			w := common.NewWriter(client.Conn)
			w.Write(reply)
			w.Flush()
			return
		}
	}

	var reply *common.Value

	// If client is in a transaction and the command is NOT a transaction control command, queue it.
	if client.InTx && cmd != "MULTI" && cmd != "EXEC" && cmd != "DISCARD" {
		// Append a copy of the command to the client's private queue
		client.Tx.Cmds = append(client.Tx.Cmds, &common.TxCommand{
			Value:   v,
			Handler: handler,
		})
		reply = common.NewStringValue("QUEUED")
	} else {
		// Otherwise, execute the handler immediately (normal command or MULTI/EXEC/DISCARD).
		// Transaction control commands are executed directly to avoid deadlocks with txMu.
		if cmd == "MULTI" || cmd == "EXEC" || cmd == "DISCARD" || cmd == "WATCH" || cmd == "UNWATCH" {
			reply = handler(client, v, state)
		} else {
			// Normal commands must wait if a transaction is currently executing.
			database.DB.TxMu.RLock()
			reply = handler(client, v, state)
			database.DB.TxMu.RUnlock()
		}
	}

	if client != nil && client.Conn != nil {
		w := common.NewWriter(client.Conn)
		w.Write(reply)
		w.Flush()
	}

	// for MONITOR handle will send to all monitors
	go func() {
		for _, mon := range state.Monitors {
			if &mon != client {
				mon.WriterMonitorLog(v, client)
			}
		}
	}()

}

// isSafeCmd checks whether a command can be executed without authentication.
func isSafeCmd(cmd string, commands []string) bool {
	for _, command := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}

// isAlphanumeric checks if a string contains only alphanumeric characters.
func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// getUserDetails formats user details into a slice of common.Value for response.
func getUserDetails(user common.User) []common.Value {
	details := make([]common.Value, 0)
	details = append(details, *common.NewBulkValue(fmt.Sprintf("Username  : %s", user.Username)))
	details = append(details, *common.NewBulkValue(fmt.Sprintf("Client IP : %s", user.ClientIP)))
	details = append(details, *common.NewBulkValue(fmt.Sprintf("Admin     : %v", user.Admin)))
	details = append(details, *common.NewBulkValue(fmt.Sprintf("Full Name : %s", user.FullName)))
	return details
}

// saveDBState will save AOF and RDB
func saveDBState(state *common.AppState, v *common.Value) {
	if state.Config.AofEnabled {
		state.Aof.W.Write(v)
		if state.Config.AofFsync == common.Always {
			state.Aof.W.Flush()
		}
	}
	if len(state.Config.Rdb) > 0 {
		database.DB.IncrTrackers()
	}
}
