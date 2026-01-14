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
	"PASSWD",
	"WHOAMI",
}

// sensitiveCommands is a set of commands that need root user
var sensitiveCommands = map[string]bool{
	"FLUSHDB": true,
	"DROPDB":  true,

	"USERADD": true,
	"USERDEL": true,
	"USERS":   true,

	"BGREWRITEAOF": true,
	"BGSAVE":       true,
	"SAVE":         true,
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

func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
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
	if len(args) != 2 {
		return common.NewErrorValue("ERR usage: AUTH <user> <password>")
	}

	username := args[0].Blk
	password := args[1].Blk

	state.UsersMu.RLock()
	user, ok := state.Users[username]
	state.UsersMu.RUnlock()

	if !ok {
		return common.NewErrorValue("ERR user not found")
	}

	if user.Password == password {
		c.Authenticated = true
		c.User = user
		user.ClientIP = c.Conn.RemoteAddr().String()
		return common.NewStringValue("OK")
	}

	return common.NewErrorValue("ERR invalid password")
}

// UserAdd handles the USERADD command.
func UserAdd(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	if c.User == nil || !c.User.Admin {
		return common.NewErrorValue("ERR only admins can add users")
	}

	args := v.Arr[1:]
	if len(args) != 3 {
		return common.NewErrorValue("ERR usage: USERADD <admin_flag 1/0> <user> <password>")
	}

	isAdmin := args[0].Blk == "1"
	username := args[1].Blk
	password := args[2].Blk

	if !isAlphanumeric(password) {
		return common.NewErrorValue("ERR password must be alphanumeric")
	}

	state.UsersMu.Lock()
	state.Users[username] = &common.User{
		Username: username,
		FullName: username,
		Password: password,
		Admin:    isAdmin,
	}
	state.UsersMu.Unlock()
	state.SaveUsers()

	return common.NewStringValue("OK")
}

// Passwd handles the PASSWD command.
func Passwd(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR usage: PASSWD <user> <password>")
	}

	targetUser := args[0].Blk
	newPass := args[1].Blk

	if !isAlphanumeric(newPass) {
		return common.NewErrorValue("ERR password must be alphanumeric")
	}

	if c.User == nil || (c.User.Username != targetUser && !c.User.Admin) {
		return common.NewErrorValue("ERR permission denied")
	}

	state.UsersMu.Lock()
	if user, ok := state.Users[targetUser]; ok {
		user.Password = newPass
		state.UsersMu.Unlock()
		state.SaveUsers()
		return common.NewStringValue("OK")
	}
	state.UsersMu.Unlock()

	return common.NewErrorValue("ERR user not found")
}

// Users handles the USERS command.
func Users(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) > 1 {
		return common.NewErrorValue("ERR usage: USERS [username]")
	}

	state.UsersMu.RLock()
	defer state.UsersMu.RUnlock()

	if len(args) == 0 {
		var usernames []string
		for uname := range state.Users {
			usernames = append(usernames, uname)
		}
		sort.Strings(usernames)

		var result []common.Value
		for _, uname := range usernames {
			result = append(result, *common.NewBulkValue(uname))
		}
		return common.NewArrayValue(result)
	}

	targetUsername := args[0].Blk
	user, ok := state.Users[targetUsername]
	if !ok {
		return common.NewErrorValue("ERR user not found")
	}
	details := getUserDetails(*user)
	return common.NewArrayValue(details)
}

// WhoAmI handles the WHOAMI command.
func WhoAmI(c *common.Client, v *common.Value, state *common.AppState) *common.Value {
	if c.User == nil {
		return common.NewErrorValue("ERR not authenticated")
	}

	details := getUserDetails(*c.User)
	return common.NewArrayValue(details)
}

func getUserDetails(user common.User) []common.Value {
	details := make([]common.Value, 0)
	details = append(details, *common.NewBulkValue(fmt.Sprintf("Username  : %s", user.Username)))
	details = append(details, *common.NewBulkValue(fmt.Sprintf("Client IP : %s", user.ClientIP)))
	details = append(details, *common.NewBulkValue(fmt.Sprintf("Admin     : %v", user.Admin)))
	details = append(details, *common.NewBulkValue(fmt.Sprintf("Full Name : %s", user.FullName)))
	return details
}
