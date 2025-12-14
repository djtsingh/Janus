#!/usr/bin/env bash
set -euo pipefail

BASE_URL="https://localhost:8080"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if ! command -v jq >/dev/null 2>&1; then
  echo "This script requires 'jq'." >&2
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
