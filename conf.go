package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	rdb        []RDBSnapshot
	rdbFn      string
	dir        string
	aofEnabled bool
	aofFn      string
	aofFsync   FSyncMode
}

type RDBSnapshot struct {
	Secs        int
	KeysChanged int
}

type FSyncMode string

const (
	Always   FSyncMode = "always"
	Everysec FSyncMode = "everysec"
	No       FSyncMode = "no"
)

func NewConfig() *Config {
	return &Config{}

}

func ReadConf(filename string) *Config {
	config := NewConfig()
	f, err := os.Open(filename)
	if err != nil {
		fmt.Printf("can't read file %s - using default config\n", filename)
		return config
	}
	defer f.Close()

	// now we will read the file
	s := bufio.NewScanner(f)
	for s.Scan() {
		l := s.Text()
		parseLine(l, config)

	}

	if err := s.Err(); err != nil {
		fmt.Printf("Error scanning config file %s", filename)
		return config
	}

	if config.dir != "" {
		os.MkdirAll(config.dir, 0755)
	}
	return config
}

func parseLine(l string, config *Config) {

	args := strings.Split(l, " ")
	cmd := args[0]

	switch cmd {
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
	}
}
