#!/bin/bash
go mod tidy
go build ./...
echo "Go client (goredis) built successfully."

# Install the Go client package
go get install github.com/akashmaji946/go-redis/go-client@latest
echo "Go client (goredis) installed successfully."
