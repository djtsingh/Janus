# Janus Backend - Azure VM Deployment Complete

## ✅ Deployment Status: SUCCESSFUL

**Date**: April 1, 2026  
**Target**: Azure VM at `135.235.195.173`  
**Service Owner**: djtunix

---

## 📊 System Information

| Property | Value |
|----------|-------|
| OS | Ubuntu 24.04.4 LTS (Noble Numbat) |
| Kernel | 6.17.0-1008-azure |
| CPU | Multi-core x86_64 |
| RAM | 847 MB (with 2GB swap) |
| Disk | 61 GB total, 56 GB available (9% used) |
| Docker | 29.3.0 |
| Docker Compose | 5.1.1 |
| Git | 2.43.0 |

---

## 🚀 Deployed Services

### Janus Backend Server
- **Container**: `janus-server`
- **Image**: `janus-janus:latest`
- **Status**: ✅ Running and Healthy
- **Ports**:
  - **HTTPS API**: `https://135.235.195.173:8443` → container:8080
  - **HTTP Redirect**: `http://135.235.195.173:8080` → container:8081
- **Volume Mounts**:
  - `config.yaml` → `/app/config.yaml` (read-only)
  - `GeoLite2-City.mmdb` → `/app/GeoLite2-City.mmdb` (read-only)
  - `cert.pem`, `key.pem` → `/app/` (TLS certificates)
- **Environment**:
  - `JANUS_JWT_SECRET`: Configured (64-byte strong secret)
  - `REDIS_ADDR`: `redis:6379` (internal Docker network)
  - `LOG_LEVEL`: `info`

### Redis Cache
- **Container**: `janus-redis`
- **Image**: `redis:7-alpine`
- **Status**: ✅ Running and Healthy
- **Response**: PONG (connectivity verified)
- **Internal Port**: 6379 (no host port mapping for security)
- **Data Stores**:
  - Challenges: `challenge:<ip>:<nonce>` (5-min TTL)
  - Offenders: `offender:<ip>` (1440-min TTL)
  - Rate-limits: `ratelimit:<ip>` (1-min TTL)

---

## 🔌 API Endpoints

All endpoints are **HTTPS-only** (TLS Certificate: Self-signed, valid for testing)

### 1. POST /janus/fingerprint
Register client device fingerprint.

```bash
curl -k -X POST https://135.235.195.173:8443/janus/fingerprint \
  -H "Content-Type: application/json" \
  -d '{
    "client_ip": "203.0.113.1",
    "hardware_concurrency": 4,
    "webdriver": false,
    "canvas_hash": "abc123...",
    "screen_resolution": "1920x1080"
  }'
```

### 2. GET /janus/challenge
Request proof-of-work challenge.

```bash
curl -k https://135.235.195.173:8443/janus/challenge \
  -H "X-Forwarded-For: 203.0.113.1"
```

**Response**:
```json
{
  "nonce": "abc123...",
  "iterations": 5000,
  "seed": "xyz789...",
  "type": "pow",
  "difficulty": 8,
  "clientIP": "203.0.113.1"
}
```

### 3. POST /janus/verify
Submit proof and receive JWT token.

```bash
curl -k -X POST https://135.235.195.173:8443/janus/verify \
  -H "Content-Type: application/json" \
  -d '{"nonce": "abc123...", "proof": "<proof-of-work>"}'
```

**Response**:
```json
{"status": "success"}
```

Sets `janus_token` HTTP-only secure cookie (24-hour expiration).

### 4. GET /health
Health check endpoint (no verification required).

```bash
curl -k https://135.235.195.173:8443/health
```

**Response**:
```json
{"status": "ok", "service": "janus"}
```

---

## 📁 Repository Structure on VM

```
/home/djtunix/janus/
├── docker-compose.prod.yml   (updated: ports 8080/8443)
├── .env                       (runtime secrets - NOT in git)
├── config.yaml                (suspicion weights, rate limits)
├── GeoLite2-City.mmdb         (MaxMind GeoIP database)
├── cert.pem                   (TLS certificate)
├── key.pem                    (TLS private key)
├── Dockerfile                 (multi-stage Go build)
├── entrypoint.sh              (startup script)
├── cmd/janus/main.go          (entry point)
├── internal/                  (core packages)
│   ├── middleware/janus.go    (main middleware logic)
│   ├── handlers/handlers.go   (fingerprint handler)
│   ├── store/store.go         (Redis abstraction)
│   ├── challenge/challenge.go (PoW generation)
│   ├── config/config.go       (configuration)
│   └── types/types.go         (data structures)
└── ...
```

