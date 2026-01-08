/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/main.go
*/
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

// Entry point of the Go-Redis-Server application.
// It initializes the server, loads configuration, restores data from persistence,
// and starts accepting client connections on port 6379.
//
// Server Startup Sequence:
//  1. Print server banner
//  2. Read configuration from redis.conf file
//  3. Initialize application state (AOF, RDB trackers)
//  4. Restore data from AOF if enabled (Synchronize)
//  5. Restore data from RDB if configured (SyncRDB)
//  6. Initialize RDB snapshot trackers if RDB is enabled
//  7. Start TCP listener on port 6379
//  8. Accept and handle client connections concurrently
//
// Persistence Restoration:
//   - AOF: Replays all commands from AOF file to rebuild database
//   - RDB: Loads database snapshot from RDB file
//   - Both can be enabled simultaneously (AOF takes precedence if both exist)
//
// Connection Handling:
//   - Each client connection is handled in a separate goroutine
//   - Uses sync.WaitGroup to track active connections
//   - Server runs indefinitely until terminated (Ctrl+C or kill signal)
//
// Port:
//   - Default Redis port: 6379
//   - Listens on all network interfaces (":6379")
//
// Error Handling:
//   - Fatal errors (listen failure) cause server to exit
//   - Individual connection errors are logged but don't stop the server
func main() {

	fmt.Println(">>> Go-Redis Server v1.0 <<<")
	fmt.Println(ASCII_ART)

	// defaults for config file and data directory
	configFilePath := "./config/redis.conf"
	dataDirectoryPath := "./data/"

	// override from command line args if provided
	args := os.Args[1:]
	if len(args) > 0 {
		configFilePath = args[0]
	}
	if len(args) > 1 {
		dataDirectoryPath = args[1]
	}

	if len(args) > 2 {
		log.Fatalln("usage: ./go-redis [config-file] [data-directory]")
		os.Exit(1)
	}

	// read the config file
	log.Println("reading the config file...")

	conf := ReadConf(configFilePath, dataDirectoryPath)
	state := NewAppState(conf)

	// if aof
	if conf.aofEnabled {
		log.Println("syncing records")
		mem := NewMem(conf)
		state.aof.Synchronize(mem)
	}

	// if rdb
	if len(conf.rdb) > 0 {
		SyncRDB(conf, state)
		InitRDBTrackers(conf, state)
	}

	// setup a tcp listener
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.port))
	if err != nil {
		log.Fatalf("cannot listen on port %d due to: %v", conf.port, err)
	}

	// listener setup success
	log.Printf("listening on port %d\n", conf.port)

	// Signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n[SHUTDOWN] Signal received, starting graceful shutdown...")

		// 1. Stop accepting new connections
		l.Close()

		// 2. Close all existing client connections
		state.CloseAllConnections()
	}()

	var connectionCount int32 = 0

	// listener awaiting connection(s)
	var wg sync.WaitGroup
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("[SHUTDOWN] Listener closed, stopping accept loop.")
			break
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

	// 3. Final persistence save
	log.Println("[SHUTDOWN] All connections closed. Saving final state...")
	SaveRDB(state)

	if state.config.aofEnabled && state.aof != nil {
		log.Println("[SHUTDOWN] Flushing AOF to disk...")
		state.aof.w.Flush()
		state.aof.f.Sync()
	}

	log.Println("[SHUTDOWN] Graceful shutdown complete. Goodbye!")
}

// handleOneConnection manages a single client connection for its entire lifetime.
// This function runs in a separate goroutine for each connected client, allowing
// the server to handle multiple clients concurrently.
//
// Parameters:
//   - conn: The network connection to the client
//   - state: The shared application state (config, AOF, database)
//   - connectionCount: Pointer to the global connection counter (for logging)
//
// Behavior:
//  1. Logs the connection acceptance with connection number
//  2. Increments the connection counter
//  3. Creates a new Client instance for this connection
//  4. Enters a loop to process commands:
//     a. Reads a RESP array from the connection (parses command)
//     b. Logs the received command (for debugging)
//     c. Handles the command and sends response
//     d. Repeats until connection is closed
//  5. Decrements connection counter on disconnect
//
// Command Processing:
//   - Each command is parsed as a RESP array using ReadArray()
//   - Commands are handled by the handle() function which routes to appropriate handlers
//   - Responses are automatically sent back to the client
//
// Connection Lifecycle:
//   - Connection remains open until client disconnects or error occurs
//   - On EOF or read error: connection is closed and function returns
//   - Connection is not explicitly closed here (handled by OS on return)
//
// Concurrency:
//   - Each client connection runs in its own goroutine
//   - Multiple clients can be served simultaneously
//   - Shared state (database, AOF) is protected by mutexes in handlers
//
// Error Handling:
//   - Read errors (EOF, network errors): Logged and connection closed gracefully
//   - Command parsing errors: Handled by handle() function
//   - Does not crash the server on individual connection errors
//
// Example Flow:
//
//	Client connects -> handleOneConnection() starts
//	-> Client sends: "*2\r\n$3\r\nGET\r\n$4\r\nname\r\n"
//	-> Parsed as: Value{typ: ARRAY, arr: [BULK("GET"), BULK("name")]}
//	-> handle() routes to Get() handler
//	-> Response sent back to client
//	-> Loop continues for next command
func handleOneConnection(conn net.Conn, state *AppState, connectionCount *int32) {

	newCount := atomic.AddInt32(connectionCount, 1)
	state.clients = int(newCount)
	state.genStats.total_connections_received += 1
	log.Printf("[%2d] [ACCEPT] Accepted connection from: %s\n", newCount, conn.LocalAddr().String())

	state.AddConn(conn)
	defer state.RemoveConn(conn)

	client := NewClient(conn)
	reader := bufio.NewReader(conn)

	// remove from monitors list if there
	defer func() {
		newmonitors := state.monitors[:0] // same capacity but zero size
		for _, mon := range state.monitors {
			if &mon != client {
				newmonitors = append(newmonitors, mon)
			}
		}
		state.monitors = newmonitors
	}()

	for {

		v := Value{
			typ: ARRAY,
		}

		// receive a Value and print it
		err := v.ReadArray(reader)
		if err != nil {
			log.Println("[CLOSE] Closing connection due to: ", err)
			break
		}

		// optional: print what we got
		log.Printf("%v\n", v)

		// handle the Value (abstracting the command and its args)
		handle(client, &v, state)
	}

	newCount = atomic.AddInt32(connectionCount, -1)
	state.clients = int(newCount)
	log.Printf("[%2d] [CLOSED] Closed connection from: %s\n", newCount, conn.LocalAddr().String())

}
