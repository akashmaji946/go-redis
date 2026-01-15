package goredis

// SAdd adds one or more members to a set stored at key.
// It returns the server's response or an error if the command fails.
func SAdd(key string, members ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"SADD", key}, members...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// SRem removes one or more members from a set stored at key.
// It returns the server's response or an error if the command fails.
func SRem(key string, members ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"SREM", key}, members...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// SMembers retrieves all members of the set stored at key.
// It returns the server's response or an error if the command fails.
func SMembers(key string) (interface{}, error) {
	return mustGetClient().SendCommand("SMEMBERS", key)
}

// SIsMember checks if a member is part of the set stored at key.
// It returns the server's response or an error if the command fails.
func SIsMember(key string, member interface{}) (interface{}, error) {
	return mustGetClient().SendCommand("SISMEMBER", key, member)
}

// SCard returns the number of members in the set stored at key.
// It returns the server's response or an error if the command fails.
func SCard(key string) (interface{}, error) {
	return mustGetClient().SendCommand("SCARD", key)
}

// SDiff returns the members of the set resulting from the difference between the first set and all the successive sets.
// It returns the server's response or an error if the command fails.
func SDiff(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SDIFF"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// SInter returns the members of the set resulting from the intersection of all the given sets.
// It returns the server's response or an error if the command fails.
func SInter(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SINTER"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// SUnion returns the members of the set resulting from the union of all the given sets.
// It returns the server's response or an error if the command fails.
func SUnion(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SUNION"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// SRandMember returns one or more random members from the set stored at key.
// If count is provided, it returns that many random members.
// It returns the server's response or an error if the command fails.
func SRandMember(key string, count ...int) (interface{}, error) {
	cmdArgs := []interface{}{"SRANDMEMBER", key}
	if len(count) > 0 {
		cmdArgs = append(cmdArgs, count[0])
	}
	return mustGetClient().SendCommand(cmdArgs...)
}
