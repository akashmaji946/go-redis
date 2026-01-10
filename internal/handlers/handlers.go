/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handlers.go
*/
package handlers

import (
	"fmt"
	"log"
	"strings"

	"github.com/akashmaji946/go-redis/internal/database"

	"github.com/akashmaji946/go-redis/internal/common"
)

func init() {
	Handlers["COMMANDS"] = Commands
}

var Handlers = map[string]common.Handler{

	// health check
	"COMMAND": Command,
	"PING":    Ping,

	"GET": Get,
	"SET": Set,
	// "SETNX":  Setnx,
	"INCR":   Incr,
	"DECR":   Decr,
	"INCRBY": IncrBy,
	"DECRBY": DecrBy,
	"MGET":   Mget,
	"MSET":   Mset,

	// List commands
	"LPUSH":  Lpush,
	"RPUSH":  Rpush,
	"LPOP":   Lpop,
	"RPOP":   Rpop,
	"LRANGE": Lrange,
	"LLEN":   Llen,
	"LINDEX": Lindex,
	"LGET":   Lget,

	// Set commands
	"SADD":        Sadd,
	"SREM":        Srem,
	"SMEMBERS":    Smembers,
	"SISMEMBER":   Sismember,
	"SCARD":       Scard,
	"SINTER":      Sinter,
	"SUNION":      Sunion,
	"SDIFF":       Sdiff,
	"SRANDMEMBER": Srandmember,

	// Sorted Set commands
	"ZADD":      Zadd,
	"ZREM":      Zrem,
	"ZSCORE":    Zscore,
	"ZCARD":     Zcard,
	"ZRANGE":    Zrange,
	"ZREVRANGE": Zrevrange,
	"ZGET":      Zget,

	"TYPE": Type,

	"DEL":    Del,
	"RENAME": Rename,

	"EXISTS": Exists,

	"KEYS": Keys,

	"SAVE":         Save,
	"BGSAVE":       BGSave,
	"BGREWRITEAOF": BGRewriteAOF,

	"FLUSHDB": FlushDB,
	"DBSIZE":  DBSize,

	"EXPIRE":  Expire,
	"TTL":     Ttl,
	"PERSIST": Persist,

	// Hash commands
	"HSET":    Hset,
	"HGET":    Hget,
	"HDEL":    Hdel,
	"HGETALL": Hgetall,
	"HDELALL": Hdelall,
	"HINCRBY": Hincrby,
	"HMSET":   Hmset,
	"HEXISTS": Hexists,
	"HLEN":    Hlen,
	"HKEYS":   Hkeys,
	"HVALS":   Hvals,
	"HEXPIRE": Hexpire,

	// authorize
	"AUTH": Auth,

	// transaction
	"MULTI":   Multi,
	"EXEC":    Exec,
	"DISCARD": Discard,
	"WATCH":   Watch,
	"UNWATCH": Unwatch,

	"MONITOR": Monitor,
	"INFO":    Info,

	// pubsub
	"PUBLISH":     Publish,
	"SUBSCRIBE":   Subscribe,
	"UNSUBSCRIBE": Unsubscribe,

	"PSUBSCRIBE":   Psubscribe,
	"PUNSUBSCRIBE": Punsubscribe,
	// "PUBSUB":       Pubsub,
	// "ECHO":         Echo,
	// "QUIT":         Quit,
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

	if !state.Config.Sensitive {
		cmd = strings.ToUpper(cmd)
	}

	handler, ok := Handlers[cmd]

	if !ok {
		log.Println("common.ERROR: no such command:", cmd)
		msg := fmt.Sprintf("ERR no such command '%s', use COMMANDS for help", cmd)
		reply := common.NewErrorValue(msg)
		w := common.NewWriter(client.Conn)
		w.Write(reply)
		w.Flush()
		return
	}

	// handle authentication: if password needed & not Authenticated, then block running command
	if state.Config.Requirepass && !client.Authenticated && !IsSafeCmd(cmd, safeCommands) {
		reply := common.NewErrorValue("NOAUTH client not Authenticated, use AUTH <password>")
		w := common.NewWriter(client.Conn)
		w.Write(reply)
		w.Flush()
		return
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

	w := common.NewWriter(client.Conn)
	w.Write(reply)
	w.Flush()

	// for MONITOR handle will send to all monitors
	go func() {
		for _, mon := range state.Monitors {
			if &mon != client {
				mon.WriterMonitorLog(v, client)
			}
		}
	}()

}
