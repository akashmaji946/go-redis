package goredis

func SAdd(key string, members ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"SADD", key}, members...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func SRem(key string, members ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"SREM", key}, members...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func SMembers(key string) (interface{}, error) {
	return mustGetClient().SendCommand("SMEMBERS", key)
}

func SIsMember(key string, member interface{}) (interface{}, error) {
	return mustGetClient().SendCommand("SISMEMBER", key, member)
}

func SCard(key string) (interface{}, error) {
	return mustGetClient().SendCommand("SCARD", key)
}

func SDiff(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SDIFF"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func SInter(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SINTER"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func SUnion(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SUNION"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func SRandMember(key string, count ...int) (interface{}, error) {
	cmdArgs := []interface{}{"SRANDMEMBER", key}
	if len(count) > 0 {
		cmdArgs = append(cmdArgs, count[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}
