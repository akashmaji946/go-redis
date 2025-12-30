package main

import (
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"strconv"
	"time"
)

// Success Message:
// +OK\r\n

// initial command just before connection
// COMMAND DOCS
func Command(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	return &Value{
		typ: STRING,
		str: "OK",
	}
}

var UNIX_TS_EPOCH = time.Time{}.Unix()

type Handler func(*Client, *Value, *AppState) *Value

var Handlers = map[string]Handler{
	"COMMAND": Command,

	"GET": Get,
	"SET": Set,

	"DEL": Del,

	"EXISTS": Exists,

	"KEYS": Keys,

	"SAVE":         Save,
	"BGSAVE":       BGSave,
	"BGREWRITEAOF": BGRewriteAOF,

	"FLUSHDB": FlushDB,
	"DBSIZE":  DBSize,

	"EXPIRE": Expire,
	"TTL":    Ttl,

	"AUTH": Auth,
}

func BGRewriteAOF(c *Client, v *Value, state *AppState) *Value {

	go func() {
		DB.mu.RLock()
		cp := make(map[string]*VAL, len(DB.store))
		maps.Copy(cp, DB.store)
		DB.mu.RUnlock()

		state.aof.Rewrite(cp)

	}()
	return &Value{
		typ: STRING,
		str: "Started.",
	}
}

// GET <key>
func Get(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	args := v.arr[1:]
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid command uage with GET",
		}
	}
	key := args[0].blk // grab the key

	// lock before read
	DB.mu.RLock()
	Val, ok := DB.Poll(key)
	DB.mu.RUnlock()
	if !ok {
		fmt.Println("Not Found: ", key)
		return &Value{
			typ: NULL,
		}
	}

	// delete if expired
	// if expiry is set and ttl is <= 0, delete and return NULL
	if Val.exp.Unix() != UNIX_TS_EPOCH && time.Until(Val.exp).Seconds() <= 0 {
		DB.mu.Lock()
		DB.Del(key)
		DB.mu.Unlock()
		return &Value{
			typ: NULL,
		}
	}

	return &Value{
		typ: BULK,
		blk: Val.v,
	}

}

func Set(c *Client, v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	args := v.arr[1:]
	if len(args) != 2 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid command usage with SET",
		}
	}

	key := args[0].blk // grab the key
	val := args[1].blk // grab the value

	DB.mu.Lock()
	DB.Put(key, val)
	// record it for AOF
	if state.config.aofEnabled {
		state.aof.w.Write(v)

		if state.config.aofFsync == Always {
			fmt.Println("save AOF record on SET")
			state.aof.w.Flush()
		}

	}

	if len(state.config.rdb) > 0 {
		IncrRDBTrackers()
	}

	DB.mu.Unlock()

	return &Value{
		typ: STRING,
		str: "OK",
	}
}

func Del(c *Client, v *Value, state *AppState) *Value {
	// DEL k1 k2 k3 ... kn
	// returns m, number of keys actually deleted (m <= n)
	args := v.arr[1:]
	m := 0
	DB.mu.Lock()
	for _, arg := range args {
		key := arg.blk
		_, ok := DB.store[key]
		if !ok {
			// doesnot exist
			continue
		}
		// delete
		delete(DB.store, key)
		m += 1
	}
	DB.mu.Unlock()
	return &Value{
		typ: INTEGER,
		num: m,
	}
}

func Exists(c *Client, v *Value, state *AppState) *Value {
	// Exists k1 k2 k3 .. kn
	// m (m <= n)
	args := v.arr[1:]
	m := 0
	DB.mu.RLock()
	for _, arg := range args {
		_, ok := DB.store[arg.blk]
		if ok {
			m += 1
		}
	}
	DB.mu.RUnlock()

	return &Value{
		typ: INTEGER,
		num: m,
	}
}

func Keys(c *Client, v *Value, state *AppState) *Value {
	// Keys pattern
	// all keys matching pattern (in an array)
	args := v.arr[1:]
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: "ERR invlid arg to Keys",
		}
	}

	pattern := args[0].blk // string representing the pattern e.g. "*name*" matches name, names, firstname, lastname

	DB.mu.RLock()
	var matches []string
	for key, _ := range DB.store {
		matched, err := filepath.Match(pattern, key)
		if err != nil {
			fmt.Printf("error matching for keys: (key=%s, pattern=%s)\nError: %s\n", key, pattern, err)
			continue
		}
		if matched {
			matches = append(matches, key)
		}
	}
	DB.mu.RUnlock()

	reply := Value{
		typ: ARRAY,
	}
	for _, key := range matches {
		reply.arr = append(reply.arr, Value{typ: BULK, blk: key})
	}
	return &reply
}

// saves with mutex read lock => inefficent, and no other command can be run to write
func Save(c *Client, v *Value, state *AppState) *Value {
	SaveRDB(state)
	return &Value{
		typ: STRING,
		str: "OK",
	}
}

