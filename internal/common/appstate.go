/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/appstate.go
*/
package common

import (
	"net"
	"sync"
	"time"
)

type RDBStats struct {
	RDBLastSavedTS int64
	RDBSavesCount  int
	KeysChanged    int
}

type AOFStats struct {
	AofLastRewriteTS int64
	AofRewriteCount  int
}

type GeneralStats struct {
	TotalConnectionsReceived int
	TotalCommandsExecuted    int
	TotalTxnExecuted         int
	TotalExpiredKeys         int
	TotalEvictedKeys         int
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
	ServerStartTime time.Time

	Config     *Config
	ConfigPath string

	Aof *Aof

	Bgsaving     bool
	Aofrewriting bool

	DBCopy map[string]*Item
	Tx     *Transaction

	Monitors   []Client
	NumClients int

	RedisInfo *RedisInfo
	RdbStats  *RDBStats
	AofStats  *AOFStats
	GenStats  *GeneralStats

	// we will have in-memory pub-sub system
	Channels map[string][]*Client
	Topics   map[string][]*Client
	PubsubMu sync.RWMutex

	ActiveConns   map[net.Conn]struct{}
	ActiveConnsMu sync.Mutex
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
		Config:          config,
		ServerStartTime: time.Now(),
		RedisInfo:       NewRedisInfo(),
		RdbStats:        &RDBStats{},
		AofStats:        &AOFStats{},
		GenStats:        &GeneralStats{},
		// initialize pubsub maps
		Channels:      make(map[string][]*Client),
		Topics:        make(map[string][]*Client),
		PubsubMu:      sync.RWMutex{},
		ActiveConns:   make(map[net.Conn]struct{}),
		ActiveConnsMu: sync.Mutex{},
	}

	if config.AofEnabled {
		state.Aof = NewAof(config)
		if config.AofFsync == Everysec {
			go func() {
				t := time.NewTicker(time.Second)
				defer t.Stop()

				for range t.C {
					state.Aof.W.Flush()
				}
			}()
		}
	}
	return &state
}

func (s *AppState) AddConn(conn net.Conn) {
	s.ActiveConnsMu.Lock()
	defer s.ActiveConnsMu.Unlock()
	s.ActiveConns[conn] = struct{}{}
}

func (s *AppState) RemoveConn(conn net.Conn) {
	s.ActiveConnsMu.Lock()
	defer s.ActiveConnsMu.Unlock()
	delete(s.ActiveConns, conn)
}

func (s *AppState) CloseAllConnections() {
	s.ActiveConnsMu.Lock()
	defer s.ActiveConnsMu.Unlock()
	for conn := range s.ActiveConns {
		conn.Close()
	}
}
