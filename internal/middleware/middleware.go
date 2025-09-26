package middleware

import (
	"janus/internal/config"
	"janus/internal/store"
	"log"
	"net"
	"net/http"
)

type Middleware struct {
	Cfg *config.Config
	St  *store.Store
}

func New(cfg *config.Config, st *store.Store) *Middleware {
	return &Middleware{
		Cfg: cfg,
		St:  st,
	}
}

func (m *Middleware) Session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("INFO: Session Middleware Executed")
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) RateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var limit int
		var identifier string

		// This logic is for our future API key feature.
		if r.Header.Get("X-Janus-User-Type") == "api" {
			limit = 1000
			identifier = r.Header.Get("X-Janus-API-Key")
		} else {
			limit = 100

			// --- THIS IS THE FIX ---
			// We now parse the IP from RemoteAddr to exclude the port.
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// If parsing fails, fallback to using RemoteAddr as is.
				identifier = r.RemoteAddr
			} else {
				identifier = ip
			}
			// --- END OF FIX ---
		}

		isLimited, err := m.St.IsRateLimited(identifier, limit)
		if err != nil {
			log.Printf("ERROR: Rate limiter check failed: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if isLimited {
			log.Printf("RATE LIMIT EXCEEDED for identifier: %s", identifier)
			http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
