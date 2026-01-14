package goredis

func Publish(channel, message string) (interface{}, error) {
	return mustGetClient().SendCommand("PUBLISH", channel, message)
}

func Subscribe(channels ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"SUBSCRIBE"}, toInterfaceSlice(channels)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func Unsubscribe(channels ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"UNSUBSCRIBE"}, toInterfaceSlice(channels)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func PSubscribe(patterns ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"PSUBSCRIBE"}, toInterfaceSlice(patterns)...)
	return mustGetClient().SendCommand(cmdArgs...)
}

func PUnsubscribe(patterns ...string) (interface{}, error) {
	cmdArgs := append([]interface{}{"PUNSUBSCRIBE"}, toInterfaceSlice(patterns)...)
	return mustGetClient().SendCommand(cmdArgs...)
}
