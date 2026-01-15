package goredis

// HSet sets the value of a field in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HSet(key, field string, value interface{}, extra ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"HSET", key, field, value}, extra...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// HGet retrieves the value of a field in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HGet(key, field string) (interface{}, error) {
	return mustGetClient().SendCommand("HGET", key, field)
}

// HDel deletes one or more fields from a hash stored at key.
// It returns the server's response or an error if the command fails.
func HDel(key string, fields ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"HDEL", key}, toInterfaceSlice(fields)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// HGetAll retrieves all fields and values of a hash stored at key.
// It returns the server's response or an error if the command fails.
func HGetAll(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HGETALL", key)
}

// HIncrBy increments the integer value of a field in a hash stored at key by the given increment.
// It returns the server's response or an error if the command fails.
func HIncrBy(key, field string, increment int) (interface{}, error) {
	return mustGetClient().SendCommand("HINCRBY", key, field, increment)
}

// HExists checks if a field exists in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HExists(key, field string) (interface{}, error) {
	return mustGetClient().SendCommand("HEXISTS", key, field)
}

// HLen returns the number of fields in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HLen(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HLEN", key)
}

// HKeys retrieves all field names in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HKeys(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HKEYS", key)
}

// HVals retrieves all values in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HVals(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HVALS", key)
}

// HMSet sets multiple fields in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HMSet(key string, mapping map[string]interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"HMSET", key}
	for k, v := range mapping {
		cmdArgs = append(cmdArgs, k, v)
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// HMGet retrieves the values of multiple fields in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HMGet(key string, fields ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"HMGET", key}, toInterfaceSlice(fields)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// HDelAll deletes all fields in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HDelAll(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HDELALL", key)
}

// HExpire sets a timeout on a field in a hash stored at key.
// It returns the server's response or an error if the command fails.
func HExpire(key, field string, seconds int) (interface{}, error) {
	return mustGetClient().SendCommand("HEXPIRE", key, field, seconds)
}
