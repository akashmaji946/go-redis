# TODO: Add INSIDE_CONTAINER flag for TLS certificate paths

## Plan
- [x] **Dockerfile** - Add environment variable `INSIDE_CONTAINER=true`
- [x] **cmd/main.go** - Add logic to check `INSIDE_CONTAINER` env var and override TLS paths to `/app/config/cert.pem` and `/app/config/key.pem`

## Followup
- [x] Rebuild Docker image (`go-redis:test`)
- [x] Test TLS connection with `redis-cli -p 7380 --tls --insecure PING` - **SUCCESS: PONG**

## Test Results
- Container logs show: `[INFO] Running inside container, using container TLS certificate paths`
- TLS listener started successfully on port 7380
- `redis-cli -p 7380 --tls --insecure PING` returns `PONG`
