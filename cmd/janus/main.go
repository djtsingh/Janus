package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"janus/internal/config"
	"janus/internal/handlers"
	"janus/internal/proxy"
	"janus/internal/store"
)

// AppVersion defines the current version of the service
const AppVersion = "v1.0.0"

// HealthHandler responds with a simple health check
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf("Service is running. Version: %s", AppVersion)))
}

var currentNonce = "test-nonce"

// TelemetryHandler receives telemetry from sensor.js
func TelemetryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json") // always set headers first

	// Parse request body
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest) // only one WriteHeader
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid payload",
		})
		return
	}

	// Check nonce (for testing, accept only currentNonce)
	nonce, ok := payload["nonce"].(string)
	if !ok || nonce != currentNonce {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid nonce",
		})
		return
	}

	// Accept telemetry
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":  true,
		"msg": "telemetry accepted",
	})
}

func main() {

	// Create root context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Router setup
	router := mux.NewRouter()
	router.HandleFunc("/health", HealthHandler).Methods("GET")

	// Add more routes here (auth, proof-of-humanity, proxy, etc.)
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// init store
	store.Init(cfg.SessionTimeoutSeconds, cfg.RateLimitRPS, cfg.RateLimitBurst)

	// static files
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(cfg.StaticDir))))

	// telemetry & verify endpoints (handlers package)
	//router.Handle(cfg.TelemetryPath, handlers.TelemetryHandler(cfg)).Methods("POST")
	router.Handle(cfg.VerifyPath, handlers.VerifyHandler(cfg)).Methods("POST")
	router.HandleFunc("/telemetry", TelemetryHandler).Methods("POST")
	router.HandleFunc("/telemetry", handlers.TelemetryHandler).Methods("POST")
	handlers.GenerateNonce("test-nonce")

	// proxy as catch-all: create proxy handler and mount to root
	p, err := proxy.NewProxy(cfg)
	if err != nil {
		log.Fatalf("proxy init: %v", err)
	}
	router.PathPrefix("/").Handler(p)

	// Server configuration
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Starting secure identity service on %s (version %s)\n", server.Addr, AppVersion)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Could not start server: %v\n", err)
		}
	}()

	// Block until shutdown signal
	<-ctx.Done()
	stop()
	log.Println("⚠️ Shutdown signal received")

	// Gracefully shut down
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited gracefully")
}
