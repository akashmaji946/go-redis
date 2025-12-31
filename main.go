package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
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

// Client represents a connected client session.
// Each client connection has its own Client instance that tracks connection-specific state.
//
// Fields:
//   - conn: The network connection to the client (used for reading commands and sending responses)
//   - authenticated: Whether this client has successfully authenticated
//     (required if requirepass is set in config)
//
// Authentication:
//   - Initially false for all new connections
//   - Set to true after successful AUTH command
//   - Checked before executing commands (if requirepass is enabled)
//   - Safe commands (COMMAND, AUTH) can be executed without authentication
//
// Lifecycle:
//   - Created when client connects (in handleOneConnection)
//   - Exists for the duration of the client connection
//   - Destroyed when connection closes (garbage collected)
//
// Thread Safety:
//   - Each Client is used by a single goroutine (one per connection)
//   - No synchronization needed for Client fields
type Client struct {
	conn          net.Conn
	authenticated bool
}

// NewClient creates a new Client instance for a network connection.
// Initializes a client with the given connection and sets authentication to false.
//
// Parameters:
//   - conn: The network connection associated with this client
//
// Returns: A pointer to a new Client instance with authenticated=false
//
// Usage:
//   - Called once per client connection in handleOneConnection()
//   - The Client instance is then used for all command handling for that connection
func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:          conn,
		authenticated: false,
	}
}

// AppState holds the global application state shared across all client connections.
// This structure contains configuration, persistence mechanisms, background operation flags,
// and transaction state.
//
// Fields:
//   - config: Server configuration (persistence settings, authentication, etc.)
//   - aof: AOF persistence instance (nil if AOF is disabled)
//   - bgsaving: Flag indicating if a background RDB save is currently in progress
//     Used to prevent concurrent background saves
//   - DBCopy: Copy of the database used during background saves
//     Contains a snapshot of DB.store taken at the start of BGSAVE
//     Only populated during background saves, nil otherwise
//   - tx: Current transaction context for this client connection
//     Set to non-nil when MULTI is called, cleared by EXEC or DISCARD
//     Each client connection has its own transaction state
//
// ... rest of existing documentation ...
type AppState struct {
	config   *Config
	aof      *Aof
	bgsaving bool
	DBCopy   map[string]*VAL
	tx       *Transaction
}

// NewAppState creates and initializes a new AppState instance.
// Sets up persistence mechanisms (AOF) and background workers based on configuration.
//
// Parameters:
//   - config: The server configuration containing persistence and other settings
//
// Returns: A pointer to a new AppState instance with persistence initialized
//
// Behavior:
//   - Creates AppState with the provided config
//   - If AOF is enabled:
//   - Creates and initializes AOF instance (opens AOF file)
//   - If fsync mode is "everysec", starts a background goroutine that
//     flushes the AOF writer every second
//   - If AOF is disabled, aof field remains nil
//
// Background Workers:
//   - AOF fsync worker (if aofFsync == Everysec):
//   - Runs in a separate goroutine
//   - Flushes AOF writer every second using a ticker
//   - Ensures AOF data is written to disk periodically
//   - Stops automatically when ticker is stopped (on server shutdown)
//
// Initialization Order:
//  1. Create AppState with config
//  2. Initialize AOF if enabled
//  3. Start background workers if needed
//  4. Return initialized state
//
// Example:
//
//	conf := ReadConf("./redis.conf")
//	state := NewAppState(conf)
//	// state is ready to use, AOF initialized if enabled
//
// Note: RDB initialization (trackers) happens separately in main() after AppState creation
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
