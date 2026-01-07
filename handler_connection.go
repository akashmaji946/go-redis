1/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler/connection.go
*/
package main

import (
	"fmt"
)

// can run these even if authenticated=0
var safeCommands = []string{
	"COMMAND",
	"PING",
	"COMMANDS",
	"HELP",
	"AUTH",
}

// IsSafeCmd checks whether a command can be executed without authentication.
func IsSafeCmd(cmd string, commands []string) bool {
	for _, command := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}

// COMMAND DOCS
// Command handles the COMMAND command.
func Command(c *Client, v *Value, state *AppState) *Value {
	return NewStringValue("OK")
}

// Commands handles the COMMANDS command.
func Commands(c *Client, v *Value, state *AppState) *Value {
	var cmds []string
	for k := range Handlers {
		cmds = append(cmds, k)
	}

	var arr []Value
	for _, cmd := range cmds {
		arr = append(arr, Value{typ: BULK, blk: cmd})
	}
	return NewArrayValue(arr)
}

func Ping(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) > 1 {
		return NewErrorValue("ERR wrong number of arguments for 'ping' command")
	}
	if len(args) == 1 {
		return NewStringValue(args[0].blk)
	}
	return NewStringValue("PONG")
}

// Auth handles the AUTH command.
func Auth(c *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 1 {
		return NewErrorValue(fmt.Sprintf("ERR invalid argument to AUTH, given=%d, needed=1\n", len(args)))
	}

	password := args[0].blk // AUTH <password>
	if state.config.password == password {
		c.authenticated = true
		return NewStringValue("OK")
	}
	c.authenticated = false
	return NewErrorValue(fmt.Sprintf("ERR invalid password, given=%s", password))
}
