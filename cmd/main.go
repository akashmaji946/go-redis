/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/main.go
*/
package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/akashmaji946/go-redis/internal/common"
	"github.com/akashmaji946/go-redis/internal/database"
	"github.com/akashmaji946/go-redis/internal/handlers"
	_ "github.com/akashmaji946/go-redis/internal/handlers"
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

	fmt.Println(common.ASCII_ART)
	log.Println(">>>> Go-Redis Server v1.0 <<<<")

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
	log.Printf("[INFO] config file   : %s\n", configFilePath)
	log.Printf("[INFO] data directory: %s\n", dataDirectoryPath)

	conf := common.ReadConf(configFilePath, dataDirectoryPath)
	state := common.NewAppState(conf)

	// Register BGSave callback so `InitRDBTrackers` can trigger background
	// saves without creating import cycles.
	state.BGSaveFunc = func(s *common.AppState) {
		// reuse handlers.BGSave which handles copying DB and launching SaveRDB
		handlers.BGSave(nil, nil, s)
	}

	// if aof
	if conf.AofEnabled {
		log.Println("syncing records")
		state.Aof.Synchronize(state, func(client *common.Client, v *common.Value, appState *common.AppState) *common.Value {
			handlers.Handle(client, v, appState)
			return nil
		})
	}

	// if rdb
	if len(conf.Rdb) > 0 {
		restored, err := common.SyncRDB(conf, state)
		if err == nil && restored != nil {
			// apply restored map to the global DB
			database.DB.Mu.Lock()
			// replace store
			database.DB.Store = restored
			// recompute memory accounting
			var total int64 = 0
			for k, item := range database.DB.Store {
				if item == nil {
					continue
				}
				total += item.ApproxMemoryUsage(k)
			}
			database.DB.Mem = total
			if database.DB.Mem > database.DB.Mempeak {
				database.DB.Mempeak = database.DB.Mem
			}
			database.DB.Mu.Unlock()
		}
		common.InitRDBTrackers(conf, state)
	}

	// Start active expiration worker
	go database.DB.ActiveExpire(state)

	// Prepare listeners
	var listeners []net.Listener
	binds := conf.Binds
	if len(binds) == 0 {
		binds = []string{""} // Listen on all interfaces if none specified
	}

	for _, ip := range binds {
		// Standard TCP Listener
		addr := fmt.Sprintf("%s:%d", ip, conf.Port)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("Failed to listen on %s: %v", addr, err)
			continue
		}
		listeners = append(listeners, l)
		log.Printf("Listening on %s (TCP)", addr)

		// TLS Listener
		if conf.TlsPort > 0 && conf.TlsCertFile != "" && conf.TlsKeyFile != "" {
			cert, err := tls.LoadX509KeyPair(conf.TlsCertFile, conf.TlsKeyFile)
			if err != nil {
				log.Printf("Failed to load TLS keys: %v", err)
			} else {
				tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
				tlsAddr := fmt.Sprintf("%s:%d", ip, conf.TlsPort)
				tl, err := tls.Listen("tcp", tlsAddr, tlsConfig)
				if err != nil {
					log.Printf("Failed to listen on %s (TLS): %v", tlsAddr, err)
				} else {
					listeners = append(listeners, tl)
					log.Printf("Listening on %s (TLS)", tlsAddr)
				}
			}
		}
	}

	if len(listeners) == 0 {
		log.Fatal("No listeners could be started.")
	}

	// print to console
	fmt.Printf("[INFO] Go-Redis Server is up on port: %d (TCP)\n", conf.Port)
	if conf.TlsPort > 0 {
		fmt.Printf("[INFO] Go-Redis Server is up on port: %d (TLS)\n", conf.TlsPort)
	}

	// Signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n[SHUTDOWN] Signal received, starting graceful shutdown...")

		for _, l := range listeners {
			l.Close()
		}

		// 2. Close all existing client connections
		state.CloseAllConnections()
	}()

	var connectionCount int32 = 0
	var wg sync.WaitGroup

	for _, l := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			for {
				conn, err := ln.Accept()
				if err != nil {
					log.Printf("[SHUTDOWN] Listener on %s closed.", ln.Addr())
					break
				}
				wg.Add(1)
				go func() {
					handleOneConnection(conn, state, &connectionCount)
					wg.Done()
				}()
			}
		}(l)
	}
	wg.Wait()

	// 3. Final persistence save
	log.Println("[SHUTDOWN] All connections closed. Saving final state...")
	// Prepare a DBCopy and perform a synchronous SaveRDB to avoid truncating the
	// RDB file prematurely. This mirrors BGSave's copy step.
	database.DB.Mu.RLock()
	copy := make(map[string]*common.Item, len(database.DB.Store))
	for k, v := range database.DB.Store {
		copy[k] = v
	}
	state.Bgsaving = true
	state.DBCopy = copy
	database.DB.Mu.RUnlock()

	common.SaveRDB(state)

	state.Bgsaving = false
	state.DBCopy = nil

	if state.Config.AofEnabled && state.Aof != nil {
		log.Println("[SHUTDOWN] Flushing AOF to disk...")
		state.Aof.W.Flush()
		state.Aof.F.Sync()
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
func handleOneConnection(conn net.Conn, state *common.AppState, connectionCount *int32) {

	protocol := "TCP"
	if _, ok := conn.(*tls.Conn); ok {
		protocol = "TLS"
	}

	fmt.Printf("[INFO] Accepted connection from  %-3s   : %s\n", protocol, conn.RemoteAddr())

	newCount := atomic.AddInt32(connectionCount, 1)
	state.NumClients = int(newCount)
	state.GenStats.TotalConnectionsReceived += 1

	// Explicitly trigger handshake for TLS connections to catch protocol errors early
	if tlsConn, ok := conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			log.Printf("[%2d] [TLS_ERROR] Handshake failed from %s: %v", newCount, conn.RemoteAddr(), err)
			conn.Close()
			return
		}
	}

	log.Printf("[%2d] [ACCEPT] Protocol: %s | Client: %s", newCount, protocol, conn.RemoteAddr().String())

	state.AddConn(conn)
	defer state.RemoveConn(conn)

	client := common.NewClient(conn)
	reader := bufio.NewReader(conn)

	// Remove from monitors list on disconnect


	for {

		v := common.Value{
			Typ: common.ARRAY,
		}

		// receive a Value and print it
		err := v.ReadArray(reader)
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("[%2d] [ERROR] Read error: %v", newCount, err)
			}
			break
		}

		// Log the command name for debugging
		if len(v.Arr) > 0 {
			log.Printf("[%2d] [EXEC] %s", newCount, v.Arr[0].Blk)
		}

		// handle the Value (abstracting the command and its args)
		handlers.Handle(client, &v, state)
	}

	newCount = atomic.AddInt32(connectionCount, -1)
	state.NumClients = int(newCount)
	log.Printf("[%2d] [CLOSED] Client disconnected: %s\n", newCount, conn.RemoteAddr().String())

	fmt.Printf("[INFO] Closed      connection from %-3s    : %s\n", protocol, conn.RemoteAddr())
}
