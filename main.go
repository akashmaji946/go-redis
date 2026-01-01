package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

// main is the entry point of the Go-Redis server application.
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

	fmt.Println(">>> Go-Redis Server v0.1 <<<")
	fmt.Println(`
:'######::::'#######:::::::::::'########::'########:'########::'####::'######::
'##... ##::'##.... ##:::::::::: ##.... ##: ##.....:: ##.... ##:. ##::'##... ##:
 ##:::..::: ##:::: ##:::::::::: ##:::: ##: ##::::::: ##:::: ##:: ##:: ##:::..::
 ##::'####: ##:::: ##:'#######: ########:: ######::: ##:::: ##:: ##::. ######::
 ##::: ##:: ##:::: ##:........: ##.. ##::: ##...:::: ##:::: ##:: ##:::..... ##:
 ##::: ##:: ##:::: ##:::::::::: ##::. ##:: ##::::::: ##:::: ##:: ##::'##::: ##:
. ######:::. #######::::::::::: ##:::. ##: ########: ########::'####:. ######::
:......:::::.......::::::::::::..:::::..::........::........:::....:::......:::
`)

	configFilePath := "./config/redis.conf"
	dataDirectoryPath := "./data/"

	args := os.Args[1:]
	if len(args) > 0 {
		configFilePath = args[0]
	}
	if len(args) > 1 {
		dataDirectoryPath = args[1]
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
func handleOneConnection(conn net.Conn, state *AppState, connectionCount *int) {

	*connectionCount += 1
	state.clients = *connectionCount
	state.genStats.total_connections_received += 1
	log.Printf("[%2d] [ACCEPT] Accepted connection from: %s\n", *connectionCount, conn.LocalAddr().String())

	client := NewClient(conn)
	reader := bufio.NewReader(conn)

	// remove from monitors list if there
	defer func() {
		newmonitors := state.monitors[:0] // same capacity but zero size
		for _, mon := range state.monitors {
			if mon != *client {
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
			*connectionCount -= 1
			break
		}

		// optional: print what we got
		log.Printf("%v\n", v)

		// handle the Value (abstracting the command and its args)
		handle(client, &v, state)
	}

	*connectionCount -= 1
	state.clients = *connectionCount
	log.Printf("[%2d] [CLOSED] Closed connection from: %s\n", *connectionCount, conn.LocalAddr().String())

}
