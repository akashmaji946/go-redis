package common

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

var (
	UNIX_TS_EPOCH = time.Time{}.Unix()
)

var ASCII_ART = `
		  [91m ██████╗  ██████╗ [0m
		  [91m██╔════╝ ██╔═══██╗[0m
		  [91m██║  ███╗██║   ██║[0m
		  [91m██║   ██║██║   ██║[0m
		  [91m╚██████╔╝╚██████╔╝[0m
		  [91m ╚═════╝  ╚═════╝ [0m

	   [92m██████╗ ███████╗██████╗ ██╗███████╗[0m
	   [92m██╔══██╗██╔════╝██╔══██╗██║██╔════╝[0m
	   [92m██████╔╝█████╗  ██║  ██║██║███████╗[0m
	   [92m██╔══██╗██╔══╝  ██║  ██║██║╚════██║[0m
	   [92m██║  ██║███████╗██████╔╝██║███████║[0m
	   [92m╚═╝  ╚═╝╚══════╝╚═════╝ ╚═╝╚══════╝[0m

   [94m███████╗███████╗██████╗ ██╗   ██╗███████╗██████╗ [0m
   [94m██╔════╝██╔════╝██╔══██╗██║   ██║██╔════╝██╔══██╗[0m
   [94m███████╗█████╗  ██████╔╝██║   ██║█████╗  ██████╔╝[0m
   [94m╚════██║██╔══╝  ██╔══██╗╚██╗ ██╔╝██╔══╝  ██╔══██╗[0m
   [94m███████║███████╗██║  ██║ ╚████╔╝ ███████╗██║  ██║[0m
   [94m╚══════╝╚══════╝╚═╝  ╚═╝  ╚═══╝  ╚══════╝╚═╝  ╚═╝[0m
   [93m         [93m >>> Go-Redis Server v1.0 <<<      [0m
`

