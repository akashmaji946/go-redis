package goredis

func ZAdd(key string, members ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"ZADD", key}, members...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func ZRem(key string, members ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"ZREM", key}, members...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func ZScore(key string, member interface{}) (interface{}, error) {
	return mustGetClient().SendCommand("ZSCORE", key, member)
}

func ZCard(key string) (interface{}, error) {
	return mustGetClient().SendCommand("ZCARD", key)
}

func ZRange(key string, start, stop int, withScores bool) (interface{}, error) {
	cmdArgs := []interface{}{"ZRANGE", key, start, stop}
	if withScores {
		cmdArgs = append(cmdArgs, "WITHSCORES")
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

func ZRevRange(key string, start, stop int, withScores bool) (interface{}, error) {
	cmdArgs := []interface{}{"ZREVRANGE", key, start, stop}
	if withScores {
		cmdArgs = append(cmdArgs, "WITHSCORES")
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

func ZGet(key string, member ...interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"ZGET", key}
	if len(member) > 0 {
		cmdArgs = append(cmdArgs, member[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}
