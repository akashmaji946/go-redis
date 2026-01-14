package goredis

func HSet(key, field string, value interface{}, extra ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"HSET", key, field, value}, extra...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func HGet(key, field string) (interface{}, error) {
	return mustGetClient().SendCommand("HGET", key, field)
}

func HDel(key string, fields ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"HDEL", key}, toInterfaceSlice(fields)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func HGetAll(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HGETALL", key)
}

func HIncrBy(key, field string, increment int) (interface{}, error) {
	return mustGetClient().SendCommand("HINCRBY", key, field, increment)
}

func HExists(key, field string) (interface{}, error) {
	return mustGetClient().SendCommand("HEXISTS", key, field)
}

func HLen(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HLEN", key)
}

func HKeys(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HKEYS", key)
}

func HVals(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HVALS", key)
}

func HMSet(key string, mapping map[string]interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"HMSET", key}
	for k, v := range mapping {
		cmdArgs = append(cmdArgs, k, v)
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

func HDelAll(key string) (interface{}, error) {
	return mustGetClient().SendCommand("HDELALL", key)
}

func HExpire(key, field string, seconds int) (interface{}, error) {
	return mustGetClient().SendCommand("HEXPIRE", key, field, seconds)
}
