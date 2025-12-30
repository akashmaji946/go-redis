package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

func main() {

	fmt.Println(">>> Go-Redis Server v0.1 <<<")

	// read the config file
	log.Println("reading the config file...")
	conf := ReadConf("./redis.conf")
	state := NewAppState(conf)

	// if aof
	if conf.aofEnabled {
		log.Println("syncing records")
		state.aof.Synchronize()
	}

	// if rdb
	if len(conf.rdb) > 0 {
		SyncRDB(conf, state)
		InitRDBTrackers(conf, state)
	}

	// setup a tcp listener at localhost:6379
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		log.Fatal("cannot listen on port 6379 due to:", err)
	}
	defer l.Close()

	// listener setup success
	log.Println("listening on port 6379")

	var connectionCount int = 0

	// listener awaiting connection(s)
	var wg sync.WaitGroup
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("cannot accept connection due to:", err)
		}
		// defer conn.Close()

		// connection(s) accepted from client (here redis-cli)
		log.Println("accepted connection from:", conn.RemoteAddr())

		wg.Add(1)
		go func() {
			handleOneConnection(conn, state, &connectionCount)
			wg.Done()
		}()

	}
	wg.Wait()

}

func handleOneConnection(conn net.Conn, state *AppState, connectionCount *int) {
	log.Printf("[%2d] [ACCEPT] Accepted connection from: %s\n", *connectionCount, conn.LocalAddr().String())
	*connectionCount += 1

	client := NewClient(conn)

	for {

		v := Value{
			typ: ARRAY,
		}

		// receive a Value and print it
		err := v.ReadArray(conn)
		if err != nil {
			log.Println("[CLOSE] Closing connection due to: ", err)
			*connectionCount -= 1
			break
		}

		// optional: print what we got
		log.Printf("%v\n", v)

		// handle the Value (abstracting the command and its args)
		handle(client, &v, state)
	}
}

type Client struct {
	conn          net.Conn
	authenticated bool
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:          conn,
		authenticated: false,
	}
}

type AppState struct {
	config   *Config
	aof      *Aof
	bgsaving bool
	DBCopy   map[string]*VAL
}

func NewAppState(config *Config) *AppState {
	state := AppState{
		config: config,
	}
	if config.aofEnabled {
		state.aof = NewAof(config)
		if config.aofFsync == Everysec {
			go func() {
				t := time.NewTicker(time.Second)
				defer t.Stop()

				for range t.C {
					state.aof.w.Flush()
				}
			}()
		}
	}
	return &state
}
