#!/bin/bash
# VM Analysis Script for Janus Deployment
# Run this on the Azure VM to gather system and deployment readiness info

echo "=== JANUS VM DEPLOYMENT ANALYSIS ==="
echo "Date: $(date)"
echo ""

echo "=== OPERATING SYSTEM ==="
uname -a
echo ""
echo "Distribution:"
cat /etc/os-release 2>/dev/null || echo "Could not read /etc/os-release"
echo ""

echo "=== AVAILABLE RESOURCES ==="
echo "CPU info:"
nproc
lscpu | head -10
echo ""
echo "Memory:"
free -h
echo ""
echo "Disk space:"
df -h
echo ""

echo "=== DOCKER & CONTAINERS ==="
echo "Docker version:"
docker --version 2>/dev/null || echo "Docker NOT installed"
echo ""
echo "Docker Compose version:"
docker-compose --version 2>/dev/null || echo "Docker Compose NOT installed"
echo ""
echo "Current containers:"
docker ps -a 2>/dev/null || echo "Could not list containers"
echo ""
echo "Docker images:"
docker images 2>/dev/null || echo "Could not list images"
echo ""

echo "=== GIT ==="
git --version 2>/dev/null || echo "Git NOT installed"
echo ""

echo "=== NETWORK & PORTS ==="
echo "Network interfaces:"
ip addr 2>/dev/null || hostname -I
echo ""
echo "Hostname:"
hostname
echo ""
echo "Listening ports (HTTP/HTTPS/Redis relevant):"
sudo netstat -tlnp 2>/dev/null | grep -E ':(80|443|6379|8080|8081)' || echo "Could not check ports"
echo ""

echo "=== FIREWALL ==="
echo "UFW status:"
sudo ufw status 2>/dev/null || echo "UFW not available"
echo ""
echo "Open ports:"
sudo ufw show added 2>/dev/null || echo "Could not list firewall rules"
echo ""

echo "=== FILE SYSTEM CRITICAL PATHS ==="
echo "/app directory:"
ls -la /app 2>/dev/null || echo "/app does not exist"
echo ""
echo "Current user:"
whoami
echo ""
echo "User groups:"
id
echo ""

echo "=== JANUS REPO (if exists) ==="
if [ -d "/app/janus" ]; then
  echo "Janus repo found at /app/janus"
  cd /app/janus
  echo "Git status:"
  git status 2>/dev/null || echo "Not a git repo"
  echo ""
  echo "Key files present:"
  ls -la docker-compose.prod.yml config.yaml GeoLite2-City.mmdb .env 2>/dev/null | grep -v "cannot access"
else
  echo "Janus repo not found at /app/janus"
fi
echo ""

echo "=== DOCKER DAEMON ==="
docker info 2>/dev/null | head -20 || echo "Docker daemon not available"
echo ""

echo "=== SUMMARY FOR DEPLOYMENT ==="
echo "Ready for deployment if all of the following are present:"
echo "  ✓ Docker installed"
echo "  ✓ Docker Compose installed (v2+)"
echo "  ✓ Git installed (optional, for git pull)"
echo "  ✓ Ports 80, 443 available (and 6379 if not using remote Redis)"
echo "  ✓ At least 2GB free disk space"
echo "  ✓ At least 2GB available RAM"
echo ""
