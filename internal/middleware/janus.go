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
	"os"
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

// Redis-backed stores are used for challenges and offenders; fingerprints remain in-memory.
var redisStoreGlobal *store.Store

var (
	fingerprintStore = &types.FingerprintStore{Data: make(map[string]types.Fingerprint)}
	loadedConfig     *config.JanusConfig
	configOnce       sync.Once
	jwtSecret        = []byte("your-secure-random-secret-key-32bytes")
	geoDB            *geoip2.Reader
)

var janusRouter *chi.Mux

func init() {
	janusRouter = chi.NewRouter()
	janusRouter.Post("/janus/fingerprint", handlers.HandleFingerprint(fingerprintStore))
	janusRouter.Get("/janus/challenge", handleChallenge)
	janusRouter.Post("/janus/verify", handleVerify)
	janusRouter.Get("/janus/interactive-challenge", handleInteractiveChallenge)
	janusRouter.Post("/janus/verify-interactive", handleVerifyInteractive)
}

func init() {
	var err error
	geoDB, err = geoip2.Open("GeoLite2-City.mmdb")
	if err != nil {
		log.Printf("init: GeoIP database load error: %v, geo checks disabled", err)
	}
	// With Redis-backed challenges and offenders, no in-process cleanup required.
}

func JanusMiddleware(next http.Handler) http.Handler {
	configOnce.Do(func() {
		var err error
		loadedConfig, err = config.LoadConfig("config.yaml")
		if err != nil {
			log.Printf("Failed to load config: %v, using default config", err)
			loadedConfig = config.DefaultConfig()
		}
		// Allow overriding JWT secret via environment for production
		if s := os.Getenv("JANUS_JWT_SECRET"); s != "" {
			jwtSecret = []byte(s)
			log.Printf("JanusMiddleware: Using JWT secret from environment")
		}
	})

	// Prefer REDIS_ADDR env var if provided, otherwise use config.yaml value, fallback to localhost
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = loadedConfig.RedisAddr
	}
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisStore := store.New(redisAddr)
	// expose redis store to package-level handlers
	redisStoreGlobal = redisStore
	rateLimit := loadedConfig.RateLimit.RequestsPerMinute
	if rateLimit == 0 {
		rateLimit = 60
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		log.Printf("Request: %s, Method: %s, IP: %s, UA: %s", r.URL.Path, r.Method, clientIP, r.Header.Get("User-Agent"))

		// health endpoint bypasses verification
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","service":"janus"}`))
			return
		}

		// favicon bypasses verification so browsers can fetch it without a token
		if r.URL.Path == "/favicon.svg" || r.URL.Path == "/favicon.ico" {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/janus/") {
			log.Printf("Serving Janus API endpoint: %s", r.URL.Path)
			janusRouter.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/verify-ui" {
			log.Printf("Serving verification UI asset")
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			http.ServeFile(w, r, "assets/verification-ui.html")
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

		// Blacklisted IPs always get challenged, even with a valid token.
		if isIPBlacklisted(clientIP, loadedConfig) {
			log.Printf("Blacklisted IP %s - forcing challenge", clientIP)
			issueChallenge(w, r)
			return
		}

		// challenge_all: force a fresh challenge on every request, token or not.
		if !loadedConfig.ChallengeAll && isVerified(r) {
			log.Printf("Serving content for verified user %s", clientIP)
			next.ServeHTTP(w, r)
			return
		}

		suspicious, score := isSuspicious(r, loadedConfig)
		if loadedConfig.ChallengeAll {
			log.Printf("challenge_all=true: challenging %s regardless of token (score: %d)", clientIP, score)
		} else {
			log.Printf("Unverified user. Suspicious: %v, Score: %d. Issuing challenge.", suspicious, score)
		}
		issueChallenge(w, r)
	})
}

