package middleware

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"janus/internal/challenge"
	"janus/internal/config"
	"janus/internal/handlers"
	"janus/internal/store"
	"janus/internal/types"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oschwald/geoip2-golang/v2"
)

type contextKey string

const (
	ja3ContextKey contextKey = "ja3"
)

type ChallengeStore struct {
	sync.RWMutex
	data map[string]struct {
		Challenge *types.Challenge
		Expires   time.Time
	}
}

// OffenderRecord tracks repeat suspicious behavior for tarpit escalation
type OffenderRecord struct {
	Attempts   int       // number of suspicious requests
	FirstSeen  time.Time // when first flagged
	LastSeen   time.Time // most recent suspicious request
	TotalScore int       // cumulative suspicion score
	LastScore  int       // most recent suspicion score
}

// OffenderStore tracks IPs with suspicious behavior for tarpit escalation
type OffenderStore struct {
	sync.RWMutex
	data map[string]*OffenderRecord
}

var (
	fingerprintStore = &types.FingerprintStore{Data: make(map[string]types.Fingerprint)}

	challengeStore = &ChallengeStore{data: make(map[string]struct {
		Challenge *types.Challenge
		Expires   time.Time
	})}

	// offenderStore tracks repeat offenders for progressive penalties
	offenderStore = &OffenderStore{data: make(map[string]*OffenderRecord)}
	loadedConfig  *config.JanusConfig
	configOnce    sync.Once
	jwtSecret     = []byte("your-secure-random-secret-key-32bytes")
	geoDB         *geoip2.Reader
)

var janusRouter *chi.Mux

func init() {
	janusRouter = chi.NewRouter()
	janusRouter.Post("/janus/fingerprint", handlers.HandleFingerprint(fingerprintStore))
	janusRouter.Get("/janus/challenge", handleChallenge)
	janusRouter.Post("/janus/verify", handleVerify)
}

func init() {
	var err error
	geoDB, err = geoip2.Open("GeoLite2-City.mmdb")
	if err != nil {
		log.Printf("init: GeoIP database load error: %v, geo checks disabled", err)
	}

	go func() {
		for {
			time.Sleep(1 * time.Minute)
			// Cleanup expired challenges
			challengeStore.Lock()
			for key, stored := range challengeStore.data {
				if time.Now().After(stored.Expires) {
					delete(challengeStore.data, key)
				}
			}
			challengeStore.Unlock()

			// Cleanup old offender records
			if loadedConfig != nil && loadedConfig.Tarpit.Enabled {
				offenderStore.Lock()
				expiry := time.Duration(loadedConfig.Tarpit.OffenderMemoryMins) * time.Minute
				for ip, record := range offenderStore.data {
					if time.Since(record.LastSeen) > expiry {
						log.Printf("Tarpit: Clearing offender record for %s (expired after %v)", ip, expiry)
						delete(offenderStore.data, ip)
					}
				}
				offenderStore.Unlock()
			}
		}
	}()
}

func JanusMiddleware(next http.Handler) http.Handler {
	configOnce.Do(func() {
		var err error
		loadedConfig, err = config.LoadConfig("config.yaml")
		if err != nil {
			log.Printf("Failed to load config: %v, using default config", err)
			loadedConfig = config.DefaultConfig()
		}
	})

	redisAddr := loadedConfig.RedisAddr
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisStore := store.New(redisAddr)
	rateLimit := loadedConfig.RateLimit.RequestsPerMinute
	if rateLimit == 0 {
		rateLimit = 60
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		log.Printf("Request: %s, Method: %s, IP: %s, UA: %s", r.URL.Path, r.Method, clientIP, r.Header.Get("User-Agent"))

		if strings.HasPrefix(r.URL.Path, "/janus/") {
			log.Printf("Serving Janus API endpoint: %s", r.URL.Path)
			janusRouter.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/sensor.js" {
			log.Printf("Serving sensor.js asset")
			http.ServeFile(w, r, "assets/sensor.js")
			return
		}

		limited, err := redisStore.IsRateLimited(clientIP, rateLimit)
		if err != nil {
			// Redis unavailable - fail open (allow request) for graceful degradation
			log.Printf("Redis rate limit unavailable for %s: %v (allowing request)", clientIP, err)
		} else if limited {
			log.Printf("Rate limit exceeded for %s", clientIP)
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		if isVerified(r) {
			log.Printf("Serving content for verified user %s", clientIP)
			next.ServeHTTP(w, r)
			return
		}

		suspicious, score := isSuspicious(r, loadedConfig)
		log.Printf("Unverified user. Suspicious: %v, Score: %d. Issuing challenge.", suspicious, score)
		issueChallenge(w, r)
	})
}

