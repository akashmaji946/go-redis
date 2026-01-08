package main

import (
	"path"
)

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

	// 1. Handle exact channel matches
	var totalSent int64
	if subscribers, exists := state.channels[channel]; exists {
		reply := NewArrayValue([]Value{
			*NewBulkValue("message"),
			*NewBulkValue(channel),
			*NewBulkValue(message),
		})
		serialized := (&Writer{}).Deserialize(reply)
		for _, subClient := range subscribers {
			subClient.conn.Write([]byte(serialized))
			totalSent++
		}
	}

	// 2. Handle pattern (topic) matches
	for pattern, subscribers := range state.topics {
		matched, _ := path.Match(pattern, channel)
		if matched {
			reply := NewArrayValue([]Value{
				*NewBulkValue("pmessage"),
				*NewBulkValue(pattern),
				*NewBulkValue(channel),
				*NewBulkValue(message),
			})
			serialized := (&Writer{}).Deserialize(reply)
			for _, subClient := range subscribers {
				subClient.conn.Write([]byte(serialized))
				totalSent++
			}
		}
	}
	state.pubsubMu.Unlock()

	return NewIntegerValue(totalSent)
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

// Psubscribe subscribes the client to the specified patterns (topics).
// Syntax: PSUBSCRIBE pattern [pattern ...]
func Psubscribe(client *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 1 {
		return NewErrorValue("ERR wrong number of arguments for 'psubscribe' command")
	}

	state.pubsubMu.Lock()
	defer state.pubsubMu.Unlock()
	var lastReply *Value
	for _, arg := range args {
		pattern := arg.blk
		subscribers := state.topics[pattern]

		alreadySubscribed := false
		for _, subClient := range subscribers {
			if subClient == client {
				alreadySubscribed = true
				break
			}
		}
		if !alreadySubscribed {
			subscribers = append(subscribers, client)
			state.topics[pattern] = subscribers
		}

		reply := NewArrayValue([]Value{
			*NewBulkValue("psubscribe"),
			*NewBulkValue(pattern),
			*NewIntegerValue(int64(len(subscribers))),
		})

		if pattern != args[len(args)-1].blk {
			w := NewWriter(client.conn)
			w.Write(reply)
			w.Flush()
		} else {
			lastReply = reply
		}
	}
	return lastReply
}

// Punsubscribe unsubscribes the client from the specified patterns.
func Punsubscribe(client *Client, v *Value, state *AppState) *Value {
	args := v.arr[1:]
	if len(args) < 1 {
		return NewErrorValue("ERR wrong number of arguments for 'punsubscribe' command")
	}

	state.pubsubMu.Lock()
	defer state.pubsubMu.Unlock()
	var lastReply *Value
	for _, arg := range args {
		pattern := arg.blk
		subscribers, exists := state.topics[pattern]
		if exists {
			newSubscribers := []*Client{}
			for _, subClient := range subscribers {
				if subClient != client {
					newSubscribers = append(newSubscribers, subClient)
				}
			}
			if len(newSubscribers) == 0 {
				delete(state.topics, pattern)
			} else {
				state.topics[pattern] = newSubscribers
			}

			reply := NewArrayValue([]Value{
				*NewBulkValue("punsubscribe"),
				*NewBulkValue(pattern),
				*NewIntegerValue(int64(len(newSubscribers))),
			})

			if pattern != args[len(args)-1].blk {
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
