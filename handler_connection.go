/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler/connection.go
*/
package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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
	args := v.arr[1:]

	if len(args) > 1 {
		return NewErrorValue("ERR wrong number of arguments for 'commands' command")
	}

	// Case 1: COMMANDS or COMMANDS * (Return list of all command names)
	if len(args) == 0 || (len(args) == 1 && args[0].blk == "*") {
		var cmds []string
		for k := range Handlers {
			cmds = append(cmds, k)
		}
		sort.Strings(cmds)
		var arr []Value
		for _, cmd := range cmds {
			arr = append(arr, Value{typ: BULK, blk: cmd})
		}
		return NewArrayValue(arr)
	}

	// Case 2: COMMANDS <cmd> or <pattern>
	arg := args[0].blk
	if !state.config.sensitive {
		arg = strings.ToUpper(arg)
	}

	// If it's an exact command match, show detailed info in 3 lines
	if info, ok := CommandDetails[arg]; ok {
		return NewArrayValue([]Value{
			{typ: BULK, blk: fmt.Sprintf("Usage       : %s", info.Usage)},
			{typ: BULK, blk: fmt.Sprintf("Description : %s", info.Description)},
			{typ: BULK, blk: fmt.Sprintf("Category    : %s", info.Category)},
		})
	}

	// Otherwise, treat as a pattern and return matching command names
	var keys []string
	for k := range Handlers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var results []Value
	for _, cmd := range keys {
		matched, _ := filepath.Match(arg, cmd)
		if matched {
			results = append(results, Value{typ: BULK, blk: cmd})
		}
	}

	if len(results) == 0 {
		return NewErrorValue(fmt.Sprintf("ERR unknown command or no match for '%s'", arg))
	}
	return NewArrayValue(results)
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
		return NewErrorValue(fmt.Sprintf("ERR invalid argument to AUTH, given=%d, needed=1", len(args)))
	}

	password := args[0].blk // AUTH <password>
	if state.config.password == password {
		c.authenticated = true
		return NewStringValue("OK")
	}
	c.authenticated = false
	return NewErrorValue(fmt.Sprintf("ERR invalid password, given=%s", password))
}
