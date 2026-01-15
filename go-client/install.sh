#!/bin/bash
go mod tidy
go build ./...
echo "Go client (goredis) built successfully."

# Install the Go client package
go get github.com/akashmaji946/go-redis/go-client@v1.0.0
echo "Go client (goredis) installed successfully."
