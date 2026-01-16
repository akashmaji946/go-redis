package common

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	UNIX_TS_EPOCH = time.Time{}.Unix()
)

var ASCII_ART = `
		  [91m ██████╗  ██████╗ [0m
		  [91m██╔════╝ ██╔═══██╗[0m
		  [91m██║  ███╗██║   ██║[0m
		  [91m██║   ██║██║   ██║[0m
		  [91m╚██████╔╝╚██████╔╝[0m
		  [91m ╚═════╝  ╚═════╝ [0m

	   [92m██████╗ ███████╗██████╗ ██╗███████╗[0m
	   [92m██╔══██╗██╔════╝██╔══██╗██║██╔════╝[0m
	   [92m██████╔╝█████╗  ██║  ██║██║███████╗[0m
	   [92m██╔══██╗██╔══╝  ██║  ██║██║╚════██║[0m
	   [92m██║  ██║███████╗██████╔╝██║███████║[0m
	   [92m╚═╝  ╚═╝╚══════╝╚═════╝ ╚═╝╚══════╝[0m

   [94m███████╗███████╗██████╗ ██╗   ██╗███████╗██████╗ [0m
   [94m██╔════╝██╔════╝██╔══██╗██║   ██║██╔════╝██╔══██╗[0m
   [94m███████╗█████╗  ██████╔╝██║   ██║█████╗  ██████╔╝[0m
   [94m╚════██║██╔══╝  ██╔══██╗╚██╗ ██╔╝██╔══╝  ██╔══██╗[0m
   [94m███████║███████╗██║  ██║ ╚████╔╝ ███████╗██║  ██║[0m
   [94m╚══════╝╚══════╝╚═╝  ╚═╝  ╚═══╝  ╚══════╝╚═╝  ╚═╝[0m
   [93m         [93m >>> Go-Redis Server v1.0 <<<      [0m
`

// CommandInfo stores metadata for a command.
type CommandInfo struct {
	Usage       string `json:"usage"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// CommandDetails maps command names to their usage and description.

var CommandDetails map[string]CommandInfo

func init() {

	log.Println(">>>> Go-Redis Server v1.0 <<<<")

	CommandDetails = make(map[string]CommandInfo)
	staticDir := "./static"
	excludeFile := "commands.json"

	// read all entries in the static directory
	files, err := os.ReadDir(staticDir)
	if err != nil {
		log.Fatalf("failed to read directory %s: %v", staticDir, err)
	}

	for _, file := range files {
		// skip directories, the excluded file, and non-JSON files
		if file.IsDir() || file.Name() == excludeFile || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		// construct full path and read the file
		path := filepath.Join(staticDir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("warning: failed to read %s: %v", path, err)
			continue
		}

		// unmarshal into a temporary map
		var tempDetails map[string]CommandInfo
		if err := json.Unmarshal(data, &tempDetails); err != nil {
			log.Fatalf("failed to parse %s: %v", path, err)
		}

		// merge temporary map into the main CommandDetails map
		for k, v := range tempDetails {
			CommandDetails[k] = v
		}

		log.Printf("[INFO] successfully loaded commands from %s", file.Name())
	}
}
