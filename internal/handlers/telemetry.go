package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

// TelemetryPayload represents the structure sent from sensor.js
type TelemetryPayload struct {
	Nonce   string        `json:"nonce"`
	Moves   []interface{} `json:"moves"`
	Scrolls []interface{} `json:"scrolls"`
	Accel   []interface{} `json:"accel"`
	Canvas  string        `json:"canvas"`
}

// In-memory nonce store (for demo, replace with Redis or DB in prod)
var validNonces = map[string]bool{}

// GenerateNonce registers a new valid nonce for the session
func GenerateNonce(nonce string) {
	validNonces[nonce] = true
}

// TelemetryHandler handles POST requests from sensor.js
func TelemetryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload TelemetryPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate nonce
	if _, ok := validNonces[payload.Nonce]; !ok || payload.Nonce == "" {
		http.Error(w, "Invalid nonce", http.StatusForbidden)
		return
	}

	// Optionally: remove nonce after first use for extra security
	delete(validNonces, payload.Nonce)

	// Log telemetry for debugging (or save to DB)
	log.Printf("Telemetry received: Moves: %d, Scrolls: %d, Accel: %d, Canvas: %s",
		len(payload.Moves), len(payload.Scrolls), len(payload.Accel), payload.Canvas[:min(20, len(payload.Canvas))]+"...")

	// Respond success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Telemetry accepted",
	})
}

// helper function to avoid slice out of range
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