// isIPBlacklisted returns true if clientIP exactly matches any entry in the blacklist.
func isIPBlacklisted(ip string, cfg *config.JanusConfig) bool {
	for _, entry := range cfg.BlacklistedIPs {
		if entry == ip {
			return true
		}
		// CIDR range check
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err == nil && ipNet.Contains(net.ParseIP(ip)) {
				return true
			}
		}
	}
	return false
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
		if fp.BotScore > 0 {
			addition := fp.BotScore
			if addition > 50 {
				addition = 50
			}
			score += addition
			log.Printf("isSuspicious: Client bot score %d for IP %s, Score: %d", fp.BotScore, clientIP, score)
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
	requestStart := time.Now() // record before any tarpit delay
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
	var offender *types.OffenderRecord
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

		// Scale iterations so the challenge is always solvable.
		// Need ~2^difficulty attempts on average; provide 4× margin.
		if chal.Difficulty > baseDifficulty {
			minIter := 1 << uint(chal.Difficulty+2) // 4× expected
			if chal.Iterations < minIter {
				chal.Iterations = minIter
			}
			if chal.Iterations > 500000 {
				chal.Iterations = 500000
			}
		}
	}

	// Backdate IssuedAt to before the tarpit delay so timing checks are accurate
	chal.IssuedAt = requestStart

	// Persist challenge in Redis with 5 minute TTL so it survives restarts and supports multiple instances
	if err := redisStoreGlobal.SetChallenge(clientIP, chal.Nonce, chal, 5*time.Minute); err != nil {
		log.Printf("handleChallenge: Failed to persist challenge for IP %s: %v", clientIP, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

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
		Nonce string               `json:"nonce"`
		Proof string               `json:"proof"`
		B     types.BehavioralData `json:"b"`
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

	var storedChallenge types.Challenge
	exists, err := redisStoreGlobal.GetChallenge(clientIP, req.Nonce, &storedChallenge)
	if err != nil {
		log.Printf("handleVerify: Redis error fetching challenge for IP %s, nonce %s: %v", clientIP, req.Nonce, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !exists {
		log.Printf("handleVerify: No valid challenge for IP %s, nonce %s", clientIP, req.Nonce)
		http.Error(w, "No valid challenge", http.StatusBadRequest)
		return
	}

	// Timing enforcement: reject proofs submitted impossibly fast (bypass detection).
	// Threshold is intentionally low — the tarpit already enforces wait times for
	// suspicious users. This guard only catches submissions that somehow skip PoW
	// entirely and arrive in under 200ms.
	if !storedChallenge.IssuedAt.IsZero() {
		elapsed := time.Since(storedChallenge.IssuedAt)
		if elapsed < 200*time.Millisecond {
			log.Printf("handleVerify: Timing violation for %s - elapsed %v", clientIP, elapsed)
			http.Error(w, "Verification failed", http.StatusUnauthorized)
			return
		}
	}

	// Behavioral validation: require evidence of human interaction
	if !validateBehavioral(&req.B, fp.IsMobile) {
		log.Printf("handleVerify: Behavioral check failed for %s - moves:%d entropy:%.3f time:%dms",
			clientIP, req.B.MouseMoves, req.B.MouseEntropy, req.B.InteractionMs)
		http.Error(w, "Verification failed", http.StatusUnauthorized)
		return
	}

	// Verify computational proof of work (no trivial image/logic bypasses)
	verified := challenge.VerifyChallenge(req.Proof, req.Nonce, clientIP, storedChallenge.Seed, fp.IsMobile, fp.CanvasHash, loadedConfig, storedChallenge.Difficulty, storedChallenge.Iterations)

	if !verified {
		log.Printf("handleVerify: Proof verification failed for IP %s, nonce %s, proof %s, type %s", clientIP, req.Nonce, req.Proof, storedChallenge.Type)
		http.Error(w, "Verification failed", http.StatusUnauthorized)
		return
	}

	if err := redisStoreGlobal.DeleteChallenge(clientIP, req.Nonce); err != nil {
		log.Printf("handleVerify: Failed to delete challenge for IP %s, nonce %s: %v", clientIP, req.Nonce, err)
	}

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

// validateBehavioral checks that the verification request contains evidence of human interaction.
func validateBehavioral(b *types.BehavioralData, isMobile bool) bool {
	// Must have spent some time on the page
	if b.InteractionMs < 800 {
		return false
	}
	hasInteraction := b.MouseMoves > 0 || b.TouchEvents > 0 || b.ScrollEvents > 0 || b.KeyPresses > 0
	// Zero interaction events + short time = automated
	if !hasInteraction && b.InteractionMs < 3000 {
		return false
	}
	// Desktop: check mouse movement quality if enough data points
	if !isMobile && b.MouseMoves >= 5 && b.MouseEntropy < 0.01 {
		return false
	}
	return true
}

func handleInteractiveChallenge(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now() // record before any tarpit delay
	clientIP := getClientIP(r)
	fingerprintStore.RLock()
	_, hasFingerprint := fingerprintStore.Data[clientIP]
	fingerprintStore.RUnlock()
	if !hasFingerprint {
		log.Printf("handleInteractiveChallenge: No fingerprint for IP %s", clientIP)
		http.Error(w, "No fingerprint", http.StatusBadRequest)
		return
	}

	// Tarpit delay for suspicious users
	suspicious, riskScore := isSuspicious(r, loadedConfig)
	if suspicious && loadedConfig.Tarpit.Enabled {
		offender := recordOffender(clientIP, riskScore)
		applyTarpitDelay(loadedConfig, clientIP, riskScore, offender)
	}

	chal, err := challenge.GenerateInteractiveChallenge()
	if err != nil {
		log.Printf("handleInteractiveChallenge: Generation failed for IP %s: %v", clientIP, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	// Backdate IssuedAt to before the tarpit delay so timing checks are accurate
	chal.IssuedAt = requestStart

	if err := redisStoreGlobal.SetInteractiveChallenge(clientIP, chal.Nonce, chal, 5*time.Minute); err != nil {
		log.Printf("handleInteractiveChallenge: Redis persist failed for IP %s: %v", clientIP, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"nonce":     chal.Nonce,
		"sequence":  chal.Sequence,
		"grid_size": chal.GridSize,
	}
	log.Printf("handleInteractiveChallenge: Issued for IP %s, nonce %s, seq %v", clientIP, chal.Nonce, chal.Sequence)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleVerifyInteractive(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	var req struct {
		Nonce  string `json:"nonce"`
		Clicks []struct {
			Cell int   `json:"cell"`
			Time int64 `json:"time"`
		} `json:"clicks"`
		B types.BehavioralData `json:"b"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("handleVerifyInteractive: Bad request from IP %s: %v", clientIP, err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	fingerprintStore.RLock()
	fp, hasFingerprint := fingerprintStore.Data[clientIP]
	fingerprintStore.RUnlock()
	if !hasFingerprint {
		log.Printf("handleVerifyInteractive: No fingerprint for IP %s", clientIP)
		http.Error(w, "No fingerprint", http.StatusBadRequest)
		return
	}

	var stored types.InteractiveChallenge
	exists, err := redisStoreGlobal.GetInteractiveChallenge(clientIP, req.Nonce, &stored)
	if err != nil {
		log.Printf("handleVerifyInteractive: Redis error for IP %s: %v", clientIP, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !exists {
		log.Printf("handleVerifyInteractive: No valid challenge for IP %s, nonce %s", clientIP, req.Nonce)
		http.Error(w, "No valid challenge", http.StatusBadRequest)
		return
	}

	// Timing: interactive takes at least 2 seconds
	if !stored.IssuedAt.IsZero() && time.Since(stored.IssuedAt) < 200*time.Millisecond {
		log.Printf("handleVerifyInteractive: Timing violation for %s — elapsed %v", clientIP, time.Since(stored.IssuedAt))
		http.Error(w, "Verification failed", http.StatusUnauthorized)
		return
	}

	// Behavioral validation
	if !validateBehavioral(&req.B, fp.IsMobile) {
		log.Printf("handleVerifyInteractive: Behavioral check failed for %s", clientIP)
		http.Error(w, "Verification failed", http.StatusUnauthorized)
		return
	}

	// Extract clicks and times
	var clicks []int
	var clickTimes []int64
	for _, c := range req.Clicks {
		clicks = append(clicks, c.Cell)
		clickTimes = append(clickTimes, c.Time)
	}

	if !challenge.VerifyInteractiveChallenge(clicks, clickTimes, stored.Sequence) {
		log.Printf("handleVerifyInteractive: Sequence verification failed for IP %s", clientIP)
		http.Error(w, "Verification failed", http.StatusUnauthorized)
		return
	}

	// Clean up
	redisStoreGlobal.DeleteInteractiveChallenge(clientIP, req.Nonce)
	if loadedConfig.Tarpit.Enabled {
		reduceOffenderScore(clientIP)
	}

	log.Printf("handleVerifyInteractive: Interactive challenge verified for IP %s", clientIP)

	// Issue JWT — identical to handleVerify
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"ip":  clientIP,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("handleVerifyInteractive: Token generation failed for IP %s: %v", clientIP, err)
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

	log.Printf("handleVerifyInteractive: Issued token for IP %s", clientIP)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func issueChallenge(w http.ResponseWriter, r *http.Request) {
	log.Printf("issueChallenge: Serving verification UI for %s", getClientIP(r))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	if data, err := os.ReadFile("assets/verification-ui.html"); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
	// Inline fallback
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Verification</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1>🔐 Verification Required</h1>
<p>Please solve the challenge...</p>
</body>
</html>`))
}

// ============================================================================
// TARPIT FUNCTIONS - Adaptive bot mitigation through delays and difficulty
// ============================================================================

// recordOffender tracks suspicious behavior for progressive penalties
func recordOffender(clientIP string, riskScore int) *types.OffenderRecord {
	var rec types.OffenderRecord
	exists, err := redisStoreGlobal.GetOffender(clientIP, &rec)
	if err != nil {
		log.Printf("recordOffender: Redis error reading offender %s: %v", clientIP, err)
	}
	now := time.Now()
	if !exists {
		rec = types.OffenderRecord{Attempts: 0, FirstSeen: now, TotalScore: 0}
	}
	rec.Attempts++
	rec.LastSeen = now
	rec.TotalScore += riskScore
	rec.LastScore = riskScore

	ttl := time.Duration(loadedConfig.Tarpit.OffenderMemoryMins) * time.Minute
	if err := redisStoreGlobal.SetOffender(clientIP, rec, ttl); err != nil {
		log.Printf("recordOffender: Failed to persist offender %s: %v", clientIP, err)
	}

	log.Printf("Tarpit: Recorded offense for %s - attempts: %d, total_score: %d, last_score: %d",
		clientIP, rec.Attempts, rec.TotalScore, rec.LastScore)

	return &rec
}

// getOffenderRecord retrieves the offender record without modifying it
func getOffenderRecord(clientIP string) *types.OffenderRecord {
	var rec types.OffenderRecord
	exists, err := redisStoreGlobal.GetOffender(clientIP, &rec)
	if err != nil {
		log.Printf("getOffenderRecord: Redis error for %s: %v", clientIP, err)
		return nil
	}
	if !exists {
		return nil
	}
	return &rec
}

// calculateTarpitDelay determines how long to delay response based on suspicion
func calculateTarpitDelay(cfg *config.JanusConfig, riskScore int, offender *types.OffenderRecord) time.Duration {
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

// calculateAdaptiveDifficulty computes PoW difficulty based on risk assessment.
// Difficulty is kept solvable: iterations are scaled in handleChallenge to match.
func calculateAdaptiveDifficulty(cfg *config.JanusConfig, baseDifficulty, riskScore int, offender *types.OffenderRecord) int {
	if !cfg.Tarpit.Enabled {
		if riskScore > 80 {
			return baseDifficulty + 2
		}
		return baseDifficulty
	}

	difficulty := baseDifficulty
	multiplier := cfg.Tarpit.DifficultyMultiplier

	// Risk tier scaling — kept modest so PoW stays solvable with scaled iterations
	switch {
	case riskScore >= 80:
		difficulty += multiplier // Extreme: +4 (e.g. diff 12)
	case riskScore >= 50:
		difficulty += (multiplier * 3) / 4 // High: +3
	case riskScore >= 30:
		difficulty += multiplier / 2 // Medium: +2
	case riskScore >= 10:
		difficulty += max(multiplier/4, 1) // Low: +1
	}

	// Repeat offender penalty (capped)
	if offender != nil && offender.Attempts > 1 {
		penalty := (offender.Attempts - 1) * cfg.Tarpit.RepeatPenalty
		if penalty > 4 {
			penalty = 4
		}
		difficulty += penalty
	}

	// Absolute cap — difficulty 16 requires ~65K hashes on average, solvable in seconds
	if difficulty > 16 {
		difficulty = 16
	}

	return difficulty
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// applyTarpitDelay blocks the goroutine for the calculated delay
func applyTarpitDelay(cfg *config.JanusConfig, clientIP string, riskScore int, offender *types.OffenderRecord) {
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
	var rec types.OffenderRecord
	exists, err := redisStoreGlobal.GetOffender(clientIP, &rec)
	if err != nil {
		log.Printf("reduceOffenderScore: Redis error for %s: %v", clientIP, err)
		return
	}
	if !exists {
		return
	}
	rec.Attempts = rec.Attempts / 2
	rec.TotalScore = rec.TotalScore / 2
	if rec.Attempts <= 0 {
		if err := redisStoreGlobal.DeleteOffender(clientIP); err != nil {
			log.Printf("reduceOffenderScore: Failed to delete offender %s: %v", clientIP, err)
		} else {
			log.Printf("Tarpit: Cleared offender record for %s after successful verification", clientIP)
		}
	} else {
		ttl := time.Duration(loadedConfig.Tarpit.OffenderMemoryMins) * time.Minute
		if err := redisStoreGlobal.SetOffender(clientIP, rec, ttl); err != nil {
			log.Printf("reduceOffenderScore: Failed to persist offender %s: %v", clientIP, err)
		} else {
			log.Printf("Tarpit: Reduced offender score for %s - attempts now: %d, total_score: %d",
				clientIP, rec.Attempts, rec.TotalScore)
		}
	}
}
