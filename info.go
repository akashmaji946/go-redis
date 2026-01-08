/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/info.go
*/
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

// RedisInfo holds server information organized into categories for the INFO command.
// Each category is a map of key-value pairs that will be formatted and displayed.
type RedisInfo struct {
	server      map[string]string
	clients     map[string]string
	memory      map[string]string
	persistence map[string]string
	general     map[string]string
}

// NewRedisInfo creates and returns a new RedisInfo instance.
// Returns: A pointer to an empty RedisInfo ready to be populated.
func NewRedisInfo() *RedisInfo {
	return &RedisInfo{}
}

// Build populates the RedisInfo structure with current server statistics.
// Gathers information from AppState, database, and system to build all info categories.
//
// Parameters:
//   - state: The application state containing server statistics and configuration
//
// Categories populated:
//   - server: Version, PID, port, uptime, paths
//   - clients: Number of connected clients
//   - memory: Used/peak/total memory, eviction policy
//   - persistence: RDB/AOF status, save times, counts
//   - general: Connections, commands, transactions, expired/evicted keys
func (info *RedisInfo) Build(state *AppState) {
	exePath, err := os.Executable()
	if err != nil {
		exePath = ""
	}
	info.server = map[string]string{
		"redis_version  ": "0.1",
		"author         ": "akashmaji(@iisc.ac.in)",
		"process_id     ": strconv.Itoa(os.Getpid()),
		"tcp_port       ": strconv.Itoa(state.config.port),
		"server_time    ": fmt.Sprint(time.Now().UnixMicro()),
		"server_uptime  ": fmt.Sprint(int64(time.Since(state.serverStartTime).Seconds())),
		"server_path    ": exePath,
		"config_path    ": state.config.filepath,
	}

	info.clients = map[string]string{
		"clients": fmt.Sprint(state.clients),
	}

	virtual_memory, err := mem.VirtualMemory()
	var memoryTotal uint64
	if err == nil {
		memoryTotal = virtual_memory.Total
	}
	info.memory = map[string]string{
		"used_memory":         fmt.Sprintf("%d B", DB.mem),
		"used_memory_peak":    fmt.Sprintf("%d B", DB.mempeak),
		"total_memory_peak":   fmt.Sprintf("%d B", memoryTotal),
		"total_memory_usable": fmt.Sprintf("%d B", state.config.maxmemory),
		"eviction_policy":     string(state.config.eviction),
	}

	info.persistence = map[string]string{
		"rdb_bgsave_running":    fmt.Sprint(state.bgsaving),
		"rdb_last_save_time":    fmt.Sprint(state.rdbStats.rdb_last_saved_ts),
		"rdb_saves_count":       fmt.Sprint(state.rdbStats.rdb_saves_count),
		"aof_enabled":           fmt.Sprint(state.config.aofEnabled),
		"aof_rewrite_running":   fmt.Sprint(state.aofrewriting),
		"aof_last_rewrite_time": fmt.Sprint(state.aofStats.aof_last_rewrite_ts),
		"rdb_rewrite_count":     fmt.Sprint(state.aofStats.aof_rewrite_count),
	}

	info.general = map[string]string{
		"total_connections_received": fmt.Sprint(state.genStats.total_connections_received),
		"total_commands_executed":    fmt.Sprint(state.genStats.total_commands_executed),
		"total_txn_executed":         fmt.Sprint(state.genStats.total_txn_executed),
		"total_keys_expired":         fmt.Sprint(state.genStats.total_expired_keys),
		"total_keys_evicted":         fmt.Sprint(state.genStats.total_evicted_keys),
	}
}

// PrintCategory formats a category header and its key-value pairs.
//
// Parameters:
//   - header: Category name (e.g., "Server", "Memory")
//   - m: Map of key-value pairs to display
//
// Returns: Formatted string with header and aligned key-value pairs
//
// Format:
//
//	# <header>
//	<key>: <value>
//	...
func (info *RedisInfo) PrintCategory(header string, m map[string]string) string {
	s := fmt.Sprintf("# %s\n", header)
	for k, v := range m {
		s += fmt.Sprintf("%30s: %s\n", k, v)
	}
	s += "\n"
	return s
}

// Print generates the complete INFO command output by building and formatting all categories.
//
// Parameters:
//   - state: The application state to gather information from
//
// Returns: Complete formatted INFO output as a string
//
// Process:
//  1. Calls Build() to populate all categories with current data
//  2. Formats each category using PrintCategory()
//  3. Combines all categories into a single formatted string
//
// Output includes: Server, Clients, Memory, Persistence, and General statistics.
func (info *RedisInfo) Print(state *AppState) string {

	info.Build(state)

	var msg string = "\n"

	msg += info.PrintCategory("Server", info.server)
	msg += info.PrintCategory("Clients", info.clients)
	msg += info.PrintCategory("Memory", info.memory)
	msg += info.PrintCategory("Persistence", info.persistence)
	msg += info.PrintCategory("General", info.general)

	msg += "\n"

	return msg
}
