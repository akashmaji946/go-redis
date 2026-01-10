#!/bin/bash

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${YELLOW}====================================${NC}"
echo -e "${BLUE}   Go-Redis Build Script Started   ${NC}"
echo -e "${YELLOW}====================================${NC}"

## usage: ./run.sh [config-file] [data-directory] [bin-directory]
# optional parameters: ./run.sh [config-file] [data-directory] [bin-directory]
CONFIG_FILE="${1:-./config/redis.conf}"
DATA_DIR="${2:-./data/}"
BIN_DIR="${3:-./}"

# exit if no of arguments is more than 3
if [ $# -gt 3 ]; then
    echo -e "${RED}[ERROR] Too many arguments supplied. Exitting."
    echo -e "        Usage: ./run.sh [config-file] [data-directory] [bin-directory]${NC}"
    exit
fi

# create bin directory if it doesn't exist
mkdir -p "$BIN_DIR"

# if user supplied a config file, require it to exist; otherwise create a default
if [ -n "$1" ]; then
    if [ ! -f "$CONFIG_FILE" ]; then
        echo -e "${RED}[ERROR] Config file '$CONFIG_FILE' not found. Exitting.${NC}"
        exit 1
    fi
else
    if [ ! -f "$CONFIG_FILE" ]; then
        echo -e "${GREEN}[INFO] Default config '$CONFIG_FILE' not found; creating. ${NC}"
        mkdir -p "$(dirname "$CONFIG_FILE")"
        cat > "$CONFIG_FILE" <<EOF
# default config
EOF
    fi
fi

# if user supplied a data directory, require it to exist; otherwise create it
if [ -n "$2" ]; then
    if [ ! -d "$DATA_DIR" ]; then
        echo -e "${RED}[ERROR] Data directory '$DATA_DIR' not found. Exitting.${NC}"
        exit 1
    fi
else
    if [ ! -d "$DATA_DIR" ]; then
        echo -e "${GREEN}[INFO] Creating data directory '$DATA_DIR'. ${NC}"
        mkdir -p "$DATA_DIR"
    fi
fi

# export variables for use in the program

export CONFIG_FILE DATA_DIR
echo -e "${BLUE}[INFO] Using config: $CONFIG_FILE ${NC}"
echo -e "${BLUE}[INFO] Using data dir: $DATA_DIR ${NC}"
echo -e "${NC}"


# prepare log files
BUILD_ERROR_LOG="$BIN_DIR/build.log"
RUN_ERROR_LOG="$BIN_DIR/run.log"
# clean up old logs
if [ -f "$BUILD_ERROR_LOG" ]; then
    rm "$BUILD_ERROR_LOG"
fi
if [ -f "$RUN_ERROR_LOG" ]; then
    rm "$RUN_ERROR_LOG"
fi

touch "$BUILD_ERROR_LOG"
touch "$RUN_ERROR_LOG"

# build the project
echo -e "${GREEN}[INFO] Building go-redis...${NC}"
# save the build output error to a log file
go build -o "$BIN_DIR/go-redis" ./cmd/main.go  2> "$BUILD_ERROR_LOG"
# check if build succeeded
if [ $? -ne 0 ]; then
    echo -e "${RED}[ERROR] Build failed. See 'build.log'. Exiting.${NC}"
    exit 1
fi      
# run the project
echo -e "${GREEN}[INFO] Running go-redis...${NC}"
# run with config and data dir
# ./go-redis  "$CONFIG_FILE" "$DATA_DIR"
# run and capture output to log (preserve real exit status with PIPESTATUS)

"$BIN_DIR/go-redis" "$CONFIG_FILE" "$DATA_DIR" 2> "$RUN_ERROR_LOG"
STATUS=${PIPESTATUS[0]}
if [ $STATUS -ne 0 ]; then
    echo -e "${RED}[ERROR] Go-Redis failed.${NC}"
    exit 1
fi
# ensure the script's "$?" reflects the program exit code for the following check
( exit $STATUS )
if [ $? -ne 0 ]; then
    echo -e "${RED}[ERROR] Execution failed. See 'run.log'. Exiting.${NC}"
    exit 1
fi
echo -e "${GREEN}[INFO] Go-Redis execution completed.${NC}" 

# done
echo -e "${YELLOW}====================================${NC}"
echo -e "${BLUE}  Go-Redis Build Script Completed   ${NC}"
echo -e "${YELLOW}====================================${NC}"
exit 0