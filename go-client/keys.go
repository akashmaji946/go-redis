package goredis

// Del deletes one or more keys.
// It returns the server's response or an error if the command fails.
func Del(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"DEL"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// Delete is an alias for Del.
func Delete(keys ...string) (interface{}, error) {
	return Del(keys...)
}

// Exists checks if one or more keys exist.
// It returns the server's response or an error if the command fails.
func Exists(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"EXISTS"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// Keys retrieves all keys matching the given pattern.
// It returns the server's response or an error if the command fails.
func Keys(pattern string) (interface{}, error) {
	return mustGetClient().SendCommand("KEYS", pattern)
}

// Rename renames a key to newkey.
// It returns the server's response or an error if the command fails.
func Rename(key, newkey string) (interface{}, error) {
	return mustGetClient().SendCommand("RENAME", key, newkey)
}

// Type returns the data type of the value stored at key.
// It returns the server's response or an error if the command fails.
func Type(key string) (interface{}, error) {
	return mustGetClient().SendCommand("TYPE", key)
}

// Expire sets a timeout on a key.
// It returns the server's response or an error if the command fails.
func Expire(key string, seconds int) (interface{}, error) {
	return mustGetClient().SendCommand("EXPIRE", key, seconds)
}

// Ttl returns the remaining time to live of a key that has a timeout.
// It returns the server's response or an error if the command fails.
func Ttl(key string) (interface{}, error) {
	return mustGetClient().SendCommand("TTL", key)
}

// Persist removes the timeout on a key.
// It returns the server's response or an error if the command fails.
func Persist(key string) (interface{}, error) {
	return mustGetClient().SendCommand("PERSIST", key)
}
