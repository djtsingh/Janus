package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv" // <-- IMPORT ADDED
	"strings"
	"time"

	"janus/internal/config"
	"janus/internal/handlers"
	"janus/internal/middleware"
	"janus/internal/store"
)

var sensorScript []byte

func main() {
	// 1. Load Configuration
	cfgPath := flag.String("config", "config.dev.json", "path to config file")
	flag.Parse()
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("FATAL: could not load config: %v", err)
	}

	// Read sensor script into memory
	sensorPath := cfg.StaticDir + "/sensor.js"
	sensorScript, err = os.ReadFile(sensorPath)
	if err != nil {
		log.Fatalf("FATAL: could not read sensor.js at %s: %v", sensorPath, err)
	}

	// 2. Initialize Dependencies
	st := store.New(cfg.RedisAddr)
	h := handlers.New(cfg, st)
	mw := middleware.New(cfg, st)

	// 3. Set up Proxy and Static File Server
	originUrl, err := url.Parse(cfg.BackendURL)
	if err != nil {
		log.Fatalf("FATAL: invalid backend URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(originUrl)
	proxy.ModifyResponse = modifyHTMLResponse // Set our script injector
	fs := http.FileServer(http.Dir(cfg.StaticDir))

	// 4. Set up the Router (ServeMux)
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.VerifyPath, h.VerifyHandler)
	mux.HandleFunc(cfg.TelemetryPath, h.TelemetryHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", fs))
	mux.Handle("/", proxy) // Proxy is the catch-all

	// 5. Chain the Middleware
	chainedHandler := mw.Session(mw.RateLimiter(mux))

	// 6. Configure and Start the Server
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      chainedHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("INFO: Janus listening on %s, proxying to %s", cfg.ListenAddr, cfg.BackendURL)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("FATAL: could not start server: %v", err)
	}
}

// modifyHTMLResponse injects our sensor into HTML pages.
func modifyHTMLResponse(resp *http.Response) error {
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return nil
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	bodyString := string(bodyBytes)
	injectionPoint := strings.LastIndex(bodyString, "</body>")
	if injectionPoint == -1 {
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return nil
	}

	scriptTag := "<script>" + string(sensorScript) + "</script>"
	modifiedBody := bodyString[:injectionPoint] + scriptTag + bodyString[injectionPoint:]

	// --- THIS IS THE FIX ---
	resp.Body = io.NopCloser(bytes.NewBufferString(modifiedBody))
	// Correctly calculate and set the new Content-Length.
	resp.ContentLength = int64(len(modifiedBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(modifiedBody)))
	// --- END OF FIX ---

	return nil
}
