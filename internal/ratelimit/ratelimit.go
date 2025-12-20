package ratelimit

import (
	"janus/internal/config"
	"sync"
	"time"
)

type Store struct {
	sync.Mutex
	data map[string]struct {
		Count     int
		LastReset time.Time
	}
	cfg *config.JanusConfig
}

func NewStore(cfg *config.JanusConfig) *Store {
	return &Store{
		data: make(map[string]struct {
			Count     int
			LastReset time.Time
		}),
		cfg: cfg,
	}
}

func (s *Store) Allow(ip string) bool {
	s.Lock()
	defer s.Unlock()
	entry, exists := s.data[ip]
	if !exists || time.Since(entry.LastReset) > time.Minute {
		s.data[ip] = struct {
			Count     int
			LastReset time.Time
		}{Count: 1, LastReset: time.Now()}
		return true
	}
	if entry.Count >= 100 {
		return false
	}
	entry.Count++
	s.data[ip] = entry
	return true
}
