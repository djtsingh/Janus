# Frontend Integration Guide for Janus Backend

## Quick Start

The Janus backend is now running at: **`https://135.235.195.173:8443`**

## Setup Steps

### 1. Install Janus Client SDK (if available)
```bash
npm install janus-client-sdk
```

### 2. Initialize Janus in Your Frontend

```javascript
import { JanusClient } from 'janus-client-sdk';

const janus = new JanusClient({
  apiBase: 'https://135.235.195.173:8443',
  allowInsecureSSL: true  // Only for self-signed certs; use false in production
});

// On page load, register fingerprint
await janus.registerFingerprint();
```

### 3. Request Challenge When Needed

```javascript
// Trigger when user makes a protected request
const challenge = await janus.requestChallenge();
// challenge: { nonce, iterations, seed, type, difficulty }
```

### 4. Solve & Submit Proof

```javascript
// Solve the proof-of-work challenge (handled by SDK)
const proof = await janus.solveChallenge(challenge);

// Verify and get JWT token
const token = await janus.verify(challenge.nonce, proof);
// token is set as 'janus_token' cookie automatically
```

### 5. Make Protected Requests

```javascript
// All subsequent requests include janus_token cookie
const response = await fetch('https://your-api.com/protected-endpoint', {
  credentials: 'include'  // Sends cookies including janus_token
});
```

---

## Manual Integration (No SDK)

If not using an SDK, implement manually:

### Step 1: Register Fingerprint
```javascript
const fingerprint = {
  client_ip: clientIp,  // Optional; server derives from request
  hardware_concurrency: navigator.hardwareConcurrency,
  webdriver: navigator.webdriver,
  canvas_hash: await getCanvasHash(),
  screen_resolution: `${window.innerWidth}x${window.innerHeight}`,
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  is_mobile: /Mobile|Android/i.test(navigator.userAgent)
};

await fetch('https://135.235.195.173:8443/janus/fingerprint', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(fingerprint)
});
```

### Step 2: Request Challenge
```javascript
const challengeResponse = await fetch('https://135.235.195.173:8443/janus/challenge');
const challenge = await challengeResponse.json();
// challenge: { nonce, iterations, seed, type, difficulty, clientIP }
```

### Step 3: Solve Proof-of-Work
```javascript
function solvePoW(nonce, difficulty, seed, iterations) {
  let proof = 0;
  let hash;
  
  do {
    hash = sha256(`${nonce}${proof}${seed}`);
    proof++;
  } while (!hash.startsWith('0'.repeat(difficulty)) && proof < Math.pow(2, 32));
  
  return proof;
}

const proof = solvePoW(challenge.nonce, challenge.difficulty, challenge.seed, challenge.iterations);
```

### Step 4: Verify & Get Token
```javascript
const verifyResponse = await fetch('https://135.235.195.173:8443/janus/verify', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  credentials: 'include',
  body: JSON.stringify({ nonce: challenge.nonce, proof })
});

if (verifyResponse.ok) {
  // janus_token cookie is set automatically
  // Check with:
  const cookie = document.cookie.split('; ').find(c => c.startsWith('janus_token='));
  console.log('Token received:', !!cookie);
}
```

---

## Configuration

### Environment Variables (Optional)
```javascript
const API_BASE = process.env.JANUS_API_BASE || 'https://135.235.195.173:8443';
const ALLOW_INSECURE = process.env.NODE_ENV !== 'production';
```

### Timeouts & Retries
```javascript
const JANUS_CONFIG = {
  requestTimeout: 30000,  // 30 seconds
  retries: 3,
  retryDelay: 1000
};
```

---

## Error Handling

### Common Responses

| Status | Meaning | Action |
|--------|---------|--------|
| 200 | Success | Proceed |
| 400 | Bad Request | Check JSON format, required fields |
| 429 | Rate Limited | Exponential backoff, wait before retry |
| 500 | Server Error | Retry after delay, check logs |

### Example Error Handling
```javascript
try {
  const challenge = await fetch('https://135.235.195.173:8443/janus/challenge')
    .then(r => {
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return r.json();
    });
} catch (error) {
  console.error('Challenge request failed:', error);
  // Fallback: allow user to proceed with higher suspicion score
}
```

---

## Testing

### Health Check
```bash
curl -k https://135.235.195.173:8443/health
# Expected: {"status":"ok","service":"janus"}
```

### Test Full Flow
```bash
# 1. Register fingerprint
curl -k -X POST https://135.235.195.173:8443/janus/fingerprint \
  -H "Content-Type: application/json" \
  -d '{"hardware_concurrency":4,"webdriver":false}'

# 2. Request challenge
curl -k https://135.235.195.173:8443/janus/challenge

# 3. Verify (with dummy proof for testing)
curl -k -X POST https://135.235.195.173:8443/janus/verify \
  -H "Content-Type: application/json" \
  -d '{"nonce":"test","proof":"test"}'
```

---

## Monitoring

### Check API Logs
```bash
ssh djtunix@135.235.195.173 'docker logs -f janus-server'
```

### Monitor Request Volume
```bash
ssh djtunix@135.235.195.173 'docker exec janus-redis redis-cli DBSIZE'
```

### View Active Challenges
```bash
ssh djtunix@135.235.195.173 'docker exec janus-redis redis-cli keys "challenge:*" | wc -l'
```

---

## Troubleshooting

### SSL Certificate Warnings
The production deployment uses a self-signed certificate. For testing:
- Browser: Click "Advanced" → "Proceed" (or equivalent)
- cURL: Use `-k` flag
- JavaScript: Use `fetch(..., { skipCertificateValidation: true })` or equivalent

For production: Replace `cert.pem` and `key.pem` with proper certificates from Let's Encrypt or your CA.

### Rate Limiting
If you see "Rate limit exceeded":
- Default: 60 requests/minute per IP
- Wait 1 minute before retrying
- Contact admin to adjust in `config.yaml`

### Challenge Timeout
If challenge expires (5 minutes):
- Request a new challenge
- Re-solve the proof-of-work

### CORS Issues
Add these headers to your Janus API responses (nginx/reverse proxy):
```nginx
add_header 'Access-Control-Allow-Origin' 'https://your-frontend.com' always;
add_header 'Access-Control-Allow-Credentials' 'true' always;
add_header 'Access-Control-Allow-Methods' 'GET, POST, OPTIONS' always;
add_header 'Access-Control-Allow-Headers' 'Content-Type' always;
```

---

## Performance Tips

1. **Cache Fingerprints**: Store locally to avoid re-registering on every page load
2. **Parallelize**: Request challenge while user interacts with UI
3. **Progressive Enhancement**: Show UI while challenge solves in background
4. **Batching**: Send multiple requests together if possible
5. **Caching**: Use HTTP caching headers for static assets

---

## Security Best Practices

1. **Always use HTTPS**: `https://135.235.195.173:8443` (not HTTP)
2. **Store JWT**: Keep `janus_token` in secure HTTP-only cookie
3. **Validate Server**: In production, validate TLS certificate properly
4. **Rate Limiting**: Client-side: Don't spam requests; use exponential backoff
5. **Error Messages**: Don't expose internal error details in UI

---

## Support

For issues or questions:
1. Check `docker logs -f janus-server` on the VM
2. Verify health: `curl -k https://135.235.195.173:8443/health`
3. Review this guide and error handling section
4. Contact system administrator

---

**Last Updated**: April 1, 2026  
**Janus API Version**: 1.0.0  
**Backend Host**: https://135.235.195.173:8443
