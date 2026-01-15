package goredis

// Get retrieves the value of a key.
func Get(key string) (interface{}, error) {
	return mustGetClient().SendCommand("GET", key)
}

// Set sets the value of a key.
func Set(key string, value interface{}) (interface{}, error) {
	return mustGetClient().SendCommand("SET", key, value)
}

// Incr increments the integer value of a key by one.
func Incr(key string) (interface{}, error) {
	return mustGetClient().SendCommand("INCR", key)
}

// Decr decrements the integer value of a key by one.
func Decr(key string) (interface{}, error) {
	return mustGetClient().SendCommand("DECR", key)
}

// IncrBy increments the integer value of a key by the given increment.
func IncrBy(key string, increment int) (interface{}, error) {
	return mustGetClient().SendCommand("INCRBY", key, increment)
}

// DecrBy decrements the integer value of a key by the given decrement.
func DecrBy(key string, decrement int) (interface{}, error) {
	return mustGetClient().SendCommand("DECRBY", key, decrement)
}

// MGet retrieves the values of multiple keys.
func MGet(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"MGET"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// MSet sets multiple keys to their respective values.
func MSet(mapping map[string]interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"MSET"}
	for k, v := range mapping {
		cmdArgs = append(cmdArgs, k, v)
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// StrLen returns the length of the string value stored at key.
func StrLen(key string) (interface{}, error) {
	return mustGetClient().SendCommand("STRLEN", key)
}
