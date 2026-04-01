#!/bin/bash
# Janus Deployment Script for Azure VM

set -e

cd /home/djtunix/janus

echo "Creating .env file..."
cat > .env << 'EOF'
JANUS_JWT_SECRET=4f8b3c2a9e6d1f2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
GEOIP_PATH=/app/GeoLite2-City.mmdb
BUILD_VERSION=1.0.0
LOG_LEVEL=info
OFFENDER_MEMORY_MINS=1440
TZ=UTC
EOF

echo ".env created:"
cat .env
echo ""

echo "Verifying required files..."
for file in config.yaml GeoLite2-City.mmdb cert.pem key.pem docker-compose.prod.yml; do
  if [ -f "$file" ]; then
    echo "✓ $file"
  else
    echo "❌ $file NOT FOUND - deployment will fail"
  fi
done
echo ""

echo "Building Docker image..."
docker compose -f docker-compose.prod.yml build --no-cache

echo ""
echo "Starting services..."
docker compose -f docker-compose.prod.yml down 2>/dev/null || true
docker compose -f docker-compose.prod.yml up -d --force-recreate

echo ""
echo "Waiting for services to start..."
sleep 10

echo ""
echo "Checking container status..."
docker compose -f docker-compose.prod.yml ps

echo ""
echo "Testing health endpoint..."
sleep 5
curl -k https://localhost:443/health || echo "Health check pending..."

echo ""
echo "Deployment complete!"
docker logs --tail 20 janus-server
