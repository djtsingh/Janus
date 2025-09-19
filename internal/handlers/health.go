// internal/handlers/health.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse defines the health-check response payload
type HealthResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

var startTime = time.Now()

// HealthHandler returns service health and uptime
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status: "ok",
		Uptime: time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
