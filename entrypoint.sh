#!/usr/bin/env bash
set -euo pipefail

cd /app

# Generate self-signed certs if not present
if [ ! -f cert.pem ] || [ ! -f key.pem ]; then
  echo "Generating self-signed certs..."
  openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
fi

echo "Starting janus server"
exec ./janus