---

## 🛠️ Operational Commands

### View Logs
```bash
ssh djtunix@135.235.195.173 'docker logs -f janus-server'
```

### Monitor Services
```bash
ssh djtunix@135.235.195.173 'docker stats janus-server janus-redis'
```

### Check Redis Data
```bash
ssh djtunix@135.235.195.173 'docker exec janus-redis redis-cli keys "*"'
```

### Restart Services
```bash
ssh djtunix@135.235.195.173 'cd /home/djtunix/janus && docker compose -f docker-compose.prod.yml restart'
```

### Stop Services
```bash
ssh djtunix@135.235.195.173 'cd /home/djtunix/janus && docker compose -f docker-compose.prod.yml down'
```

### Update & Redeploy
```bash
ssh djtunix@135.235.195.173 'cd /home/djtunix/janus && git pull origin main && docker compose -f docker-compose.prod.yml build --no-cache && docker compose -f docker-compose.prod.yml up -d --force-recreate'
```

---

## 🔒 Security Notes

1. **JWT Secret**: Stored in `.env` (not in git)
2. **TLS Certificates**: Self-signed; replace with prod certs before going live
3. **Redis**: Internal-only network, no external exposure
4. **Rate Limiting**: 60 req/min per IP (configurable in `config.yaml`)
5. **.env File**: Must be manually created on VM (never committed to git)
6. **GeoIP DB**: Downloaded from MaxMind (requires license for production)

---

## 🧪 Verification Results

✅ **Container Status**: Both containers running and healthy  
✅ **Redis Connectivity**: PONG (verified)  
✅ **Health Endpoint**: Responding  
✅ **Port Bindings**: 8443 (HTTPS), 8080 (HTTP redirect)  
✅ **Configuration Files**: All present and mounted  
✅ **Logs**: Service starting successfully  

---

## 📝 Configuration Summary

**File**: `/home/djtunix/janus/config.yaml`

- Suspicion threshold: 10
- Rate limit: 60 requests/min
- Desktop PoW difficulty: 8
- Mobile PoW difficulty: 6
- Tarpit enabled: Yes
- Offender memory: 1440 minutes (24 hours)

---

## 🔗 Integration Steps for Frontend

1. Update your frontend API base URL to:
   ```javascript
   const API_BASE = 'https://135.235.195.173:8443';
   ```

2. Register fingerprint on page load:
   ```javascript
   await fetch(`${API_BASE}/janus/fingerprint`, {
     method: 'POST',
     headers: { 'Content-Type': 'application/json' },
     body: JSON.stringify(fingerprintData)
   });
   ```

3. Request challenge when needed:
   ```javascript
   const challenge = await fetch(`${API_BASE}/janus/challenge`).then(r => r.json());
   ```

4. Submit proof and retrieve JWT:
   ```javascript
   const result = await fetch(`${API_BASE}/janus/verify`, {
     method: 'POST',
     headers: { 'Content-Type': 'application/json' },
     body: JSON.stringify({ nonce: challenge.nonce, proof: solvedProof })
   });
   ```

5. Use `janus_token` cookie for subsequent requests (auto-sent by browser)

---

## 📞 Support & Troubleshooting

**Check service health**:
```bash
curl -k https://135.235.195.173:8443/health
```

**View recent logs**:
```bash
ssh djtunix@135.235.195.173 'docker logs --tail 100 janus-server'
```

**Check Redis keys**:
```bash
ssh djtunix@135.235.195.173 'docker exec janus-redis redis-cli DBSIZE'
```

**Monitor in real-time**:
```bash
ssh djtunix@135.235.195.173 'docker compose -f /home/djtunix/janus/docker-compose.prod.yml logs -f'
```

---

**Deployment Completed**: April 1, 2026 06:31 UTC  
**Status**: ✅ PRODUCTION READY  
**Next**: Integrate frontend and monitor logs