func getClientIP(r *http.Request) string {
	log.Printf("getClientIP: X-Forwarded-For: %s, RemoteAddr: %s", r.Header.Get("X-Forwarded-For"), r.RemoteAddr)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip := strings.Split(forwarded, ",")[0]
		ip = strings.TrimSpace(ip)
		if ip != "" && ip != "[" {
			return ip
		}
	}
	addr := r.RemoteAddr
	if strings.HasPrefix(addr, "[") {
		end := strings.LastIndex(addr, "]")
		if end != -1 {
			ip := addr[1:end]
			if ip != "" {
				return ip
			}
		}
	} else {
		parts := strings.Split(addr, ":")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	return "unknown"
}

func getJA3Fingerprint(r *http.Request) string {
	if r.TLS == nil {
		log.Printf("getJA3Fingerprint: No TLS data for %s", r.RemoteAddr)
		return "no-tls"
	}
	var parts []string
	parts = append(parts, strconv.Itoa(int(r.TLS.Version)))
	var ciphers []string
	ciphers = append(ciphers, strconv.Itoa(int(r.TLS.CipherSuite)))
	parts = append(parts, strings.Join(ciphers, "-"))
	parts = append(parts, "0-23-65281-10-11-35-16-5-13-18-51-45-43-27")
	parts = append(parts, "29-23-24")
	parts = append(parts, "0")
	ja3String := strings.Join(parts, ",")
	ja3Hash := fmt.Sprintf("%x", md5.Sum([]byte(ja3String)))
	log.Printf("getJA3Fingerprint: Generated fingerprint %s for %s", ja3Hash, r.RemoteAddr)
	return ja3Hash
}

func isKnownBrowserJA3(ja3 string) bool {
	known := []string{
		"b2fa5d224d65e7c692fd46a0f52fce6b",
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		"771,49195-49199-52393-52392-49196-49200-49161-49162-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27,29-23-24,0",
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-34-13-18-51-45-43-27-17513,29-23-24,0",
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24-25,0",
	}
	for _, k := range known {
		if ja3 == k {
			log.Printf("isKnownBrowserJA3: Matched JA3 %s", ja3)
			return true
		}
	}
	log.Printf("isKnownBrowserJA3: No match for JA3 %s", ja3)
	return false
}

func getHeaderOrder(r *http.Request) string {
	headers := []string{}
	for name := range r.Header {
		headers = append(headers, strings.ToLower(name))
	}
	sort.Strings(headers)
	return strings.Join(headers, ",")
}

