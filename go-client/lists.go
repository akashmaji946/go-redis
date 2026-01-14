package goredis

func LPush(key string, values ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"LPUSH", key}, values...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func RPush(key string, values ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"RPUSH", key}, values...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func LPop(key string) (interface{}, error) {
	return mustGetClient().SendCommand("LPOP", key)
}

func RPop(key string) (interface{}, error) {
	return mustGetClient().SendCommand("RPOP", key)
}

func LRange(key string, start, stop int) (interface{}, error) {
	return mustGetClient().SendCommand("LRANGE", key, start, stop)
}

func LLen(key string) (interface{}, error) {
	return mustGetClient().SendCommand("LLEN", key)
}

func LIndex(key string, index int) (interface{}, error) {
	return mustGetClient().SendCommand("LINDEX", key, index)
}

func LGet(key string) (interface{}, error) {
	return mustGetClient().SendCommand("LGET", key)
}
