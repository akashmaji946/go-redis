/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/handler/connection.go
*/
package handlers

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akashmaji946/go-redis/internal/common"
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
func Command(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	return common.NewStringValue("OK")
}

// Commands handles the COMMANDS command.
func Commands(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]

	if len(args) > 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'commands' command")
	}

	// Case 1: COMMANDS or COMMANDS * (Return list of all command names)
	if len(args) == 0 || (len(args) == 1 && args[0].Blk == "*") {
		var cmds []string
		for k := range Handlers {
			cmds = append(cmds, k)
		}
		sort.Strings(cmds)
		var Arr []common.Value
		for _, cmd := range cmds {
			Arr = append(Arr, common.Value{Typ: common.BULK, Blk: cmd})
		}
		return common.NewArrayValue(Arr)
	}

	// Case 2: COMMANDS <cmd> or <pattern>
	arg := args[0].Blk
	if !state.Config.Sensitive {
		arg = strings.ToUpper(arg)
	}

	// If it's an exact command match, show detailed info in 3 lines
	if info, ok := common.CommandDetails[arg]; ok {
		return common.NewArrayValue([]common.Value{
			{Typ: common.BULK, Blk: fmt.Sprintf("Usage       : %s", info.Usage)},
			{Typ: common.BULK, Blk: fmt.Sprintf("Description : %s", info.Description)},
			{Typ: common.BULK, Blk: fmt.Sprintf("Category    : %s", info.Category)},
		})
	}

	// Otherwise, treat as a pattern and return matching command names
	var keys []string
	for k := range Handlers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var results []common.Value
	for _, cmd := range keys {
		matched, _ := filepath.Match(arg, cmd)
		if matched {
			results = append(results, common.Value{Typ: common.BULK, Blk: cmd})
		}
	}

	if len(results) == 0 {
		return common.NewErrorValue(fmt.Sprintf("ERR unknown command or no match for '%s'", arg))
	}
	return common.NewArrayValue(results)
}

func Ping(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) > 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'ping' command")
	}
	if len(args) == 1 {
		return common.NewStringValue(args[0].Blk)
	}
	return common.NewStringValue("PONG")
}

// Auth handles the AUTH command.
func Auth(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 1 {
		return common.NewErrorValue(fmt.Sprintf("ERR invalid argument to AUTH, given=%d, needed=1", len(args)))
	}

	password := args[0].Blk // AUTH <password>
	if state.Config.Password == password {
		c.Authenticated = true
		return common.NewStringValue("OK")
	}
	c.Authenticated = false
	return common.NewErrorValue(fmt.Sprintf("ERR invalid password, given=%s", password))
}
