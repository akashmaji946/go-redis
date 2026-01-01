# go-redis: installation and usage guide for Docker
## Prerequities in host
```bash
- redis-cli
- docker
```

## Stop any existing service/redis-server running at port 6379
```bash
- systemctl status redis-server.service
- systemctl stop redis-server.service
- #systemctl start redis-server.service
```

# Using the prebuilt image from docker-hub
```bash
# Pull the image
docker pull akashmaji/go-redis:latest

# Run it
docker run -d -p 6379:6379 \
  -v redis-data:/app/data \
  akashmaji/go-redis:latest

# Run these
systemctl stop redis-server.service

# Access from your host
redis-cli
```

# Using the dockerfile to build an image
## Build the Docker image
```bash
docker build -t go-redis:latest .
```

## See the docker image so built
```bash
docker images | grep go-redis
```

## Run with default config and data directory
```bash
docker run -d \
  --name go-redis \
  -p 6379:6379 \
  -v $(pwd)/data:/app/data \
  go-redis:latest
```

## Run with custom config and data directory
```bash
docker run -d \
  --name go-redis \
  -p 6379:6379 \
  -v <PathToYourConfigFile>:/app/config/redis.conf:ro \
  -v <PathToYourDataDir>:/app/data \
  go-redis:latest /app/config/redis.conf /app/data
```

## Access container using using redis-cli from host
```bash
redis-cli
```

## See container running as:
```bash
docker ps | grep go-redis
```

## Exec to see contents
```bash
docker exec -it go-redis /bin/sh
```

## Stop container
```bash
docker stop go-redis
```

## Remove conatiner
```bash
docker rm go-redis:latest
```
