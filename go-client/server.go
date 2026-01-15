package goredis

import (
	"bufio"
	"fmt"
	"net"
)

// Connect establishes a connection to the Redis server
// at the specified host and port.
// It returns a success message or an error if the connection fails.
func Connect(host string, port int) (string, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", err
	}

	globalClient = &GoRedisClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}
	return "Connected", nil
}

// Close terminates the connection to the Redis server.
// It returns an error if the connection is already closed.
func Close() error {
	if globalClient != nil {
		err := globalClient.Close()
		globalClient = nil
		return err
	}
	return nil
}

// Auth authenticates the client with the Redis server using the provided username and password.
// It returns the server's response or an error if authentication fails.
func Auth(username string, password ...string) (interface{}, error) {
	if len(password) > 0 {
		return mustGetClient().SendCommand("AUTH", username, password[0])
	}
	return mustGetClient().SendCommand("AUTH", username)
}

// Ping sends a PING command to the Redis server.
// If a message is provided, it is included in the PING command.
// It returns the server's response or an error if the command fails.
func Ping(message ...string) (interface{}, error) {
	cmdArgs := []interface{}{"PING"}
	if len(message) > 0 {
		cmdArgs = append(cmdArgs, message[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// Select changes the selected database for the current connection to the specified index.
// It returns the server's response or an error if the command fails.
func Select(index int) (interface{}, error) {
	return mustGetClient().SendCommand("SELECT", index)
}

// Info retrieves information and statistics about the Redis server.
// If a key is provided, it retrieves information specific to that key.
// It returns the server's response or an error if the command fails.
func Info(key ...string) (interface{}, error) {
	cmdArgs := []interface{}{"INFO"}
	if len(key) > 0 {
		cmdArgs = append(cmdArgs, key[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// Monitor puts the Redis server into monitoring mode, where it streams back every command processed by the server.
// It returns the server's response or an error if the command fails.
func Monitor() (interface{}, error) {
	return mustGetClient().SendCommand("MONITOR")
}

// DbSize returns the number of keys in the currently selected database.
// It returns the server's response or an error if the command fails.
func DbSize() (interface{}, error) {
	return mustGetClient().SendCommand("DBSIZE")
}

// FlushDb removes all keys from the currently selected database.
// It returns the server's response or an error if the command fails.
func FlushDb() (interface{}, error) {
	return mustGetClient().SendCommand("FLUSHDB")
}

// FlushAll removes all keys from all databases.
// It returns the server's response or an error if the command fails.
func FlushAll() (interface{}, error) {
	return mustGetClient().SendCommand("FLUSHALL")
}

// Size returns the number of keys in the specified database index.
// If no index is provided, it defaults to the currently selected database.
// It returns the server's response or an error if the command fails.
func Size(dbIndex ...int) (interface{}, error) {
	cmdArgs := []interface{}{"SIZE"}
	if len(dbIndex) > 0 {
		cmdArgs = append(cmdArgs, dbIndex[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// UserAdd adds a new user with the specified username and password.
// The adminFlag indicates whether the user has administrative privileges.
// It returns the server's response or an error if the command fails.
func UserAdd(adminFlag int, user, password string) (interface{}, error) {
	return mustGetClient().SendCommand("USERADD", adminFlag, user, password)
}

// UserDel deletes the specified user.
// It is a administrative command.
// It returns the server's response or an error if the command fails.
func UserDel(user string) (interface{}, error) {
	return mustGetClient().SendCommand("USERDEL", user)
}

// Passwd changes the password for the specified user.
// It returns the server's response or an error if the command fails.
func Passwd(user, password string) (interface{}, error) {
	return mustGetClient().SendCommand("PASSWD", user, password)
}

// Users retrieves information about users.
// It is an administrative command.
// If a username is provided, it retrieves information specific to that user.
// It returns the server's response or an error if the command fails.
func Users(username ...string) (interface{}, error) {
	cmdArgs := []interface{}{"USERS"}
	if len(username) > 0 {
		cmdArgs = append(cmdArgs, username[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// WhoAmI returns information about the current user.
// It returns the server's response or an error if the command fails.
func WhoAmI() (interface{}, error) {
	return mustGetClient().SendCommand("WHOAMI")
}

// Save performs a synchronous save of the dataset to disk.
// It returns the server's response or an error if the command fails.
func Save() (interface{}, error) {
	return mustGetClient().SendCommand("SAVE")
}

// BgSave performs an asynchronous save of the dataset to disk.
// It returns the server's response or an error if the command fails.
func BgSave() (interface{}, error) {
	return mustGetClient().SendCommand("BGSAVE")
}

// BgRewriteAof performs an asynchronous rewrite of the append-only file.
// It returns the server's response or an error if the command fails.
func BgRewriteAof() (interface{}, error) {
	return mustGetClient().SendCommand("BGREWRITEAOF")
}

// Command retrieves details about all Redis commands.
// It returns the server's response or an error if the command fails.
func Command() (interface{}, error) {
	return mustGetClient().SendCommand("COMMAND")
}
