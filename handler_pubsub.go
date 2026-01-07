package main

// PubSub commands
// Publish sends a message to all clients subscribed to the specified channel.
// Syntax: PUBLISH channel message
func Publish(client *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) != 2 {
		return NewErrorValue("ERR wrong number of arguments for 'publish' command")
	}
	channel := args[0].blk
	message := args[1].blk

	state.pubsubMu.Lock()
	subscribers, exists := state.channels[channel]
	if !exists {
		state.pubsubMu.Unlock()
		return NewIntegerValue(0)
	}

	// Copy the subscriber list to avoid holding the lock during network I/O
	subsCopy := make([]*Client, len(subscribers))
	copy(subsCopy, subscribers)
	state.pubsubMu.Unlock()

	// Correct RESP format for a message: ["message", channel, payload]
	// Using Bulk Strings ($) as required by the protocol
	reply := NewArrayValue([]Value{
		*NewBulkValue("message"),
		*NewBulkValue(channel),
		*NewBulkValue(message),
	})

	// Serialize once to be efficient
	w := &Writer{}
	serialized := w.Deserialize(reply)

	// Send the message to all subscribers
	for _, subClient := range subsCopy {
		subClient.conn.Write([]byte(serialized))
	}

	return NewIntegerValue(int64(len(subsCopy)))
}

// Subscribe subscribes the client to the specified channels.
// Syntax: SUBSCRIBE channel [channel ...]
func Subscribe(client *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 1 {
		return NewErrorValue("ERR wrong number of arguments for 'subscribe' command")
	}

	state.pubsubMu.Lock()
	defer state.pubsubMu.Unlock()
	var lastReply *Value
	for _, arg := range args {
		channel := arg.blk
		// Add client to the channel's subscriber list if not already subscribed
		subscribers, exists := state.channels[channel]
		if !exists {
			subscribers = []*Client{}
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
			state.channels[channel] = subscribers
		}

		// Send subscription confirmation to client
		reply := NewArrayValue([]Value{
			*NewBulkValue("subscribe"),
			*NewBulkValue(channel),
			*NewIntegerValue(int64(len(subscribers))),
		})

		// If multiple channels, write all but the last one manually
		// The last one is returned to the main handler to avoid extra NullValue
		if channel != args[len(args)-1].blk {
			w := NewWriter(client.conn)
			w.Write(reply)
			w.Flush()
		} else {
			lastReply = reply
		}
	}
	return lastReply
}

func Unsubscribe(client *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 1 {
		return NewErrorValue("ERR wrong number of arguments for 'unsubscribe' command")
	}

	state.pubsubMu.Lock()
	defer state.pubsubMu.Unlock()
	var lastReply *Value
	for _, arg := range args {
		channel := arg.blk
		subscribers, exists := state.channels[channel]
		if exists {
			// Remove client from the channel's subscriber list
			newSubscribers := []*Client{}
			for _, subClient := range subscribers {
				if subClient != client {
					newSubscribers = append(newSubscribers, subClient)
				}
			}
			if len(newSubscribers) == 0 {
				delete(state.channels, channel)
			} else {
				state.channels[channel] = newSubscribers
			}

			// Send unsubscription confirmation to client
			reply := NewArrayValue([]Value{
				*NewBulkValue("unsubscribe"),
				*NewBulkValue(channel),
				*NewIntegerValue(int64(len(newSubscribers))),
			})

			if channel != args[len(args)-1].blk {
				w := NewWriter(client.conn)
				w.Write(reply)
				w.Flush()
			} else {
				lastReply = reply
			}
		}
	}
	return lastReply
}
