/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/conf.go
*/
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all configuration settings for the Redis server.
// This structure stores persistence settings (RDB and AOF), directory paths,
// filenames, and authentication settings parsed from the configuration file.
//
// Fields:
//   - rdb: List of RDB snapshot configurations defining when to save snapshots
//   - rdbFn: Filename for RDB snapshot files (e.g., "backup.rdb")
//   - dir: Directory path where persistence files (RDB and AOF) are stored
//   - aofEnabled: Whether Append-Only File (AOF) persistence is enabled
//   - aofFn: Filename for AOF files (e.g., "backup.aof")
//   - aofFsync: AOF fsync mode (Always, Everysec, or No)
//   - requirepass: Whether password authentication is required
//   - password: The password required for authentication (if requirepass is true)
//
// Default Values:
//   - All fields are zero values (empty/false) when created with NewConfig()
//   - Configuration is populated by reading from redis.conf file
type Config struct {
	rdb   []RDBSnapshot
	rdbFn string
	port  int

	dir string

	aofEnabled bool
	aofFn      string
	aofFsync   FSyncMode

	requirepass bool
	password    string
	sensitive   bool

	maxmemory        int64
	eviction         Eviction
	maxmemorySamples int64

	filepath string
}

type Eviction string

const (
	NoEviction Eviction = "no-eviction"

	AllKeysRandom Eviction = "allkeys-random"

	AllKeysLRU Eviction = "allkeys-lru"
	AllKeysLFU Eviction = "allkeys-lfu"

	VolatileRandom Eviction = "volatile-random"
	VolatileLRU    Eviction = "volatile-lru"
	VolatileLFU    Eviction = "volatile-lfu"
	VolatileTTL    Eviction = "volatile-ttl"
)

// RDBSnapshot defines a snapshot trigger condition for RDB persistence.
// When both conditions are met (time elapsed AND keys changed), a snapshot is saved.
//
// Fields:
//   - Secs: Number of seconds that must elapse before considering a snapshot
//   - KeysChanged: Minimum number of keys that must have changed in that time period
//
// Behavior:
//   - Multiple RDBSnapshot entries can be configured (multiple save rules)
//   - A snapshot is triggered when ANY of the conditions are met
//   - Example: Secs=5, KeysChanged=3 means "save if 3 keys changed in 5 seconds"
//
// Example Configurations:
//   - {Secs: 900, KeysChanged: 1}  - Save if 1 key changed in 15 minutes
//   - {Secs: 300, KeysChanged: 10} - Save if 10 keys changed in 5 minutes
//   - {Secs: 60, KeysChanged: 100} - Save if 100 keys changed in 1 minute
type RDBSnapshot struct {
	Secs        int
	KeysChanged int
}

// FSyncMode represents the AOF fsync strategy for flushing data to disk.
// This determines how frequently AOF data is synchronized to ensure durability.
type FSyncMode string

// AOF fsync mode constants defining different durability strategies.
// These modes balance between performance and data safety.
const (
	// Always syncs the AOF file after every write operation.
	// Provides maximum durability but may impact performance due to frequent disk I/O.
	// Best for: Critical applications where data loss cannot be tolerated
	Always FSyncMode = "always"

	// Everysec syncs the AOF file once per second (in a background goroutine).
	// Provides a good balance between durability and performance.
	// Best for: Most production environments (recommended default)
	Everysec FSyncMode = "everysec"

	// No lets the operating system decide when to sync the AOF file.
	// Fastest option but may lose up to 30 seconds of data in case of a crash.
	// Best for: Non-critical data or when performance is the top priority
	No FSyncMode = "no"
)

// NewConfig creates and returns a new Config instance with default (zero) values.
// This is used as a starting point before reading configuration from a file.
//
// Returns: A pointer to a new Config with all fields set to their zero values
//
// Default State:
//   - aofEnabled: false
//   - requirepass: false
//   - rdb: empty slice
//   - All string fields: empty strings
//   - aofFsync: empty FSyncMode (will be set when parsing config)
//
// Usage:
//
//	config := NewConfig()
//	// Then populate via ReadConf() or manually
func NewConfig() *Config {
	return &Config{
		port:      6379, // Default Redis port
		sensitive: true, // Default command case sensitivity
	}

}

