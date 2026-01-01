package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

type RedisInfo struct {
	server      map[string]string
	clients     map[string]string
	memory      map[string]string
	persistence map[string]string
	general     map[string]string
}

func NewRedisInfo() *RedisInfo {
	return &RedisInfo{}
}

func (info *RedisInfo) Build(state *AppState) {
	exePath, err := os.Executable()
	if err != nil {
		exePath = ""
	}
	info.server = map[string]string{
		"redis_version  ": "0.1",
		"author         ": "akashmaji(@iisc.ac.in)",
		"process_id     ": strconv.Itoa(os.Getpid()),
		"tcp_port       ": "6379",
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
		"used_memory      ": fmt.Sprintf("%d B", DB.mem),
		"used_memory_peak ": fmt.Sprintf("%d B", DB.mempeak),
		"total_memory     ": fmt.Sprintf("%d B", memoryTotal),
		"eviction_policy  ": string(state.config.eviction),
	}

	info.persistence = map[string]string{
		"rdb_bgsave_running": fmt.Sprint(state.bgsaving),
	}
}

func (info *RedisInfo) PrintCategory(header string, m map[string]string) string {
	s := fmt.Sprintf("# %s\n", header)
	for k, v := range m {
		s += fmt.Sprintf("%s: %s\n", k, v)
	}
	s += "\n"
	return s
}

func (info *RedisInfo) Print(state *AppState) string {

	info.Build(state)

	var msg string = "\n"

	msg += info.PrintCategory("Server", info.server)
	msg += info.PrintCategory("Clients", info.clients)
	msg += info.PrintCategory("Memory", info.memory)
	// msg += info.PrintCategory("Persistence", info.persistence)
	// msg += info.PrintCategory("General", info.general)

	msg += "\n"

	return msg
}
