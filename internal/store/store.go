package store

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Session stores ephemeral data per client IP (MVP in-memory)
type Session struct {
	Score        float64
	Verified     bool
	LastSeen     time.Time
	PagesVisited int
	ScrollEvents int
	Nonces       map[string]time.Time
	Banned       bool
	Limiter      *rate.Limiter
}

var (
	mu           sync.Mutex
	sessions     = make(map[string]*Session)
	sessionTTL   = 15 * time.Minute
	defaultRPS   = 10
	defaultBurst = 20
)

// Init initializes default limits (call once at startup)
func Init(ttlSec, rps, burst int) {
	if ttlSec > 0 {
		sessionTTL = time.Duration(ttlSec) * time.Second
	}
	if rps > 0 {
		defaultRPS = rps
	}
	if burst > 0 {
		defaultBurst = burst
	}
	go cleanupLoop()
}

// NormalizeIP returns host portion of addr
func NormalizeIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.TrimSpace(remoteAddr)
	}
	return host
}

func getSession(ip string) *Session {
	mu.Lock()
	defer mu.Unlock()
	s, ok := sessions[ip]
	if !ok {
		s = &Session{
			Score:    0,
			Verified: false,
			LastSeen: time.Now(),
			Nonces:   make(map[string]time.Time),
			Limiter:  rate.NewLimiter(rate.Limit(defaultRPS), defaultBurst),
		}
		sessions[ip] = s
	}
	s.LastSeen = time.Now()
	return s
}

// GenerateNonce returns a crypto-random short string
func GenerateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// AddNonce records a nonce with TTL for IP
func AddNonce(remoteAddr, nonce string, ttlSec int) {
	ip := NormalizeIP(remoteAddr)
	s := getSession(ip)
	mu.Lock()
	defer mu.Unlock()
	s.Nonces[nonce] = time.Now().Add(time.Duration(ttlSec) * time.Second)
}

// ValidateNonce returns true if nonce exists and not expired; it deletes the nonce on success.
func ValidateNonce(remoteAddr, nonce string) bool {
	ip := NormalizeIP(remoteAddr)
	mu.Lock()
	defer mu.Unlock()
	s, ok := sessions[ip]
	if !ok {
		return false
	}
	exp, exists := s.Nonces[nonce]
	if !exists {
		return false
	}
	if time.Now().After(exp) {
		delete(s.Nonces, nonce)
		return false
	}
	delete(s.Nonces, nonce)
	return true
}

// UpdateScore increments the session score by delta and returns new score
func UpdateScore(remoteAddr string, delta float64) float64 {
	ip := NormalizeIP(remoteAddr)
	s := getSession(ip)
	mu.Lock()
	defer mu.Unlock()
	s.Score += delta
	if s.Score > 100 {
		s.Score = 100
	}
	if s.Score < -100 {
		s.Score = -100
	}
	return s.Score
}

func GetScore(remoteAddr string) float64 {
	ip := NormalizeIP(remoteAddr)
	mu.Lock()
	defer mu.Unlock()
	s, ok := sessions[ip]
	if !ok {
		return 0
	}
	return s.Score
}

func IncrementPages(remoteAddr string) {
	ip := NormalizeIP(remoteAddr)
	s := getSession(ip)
	mu.Lock()
	s.PagesVisited++
	mu.Unlock()
}

func IncrementScrolls(remoteAddr string) {
	ip := NormalizeIP(remoteAddr)
	s := getSession(ip)
	mu.Lock()
	s.ScrollEvents++
	mu.Unlock()
}

func SetVerified(remoteAddr string, v bool) {
	ip := NormalizeIP(remoteAddr)
	s := getSession(ip)
	mu.Lock()
	s.Verified = v
	mu.Unlock()
}

func Ban(remoteAddr string) {
	ip := NormalizeIP(remoteAddr)
	s := getSession(ip)
	mu.Lock()
	s.Banned = true
	mu.Unlock()
}

func IsBanned(remoteAddr string) bool {
	//ip := NormalizeIP(remoteAddr)
	mu.Lock()
	defer mu.Unlock()
	s, ok := sessions[remoteAddr]
	if !ok {
		// also check normalized ip
		ip := NormalizeIP(remoteAddr)
		s, ok = sessions[ip]
		if !ok {
			return false
		}
	}
	return s.Banned
}

func Allow(remoteAddr string) bool {
	ip := NormalizeIP(remoteAddr)
	s := getSession(ip)
	return s.Limiter.Allow()
}

func cleanupLoop() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		now := time.Now()
		for ip, s := range sessions {
			if now.Sub(s.LastSeen) > sessionTTL {
				delete(sessions, ip)
			}
		}
		mu.Unlock()
	}
}
