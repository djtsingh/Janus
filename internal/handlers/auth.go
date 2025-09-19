package handlers

import (
	"net/http"
	"strings"
)

// APIKeyAuthMiddleware enforces a simple API key found in header X-API-Key or Authorization Bearer
func APIKeyAuthMiddleware(next http.Handler, expectedKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			// fallback to Authorization: Bearer <key>
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				key = auth[7:]
			}
		}
		if strings.TrimSpace(key) == "" || key != expectedKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
