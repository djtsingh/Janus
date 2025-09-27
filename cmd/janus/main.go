package main

import (
	"log"
	"net/http"

	"janus/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()

	// Apply Janus middleware before any routes
	r.Use(middleware.JanusMiddleware)

	// Define routes after middleware
	r.Get("/sensor.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		http.ServeFile(w, r, "assets/sensor.js")
	})

	// Example protected route
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to your protected site!"))
	})

	log.Println("Starting JANUS server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
