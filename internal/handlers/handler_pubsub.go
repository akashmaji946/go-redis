package handlers

import (
	"path"

	"github.com/akashmaji946/go-redis/internal/common"
)

// PubSub commands
// Publish sends a message to all clients subscribed to the specified channel.
// Syntax: PUBLISH channel message
func Publish(client *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) != 2 {
		return common.NewErrorValue("ERR wrong number of arguments for 'publish' command")
	}
	channel := args[0].Blk
	message := args[1].Blk

	state.PubsubMu.Lock()

	// 1. Handle exact channel matches
	var totalSent int64
	if subscribers, exists := state.Channels[channel]; exists {
		reply := common.NewArrayValue([]common.Value{
			*common.NewBulkValue("message"),
			*common.NewBulkValue(channel),
			*common.NewBulkValue(message),
		})
		serialized := (&common.Writer{}).Deserialize(reply)
		for _, subClient := range subscribers {
			subClient.Conn.Write([]byte(serialized))
			totalSent++
		}
	}

	// 2. Handle pattern (topic) matches
	for pattern, subscribers := range state.Topics {
		matched, _ := path.Match(pattern, channel)
		if matched {
			reply := common.NewArrayValue([]common.Value{
				*common.NewBulkValue("pmessage"),
				*common.NewBulkValue(pattern),
				*common.NewBulkValue(channel),
				*common.NewBulkValue(message),
			})
			serialized := (&common.Writer{}).Deserialize(reply)
			for _, subClient := range subscribers {
				subClient.Conn.Write([]byte(serialized))
				totalSent++
			}
		}
	}
	state.PubsubMu.Unlock()

	return common.NewIntegerValue(totalSent)
}

// Subscribe subscribes the client to the specified channels.
// Syntax: SUBSCRIBE channel [channel ...]
func Subscribe(client *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'subscribe' command")
	}

	state.PubsubMu.Lock()
	defer state.PubsubMu.Unlock()
	var lastReply *common.Value
	for _, arg := range args {
		channel := arg.Blk
		// Add client to the channel's subscriber list if not already subscribed
		subscribers, exists := state.Channels[channel]
		if !exists {
			subscribers = []*common.Client{}
		}
		alreadySubscribed := false
		for _, subClient := range subscribers {
			if subClient == client {
				alreadySubscribed = true
				break
			}
		}
		if !alreadySubscribed {
			subscribers = append(subscribers, client)
			state.Channels[channel] = subscribers
		}

		// Send subscription confirmation to client
		reply := common.NewArrayValue([]common.Value{
			*common.NewBulkValue("subscribe"),
			*common.NewBulkValue(channel),
			*common.NewIntegerValue(int64(len(subscribers))),
		})

		// If multiple channels, write all but the last one manually
		// The last one is returned to the main handler to avoid extra NullValue
		if channel != args[len(args)-1].Blk {
			w := common.NewWriter(client.Conn)
			w.Write(reply)
			w.Flush()
		} else {
			lastReply = reply
		}
	}
	return lastReply
}

// Psubscribe subscribes the client to the specified patterns (topics).
// Syntax: PSUBSCRIBE pattern [pattern ...]
func Psubscribe(client *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'psubscribe' command")
	}

	state.PubsubMu.Lock()
	defer state.PubsubMu.Unlock()
	var lastReply *common.Value
	for _, arg := range args {
		pattern := arg.Blk
		subscribers := state.Topics[pattern]

		alreadySubscribed := false
		for _, subClient := range subscribers {
			if subClient == client {
				alreadySubscribed = true
				break
			}
		}
		if !alreadySubscribed {
			subscribers = append(subscribers, client)
			state.Topics[pattern] = subscribers
		}

		reply := common.NewArrayValue([]common.Value{
			*common.NewBulkValue("psubscribe"),
			*common.NewBulkValue(pattern),
			*common.NewIntegerValue(int64(len(subscribers))),
		})

		if pattern != args[len(args)-1].Blk {
			w := common.NewWriter(client.Conn)
			w.Write(reply)
			w.Flush()
		} else {
			lastReply = reply
		}
	}
	return lastReply
}

// Punsubscribe unsubscribes the client from the specified patterns.
func Punsubscribe(client *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'punsubscribe' command")
	}

	state.PubsubMu.Lock()
	defer state.PubsubMu.Unlock()
	var lastReply *common.Value
	for _, arg := range args {
		pattern := arg.Blk
		subscribers, exists := state.Topics[pattern]
		if exists {
			newSubscribers := []*common.Client{}
			for _, subClient := range subscribers {
				if subClient != client {
					newSubscribers = append(newSubscribers, subClient)
				}
			}
			if len(newSubscribers) == 0 {
				delete(state.Topics, pattern)
			} else {
				state.Topics[pattern] = newSubscribers
			}

			reply := common.NewArrayValue([]common.Value{
				*common.NewBulkValue("punsubscribe"),
				*common.NewBulkValue(pattern),
				*common.NewIntegerValue(int64(len(newSubscribers))),
			})

			if pattern != args[len(args)-1].Blk {
				w := common.NewWriter(client.Conn)
				w.Write(reply)
				w.Flush()
			} else {
				lastReply = reply
			}
		}
	}
	return lastReply
}

func Unsubscribe(client *common.Client, v *common.Value, state *common.AppState) *common.Value {
	args := v.Arr[1:]
	if len(args) < 1 {
		return common.NewErrorValue("ERR wrong number of arguments for 'unsubscribe' command")
	}

	state.PubsubMu.Lock()
	defer state.PubsubMu.Unlock()
	var lastReply *common.Value
	for _, arg := range args {
		channel := arg.Blk
		subscribers, exists := state.Channels[channel]
		if exists {
			// Remove client from the channel's subscriber list
			newSubscribers := []*common.Client{}
			for _, subClient := range subscribers {
				if subClient != client {
					newSubscribers = append(newSubscribers, subClient)
				}
			}
			if len(newSubscribers) == 0 {
				delete(state.Channels, channel)
			} else {
				state.Channels[channel] = newSubscribers
			}

			// Send unsubscription confirmation to client
			reply := common.NewArrayValue([]common.Value{
				*common.NewBulkValue("unsubscribe"),
				*common.NewBulkValue(channel),
				*common.NewIntegerValue(int64(len(newSubscribers))),
			})

			if channel != args[len(args)-1].Blk {
				w := common.NewWriter(client.Conn)
				w.Write(reply)
				w.Flush()
			} else {
				lastReply = reply
			}
		}
	}
	return lastReply
}
