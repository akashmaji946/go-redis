package main

import (
	"fmt"
	"log"
	"net"
)

// Success Message:
// +OK\r\n

// initial command just before connection
// COMMAND DOCS
func Command(v *Value, state *AppState) *Value {
	// cmd := v.arr[0].blk
	return &Value{
		typ: STRING,
		str: "OK",
	}
}

// GET <key>
func Get(v *Value, state *AppState) *Value {
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
	val, ok := DB.store[key]
	DB.mu.RUnlock()
	if !ok {
		return &Value{
			typ: NULL,
		}
	}
	return &Value{
		typ: BULK,
		blk: val,
	}

}

func Set(v *Value, state *AppState) *Value {
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
	DB.store[key] = val
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

type Handler func(*Value, *AppState) *Value

var Handlers = map[string]Handler{
	"COMMAND": Command,
	"GET":     Get,
	"SET":     Set,
}

func handle(conn net.Conn, v *Value, state *AppState) {
	// the command is in the first entry of v.arr
	cmd := v.arr[0].blk
	handler, ok := Handlers[cmd]
	if !ok {
		log.Println("ERROR: no such command:", cmd)
		return
	}
	reply := handler(v, state)
	w := NewWriter(conn)
	w.Write(reply)
	w.Flush()
}
