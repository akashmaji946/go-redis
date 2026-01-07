/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/appstate.go
*/
package main

import (
	"net"
	"sync"
	"time"
)

type RDBStats struct {
	rdb_last_saved_ts int64
	rdb_saves_count   int
}

type AOFStats struct {
	aof_last_rewrite_ts int64
	aof_rewrite_count   int
}

type GeneralStats struct {
	total_connections_received int
	total_commands_executed    int
	total_txn_executed         int
	total_expired_keys         int
	total_evicted_keys         int
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
//   - DBCopy: Copy of the databastructse used during background saves
//     Contains a snapshot of DB.store taken at the start of BGSAVE
//     Only populated during background saves, nil otherwise
//   - tx: Current transaction context for this client connection
//     Set to non-nil when MULTI is called, cleared by EXEC or DISCARD
//     Each client connection has its own transaction state
//
// ... rest of existing documentation ...
type AppState struct {
	serverStartTime time.Time

	config     *Config
	configPath string

	aof *Aof

	bgsaving     bool
	aofrewriting bool

	DBCopy map[string]*Item
	tx     *Transaction

	monitors []Client
	clients  int

	redisInfo *RedisInfo
	rdbStats  *RDBStats
	aofStats  *AOFStats
	genStats  *GeneralStats

	// we will have in-memory pub-sub system
	channels map[string][]*Client
	topics   map[string][]*Client
	pubsubMu sync.RWMutex

	activeConns   map[net.Conn]struct{}
	activeConnsMu sync.Mutex
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
		config:          config,
		serverStartTime: time.Now(),
		redisInfo:       NewRedisInfo(),
		rdbStats:        &RDBStats{},
		aofStats:        &AOFStats{},
		genStats:        &GeneralStats{},
		// initialize pubsub maps
		channels:      make(map[string][]*Client),
		topics:        make(map[string][]*Client),
		pubsubMu:      sync.RWMutex{},
		activeConns:   make(map[net.Conn]struct{}),
		activeConnsMu: sync.Mutex{},
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

func (s *AppState) AddConn(conn net.Conn) {
	s.activeConnsMu.Lock()
	defer s.activeConnsMu.Unlock()
	s.activeConns[conn] = struct{}{}
}

func (s *AppState) RemoveConn(conn net.Conn) {
	s.activeConnsMu.Lock()
	defer s.activeConnsMu.Unlock()
	delete(s.activeConns, conn)
}

func (s *AppState) CloseAllConnections() {
	s.activeConnsMu.Lock()
	defer s.activeConnsMu.Unlock()
	for conn := range s.activeConns {
		conn.Close()
	}
}
