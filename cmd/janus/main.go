package main

import (
	"crypto/tls"
	"log"
	"net/http"

	"janus/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.JanusMiddleware)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to your protected site!"))
	})
	r.Get("/sensor.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "assets/sensor.js")
	})

	// Load certificates
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatalf("Failed to load certificates: %v. Generate with: openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes", err)
	}

	// HTTPS server
	httpsServer := &http.Server{
		Addr:    ":8080",
		Handler: r,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	// HTTP server for redirect
	httpServer := &http.Server{
		Addr: ":8081",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			redirectURL := "https://localhost:8080" + r.URL.Path
			if r.URL.RawQuery != "" {
				redirectURL += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
		}),
	}

	// Start servers concurrently
	go func() {
		log.Println("Starting HTTP redirect server on :8081")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	log.Println("Starting JANUS server on https://localhost:8080")
	if err := httpsServer.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("HTTPS server failed: %v", err)
	}
}
