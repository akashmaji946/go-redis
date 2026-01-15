package goredis

// Publish posts a message to the given channel.
func Publish(channel, message string) (interface{}, error) {
	return mustGetClient().SendCommand("PUBLISH", channel, message)
}

// Pub is an alias for Publish.
func Pub(channel, message string) (interface{}, error) {
	return Publish(channel, message)
}

// Subscribe subscribes the client to the specified channels.
func Subscribe(channels ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SUBSCRIBE"}, toInterfaceSlice(channels)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// Sub is an alias for Subscribe.
func Sub(channels ...string) (interface{}, error) {
	return Subscribe(channels...)
}

// Unsubscribe unsubscribes the client from the specified channels.
func Unsubscribe(channels ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"UNSUBSCRIBE"}, toInterfaceSlice(channels)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// Unsub is an alias for Unsubscribe.
func Unsub(channels ...string) (interface{}, error) {
	return Unsubscribe(channels...)
}

// PSubscribe subscribes the client to the given patterns.
func PSubscribe(patterns ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"PSUBSCRIBE"}, toInterfaceSlice(patterns)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// PSub is an alias for PSubscribe.
func PSub(patterns ...string) (interface{}, error) {
	return PSubscribe(patterns...)
}

// PUnsubscribe unsubscribes the client from the given patterns.
func PUnsubscribe(patterns ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"PUNSUBSCRIBE"}, toInterfaceSlice(patterns)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

// PUnsub is an alias for PUnsubscribe.
func PUnsub(patterns ...string) (interface{}, error) {
	return PUnsubscribe(patterns...)
}
