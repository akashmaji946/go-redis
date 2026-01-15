# Multi-stage build for Go-Redis
# Stage 1: Build
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy all files
COPY . .

# Download dependencies
RUN go mod download


# Build the application
# CGO_ENABLED=0 creates a static binary (no C dependencies)
# -ldflags="-w -s" reduces binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o go-redis \
    ./cmd

# Stage 2: Runtime
FROM alpine:latest

# Install ca-certificates for HTTPS (if needed for dependencies)
RUN apk --no-cache add ca-certificates tzdata openssl

# # Create non-root user for security
# RUN addgroup -g 1000 redis && \
#     adduser -D -u 1000 -G redis redis 

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/go-redis /app/go-redis

# Copy static files (commands.json etc.) from builder so runtime can read them
COPY --from=builder /build/static /app/static

# Copy default configuration file
COPY config/redis.conf /app/config/redis.conf

# Create data directory with proper permissions
RUN mkdir -p /app/data && \
    mkdir -p /app/config

# Generate self-signed TLS certificates
RUN openssl req -x509 -newkey rsa:4096 \
  -keyout /app/config/key.pem \
  -out /app/config/cert.pem \
  -days 365 -nodes \
  -subj "/CN=go-redis"


# Expose Redis port
EXPOSE 7379 7380

# Health check
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#     CMD nc -z localhost 7379 || exit 1

# Default command
# Accepts config file and data directory as arguments
# Format: ./go-redis [config_file] [data_directory]
CMD ["./go-redis", "/app/config/redis.conf", "/app/data"]