// ReadConf reads and parses a Redis-style configuration file.
// Parses the configuration file line by line and populates a Config structure.
//
// Parameters:
//   - filename: Path to the configuration file (e.g., "./redis.conf")
//
// Returns: A pointer to a Config structure populated with settings from the file
//
// Behavior:
//   - If the file doesn't exist or can't be opened, returns a default Config
//     and prints a warning message
//   - Skips empty lines and comment lines (lines starting with #)
//   - Parses each line using parseLine() to extract configuration directives
//   - Automatically creates the data directory if "dir" is specified
//   - Handles parsing errors gracefully (prints errors but continues)
//
// Configuration File Format:
//   - One directive per line
//   - Format: <directive> <value> [<value2> ...]
//   - Lines starting with # are treated as comments
//   - Empty lines are ignored
//
// Supported Directives:
//   - dir <path>                    - Data directory path
//   - save <seconds> <keys>          - RDB snapshot trigger
//   - dbfilename <filename>          - RDB filename
//   - appendonly yes|no              - Enable/disable AOF
//   - appendfilename <filename>      - AOF filename
//   - appendfsync always|everysec|no - AOF fsync mode
//   - requirepass <password>         - Set authentication password
//
// Example Config File:
//
//	dir ./data
//	save 5 3
//	dbfilename backup.rdb
//	appendonly yes
//	appendfilename backup.aof
//	appendfsync always
//	requirepass mypassword
//
// Error Handling:
//   - File not found: Returns default config with warning
//   - Parse errors: Prints error but continues processing
//   - Invalid values: Prints error message, uses default or skips invalid entry
func ReadConf(filename string, dataDir string) *Config {

	config := NewConfig()

	f, err := os.Open(filename)
	if err != nil {
		log.Printf("can't read file %s - using default config\n", filename)
		return config
	}
	defer f.Close()

	// we know file exists
	config.filepath = filename

	// now we will read the file into config
	s := bufio.NewScanner(f)
	for s.Scan() {
		l := s.Text()
		parseLine(l, config)

	}

	if err := s.Err(); err != nil {
		log.Printf("error scanning config file %s", filename)
		return config
	}

	// if user has provided dataDir then use that and that in 'config' file
	// Override data directory if provided as command line argument
	if dataDir != "" {
		// Convert to absolute path
		absDataDir, err := filepath.Abs(dataDir)
		if err != nil {
			log.Printf("warning: Could not resolve absolute path for '%s', using as-is\n", dataDir)
			absDataDir = dataDir
		}
		config.dir = absDataDir
		log.Printf("using data directory from command line: %s\n", absDataDir)
	}

	// Ensure data directory exists
	if config.dir != "" {
		if err := os.MkdirAll(config.dir, 0755); err != nil {
			log.Fatalf("failed to create data directory '%s': %v\n", config.dir, err)
		}
		log.Printf("data directory: %s\n", config.dir)
	}
	return config
}

// parseLine parses a single line from the configuration file and updates the Config.
// This is a helper function called by ReadConf() for each line in the config file.
//
// Parameters:
//   - l: A single line from the configuration file
//   - config: The Config structure to update with parsed values
//
// Behavior:
//   - Splits the line by spaces to extract directive and arguments
//   - First token is the directive name (case-sensitive)
//   - Remaining tokens are arguments
//   - Skips lines that are empty or start with "#" (comments)
//   - Updates the appropriate field in config based on the directive
//
// Supported Directives:
//
//	"dir" <path>
//	  Sets the data directory where persistence files are stored.
//	  Example: dir ./data
//
//	"save" <seconds> <keys>
//	  Adds an RDB snapshot trigger rule.
//	  Example: save 5 3  (save if 3 keys changed in 5 seconds)
//	  Multiple save directives can be specified.
//
//	"dbfilename" <filename>
//	  Sets the filename for RDB snapshot files.
//	  Example: dbfilename backup.rdb
//
//	"appendfilename" <filename>
//	  Sets the filename for AOF files.
//	  Example: appendfilename backup.aof
//
//	"appendfsync" <mode>
//	  Sets the AOF fsync mode (always, everysec, or no).
//	  Example: appendfsync always
//
//	"appendonly" yes|no
//	  Enables or disables AOF persistence.
//	  Example: appendonly yes
//
//	"requirepass" <password>
//	  Enables password authentication and sets the password.
//	  Example: requirepass mypassword
//
// Error Handling:
//   - Invalid format: Prints error message but continues processing
//   - Missing arguments: May cause index out of bounds (handled by Go runtime)
//   - Invalid integer conversion: Prints error, uses zero value
//
// Note: This function does not handle comments or empty lines - those should
//
//	be filtered before calling this function (currently handled by ReadConf).
func parseLine(l string, config *Config) {

	args := strings.Split(l, " ")
	cmd := args[0]

	switch cmd {
	case "port":
		p, err := strconv.Atoi(args[1])
		if err == nil {
			config.port = p
		}
	case "sensitive":
		if args[1] == "no" {
			config.sensitive = false
		} else {
			config.sensitive = true
		}
	case "dir":
		config.dir = args[1]

	case "save":
		secs, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Invalid Seconds")
		}
		keysChanged, err := strconv.Atoi(args[2])
		if err != nil {
			fmt.Println("Invalid Keys Changed")
		}
		snapshot := RDBSnapshot{
			Secs:        secs,
			KeysChanged: keysChanged,
		}
		config.rdb = append(config.rdb, snapshot)

	case "dbfilename":
		filename := args[1]
		config.rdbFn = filename
	case "appendfilename":
		filename := args[1]
		config.aofFn = filename
	case "appendfsync":
		fsyncmode := FSyncMode(args[1])
		config.aofFsync = fsyncmode
	case "appendonly":
		if args[1] == "yes" {
			config.aofEnabled = true
		} else {
			config.aofEnabled = false
		}
	case "requirepass":
		config.requirepass = true
		config.password = args[1]
	case "maxmemory":
		mem, err := parseMemory(args[1])
		if err != nil {
			config.maxmemory = 1024 // 1kb
		} else {
			config.maxmemory = mem
		}
	case "maxmemory-policy":
		config.eviction = Eviction(args[1])
	case "maxmemory-samples":
		samples, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("invalid maxmemory-samples, setting to default 5")
			config.maxmemorySamples = 5
		} else {
			config.maxmemorySamples = int64(samples)
		}

	}
}

func parseMemory(s string) (mem int64, err error) {
	s = strings.TrimSpace(strings.ToLower(s))
	var mult int64
	switch {
	case strings.HasSuffix(s, "gb"):
		mult = 1024 * 1024 * 1024
	case strings.HasSuffix(s, "mb"):
		mult = 1024 * 1024
	case strings.HasSuffix(s, "kb"):
		mult = 1024
	default:
		mult = 1
	}

	num, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return num * mult, nil
}
