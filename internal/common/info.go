/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/info.go
*/
package common

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
func (info *RedisInfo) Build(state *AppState, usedMem int64, usedMemPeak int64) {
	exePath, err := os.Executable()
	if err != nil {
		exePath = ""
	}
	info.server = map[string]string{
		"redis_version  ": "v1.0.0",
		"author         ": "akashmaji(@iisc.ac.in)",
		"process_id     ": strconv.Itoa(os.Getpid()),
		"tcp_port       ": strconv.Itoa(state.Config.Port),
		"tls_port       ": strconv.Itoa(state.Config.TlsPort),
		"server_time    ": fmt.Sprint(time.Now().UnixMicro()),
		"server_uptime  ": fmt.Sprintf("%d sec", int64(time.Since(state.ServerStartTime).Seconds())),
		"server_path    ": exePath,
		"config_path    ": state.Config.Filepath,
	}

	info.clients = map[string]string{
		"clients": fmt.Sprint(state.NumClients),
	}

	virtual_memory, err := mem.VirtualMemory()
	var memoryTotal uint64
	if err == nil {
		memoryTotal = virtual_memory.Total
	}
	info.memory = map[string]string{
		"used_memory":         fmt.Sprintf("%d B", usedMem),
		"used_memory_peak":    fmt.Sprintf("%d B", usedMemPeak),
		"total_memory_peak":   fmt.Sprintf("%d B", memoryTotal),
		"total_memory_usable": fmt.Sprintf("%d B", state.Config.Maxmemory),
		"eviction_policy":     string(state.Config.Eviction),
	}

	info.persistence = map[string]string{
		"rdb_bgsave_running":    fmt.Sprint(state.Bgsaving),
		"rdb_last_save_time":    fmt.Sprint(state.RdbStats.RDBLastSavedTS),
		"rdb_saves_count":       fmt.Sprint(state.RdbStats.RDBSavesCount),
		"aof_enabled":           fmt.Sprint(state.Config.AofEnabled),
		"aof_rewrite_running":   fmt.Sprint(state.Aofrewriting),
		"aof_last_rewrite_time": fmt.Sprint(state.AofStats.AofLastRewriteTS),
		"rdb_rewrite_count":     fmt.Sprint(state.AofStats.AofRewriteCount),
	}

	info.general = map[string]string{
		"total_connections_received": fmt.Sprint(state.GenStats.TotalConnectionsReceived),
		"total_commands_executed":    fmt.Sprint(state.GenStats.TotalCommandsExecuted),
		"total_txn_executed":         fmt.Sprint(state.GenStats.TotalTxnExecuted),
		"total_keys_expired":         fmt.Sprint(state.GenStats.TotalExpiredKeys),
		"total_keys_evicted":         fmt.Sprint(state.GenStats.TotalEvictedKeys),
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
func (info *RedisInfo) Print(state *AppState, usedMem int64, usedMemPeak int64) string {

	info.Build(state, usedMem, usedMemPeak)

	var msg string = "\n"

	msg += info.PrintCategory("Server", info.server)
	msg += info.PrintCategory("Clients", info.clients)
	msg += info.PrintCategory("Memory", info.memory)
	msg += info.PrintCategory("Persistence", info.persistence)
	msg += info.PrintCategory("General", info.general)

	msg += "\n"

	return msg
}
