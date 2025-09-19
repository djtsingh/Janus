package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"janus/internal/config"
	"janus/internal/store"
)

// VerifyRequest is minimal payload for proof submissions
type VerifyRequest struct {
	Nonce string `json:"nonce"`
	Token string `json:"token"`
}

// VerifyResponse is the JSON response
type VerifyResponse struct {
	Verified  bool   `json:"verified"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message,omitempty"`
}

// VerifyHandler returns an HTTP handler that accepts proof-of-render submissions.
// For MVP: it marks the session verified when a nonce is present/valid.
func VerifyHandler(cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// validate nonce
		if !store.ValidateNonce(r.RemoteAddr, req.Nonce) {
			http.Error(w, "invalid nonce", http.StatusForbidden)
			return
		}

		// Mark verified, increase score modestly
		store.SetVerified(r.RemoteAddr, true)
		store.UpdateScore(r.RemoteAddr, 2.0)

		resp := VerifyResponse{
			Verified:  true,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Message:   "session verified (MVP)",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}
