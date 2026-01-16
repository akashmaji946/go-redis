/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/appstate.go
*/
package common

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"io"
	"net"
	"os"
	"path"
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

	Users   map[string]*User
	UsersMu sync.RWMutex

	// we will have in-memory pub-sub system
	Channels map[string][]*Client
	Topics   map[string][]*Client
	PubsubMu sync.RWMutex

	ActiveConns   map[net.Conn]struct{}
	ActiveConnsMu sync.Mutex
	// BGSaveFunc is an optional callback that performs a background RDB save.
	// It's set by the application (main) to avoid import cycles between
	// packages. If non-nil, `InitRDBTrackers` will call this to perform
	// background saves when snapshot conditions trigger.
	BGSaveFunc func(*AppState)
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
		Users:         make(map[string]*User),
	}

	// Initialize root user
	state.Users["root"] = &User{
		Username: "root",
		FullName: "root",
		Password: config.Password,
		Admin:    true,
	}

	state.LoadUsers()

	if config.AofEnabled {
		state.Aof = NewAof(config, 0) // default DB ID 0 for now

		// start AOF fsync worker if everysec mode
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

func (s *AppState) LoadUsers() {
	fp := path.Join(s.Config.Dir, "passwd.bin")
	f, err := os.Open(fp)
	if err != nil {
		return
	}
	defer f.Close()

	content, _ := io.ReadAll(f)
	if len(content) == 0 {
		return
	}

	if s.Config.Encrypt {
		key := sha256.Sum256([]byte(s.Config.Nonce))
		block, _ := aes.NewCipher(key[:])
		gcm, _ := cipher.NewGCM(block)
		nonceSize := gcm.NonceSize()
		if len(content) > nonceSize {
			nonce, ciphertext := content[:nonceSize], content[nonceSize:]
			content, err = gcm.Open(nil, nonce, ciphertext, nil)
			if err != nil {
				logger.Error("failed to decrypt passwd.bin\n")
				return
			}
		}
	}

	var loadedUsers map[string]*User
	gob.NewDecoder(bytes.NewReader(content)).Decode(&loadedUsers)
	s.UsersMu.Lock()
	for k, v := range loadedUsers {
		s.Users[k] = v
	}
	s.UsersMu.Unlock()
}

func (s *AppState) SaveUsers() {
	var buf bytes.Buffer
	s.UsersMu.RLock()
	gob.NewEncoder(&buf).Encode(s.Users)
	s.UsersMu.RUnlock()

	data := buf.Bytes()
	if s.Config.Encrypt {
		key := sha256.Sum256([]byte(s.Config.Nonce))
		block, _ := aes.NewCipher(key[:])
		gcm, _ := cipher.NewGCM(block)
		nonce := make([]byte, gcm.NonceSize())
		io.ReadFull(rand.Reader, nonce)
		data = gcm.Seal(nonce, nonce, data, nil)
	}

	fp := path.Join(s.Config.Dir, "passwd.bin")
	os.WriteFile(fp, data, 0600)
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
