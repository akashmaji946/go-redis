/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handlers.go
*/
package main

import (
	"log"
)

type Handler func(*Client, *Value, *AppState) *Value

func init() {
	Handlers["COMMANDS"] = Commands
}

var Handlers = map[string]Handler{

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

// handle is the main command dispatcher.
//
// Responsibilities:
//  1. Extract command name from parsed Value
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
//   - No transaction running (for EXEC/DISCARD) → handled by respective handlers
//
// Command Flow:
//  1. Parse command name from Value array
//  2. Check if command exists in Handlers map
//  3. Check authentication (if required)
//  4. Check transaction state:
//     - If transaction active: queue command (unless MULTI/EXEC/DISCARD)
//     - If no transaction: execute command immediately
//  5. Send response to client
func handle(client *Client, v *Value, state *AppState) {

	state.genStats.total_commands_executed += 1

	// the command is in the first entry of v.arr
	cmd := v.arr[0].blk
	handler, ok := Handlers[cmd]

	if !ok {
		log.Println("ERROR: no such command:", cmd)
		reply := NewErrorValue("ERR no such command")
		w := NewWriter(client.conn)
		w.Write(reply)
		w.Flush()
		return
	}

	// handle authentication: if password needed & not authenticated, then block running command
	if state.config.requirepass && !client.authenticated && !IsSafeCmd(cmd, safeCommands) {
		reply := NewErrorValue("NOAUTH client not authenticated, use AUTH <password>")
		w := NewWriter(client.conn)
		w.Write(reply)
		w.Flush()
		return
	}

	// handle transaction: if already running, then queue
	if state.tx != nil && cmd != "EXEC" && cmd != "DISCARD" && cmd != "MULTI" {
		txCmd := &TxCommand{
			value:   v,
			handler: handler,
		}
		state.tx.cmds = append(state.tx.cmds, txCmd)
		reply := NewStringValue("QUEUED")
		w := NewWriter(client.conn)
		w.Write(reply)
		w.Flush()
		return
	}

	reply := handler(client, v, state)
	w := NewWriter(client.conn)
	w.Write(reply)
	w.Flush()

	// for MONITOR handle will send to all monitors
	go func() {
		for _, mon := range state.monitors {
			if mon != *client {
				mon.writerMonitorLog(v, client)
			}
		}
	}()

}