func isSuspicious(r *http.Request, cfg *config.JanusConfig) (bool, int) {
	ua := r.Header.Get("User-Agent")
	clientIP := getClientIP(r)
	score := 0

	uaLower := strings.ToLower(ua)
	uaWhitelisted := false
	for _, allowed := range cfg.WhitelistUA {
		allowedLower := strings.ToLower(allowed)
		log.Printf("isSuspicious: Checking UA %s against whitelist %s", uaLower, allowedLower)
		if strings.Contains(uaLower, allowedLower) {
			uaWhitelisted = true
			break
		}
	}
	ipWhitelisted := false
	for _, ip := range cfg.WhitelistIPs {
		log.Printf("isSuspicious: Checking IP %s against whitelist %s", clientIP, ip)
		if clientIP == ip {
			ipWhitelisted = true
			break
		}
	}
	if uaWhitelisted && ipWhitelisted {
		log.Printf("isSuspicious: Whitelisted UA %s and IP %s, bypassing checks", ua, clientIP)
		return false, 0
	}

	for _, blacklistedIP := range cfg.BlacklistedIPs {
		if strings.HasPrefix(blacklistedIP, clientIP) || strings.Contains(blacklistedIP, "/") {
			_, ipNet, err := net.ParseCIDR(blacklistedIP)
			if err == nil && ipNet.Contains(net.ParseIP(clientIP)) {
				score += cfg.SuspicionWeights["blacklisted_ip"]
				log.Printf("isSuspicious: Blacklisted IP %s, Score: %d", clientIP, score)
				return true, score
			}
			if blacklistedIP == clientIP {
				score += cfg.SuspicionWeights["blacklisted_ip"]
				log.Printf("isSuspicious: Blacklisted IP %s, Score: %d", clientIP, score)
				return true, score
			}
		}
	}
	if geoDB != nil {
		ipAddr, err := netip.ParseAddr(clientIP)
		if err == nil {
			record, err := geoDB.City(ipAddr)
			if err == nil {
				geoCode := record.Country.ISOCode
				for _, bannedGeo := range cfg.BannedGeoLocations {
					if geoCode == bannedGeo {
						score += cfg.SuspicionWeights["banned_geo"]
						log.Printf("isSuspicious: Banned geo %s for IP %s, Score: %d", geoCode, clientIP, score)
						return true, score
					}
				}
			} else {
				log.Printf("isSuspicious: GeoIP lookup failed for %s: %v", clientIP, err)
			}
		} else {
			log.Printf("isSuspicious: Invalid IP %s: %v", clientIP, err)
		}
	} else {
		log.Printf("isSuspicious: GeoIP database not loaded, skipping geo checks for %s", clientIP)
	}

	ja3Fingerprint := getJA3Fingerprint(r)
	if ja3Fingerprint != "" && !isKnownBrowserJA3(ja3Fingerprint) {
		score += cfg.SuspicionWeights["tls_mismatch"]
		log.Printf("isSuspicious: Suspicious JA3 %s for IP %s, Score: %d", ja3Fingerprint, clientIP, score)
	}

	if ja3Fingerprint != "" && ja3Fingerprint != "no-tls" && ja3Fingerprint != "unknown-ja3" {
		if strings.Contains(uaLower, "firefox") && !strings.Contains(ja3Fingerprint, "49195") {
			score += cfg.SuspicionWeights["tls_mismatch"]
			log.Printf("isSuspicious: JA3 mismatch with UA %s for IP %s, JA3: %s, Score: %d", ua, clientIP, ja3Fingerprint, score)
		}
	}

	if ua == "" || strings.Contains(uaLower, "curl") || strings.Contains(uaLower, "python") {
		score += cfg.SuspicionWeights["no_user_agent"]
		log.Printf("isSuspicious: Suspicious UA %s for IP %s, Score: %d", ua, clientIP, score)
	}
	if strings.Contains(uaLower, "headless") {
		score += cfg.SuspicionWeights["headless_browser"]
		log.Printf("isSuspicious: Headless browser detected for IP %s, Score: %d", clientIP, score)
	}
	if r.Header.Get("Accept") == "" && !strings.Contains(r.URL.Path, ".well-known") {
		score += cfg.SuspicionWeights["missing_headers"]
		log.Printf("isSuspicious: Missing Accept header for IP %s, Score: %d", clientIP, score)
	}
	headerOrder := getHeaderOrder(r)
	expectedHeaders := []string{"user-agent", "accept-language", "accept-encoding"}
	headersPresent := true
	for _, h := range expectedHeaders {
		if r.Header.Get(h) == "" {
			headersPresent = false
			break
		}
	}
	if !headersPresent && !strings.Contains(r.URL.Path, ".well-known") {
		score += cfg.SuspicionWeights["header_order_mismatch"]
		log.Printf("isSuspicious: Missing expected headers for IP %s, Score: %d", clientIP, score)
	}

	fingerprintStore.RLock()
	fp, hasFingerprint := fingerprintStore.Data[clientIP]
	fingerprintStore.RUnlock()
	if !hasFingerprint {
		score += cfg.SuspicionWeights["no_fingerprint"]
		log.Printf("isSuspicious: No fingerprint for IP %s, Score: %d", clientIP, score)
	} else {
		if fp.Webdriver {
			score += cfg.SuspicionWeights["headless_browser"]
			log.Printf("isSuspicious: Webdriver detected for IP %s, Score: %d", clientIP, score)
		}
		if !fp.ChromeExists && strings.Contains(uaLower, "chrome") {
			score += cfg.SuspicionWeights["headless_browser"]
			log.Printf("isSuspicious: Chrome UA but no window.chrome for IP %s, Score: %d", clientIP, score)
		}
		if fp.CanvasHash == "error" || fp.CanvasHash == "" {
			score += cfg.SuspicionWeights["no_fingerprint"]
			log.Printf("isSuspicious: Invalid canvas hash for IP %s, Score: %d", clientIP, score)
		}
		if fp.WebGLRenderer == "no-webgl" || fp.WebGLRenderer == "error" {
			score += cfg.SuspicionWeights["no_fingerprint"]
			log.Printf("isSuspicious: Invalid WebGL renderer for IP %s, Score: %d", clientIP, score)
		}
	}

	suspicious := score >= cfg.SuspicionThreshold
	log.Printf("isSuspicious: UA %s, IP %s, HasFingerprint: %v, JA3: %s, Headers: %s, Webdriver: %v, ChromeExists: %v, Score: %d, Suspicious: %v",
		ua, clientIP, hasFingerprint, ja3Fingerprint, headerOrder, fp.Webdriver, fp.ChromeExists, score, suspicious)

	ctx := context.WithValue(r.Context(), ja3ContextKey, ja3Fingerprint)
	*r = *r.WithContext(ctx)

	return suspicious, score
}

