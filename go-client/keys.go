package goredis

func Del(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"DEL"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func Exists(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"EXISTS"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func Keys(pattern string) (interface{}, error) {
	return mustGetClient().SendCommand("KEYS", pattern)
}

func Rename(key, newkey string) (interface{}, error) {
	return mustGetClient().SendCommand("RENAME", key, newkey)
}

func Type(key string) (interface{}, error) {
	return mustGetClient().SendCommand("TYPE", key)
}

func Expire(key string, seconds int) (interface{}, error) {
	return mustGetClient().SendCommand("EXPIRE", key, seconds)
}

func Ttl(key string) (interface{}, error) {
	return mustGetClient().SendCommand("TTL", key)
}

func Persist(key string) (interface{}, error) {
	return mustGetClient().SendCommand("PERSIST", key)
}
