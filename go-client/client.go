package goredis

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// GoRedisClient represents a client connection to the Redis server.
type GoRedisClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

// SendCommand sends a command to the Redis server and returns the response.
func (c *GoRedisClient) SendCommand(args ...interface{}) (interface{}, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		sArg := fmt.Sprintf("%v", arg)
		sb.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(sArg), sArg))
	}

	_, err := c.conn.Write([]byte(sb.String()))
	if err != nil {
		return nil, err
	}

	return c.readResponse()
}

// readResponse reads and parses the response from the Redis server.
func (c *GoRedisClient) readResponse() (interface{}, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	line = strings.TrimSuffix(line, "\r\n")
	if len(line) == 0 {
		return nil, errors.New("empty response")
	}

	prefix := line[0]
	payload := line[1:]

	switch prefix {
	case '+':
		return payload, nil
	case '-':
		return nil, errors.New("Server Error: " + payload)
	case ':':
		return strconv.ParseInt(payload, 10, 64)
	case '$':
		length, _ := strconv.Atoi(payload)
		if length == -1 {
			return nil, nil
		}
		data := make([]byte, length)
		if _, err := c.reader.Read(data); err != nil {
			return nil, err
		}
		c.reader.ReadString('\n') // Consume \r\n
		return string(data), nil
	case '*':
		count, _ := strconv.Atoi(payload)
		if count == -1 {
			return nil, nil
		}
		results := make([]interface{}, count)
		for i := 0; i < count; i++ {
			res, err := c.readResponse()
			if err != nil {
				return nil, err
			}
			results[i] = res
		}
		return results, nil
	default:
		return nil, fmt.Errorf("unknown RESP type: %c", prefix)
	}
}

// Close closes the connection to the Redis server.
func (c *GoRedisClient) Close() error {
	return c.conn.Close()
}

// globalClient holds the singleton client instance.
var globalClient *GoRedisClient

// getClient returns the global client instance.
func getClient() (*GoRedisClient, error) {
	if globalClient == nil {
		return nil, errors.New("client not connected. Call Connect() first")
	}
	return globalClient, nil
}

// mustGetClient returns the global client instance or panics if not connected.
func mustGetClient() *GoRedisClient {
	c, err := getClient()
	if err != nil {
		panic(err)
	}
	return c
}

// toInterfaceSlice converts a slice of strings to a slice of interfaces.
func toInterfaceSlice(args []string) []interface{} {
	s := make([]interface{}, len(args))
	for i, v := range args {
		s[i] = v
	}
	return s
}
