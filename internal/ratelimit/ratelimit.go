package ratelimit

import (
	"sync"
	"time"

	"janus/internal/config"
)

// Bucket represents a token bucket for an IP
type Bucket struct {
	Tokens     int
	LastRefill time.Time
}

// Store is the in-memory rate limit store
type Store struct {
	Buckets map[string]*Bucket
	mu      sync.RWMutex
	RPS     int // Requests per second
}

// NewStore creates a new rate limit store from config
func NewStore(cfg *config.JanusConfig) *Store {
	return &Store{
		Buckets: make(map[string]*Bucket),
		RPS:     cfg.RateLimitRPS,
	}
}

// Allow checks if the IP can make a request; returns true if allowed, false if limited
func (s *Store) Allow(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	b, exists := s.Buckets[ip]
	if !exists {
		b = &Bucket{
			Tokens:     s.RPS,
			LastRefill: now,
		}
		s.Buckets[ip] = b
	}

	elapsed := now.Sub(b.LastRefill).Seconds()
	tokensToAdd := int(elapsed) * s.RPS
	if tokensToAdd > 0 {
		b.Tokens = min(b.Tokens+tokensToAdd, s.RPS)
		b.LastRefill = now
	}

	if b.Tokens > 0 {
		b.Tokens--
		return true
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
