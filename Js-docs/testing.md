# Testing: automated fingerprint → challenge → verify flow

This document provides two simple scripts (Bash and PowerShell) that perform an end-to-end test of the Janus challenge flow on a local development server. These scripts assume you have set `desktop_difficulty: 0` and `mobile_difficulty: 0` in `config.yaml` (or are using `config.example.yaml`) so the proof-of-work requirement is trivial for testing.

Requirements
- `curl` (or PowerShell `Invoke-RestMethod` / `Invoke-WebRequest`)
- `jq` (for the Bash script to parse JSON)
- A running Janus server at `https://localhost:8080` (self-signed certs OK)

Bash script (save as `docs/tests/run_flow.sh` and run `bash docs/tests/run_flow.sh`)

```bash
#!/usr/bin/env bash
set -euo pipefail

BASE_URL="https://localhost:8080"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if ! command -v jq >/dev/null 2>&1; then
  echo "This script requires 'jq' (https://stedolan.github.io/jq/)." >&2
  exit 1
fi

echo "1) POST fingerprint"
FP_JSON='{ "client_ip":"127.0.0.1", "canvas_hash":"test-canvas", "isMobile":false }'
HTTP_CODE=$(curl -ks -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/janus/fingerprint" -H "Content-Type: application/json" -d "$FP_JSON")
if [ "$HTTP_CODE" != "200" ]; then
  echo "Failed to POST fingerprint: HTTP $HTTP_CODE" >&2
  exit 2
fi
echo "  fingerprint stored"

echo "2) GET challenge"
CHAL_JSON=$(curl -ks "$BASE_URL/janus/challenge")
echo "  response: $CHAL_JSON"

NONCE=$(jq -r '.nonce' <<<"$CHAL_JSON")
SEED=$(jq -r '.seed' <<<"$CHAL_JSON")
CLIENTIP=$(jq -r '.clientIP' <<<"$CHAL_JSON")
ZEROBITS=$(jq -r '.zeroBits' <<<"$CHAL_JSON")

if [ -z "$NONCE" ] || [ "$NONCE" = "null" ]; then
  echo "No nonce returned by /janus/challenge" >&2
  exit 3
fi

echo "3) Craft proof (difficulty $ZEROBITS)"
TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Use iteration 1 and the stored canvas hash "test-canvas" for desktop PoR
PROOF="$NONCE|1|$TS|$CLIENTIP|$SEED|test-canvas"

echo "4) POST verify"
VERIFY_RESP_HEADERS="$TMP_DIR/headers.txt"
VERIFY_BODY="$TMP_DIR/body.txt"
curl -ks -D "$VERIFY_RESP_HEADERS" -o "$VERIFY_BODY" -X POST "$BASE_URL/janus/verify" \
  -H "Content-Type: application/json" \
  -d "{ \"nonce\": \"$NONCE\", \"proof\": \"$PROOF\" }"

STATUS=$(awk 'NR==1{print $2}' "$VERIFY_RESP_HEADERS")
echo "  verify status: $STATUS"
if [ "$STATUS" != "200" ]; then
  echo "Verification failed. Response body:" >&2
  cat "$VERIFY_BODY" >&2
  exit 4
fi

echo "5) Check for janus_token cookie"
if grep -qi "Set-Cookie:.*janus_token" "$VERIFY_RESP_HEADERS"; then
  echo "SUCCESS: janus_token cookie set"
  grep -i "Set-Cookie:.*janus_token" "$VERIFY_RESP_HEADERS" | sed -n 's/\r$//p'
  exit 0
else
  echo "Verification succeeded but janus_token cookie not found in headers" >&2
  exit 5
fi
```

PowerShell script (save as `docs/tests/run_flow.ps1` and run in PowerShell Core or Windows PowerShell as administrator):

```powershell
# Allow self-signed certs for this script session
[System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }

$baseUrl = 'https://localhost:8080'

Write-Host "1) POST fingerprint"
$fp = @{ client_ip = '127.0.0.1'; canvas_hash = 'test-canvas'; isMobile = $false } | ConvertTo-Json
$resp = Invoke-RestMethod -Uri "$baseUrl/janus/fingerprint" -Method Post -Body $fp -ContentType 'application/json'
Write-Host "  fingerprint posted"

Write-Host "2) GET challenge"
$chal = Invoke-RestMethod -Uri "$baseUrl/janus/challenge" -Method Get
Write-Host "  challenge: $($chal | ConvertTo-Json -Depth 3)"

$nonce = $chal.nonce
$seed = $chal.seed
$clientIP = $chal.clientIP
$zeroBits = $chal.zeroBits

if (-not $nonce) { Write-Error "No nonce returned"; exit 2 }

Write-Host "3) Craft proof (difficulty $zeroBits)"
$ts = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$proof = "$nonce|1|$ts|$clientIP|$seed|test-canvas"

Write-Host "4) POST verify"
$body = @{ nonce = $nonce; proof = $proof } | ConvertTo-Json
$verifyResp = Invoke-WebRequest -Uri "$baseUrl/janus/verify" -Method Post -Body $body -ContentType 'application/json'

if ($verifyResp.StatusCode -ne 200) {
  Write-Error "Verification failed: $($verifyResp.StatusCode)"
  Write-Error $verifyResp.Content
  exit 3
}

Write-Host "5) Check for janus_token cookie"
$cookies = $verifyResp.Headers['Set-Cookie']
if ($cookies -match 'janus_token') {
  Write-Host "SUCCESS: janus_token cookie set:`n$cookies"
  exit 0
} else {
  Write-Error "Verification succeeded but janus_token cookie not found"
  exit 4
}
```

Usage notes
- Ensure the server is running at `https://localhost:8080` and that the `config.yaml` difficulty values are set to `0` for easy testing.
- The Bash script requires `jq` to parse JSON. Install it from your OS package manager if missing.
- Both scripts assume the server uses `clientIP` in the challenge response; they use that IP value when building the proof.

If you'd like, I can add a small `Makefile` or NPM-style script to run the preferred test for CI or local usage.
