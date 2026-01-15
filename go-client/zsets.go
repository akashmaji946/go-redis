package goredis

// ZAdd adds one or more members to a sorted set, or updates the score of existing members.
func ZAdd(key string, members ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"ZADD", key}, members...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// ZRem removes one or more members from a sorted set.
func ZRem(key string, members ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"ZREM", key}, members...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// ZScore retrieves the score of a member in a sorted set.
func ZScore(key string, member interface{}) (interface{}, error) {
	return mustGetClient().SendCommand("ZSCORE", key, member)
}

// ZCard returns the number of members in a sorted set.
func ZCard(key string) (interface{}, error) {
	return mustGetClient().SendCommand("ZCARD", key)
}

// ZRange retrieves members in a sorted set within the given range.
func ZRange(key string, start, stop int, withScores bool) (interface{}, error) {
	cmdArgs := []interface{}{"ZRANGE", key, start, stop}
	if withScores {
		cmdArgs = append(cmdArgs, "WITHSCORES")
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// ZRevRange retrieves members in a sorted set within the given range in reverse order.
func ZRevRange(key string, start, stop int, withScores bool) (interface{}, error) {
	cmdArgs := []interface{}{"ZREVRANGE", key, start, stop}
	if withScores {
		cmdArgs = append(cmdArgs, "WITHSCORES")
	}
	return mustGetClient().SendCommand(cmdArgs...)
}

// ZGet retrieves the member(s) and their scores from a sorted set.
func ZGet(key string, member ...interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"ZGET", key}
	if len(member) > 0 {
		cmdArgs = append(cmdArgs, member[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}
