package goredis

// Publish posts a message to the given channel.
func Publish(channel, message string) (interface{}, error) {
	return mustGetClient().SendCommand("PUBLISH", channel, message)
}

// Subscribe subscribes the client to the specified channels.
func Subscribe(channels ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SUBSCRIBE"}, toInterfaceSlice(channels)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// Unsubscribe unsubscribes the client from the specified channels.
func Unsubscribe(channels ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"UNSUBSCRIBE"}, toInterfaceSlice(channels)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// PSubscribe subscribes the client to the given patterns.
func PSubscribe(patterns ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"PSUBSCRIBE"}, toInterfaceSlice(patterns)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// PUnsubscribe unsubscribes the client from the given patterns.
func PUnsubscribe(patterns ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"PUNSUBSCRIBE"}, toInterfaceSlice(patterns)...)
	return mustGetClient().SendCommand(cmdArgs...)
}
