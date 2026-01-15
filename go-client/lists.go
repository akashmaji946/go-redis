package goredis

// LPush inserts all the specified values at the head of the list stored at key.
// It returns the server's response or an error if the command fails.
func LPush(key string, values ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"LPUSH", key}, values...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// RPush inserts all the specified values at the tail of the list stored at key.
// It returns the server's response or an error if the command fails.
func RPush(key string, values ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"RPUSH", key}, values...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// LPop removes and returns the first element of the list stored at key.
// It returns the server's response or an error if the command fails.
func LPop(key string) (interface{}, error) {
	return mustGetClient().SendCommand("LPOP", key)
}

// RPop removes and returns the last element of the list stored at key.
// It returns the server's response or an error if the command fails.
func RPop(key string) (interface{}, error) {
	return mustGetClient().SendCommand("RPOP", key)
}

// LRange returns the specified elements of the list stored at key.
// It returns the server's response or an error if the command fails.
func LRange(key string, start, stop int) (interface{}, error) {
	return mustGetClient().SendCommand("LRANGE", key, start, stop)
}

// LLen returns the length of the list stored at key.
// It returns the server's response or an error if the command fails.
func LLen(key string) (interface{}, error) {
	return mustGetClient().SendCommand("LLEN", key)
}

// LIndex returns the element at index index in the list stored at key.
// It returns the server's response or an error if the command fails.
func LIndex(key string, index int) (interface{}, error) {
	return mustGetClient().SendCommand("LINDEX", key, index)
}

// LGet retrieves all elements of the list stored at key.
// It returns the server's response or an error if the command fails.
func LGet(key string) (interface{}, error) {
	return mustGetClient().SendCommand("LGET", key)
}
