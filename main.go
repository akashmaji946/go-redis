package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {

	fmt.Println(">>> Go-Redis Server v0.1 <<<")

	// read the config file
	fmt.Println("Reading the config file...")
	conf := ReadConf("./redis.conf")
	state := NewAppState(conf)

	// if aof
	if conf.aofEnabled {
		log.Println("syncing records")
		state.aof.Synchronize()
	}

	// if rdb
	if len(conf.rdb) > 0 {
		SyncRDB(conf)
		InitRDBTrackers(conf)
	}

	// setup a tcp listener at localhost:6379
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		log.Fatal("Cannot listen on port 6379 due to:", err)
	}
	defer l.Close()

	// listener setup success
	log.Println("Listening on port 6379")

	// listener awaiting connection(s)
	conn, err := l.Accept()
	if err != nil {
		log.Fatal("Cannot accept connection due to:", err)
	}
	defer conn.Close()

	// connection(s) accepted from client (here redis-cli)
	log.Println("Accepted connection from:", conn.RemoteAddr())

	for {

		v := Value{
			typ: ARRAY,
		}

		// receive a Value and print it
		v.ReadArray(conn)
		fmt.Printf("%v\n", v)

		// handle the Value (abstracting the command and its args)
		handle(conn, &v, state)
	}

}

type AppState struct {
	config *Config
	aof    *Aof
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
