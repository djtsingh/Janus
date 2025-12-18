#!/bin/bash
# Test script for Janus middleware: full flow with logging
set -e
BASE_URL="https://localhost:8080"
COOKIE_JAR="janus_cookies.txt"
LOG_FILE="janus_test_log.txt"

rm -f "$COOKIE_JAR" "$LOG_FILE"

log() {
  echo -e "[$(date +'%H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "1. Submitting fingerprint (simulating desktop user)"
FINGERPRINT='{"client_ip":"127.0.0.1","plugins":"TestPlugin","hardware_concurrency":4,"webdriver":false,"chrome_exists":true,"canvas_hash":"test-canvas","screen_resolution":"1920x1080","color_depth":24,"fonts":"Arial,Times New Roman","webgl_renderer":"TestRenderer","ja3":"testja3","screen":{"width":1920,"height":1080},"timezone":"UTC","jsEnabled":true,"isMobile":false}'

curl -k -c "$COOKIE_JAR" -X POST "$BASE_URL/janus/fingerprint" \
  -H "Content-Type: application/json" \
  -d "$FINGERPRINT" | tee -a "$LOG_FILE"
log "2. Requesting challenge"
CHAL_JSON=$(curl -k -b "$COOKIE_JAR" "$BASE_URL/janus/challenge")
log "Challenge response: $CHAL_JSON"

CHAL_TYPE=$(echo "$CHAL_JSON" | grep -o '"type":"[^"]*"' | cut -d'"' -f4)
CHAL_NONCE=$(echo "$CHAL_JSON" | grep -o '"nonce":"[^"]*"' | cut -d'"' -f4)
CHAL_DIFF=$(echo "$CHAL_JSON" | grep -o '"difficulty":[0-9]*' | cut -d':' -f2)

if [[ "$CHAL_TYPE" == "pow" ]]; then
  log "3. Solving PoW challenge (difficulty $CHAL_DIFF)"
  # Simulate proof (difficulty 0 for test)
  PROOF="$CHAL_NONCE|1|$(date -Iseconds)|127.0.0.1|test-seed|test-canvas"
  log "Proof: $PROOF"
  log "4. Submitting proof"
  curl -k -b "$COOKIE_JAR" -X POST "$BASE_URL/janus/verify" \
    -H "Content-Type: application/json" \
    -d "{\"nonce\":\"$CHAL_NONCE\",\"proof\":\"$PROOF\"}" | tee -a "$LOG_FILE"
elif [[ "$CHAL_TYPE" == "image" ]]; then
  log "3. Simulating image puzzle solve"
  log "4. Submitting image proof"
  curl -k -b "$COOKIE_JAR" -X POST "$BASE_URL/janus/verify" \
    -H "Content-Type: application/json" \
    -d "{\"nonce\":\"$CHAL_NONCE\",\"proof\":\"image-solved\"}" | tee -a "$LOG_FILE"
elif [[ "$CHAL_TYPE" == "logic" ]]; then
  log "3. Simulating logic question answer"
  log "4. Submitting logic proof"
  curl -k -b "$COOKIE_JAR" -X POST "$BASE_URL/janus/verify" \
    -H "Content-Type: application/json" \
    -d "{\"nonce\":\"$CHAL_NONCE\",\"proof\":\"logic-4\"}" | tee -a "$LOG_FILE"
else
  log "3. Invisible challenge or unknown type, submitting dummy proof"
  curl -k -b "$COOKIE_JAR" -X POST "$BASE_URL/janus/verify" \
    -H "Content-Type: application/json" \
    -d "{\"nonce\":\"$CHAL_NONCE\",\"proof\":\"dummy-proof\"}" | tee -a "$LOG_FILE"
fi

log "5. Checking for janus_token cookie (should be set if successful)"
grep janus_token "$COOKIE_JAR" && log "janus_token present: PASS" || log "janus_token missing: FAIL"

log "Test complete. See $LOG_FILE for full details."
