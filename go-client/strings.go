package goredis

func Get(key string) (interface{}, error) {
	return mustGetClient().SendCommand("GET", key)
}

func Set(key string, value interface{}) (interface{}, error) {
	return mustGetClient().SendCommand("SET", key, value)
}

func Incr(key string) (interface{}, error) {
	return mustGetClient().SendCommand("INCR", key)
}

func Decr(key string) (interface{}, error) {
	return mustGetClient().SendCommand("DECR", key)
}

func IncrBy(key string, increment int) (interface{}, error) {
	return mustGetClient().SendCommand("INCRBY", key, increment)
}

func DecrBy(key string, decrement int) (interface{}, error) {
	return mustGetClient().SendCommand("DECRBY", key, decrement)
}

func MGet(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"MGET"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func MSet(mapping map[string]interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"MSET"}
	for k, v := range mapping {
		cmdArgs = append(cmdArgs, k, v)
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

func StrLen(key string) (interface{}, error) {
	return mustGetClient().SendCommand("STRLEN", key)
}
