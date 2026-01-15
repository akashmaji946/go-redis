package goredis

// Multi starts a transaction block.
func Multi() (interface{}, error) {
	return mustGetClient().SendCommand("MULTI")
}

// Exec executes all the commands in the transaction block.
func Exec() (interface{}, error) {
	return mustGetClient().SendCommand("EXEC")
}

// Discard flushes all previously queued commands in a transaction block.
func Discard() (interface{}, error) {
	return mustGetClient().SendCommand("DISCARD")
}

// Watch watches the given keys for modifications.
func Watch(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"WATCH"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// Unwatch flushes all the watched keys.
func Unwatch() (interface{}, error) {
	return mustGetClient().SendCommand("UNWATCH")
}
