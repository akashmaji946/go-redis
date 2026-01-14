package goredis

import (
	"bufio"
	"fmt"
	"net"
)

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

func Close() error {
	if globalClient != nil {
		err := globalClient.Close()
		globalClient = nil
		return err
	}
	return nil
}

func Auth(username string, password ...string) (interface{}, error) {
	if len(password) > 0 {
		return mustGetClient().SendCommand("AUTH", username, password[0])
	}
	return mustGetClient().SendCommand("AUTH", username)
}

func Ping(message ...string) (interface{}, error) {
	cmdArgs := []interface{}{"PING"}
	if len(message) > 0 {
		cmdArgs = append(cmdArgs, message[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

func Select(index int) (interface{}, error) {
	return mustGetClient().SendCommand("SELECT", index)
}

func Info(key ...string) (interface{}, error) {
	cmdArgs := []interface{}{"INFO"}
	if len(key) > 0 {
		cmdArgs = append(cmdArgs, key[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

func Monitor() (interface{}, error) {
	return mustGetClient().SendCommand("MONITOR")
}

func DbSize() (interface{}, error) {
	return mustGetClient().SendCommand("DBSIZE")
}

func FlushDb() (interface{}, error) {
	return mustGetClient().SendCommand("FLUSHDB")
}

func Size(dbIndex ...int) (interface{}, error) {
	cmdArgs := []interface{}{"SIZE"}
	if len(dbIndex) > 0 {
		cmdArgs = append(cmdArgs, dbIndex[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

func UserAdd(adminFlag int, user, password string) (interface{}, error) {
	return mustGetClient().SendCommand("USERADD", adminFlag, user, password)
}

func Passwd(user, password string) (interface{}, error) {
	return mustGetClient().SendCommand("PASSWD", user, password)
}

func Users(username ...string) (interface{}, error) {
	cmdArgs := []interface{}{"USERS"}
	if len(username) > 0 {
		cmdArgs = append(cmdArgs, username[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

func WhoAmI() (interface{}, error) {
	return mustGetClient().SendCommand("WHOAMI")
}

func Save() (interface{}, error) {
	return mustGetClient().SendCommand("SAVE")
}

func BgSave() (interface{}, error) {
	return mustGetClient().SendCommand("BGSAVE")
}

func BgRewriteAof() (interface{}, error) {
	return mustGetClient().SendCommand("BGREWRITEAOF")
}

func Command() (interface{}, error) {
	return mustGetClient().SendCommand("COMMAND")
}
