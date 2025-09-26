package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	ListenAddr        string `json:"listen_addr"`
	BackendURL        string `json:"backend_url"`
	RedisAddr         string `json:"redis_addr"` // <-- ADD THIS LINE
	TelemetryPath     string `json:"telemetry_path"`
	VerifyPath        string `json:"verify_path"`
	SessionTimeoutSec int    `json:"session_timeout_seconds"`
	NonceTTLSeconds   int    `json:"nonce_ttl_seconds"`
	StaticDir         string `json:"static_dir"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