func isVerified(r *http.Request) bool {
	cookie, err := r.Cookie("janus_token")
	if err != nil {
		log.Printf("isVerified: No janus_token cookie for IP %s: %v", getClientIP(r), err)
		return false
	}
	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		log.Printf("isVerified: Token parsing failed for IP %s: %v", getClientIP(r), err)
		return false
	}
	if !token.Valid {
		log.Printf("isVerified: Invalid token for IP %s", getClientIP(r))
		return false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Printf("isVerified: Invalid claims format for IP %s", getClientIP(r))
		return false
	}
	clientIP := getClientIP(r)
	if claimIP, ok := claims["ip"].(string); !ok || claimIP != clientIP {
		log.Printf("isVerified: IP mismatch for IP %s, token IP: %v", clientIP, claims["ip"])
		return false
	}
	log.Printf("isVerified: Valid token for IP %s", clientIP)
	return true
}

func handleChallenge(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	fingerprintStore.RLock()
	fp, hasFingerprint := fingerprintStore.Data[clientIP]
	fingerprintStore.RUnlock()
	if !hasFingerprint {
		log.Printf("handleChallenge: No fingerprint for IP %s", clientIP)
		http.Error(w, "No fingerprint", http.StatusBadRequest)
		return
	}

	userHistory := 0
	suspicious, riskScore := isSuspicious(r, loadedConfig)

	// Track offenders and apply tarpit measures
	var offender *OffenderRecord
	if suspicious && loadedConfig.Tarpit.Enabled {
		offender = recordOffender(clientIP, riskScore)
		log.Printf("handleChallenge: User %s is suspicious, risk score %d, offense #%d",
			clientIP, riskScore, offender.Attempts)

		// Apply server-side delay (tarpit)
		applyTarpitDelay(loadedConfig, clientIP, riskScore, offender)
	} else if suspicious {
		log.Printf("handleChallenge: User %s is suspicious, risk score %d", clientIP, riskScore)
	} else {
		// Check if this IP has prior offenses even if current request isn't suspicious
		offender = getOffenderRecord(clientIP)
	}

	// Generate challenge with adaptive difficulty
	chal, _ := challenge.GenerateChallenge(loadedConfig, fp.IsMobile, riskScore, userHistory)
	if chal == nil {
		log.Printf("handleChallenge: Failed to generate challenge for IP %s", clientIP)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Override difficulty with tarpit-aware calculation
	if loadedConfig.Tarpit.Enabled {
		baseDifficulty := loadedConfig.DesktopDifficulty
		if fp.IsMobile {
			baseDifficulty = loadedConfig.MobileDifficulty
		}
		chal.Difficulty = calculateAdaptiveDifficulty(loadedConfig, baseDifficulty, riskScore, offender)
		log.Printf("Tarpit: Adjusted difficulty for %s from base %d to %d (risk: %d)",
			clientIP, baseDifficulty, chal.Difficulty, riskScore)
	}

	challengeStore.Lock()
	challengeStore.data[clientIP+chal.Nonce] = struct {
		Challenge *types.Challenge
		Expires   time.Time
	}{Challenge: chal, Expires: time.Now().Add(5 * time.Minute)}
	challengeStore.Unlock()

	response := map[string]interface{}{
		"nonce":      chal.Nonce,
		"iterations": chal.Iterations,
		"seed":       chal.Seed,
		"clientIP":   clientIP,
		"type":       chal.Type,
		"difficulty": chal.Difficulty,
	}
	log.Printf("handleChallenge: Issued challenge for IP %s, nonce %s, type %s, difficulty %d", clientIP, chal.Nonce, chal.Type, chal.Difficulty)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("handleChallenge: Failed to encode response for IP %s: %v", clientIP, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func handleVerify(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	var req struct {
		Nonce string `json:"nonce"`
		Proof string `json:"proof"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("handleVerify: Invalid request body for IP %s: %v", clientIP, err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	fingerprintStore.RLock()
	fp, hasFingerprint := fingerprintStore.Data[clientIP]
	fingerprintStore.RUnlock()
	if !hasFingerprint {
		log.Printf("handleVerify: No fingerprint for IP %s", clientIP)
		http.Error(w, "No fingerprint", http.StatusBadRequest)
		return
	}

	challengeStore.RLock()
	stored, exists := challengeStore.data[clientIP+req.Nonce]
	challengeStore.RUnlock()
	if !exists || time.Now().After(stored.Expires) {
		log.Printf("handleVerify: No valid challenge for IP %s, nonce %s", clientIP, req.Nonce)
		http.Error(w, "No valid challenge", http.StatusBadRequest)
		return
	}

	// Handle different challenge types
	verified := false
	switch stored.Challenge.Type {
	case "image":
		// Image challenge: user clicked the correct image
		if req.Proof == "image-solved" {
			verified = true
			log.Printf("handleVerify: Image challenge solved for IP %s", clientIP)
		}
	case "logic":
		// Logic challenge: user answered correctly
		if req.Proof == "logic-4" {
			verified = true
			log.Printf("handleVerify: Logic challenge solved for IP %s", clientIP)
		}
	default:
		// PoW challenge: verify computational proof
		verified = challenge.VerifyChallenge(req.Proof, req.Nonce, clientIP, stored.Challenge.Seed, fp.IsMobile, fp.CanvasHash, loadedConfig)
	}

	if !verified {
		log.Printf("handleVerify: Proof verification failed for IP %s, nonce %s, proof %s, type %s", clientIP, req.Nonce, req.Proof, stored.Challenge.Type)
		http.Error(w, "Verification failed", http.StatusUnauthorized)
		return
	}

	challengeStore.Lock()
	delete(challengeStore.data, clientIP+req.Nonce)
	challengeStore.Unlock()

	// Graceful tarpit: reduce offender record on successful verification
	// This rewards legitimate users who solve challenges
	if loadedConfig.Tarpit.Enabled {
		reduceOffenderScore(clientIP)
	}

	log.Printf("handleVerify: Proof verified for IP %s, nonce %s", clientIP, req.Nonce)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"ip":  clientIP,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("handleVerify: Failed to generate token for IP %s: %v", clientIP, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "janus_token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   24 * 60 * 60,
	})

	log.Printf("handleVerify: Issued token for IP %s", clientIP)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.Printf("handleVerify: Failed to encode response for IP %s: %v", clientIP, err)
	}
}

func issueChallenge(w http.ResponseWriter, r *http.Request) {
	log.Printf("issueChallenge: Serving HTML challenge page for %s", getClientIP(r))
	http.ServeFile(w, r, "assets/challenge.html")
}

// ============================================================================
// TARPIT FUNCTIONS - Adaptive bot mitigation through delays and difficulty
// ============================================================================

// recordOffender tracks suspicious behavior for progressive penalties
func recordOffender(clientIP string, riskScore int) *OffenderRecord {
	offenderStore.Lock()
	defer offenderStore.Unlock()

	record, exists := offenderStore.data[clientIP]
	if !exists {
		record = &OffenderRecord{
			Attempts:   0,
			FirstSeen:  time.Now(),
			TotalScore: 0,
		}
		offenderStore.data[clientIP] = record
	}

	record.Attempts++
	record.LastSeen = time.Now()
	record.TotalScore += riskScore
	record.LastScore = riskScore

	log.Printf("Tarpit: Recorded offense for %s - attempts: %d, total_score: %d, last_score: %d",
		clientIP, record.Attempts, record.TotalScore, record.LastScore)

	return record
}

// getOffenderRecord retrieves the offender record without modifying it
func getOffenderRecord(clientIP string) *OffenderRecord {
	offenderStore.RLock()
	defer offenderStore.RUnlock()
	return offenderStore.data[clientIP]
}

// calculateTarpitDelay determines how long to delay response based on suspicion
func calculateTarpitDelay(cfg *config.JanusConfig, riskScore int, offender *OffenderRecord) time.Duration {
	if !cfg.Tarpit.Enabled {
		return 0
	}

	// Base delay from risk score
	delayMs := riskScore * cfg.Tarpit.DelayPerScoreMs

	// Add delay for repeat offenders (exponential backoff)
	if offender != nil && offender.Attempts > 1 {
		repeatDelay := (offender.Attempts - 1) * (offender.Attempts - 1) * 500 // quadratic growth
		delayMs += repeatDelay
	}

	// Cap the delay
	if delayMs > cfg.Tarpit.MaxDelayMs {
		delayMs = cfg.Tarpit.MaxDelayMs
	}

	return time.Duration(delayMs) * time.Millisecond
}

// calculateAdaptiveDifficulty computes PoW difficulty based on risk assessment
func calculateAdaptiveDifficulty(cfg *config.JanusConfig, baseDifficulty, riskScore int, offender *OffenderRecord) int {
	if !cfg.Tarpit.Enabled {
		// Fallback to original simple logic
		if riskScore > 80 {
			return baseDifficulty + 2
		}
		return baseDifficulty
	}

	difficulty := baseDifficulty

	// Risk tier scaling
	switch {
	case riskScore >= 80:
		difficulty += cfg.Tarpit.DifficultyMultiplier * 4 // Extreme: +16 default
	case riskScore >= 50:
		difficulty += cfg.Tarpit.DifficultyMultiplier * 3 // High: +12 default
	case riskScore >= 30:
		difficulty += cfg.Tarpit.DifficultyMultiplier * 2 // Medium: +8 default
	case riskScore >= 10:
		difficulty += cfg.Tarpit.DifficultyMultiplier // Low: +4 default
	}

	// Repeat offender penalty
	if offender != nil && offender.Attempts > 1 {
		penalty := (offender.Attempts - 1) * cfg.Tarpit.RepeatPenalty
		if penalty > 10 {
			penalty = 10 // cap repeat penalty at +10
		}
		difficulty += penalty
	}

	// Absolute cap to prevent infinite computation
	maxDifficulty := 28 // ~30+ seconds even on fast hardware
	if difficulty > maxDifficulty {
		difficulty = maxDifficulty
	}

	return difficulty
}

// applyTarpitDelay blocks the goroutine for the calculated delay
func applyTarpitDelay(cfg *config.JanusConfig, clientIP string, riskScore int, offender *OffenderRecord) {
	delay := calculateTarpitDelay(cfg, riskScore, offender)
	if delay > 0 {
		log.Printf("Tarpit: Delaying response to %s by %v (risk: %d, attempts: %d)",
			clientIP, delay, riskScore, func() int {
				if offender != nil {
					return offender.Attempts
				}
				return 0
			}())
		time.Sleep(delay)
	}
}

// reduceOffenderScore gracefully reduces penalties for users who successfully verify
// This rewards legitimate users who may have been falsely flagged
func reduceOffenderScore(clientIP string) {
	offenderStore.Lock()
	defer offenderStore.Unlock()

	record, exists := offenderStore.data[clientIP]
	if !exists {
		return
	}

	// Halve the attempt count on successful verification (forgiveness)
	record.Attempts = record.Attempts / 2
	record.TotalScore = record.TotalScore / 2

	// If they're back to clean slate, remove the record entirely
	if record.Attempts <= 0 {
		delete(offenderStore.data, clientIP)
		log.Printf("Tarpit: Cleared offender record for %s after successful verification", clientIP)
	} else {
		log.Printf("Tarpit: Reduced offender score for %s - attempts now: %d, total_score: %d",
			clientIP, record.Attempts, record.TotalScore)
	}
}
