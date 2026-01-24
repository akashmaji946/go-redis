## free ports 7379 and 7380
sudo kill -9 $(sudo lsof -t -i :7379)
sudo kill -9 $(sudo lsof -t -i :7380)

## standard TCP connection
redis-cli -h 127.0.0.1 -p 7379

## TLS/SSL connection
# (use --insecure for self-signed certs).
redis-cli -h 127.0.0.1 -p 7380 --tls --insecure

