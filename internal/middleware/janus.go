package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"janus/internal/challenge"
	"janus/internal/config"
	"janus/internal/handlers"
	"janus/internal/ratelimit"
	"janus/internal/types"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

// Global vars
var (
	loadedConfig     *config.JanusConfig
	configOnce       sync.Once
	jwtSecret        = []byte("your-secure-random-secret-key-32bytes") // Move to config in production
	fingerprintStore = struct {
		sync.RWMutex
		data map[string]types.Fingerprint
	}{data: make(map[string]types.Fingerprint)}
	challengeStore = struct {
		sync.RWMutex
		data map[string]string // nonce -> expectedHash
	}{data: make(map[string]string)}
)

// JanusMiddleware
func JanusMiddleware(next http.Handler) http.Handler {
	configOnce.Do(func() {
		var err error
		loadedConfig, err = config.LoadConfig("config.yaml")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	})

	rlStore := ratelimit.NewStore(loadedConfig)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		ua := r.Header.Get("User-Agent")
		log.Printf("Request: %s, Method: %s, IP: %s, UA: %s", r.URL.Path, r.Method, clientIP, ua)

		// Bypass middleware for /sensor.js
		if r.URL.Path == "/sensor.js" {
			log.Printf("Bypassing middleware for /sensor.js")
			next.ServeHTTP(w, r)
			return
		}

		// Handle /janus/* endpoints
		if strings.HasPrefix(r.URL.Path, "/janus/") {
			log.Printf("Serving Janus endpoint: %s", r.URL.Path)
			subR := chi.NewRouter()
			subR.Post("/fingerprint", handlers.HandleFingerprint(fingerprintStore.data))
			subR.Get("/challenge", handleChallenge)
			subR.Get("/mobile-challenge", handleMobileChallenge)
			subR.Post("/verify", handleVerify)
			// Strip /janus prefix for sub-router
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/janus")
			subR.ServeHTTP(w, r)
			return
		}

		if !rlStore.Allow(clientIP) {
			log.Printf("Rate limit exceeded for %s", clientIP)
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		suspicious := isSuspicious(r, loadedConfig)
		log.Printf("Suspicious: %v", suspicious)
		if suspicious {
			log.Printf("Issuing challenge for %s", clientIP)
			issueChallenge(w, r)
			return
		}

		verified := isVerified(r)
		log.Printf("Verified: %v", verified)
		if !verified {
			log.Printf("Verification failed for %s", clientIP)
			http.Error(w, "Verification required", http.StatusForbidden)
			return
		}

		log.Printf("Serving content for %s", clientIP)
		next.ServeHTTP(w, r)
	})
}

// Revamped isSuspicious
func isSuspicious(r *http.Request, cfg *config.JanusConfig) bool {
	ua := r.Header.Get("User-Agent")
	clientIP := getClientIP(r)
	for _, allowed := range cfg.WhitelistUA {
		if strings.Contains(strings.ToLower(ua), strings.ToLower(allowed)) {
			log.Printf("isSuspicious: Whitelisted UA %s for IP %s", ua, clientIP)
			return false
		}
	}
	fingerprintStore.RLock()
	_, hasFingerprint := fingerprintStore.data[clientIP]
	fingerprintStore.RUnlock()

	suspicious := ua == "" || strings.Contains(ua, "curl") || strings.Contains(ua, "python") || strings.Contains(ua, "Headless") || !hasFingerprint
	log.Printf("isSuspicious: UA %s, IP %s, HasFingerprint: %v, Suspicious: %v", ua, clientIP, hasFingerprint, suspicious)
	return suspicious
}

// Revamped isVerified
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

