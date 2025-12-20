package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"janus/internal/types"
)

func HandleFingerprint(store *types.FingerprintStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var fp types.Fingerprint
		if err := json.NewDecoder(r.Body).Decode(&fp); err != nil {
			return
		}

		fp.ClientIP = getClientIP(r)

		store.Lock()
		store.Data[fp.ClientIP] = fp
		store.Unlock()

		log.Printf("HandleFingerprint: Stored fingerprint for %s: %+v", fp.ClientIP, fp)
		w.WriteHeader(http.StatusOK)
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
	addr := r.RemoteAddr
	if strings.HasPrefix(addr, "[") {
		end := strings.LastIndex(addr, "]")
		if end != -1 {
			ip := addr[1:end]
			if ip != "" {
				return ip
			}
		}
	} else {
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
