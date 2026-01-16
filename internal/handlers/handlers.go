/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handlers.go
*/
package handlers

import (
	"fmt"

	"github.com/akashmaji946/go-redis/internal/database"

	"github.com/akashmaji946/go-redis/internal/common"
)

var logger = common.NewLogger()

func init() {
	Handlers["COMMANDS"] = Commands
}

var Handlers = map[string]common.Handler{

	// health check
	"COMMAND": Command,
	"PING":    Ping,

	// string commands
	"GET": Get,
	"SET": Set,
	// "SETNX":  Setnx,
	"INCR":   Incr,
	"DECR":   Decr,
	"INCRBY": IncrBy,
	"DECRBY": DecrBy,
	"MGET":   Mget,
	"MSET":   Mset,
	"STRLEN": Strlen,

	// list commands
	"LPUSH":  Lpush,
	"RPUSH":  Rpush,
	"LPOP":   Lpop,
	"RPOP":   Rpop,
	"LRANGE": Lrange,
	"LLEN":   Llen,
	"LINDEX": Lindex,
	"LGET":   Lget,

	// set commands
	"SADD":        Sadd,
	"SREM":        Srem,
	"SMEMBERS":    Smembers,
	"SISMEMBER":   Sismember,
	"SCARD":       Scard,
	"SINTER":      Sinter,
	"SUNION":      Sunion,
	"SDIFF":       Sdiff,
	"SRANDMEMBER": Srandmember,

	// sorted set commands
	"ZADD":      Zadd,
	"ZREM":      Zrem,
	"ZSCORE":    Zscore,
	"ZCARD":     Zcard,
	"ZRANGE":    Zrange,
	"ZREVRANGE": Zrevrange,
	"ZGET":      Zget,

	// key commands
	"TYPE":    Type,
	"DELETE":  Del,
	"DEL":     Del,
	"RENAME":  Rename,
	"EXISTS":  Exists,
	"KEYS":    Keys,
	"EXPIRE":  Expire,
	"TTL":     Ttl,
	"PERSIST": Persist,

	// hash commands
	"HSET":    Hset,
	"HGET":    Hget,
	"HDEL":    Hdel,
	"HGETALL": Hgetall,
	"HDELALL": Hdelall,
	"HINCRBY": Hincrby,
	"HMSET":   Hmset,
	"HMGET":   Hmget,
	"HEXISTS": Hexists,
	"HLEN":    Hlen,
	"HKEYS":   Hkeys,
	"HVALS":   Hvals,
	"HEXPIRE": Hexpire,

	// authorize
	"AUTH":    Auth,
	"USERADD": UserAdd,
	"USERDEL": UserDel,
	"PASSWD":  Passwd,
	"USERS":   Users,
	"WHOAMI":  WhoAmI,

	// transaction
	"MULTI":   Multi,
	"EXEC":    Exec,
	"DISCARD": Discard,
	"WATCH":   Watch,
	"UNWATCH": Unwatch,

	// server commands
	"MONITOR":      Monitor,
	"INFO":         Info,
	"SAVE":         Save,
	"BGSAVE":       BGSave,
	"BGREWRITEAOF": BGRewriteAOF,
	"FLUSHALL":     FlushAll,
	"FLUSHDB":      FlushDB,
	"DROPDB":       FlushDB,
	"DBSIZE":       DBSize,
	"SIZE":         Size,

	// pubsub
	"PUBLISH":      Publish,
	"SUBSCRIBE":    Subscribe,
	"UNSUBSCRIBE":  Unsubscribe,
	"PUB":          Publish,
	"SUB":          Subscribe,
	"UNSUB":        Unsubscribe,
	"PSUBSCRIBE":   Psubscribe,
	"PUNSUBSCRIBE": Punsubscribe,
	"PSUB":         Psubscribe,
	"PUNSUB":       Punsubscribe,
	"ECHO":         Echo,
	// "QUIT":         Quit,

	// db commands
	"SELECT": Select,
	"SEL":    Select,

	// HyperLogLog commands
	"PFADD":       PfAdd,
	"PFCOUNT":     PfCount,
	"PFMERGE":     PfMerge,
	"PFDEBUG":     PfDebug,
	"_HLLRESTORE": HLLRestore, // Internal command for AOF replay

	// Bitmap commands
	"SETBIT":   SetBit,
	"GETBIT":   GetBit,
	"BITCOUNT": BitCount,
	"BITOP":    BitOp,
	"BITPOS":   BitPos,
	"BITFIELD": BitField,

	// Geospatial commands
	"GEOADD":         GeoAdd,
	"GEOPOS":         GeoPos,
	"GEODIST":        GeoDist,
	"GEOHASH":        GeoHash,
	"GEORADIUS":      GeoRadius,
	"GEOSEARCH":      GeoSearch,
	"GEOSEARCHSTORE": GeoSearchStore,
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
	if state.Config.Requirepass && !client.Authenticated && !IsSafeCmd(cmd, safeCommands) {
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