// background save
// using COW is not possible, we will copy map first then save async
func BGSave(c *Client, v *Value, state *AppState) *Value {

	DB.mu.RLock()
	if state.bgsaving {
		// already running, return
		DB.mu.RUnlock()
		return &Value{
			typ: ERROR,
			err: "already in progress",
		}
	}

	copy := make(map[string]*VAL, len(DB.store)) // actual copy of DB.store
	maps.Copy(copy, DB.store)
	state.bgsaving = true
	state.DBCopy = copy // points to that

	DB.mu.RUnlock()

	go func() {
		defer func() {
			state.bgsaving = false
			state.DBCopy = nil
		}()

		SaveRDB(state)
	}()

	return &Value{
		typ: STRING,
		str: "OK",
	}
}

func FlushDB(c *Client, v *Value, state *AppState) *Value {
	// slower
	// DB.mu.Lock()
	// for k := range DB.store {
	// 	delete(DB.store, k)
	// }
	// DB.mu.Unlock()

	// fast
	DB.mu.Lock()
	DB.store = map[string]*VAL{}
	DB.mu.Unlock()

	return &Value{
		typ: STRING,
		str: "OK",
	}
}

func DBSize(c *Client, v *Value, state *AppState) *Value {
	// DBSIZE
	args := v.arr
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid argument to DBSIZE",
		}
	}

	DB.mu.RLock()
	size := len(DB.store)
	DB.mu.RUnlock()

	return &Value{
		typ: INTEGER,
		num: size,
	}

}

func Auth(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: fmt.Sprintf("ERR invalid argument to AUTH, given=%d, needed=1\n", len(args)),
		}
	}

	password := args[0].blk // AUTH <password>
	if state.config.password == password {
		c.authenticated = true
		return &Value{
			typ: STRING,
			str: "OK",
		}
	}
	c.authenticated = false
	return &Value{
		typ: ERROR,
		err: fmt.Sprintf("ERR invalid password, given=%s", password),
	}

}

func Expire(c *Client, v *Value, state *AppState) *Value {
	// EXPIRE <key> <secondsafter>
	args := v.arr[1:]
	if len(args) != 2 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid args for EXPIRE",
		}
	}
	k := args[0].blk
	exp := args[1].blk
	expirySeconds, err := strconv.Atoi(exp)
	if err != nil {
		return &Value{
			typ: ERROR,
			err: "ERR invalid 2nd arg for EXPIRE",
		}
	}

	DB.mu.RLock()
	Val, ok := DB.store[k]
	if !ok {
		return &Value{
			typ: INTEGER,
			num: 0,
		}
	}
	Val.exp = time.Now().Add(time.Second * time.Duration(expirySeconds))
	DB.mu.RUnlock()

	return &Value{
		typ: INTEGER,
		num: 1,
	}

}

func Ttl(c *Client, v *Value, state *AppState) *Value {
	// TTL <key>
	args := v.arr[1:]
	if len(args) != 1 {
		return &Value{
			typ: ERROR,
			err: "ERR invalid arg for TTL",
		}
	}

	k := args[0].blk

	DB.mu.RLock()
	Val, ok := DB.store[k]
	if !ok {
		return &Value{
			typ: INTEGER,
			num: -2,
		}
	}
	exp := Val.exp
	DB.mu.RUnlock()

	// is exp not set
	if exp.Unix() == UNIX_TS_EPOCH {
		return &Value{
			typ: INTEGER,
			num: -1,
		}
	}
	secondsToExpire := time.Until(exp).Seconds() //float
	if secondsToExpire <= 0.0 {
		DB.mu.Lock()
		DB.Del(k)
		DB.mu.Unlock()
		return &Value{
			typ: INTEGER,
			num: -2,
		}
	}
	fmt.Println(secondsToExpire)
	return &Value{
		typ: INTEGER,
		num: int(secondsToExpire),
	}

}

// can run these even if authenticated=0
var safeCommands = []string{
	"COMMAND",
	"AUTH",
}

func IsSafeCmd(cmd string, commands []string) bool {
	for _, command := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}

func handle(client *Client, v *Value, state *AppState) {
	// the command is in the first entry of v.arr
	cmd := v.arr[0].blk
	handler, ok := Handlers[cmd]
	if !ok {
		log.Println("ERROR: no such command:", cmd)
		reply := &Value{
			typ: ERROR,
			err: "ERR no such command",
		}
		w := NewWriter(client.conn)
		w.Write(reply)
		w.Flush()
		return
	}

	var reply *Value
	if state.config.requirepass && !client.authenticated && !IsSafeCmd(cmd, safeCommands) {
		reply = &Value{
			typ: ERROR,
			err: "NOAUTH client not authenticated, use AUTH <password>",
		}
	} else {
		reply = handler(client, v, state)
	}
	w := NewWriter(client.conn)
	w.Write(reply)
	w.Flush()
}