// CommandInfo stores metadata for a command.
type CommandInfo struct {
	Usage       string `json:"usage"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// CommandDetails maps command names to their usage and description.

var CommandDetails map[string]CommandInfo

func init() {
	CommandDetails = make(map[string]CommandInfo)

	data, err := os.ReadFile("commands.json")
	if err != nil {
		log.Fatalf("failed to read commands.json: %v", err)
	}

	if err := json.Unmarshal(data, &CommandDetails); err != nil {
		log.Fatalf("failed to parse commands.json: %v", err)
	}
}

// var CommandDetailsOld = map[string]CommandInfo{
// 	"AUTH": {
// 		Usage:       "AUTH <password>",
// 		Description: "Authenticate to the server",
// 	},
// 	"BGREWRITEAOF": {
// 		Usage:       "BGREWRITEAOF",
// 		Description: "Asynchronously rewrite the Append-Only File",
// 	},
// 	"BGSAVE": {
// 		Usage:       "BGSAVE",
// 		Description: "Asynchronously save the database to disk",
// 	},
// 	"COMMAND": {
// 		Usage:       "COMMAND",
// 		Description: "Get help about Redis commands",
// 	},
// 	"COMMANDS": {
// 		Usage:       "COMMANDS [pattern]",
// 		Description: "List available commands or get help for a specific command",
// 	},
// 	"DBSIZE": {
// 		Usage:       "DBSIZE",
// 		Description: "Return the number of keys in the database",
// 	},
// 	"DECR": {
// 		Usage:       "DECR <key>",
// 		Description: "Decrement the integer value of a key by one",
// 	},
// 	"DECRBY": {
// 		Usage:       "DECRBY <key> <decrement>",
// 		Description: "Decrement the integer value of a key by the given amount",
// 	},
// 	"DEL": {
// 		Usage:       "DEL <key> [key ...]",
// 		Description: "Delete one or more keys",
// 	},
// 	"DISCARD": {
// 		Usage:       "DISCARD",
// 		Description: "Discard all commands issued after MULTI",
// 	},
// 	"EXEC": {
// 		Usage:       "EXEC",
// 		Description: "Execute all commands issued after MULTI",
// 	},
// 	"EXISTS": {
// 		Usage:       "EXISTS <key> [key ...]",
// 		Description: "Check if keys exist",
// 	},
// 	"EXPIRE": {
// 		Usage:       "EXPIRE <key> <seconds>",
// 		Description: "Set a key's time to live in seconds",
// 	},
// 	"FLUSHDB": {
// 		Usage:       "FLUSHDB",
// 		Description: "Remove all keys from the database",
// 	},
// 	"GET": {
// 		Usage:       "GET <key>",
// 		Description: "Get the value of a key",
// 	},
// 	"HDEL": {
// 		Usage:       "HDEL <key> <field> [field ...]",
// 		Description: "Delete one or more hash fields",
// 	},
// 	"HDELALL": {
// 		Usage:       "HDELALL <key>",
// 		Description: "Delete all fields in a hash",
// 	},
// 	"HEXISTS": {
// 		Usage:       "HEXISTS <key> <field>",
// 		Description: "Check if a hash field exists",
// 	},
// 	"HEXPIRE": {
// 		Usage:       "HEXPIRE <key> <field> <seconds>",
// 		Description: "Set expiration for a hash field",
// 	},
// 	"HGET": {
// 		Usage:       "HGET <key> <field>",
// 		Description: "Get the value of a hash field",
// 	},
// 	"HGETALL": {
// 		Usage:       "HGETALL <key>",
// 		Description: "Get all fields and values in a hash",
// 	},
// 	"HINCRBY": {
// 		Usage:       "HINCRBY <key> <field> <increment>",
// 		Description: "Increment the integer value of a hash field by the given amount",
// 	},
// 	"HKEYS": {
// 		Usage:       "HKEYS <key>",
// 		Description: "Get all field names in a hash",
// 	},
// 	"HLEN": {
// 		Usage:       "HLEN <key>",
// 		Description: "Get the number of fields in a hash",
// 	},
// 	"HMSET": {
// 		Usage:       "HMSET <key> <field> <value> [field <value> ...]",
// 		Description: "Set multiple hash fields to multiple values",
// 	},
// 	"HSET": {
// 		Usage:       "HSET <key> <field> <value> [field <value> ...]",
// 		Description: "Set the string value of a hash field",
// 	},
// 	"HVALS": {
// 		Usage:       "HVALS <key>",
// 		Description: "Get all values in a hash",
// 	},
// 	"INCR": {
// 		Usage:       "INCR <key>",
// 		Description: "Increment the integer value of a key by one",
// 	},
// 	"INCRBY": {
// 		Usage:       "INCRBY <key> <increment>",
// 		Description: "Increment the integer value of a key by the given amount",
// 	},
// 	"INFO": {
// 		Usage:       "INFO [key]",
// 		Description: "Get server information and statistics or per-key metadata",
// 	},
// 	"KEYS": {
// 		Usage:       "KEYS <pattern>",
// 		Description: "Find all keys matching the given pattern",
// 	},
// 	"LGET": {
// 		Usage:       "LGET <key>",
// 		Description: "Get all elements in a list",
// 	},
// 	"LINDEX": {
// 		Usage:       "LINDEX <key> <index>",
// 		Description: "Get an element from a list by its index",
// 	},
// 	"LLEN": {
// 		Usage:       "LLEN <key>",
// 		Description: "Get the length of a list",
// 	},
// 	"LPOP": {
// 		Usage:       "LPOP <key>",
// 		Description: "Remove and get the first element in a list",
// 	},
// 	"LPUSH": {
// 		Usage:       "LPUSH <key> <value> [value ...]",
// 		Description: "Prepend one or multiple values to a list",
// 	},
// 	"LRANGE": {
// 		Usage:       "LRANGE <key> <start> <stop>",
// 		Description: "Get a range of elements from a list",
// 	},
// 	"MGET": {
// 		Usage:       "MGET <key> [key ...]",
// 		Description: "Get the values of all the given keys",
// 	},
// 	"MONITOR": {
// 		Usage:       "MONITOR",
// 		Description: "Listen for all requests received by the server in real time",
// 	},
// 	"MSET": {
// 		Usage:       "MSET <key> <value> [key <value> ...]",
// 		Description: "Set multiple keys to multiple values",
// 	},
// 	"MULTI": {
// 		Usage:       "MULTI",
// 		Description: "Mark the start of a transaction block",
// 	},
// 	"PERSIST": {
// 		Usage:       "PERSIST <key>",
// 		Description: "Remove the expiration from a key",
// 	},
// 	"PING": {
// 		Usage:       "PING [message]",
// 		Description: "Ping the server",
// 	},
// 	"PSUBSCRIBE": {
// 		Usage:       "PSUBSCRIBE <pattern> [pattern ...]",
// 		Description: "Listen for messages published to channels matching the given patterns",
// 	},
// 	"PUBLISH": {
// 		Usage:       "PUBLISH <channel> <message>",
// 		Description: "Post a message to a channel",
// 	},
// 	"PUNSUBSCRIBE": {
// 		Usage:       "PUNSUBSCRIBE [pattern ...]",
// 		Description: "Stop listening for messages posted to channels matching the given patterns",
// 	},
// 	"RENAME": {
// 		Usage:       "RENAME <key> <newkey>",
// 		Description: "Rename a key",
// 	},
// 	"RPOP": {
// 		Usage:       "RPOP <key>",
// 		Description: "Remove and get the last element in a list",
// 	},
// 	"RPUSH": {
// 		Usage:       "RPUSH <key> <value> [value ...]",
// 		Description: "Append one or multiple values to a list",
// 	},
// 	"SADD": {
// 		Usage:       "SADD <key> <member> [member ...]",
// 		Description: "Add one or more members to a set",
// 	},
// 	"SAVE": {
// 		Usage:       "SAVE",
// 		Description: "Synchronously save the database to disk",
// 	},
// 	"SCARD": {
// 		Usage:       "SCARD <key>",
// 		Description: "Get the number of members in a set",
// 	},
// 	"SDIFF": {
// 		Usage:       "SDIFF <key> [key ...]",
// 		Description: "Subtract multiple sets",
// 	},
// 	"SET": {
// 		Usage:       "SET <key> <value>",
// 		Description: "Set the string value of a key",
// 	},
// 	"SINTER": {
// 		Usage:       "SINTER <key> [key ...]",
// 		Description: "Intersect multiple sets",
// 	},
// 	"SISMEMBER": {
// 		Usage:       "SISMEMBER <key> <member>",
// 		Description: "Determine if a given value is a member of a set",
// 	},
// 	"SMEMBERS": {
// 		Usage:       "SMEMBERS <key>",
// 		Description: "Get all the members in a set",
// 	},
// 	"SRANDMEMBER": {
// 		Usage:       "SRANDMEMBER <key> [count]",
// 		Description: "Get one or multiple random members from a set",
// 	},
// 	"SREM": {
// 		Usage:       "SREM <key> <member> [member ...]",
// 		Description: "Remove one or more members from a set",
// 	},
// 	"SUBSCRIBE": {
// 		Usage:       "SUBSCRIBE <channel> [channel ...]",
// 		Description: "Listen for messages published to the given channels",
// 	},
// 	"SUNION": {
// 		Usage:       "SUNION <key> [key ...]",
// 		Description: "Add multiple sets",
// 	},
// 	"TTL": {
// 		Usage:       "TTL <key>",
// 		Description: "Get the time to live for a key in seconds",
// 	},
// 	"TYPE": {
// 		Usage:       "TYPE <key>",
// 		Description: "Determine the type stored at key",
// 	},
// 	"UNSUBSCRIBE": {
// 		Usage:       "UNSUBSCRIBE [channel ...]",
// 		Description: "Stop listening for messages posted to the given channels",
// 	},
// 	"UNWATCH": {
// 		Usage:       "UNWATCH",
// 		Description: "Forget about all watched keys",
// 	},
// 	"WATCH": {
// 		Usage:       "WATCH <key> [key ...]",
// 		Description: "Watch the given keys to determine execution of the MULTI/EXEC block",
// 	},
// 	"ZADD": {
// 		Usage:       "ZADD <key> <score> <member> [score <member> ...]",
// 		Description: "Add one or more members to a sorted set, or update its score if it already exists",
// 	},
// 	"ZCARD": {
// 		Usage:       "ZCARD <key>",
// 		Description: "Get the number of members in a sorted set",
// 	},
// 	"ZGET": {
// 		Usage:       "ZGET <key> [<member>]",
// 		Description: "Get score of a member or all members with scores",
// 	},
// 	"ZRANGE": {
// 		Usage:       "ZRANGE <key> <start> <stop> [WITHSCORES]",
// 		Description: "Return a range of members in a sorted set, by index",
// 	},
// 	"ZREM": {
// 		Usage:       "ZREM <key> <member> [member ...]",
// 		Description: "Remove one or more members from a sorted set",
// 	},
// 	"ZREVRANGE": {
// 		Usage:       "ZREVRANGE <key> <start> <stop> [WITHSCORES]",
// 		Description: "Return a range of members in a sorted set, by index, with scores ordered from high to low",
// 	},
// 	"ZSCORE": {
// 		Usage:       "ZSCORE <key> <member>",
// 		Description: "Get the score associated with the given member in a sorted set",
// 	},
// }
