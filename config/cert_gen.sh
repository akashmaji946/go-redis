#!/bin/bash
# Simple script to generate self-signed TLS certificates for testing purposes
# Generates cert.pem and key.pem in the current directory
# Put them in config/ and configure redis.conf to use them for TLS
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
chmod 600 key.pem cert.pem

# go-redis configuration example for TLS:
# tls-port 7380
# tls-cert-file ./config/cert.pem
# tls-key-file ./config/key.pem
# tls-ca-cert-file ./config/cert.pem
# tls-auth-clients no