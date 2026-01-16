package goredis

// SetBit sets or clears the bit at the specified offset in the string value stored at key.
// It returns the server's response or an error if the command fails.
func SetBit(key string, offset int, value int) (interface{}, error) {
	return mustGetClient().SendCommand("SETBIT", key, offset, value)
}

// GetBit returns the bit value at the specified offset in the string value stored at key.
// It returns the server's response or an error if the command fails.
func GetBit(key string, offset int) (interface{}, error) {
	return mustGetClient().SendCommand("GETBIT", key, offset)
}

// BitCount counts the number of set bits (population counting) in a string.
// It returns the server's response or an error if the command fails.
func BitCount(key string, startEnd ...interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"BITCOUNT", key}
	cmdArgs = append(cmdArgs, startEnd...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// BitOp performs a bitwise operation between multiple source strings and stores the result in the destination key.
// It returns the server's response or an error if the command fails.
func BitOp(operation string, destKey string, keys ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"BITOP", operation, destKey}, toInterfaceSlice(keys)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// BitPos returns the position of the first bit set to 0 or 1 in a string.
// It returns the server's response or an error if the command fails.
func BitPos(key string, bit int, startEnd ...interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"BITPOS", key, bit}
	cmdArgs = append(cmdArgs, startEnd...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// BitField performs arbitrary bitfield integer operations on strings.
// It returns the server's response or an error if the command fails.
func BitField(key string, operations ...interface{}) (interface{}, error) {
	cmdArgs := []interface{}{"BITFIELD", key}
	cmdArgs = append(cmdArgs, operations...)
	return mustGetClient().SendCommand(cmdArgs...)
}
