package goredis

// PfAdd adds one or more elements to a HyperLogLog stored at key.
// It returns the server's response or an error if the command fails.
func PfAdd(key string, elements ...interface{}) (interface{}, error) {
	cmdArgs := append([]interface{}{"PFADD", key}, elements...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// PfCount returns the approximated cardinality of the HyperLogLog(s) at key(s).
// It returns the server's response or an error if the command fails.
func PfCount(keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"PFCOUNT"}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// PfDebug returns internal debugging information about a HyperLogLog stored at key.
// It returns the server's response or an error if the command fails.
func PfDebug(key string) (interface{}, error) {
	return mustGetClient().SendCommand("PFDEBUG", key)
}

// PfMerge merges multiple HyperLogLog values into a single destination HyperLogLog.
// It returns the server's response or an error if the command fails.
func PfMerge(destKey string, sourceKeys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"PFMERGE", destKey}, toInterfaceSlice(sourceKeys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}
