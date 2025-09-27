package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"janus/internal/types"
)

func HandleFingerprint(store map[string]types.Fingerprint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		log.Printf("HandleFingerprint: Received request from IP %s, Headers: %+v", clientIP, r.Header)

		if r.Header.Get("Content-Type") != "application/json" {
			log.Printf("HandleFingerprint: Invalid Content-Type: %s for IP %s", r.Header.Get("Content-Type"), clientIP)
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		var fp types.Fingerprint
		if err := json.NewDecoder(r.Body).Decode(&fp); err != nil {
			log.Printf("HandleFingerprint: Decode error for IP %s: %v", clientIP, err)
			http.Error(w, "Invalid fingerprint", http.StatusBadRequest)
			return
		}

		if fp.CanvasHash == "" || fp.Timezone == "" {
			log.Printf("HandleFingerprint: Missing required fields for IP %s: %+v", clientIP, fp)
			http.Error(w, "Missing required fingerprint fields", http.StatusBadRequest)
			return
		}

		store[clientIP] = fp
		log.Printf("HandleFingerprint: Stored fingerprint for IP %s: %+v", clientIP, fp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}

func getClientIP(r *http.Request) string {
	log.Printf("getClientIP: X-Forwarded-For: %s, RemoteAddr: %s", r.Header.Get("X-Forwarded-For"), r.RemoteAddr)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip := strings.Split(forwarded, ",")[0]
		ip = strings.TrimSpace(ip)
		if ip != "" && ip != "[" {
			return ip
		}
	}
	// Handle IPv6 and IPv4 RemoteAddr (e.g., [::1]:60500 or 127.0.0.1:60500)
	addr := r.RemoteAddr
	if strings.HasPrefix(addr, "[") {
		// IPv6: Extract IP before port
		end := strings.LastIndex(addr, "]")
		if end != -1 {
			ip := addr[1:end]
			if ip != "" {
				return ip
			}
		}
	} else {
		// IPv4: Split on colon
		parts := strings.Split(addr, ":")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	log.Printf("getClientIP: Falling back to default IP")
	return "unknown"
}
