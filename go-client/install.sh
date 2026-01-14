#!/bin/bash
go mod tidy
go build ./...
echo "Go client (goredis) built successfully."
