#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Cleaning up build and data artifacts...${NC}"

# Clean bin directory
if [ -d "./bin" ]; then
    echo -e "${GREEN}[INFO] Removing files in ./bin...${NC}"
    rm -rf ./bin/*
else
    echo -e "${YELLOW}[WARN] Bin directory ./bin does not exist.${NC}"
fi

# Clean data directory
if [ -d "./data" ]; then
    echo -e "${GREEN}[INFO] Removing files in ./data...${NC}"
    rm -rf ./data/*
else
    echo -e "${YELLOW}[WARN] Data directory ./data does not exist.${NC}"
fi

echo -e "${BLUE}Cleanup complete.${NC}"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color