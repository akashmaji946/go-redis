# go-redis: installation and usage guide for Docker

This is a docker setup guide. The user is free to build his own image from the Dockerfile or use an existing image from the registry. Feel free to change the ports and paths for your use-case. However, our Dockerfile uses port 7379

## Prerequities in host

```bash
- redis-cli
- docker
```

## Stop any existing service/redis-server running at port 7379

```bash

## ONE WAY
# see the process running at port 7379
sudo lsof -i :7379
# kill the process
sudo kill <pid>

## OTHER WAY
# find and kill forcefully
sudo kill -9 $(sudo lsof -t -i :7379)
```

Run these if redis-server is running and you want to use port 6379.

```bash
## ONE WAY
# see the status
systemctl status redis-server.service
# stop it
systemctl stop redis-server.service
# start it later
- # systemctl start redis-server.service

## ANOTHER WAY:
# find and kill forcefully
sudo kill -9 $(sudo lsof -t -i :7379)
# start it later
- # systemctl start redis-server.service

```

# Using the prebuilt image from docker-hub

```bash
# Pull the image
docker pull akashmaji/go-redis:latest

# Run it
docker run -d -p 7379:7379 \
  -v redis-data:/app/data \
  akashmaji/go-redis:latest

# Run these
systemctl stop redis-server.service

# Access from your host
redis-cli
```

## Using the dockerfile to build an image

### Build the Docker image

```bash
docker build -t go-redis:latest .
```

Run it for TCP

```bash
docker run -d -p 7379:7379 \
  -v $(pwd)/data:/app/data \
  --name go-redis \
  go-redis:latest
```

Run it for TLS

```bash
docker run -d -p 7379:7379 -p 7380:7380 \
  -v $(pwd)/data:/app/data \
  --name go-redis \
  go-redis:latest
```

### See the docker image so built

```bash
docker images | grep go-redis
```

### Run with default config and data directory

```bash
docker run -d -p 7379:7379 -p 7380:7380\
  --name go-redis \
  -v $(pwd)/data:/app/data \
  go-redis:latest
```

### Check logs

```bash
docker logs -f go-redis
```

### Run with custom config and data directory

```bash
docker run -d \
  --name go-redis \
  -p 6379:6379 \
  -v <PathToYourConfigFile>:/app/config/redis.conf:ro \
  -v <PathToYourDataDir>:/app/data \
  go-redis:latest /app/config/redis.conf /app/data
```

### Access container using using redis-cli from host

```bash
# TCP
redis-cli -p 7379
# TLS
redis-cli -p 7380 --tls --insecure
```

### See container running as:

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
docker rm go-redis
```

## Remove image

```bash
docker rmi go-redis:latest
```

## Optional

```bash
docker tag go-redis:latest akashmaji/go-redis:latest
docker push akashmaji/go-redis:latest
```