// Revamped handleVerify
func handleVerify(w http.ResponseWriter, r *http.Request) {
	var v types.Verification
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		log.Printf("handleVerify: Invalid request body: %v", err)
		http.Error(w, "Invalid verification payload", http.StatusBadRequest)
		return
	}
	if v.Nonce == "" || v.Proof == "" {
		log.Printf("handleVerify: Missing nonce or proof")
		http.Error(w, "Missing nonce or proof", http.StatusBadRequest)
		return
	}
	challengeStore.RLock()
	expectedHash, exists := challengeStore.data[v.Nonce]
	challengeStore.RUnlock()
	if !exists {
		log.Printf("handleVerify: Nonce %s not found", v.Nonce)
		http.Error(w, "Invalid or expired nonce", http.StatusUnauthorized)
		return
	}
	if !challenge.VerifyChallenge(v.Proof, expectedHash) {
		log.Printf("handleVerify: Proof verification failed for nonce %s", v.Nonce)
		http.Error(w, "Verification failed", http.StatusUnauthorized)
		return
	}
	challengeStore.Lock()
	delete(challengeStore.data, v.Nonce)
	challengeStore.Unlock()
	clientIP := getClientIP(r)
	log.Printf("handleVerify: Proof verified for IP %s, nonce %s", clientIP, v.Nonce)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"ip":  clientIP,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("handleVerify: Failed to issue token for IP %s: %v", clientIP, err)
		http.Error(w, "Failed to issue token", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "janus_token",
		Value:    tokenString,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   false, // Set to true in production
		Path:     "/",   // Ensure cookie is available for all paths
	})
	log.Printf("handleVerify: Issued token for IP %s", clientIP)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleChallenge
func handleChallenge(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	ch, expectedHash := challenge.GenerateChallenge(loadedConfig)
	if ch == nil {
		log.Printf("handleChallenge: Failed to generate challenge for IP %s", clientIP)
		http.Error(w, "Failed to generate challenge", http.StatusInternalServerError)
		return
	}
	challengeStore.Lock()
	challengeStore.data[ch.Nonce] = expectedHash
	challengeStore.Unlock()
	log.Printf("handleChallenge: Issued challenge for IP %s, nonce %s", clientIP, ch.Nonce)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ch); err != nil {
		log.Printf("handleChallenge: Encode error for IP %s: %v", clientIP, err)
		http.Error(w, "Failed to encode challenge", http.StatusInternalServerError)
	}
}

// handleMobileChallenge
func handleMobileChallenge(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	ch, expectedHash := challenge.GenerateMobileChallenge(loadedConfig)
	if ch == nil {
		log.Printf("handleMobileChallenge: Failed to generate challenge for IP %s", clientIP)
		http.Error(w, "Failed to generate mobile challenge", http.StatusInternalServerError)
		return
	}
	challengeStore.Lock()
	challengeStore.data[ch.Nonce] = expectedHash
	challengeStore.Unlock()
	log.Printf("handleMobileChallenge: Issued challenge for IP %s, nonce %s", clientIP, ch.Nonce)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ch); err != nil {
		log.Printf("handleMobileChallenge: Encode error for IP %s: %v", clientIP, err)
		http.Error(w, "Failed to encode challenge", http.StatusInternalServerError)
	}
}

// issueChallenge - Serve HTML page with script
func issueChallenge(w http.ResponseWriter, r *http.Request) {
	log.Printf("issueChallenge: Serving HTML challenge page")
	w.Header().Set("Content-Type", "text/html")
	html := `
    <html>
    <head>
        <title>Verifying...</title>
    </head>
    <body>
        <p>Verifying your request...</p>
        <script src="/sensor.js"></script>
    </body>
    </html>
    `
	w.Write([]byte(html))
}

// getClientIP
func getClientIP(r *http.Request) string {
	log.Printf("getClientIP: X-Forwarded-For: %s, RemoteAddr: %s", r.Header.Get("X-Forwarded-For"), r.RemoteAddr)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip := strings.Split(forwarded, ",")[0]
		ip = strings.TrimSpace(ip)
		if ip != "" && ip != "[" {
			return ip
		}
	}
	// Handle IPv6 and IPv4 RemoteAddr (e.g., [::1]:60500 or 127.0.0.1:60500)
	addr := r.RemoteAddr
	if strings.HasPrefix(addr, "[") {
		// IPv6: Extract IP before port
		end := strings.LastIndex(addr, "]")
		if end != -1 {
			ip := addr[1:end]
			if ip != "" {
				return ip
			}
		}
	} else {
		// IPv4: Split on colon
		parts := strings.Split(addr, ":")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	log.Printf("getClientIP: Falling back to default IP")
	return "unknown"
}
