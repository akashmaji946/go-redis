package goredis

func Multi() (interface{}, error) {
	return mustGetClient().SendCommand("MULTI")
}

func Exec() (interface{}, error) {
	return mustGetClient().SendCommand("EXEC")
}

func Discard() (interface{}, error) {
	return mustGetClient().SendCommand("DISCARD")
}

func Watch(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"WATCH"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func Unwatch() (interface{}, error) {
	return mustGetClient().SendCommand("UNWATCH")
}
