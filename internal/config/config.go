package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"time"
)

// Config holds Janus configuration values used across the app.
type Config struct {
	// Network
	ListenAddr string `json:"listen_addr"` // e.g. ":8080"

	// Target origin to proxy to
	Backend string `json:"backend"` // e.g. "http://localhost:3000"

	// Static assets (sensor.js, proof_render.js)
	StaticDir string `json:"static_dir"`

	// Telemetry & verify endpoints
	TelemetryPath string `json:"telemetry_path"`
	VerifyPath    string `json:"verify_path"`

	// Session / nonce / scoring
	SessionTimeoutSeconds int     `json:"session_timeout_seconds"`
	NonceTTLSeconds       int     `json:"nonce_ttl_seconds"`
	BanScoreThreshold     float64 `json:"ban_score_threshold"`
	VerifyScoreThreshold  float64 `json:"verify_score_threshold"`

	// Rate limiting defaults
	RateLimitRPS   int `json:"rate_limit_rps"`
	RateLimitBurst int `json:"rate_limit_burst"`

	// Injection script path used by proxy (served under /static/)
	InjectScriptPath string `json:"inject_script_path"`
}

// Load reads config from path (JSON) and applies environment overrides.
// If path is empty, it tries "./config.json". Missing optional fields receive sensible defaults.
func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.json"
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}

	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.Backend == "" {
		cfg.Backend = "http://localhost:3000"
	}
	if cfg.StaticDir == "" {
		cfg.StaticDir = "static"
	}
	if cfg.TelemetryPath == "" {
		cfg.TelemetryPath = "/telemetry"
	}
	if cfg.VerifyPath == "" {
		cfg.VerifyPath = "/verify"
	}
	if cfg.SessionTimeoutSeconds == 0 {
		cfg.SessionTimeoutSeconds = 900 // 15 minutes
	}
	if cfg.NonceTTLSeconds == 0 {
		cfg.NonceTTLSeconds = 30
	}
	if cfg.RateLimitRPS == 0 {
		cfg.RateLimitRPS = 10
	}
	if cfg.RateLimitBurst == 0 {
		cfg.RateLimitBurst = 20
	}
	if cfg.InjectScriptPath == "" {
		cfg.InjectScriptPath = "/static/sensor.js"
	}
	// sensible scoring defaults
	if cfg.BanScoreThreshold == 0 {
		cfg.BanScoreThreshold = -5.0
	}
	if cfg.VerifyScoreThreshold == 0 {
		cfg.VerifyScoreThreshold = 2.0
	}
}

func applyEnvOverrides(cfg *Config) {
	// Convenience: allow overriding individual values via env vars.
	if v := os.Getenv("JANUS_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("JANUS_BACKEND"); v != "" {
		cfg.Backend = v
	}
	if v := os.Getenv("JANUS_STATIC_DIR"); v != "" {
		cfg.StaticDir = v
	}
	if v := os.Getenv("JANUS_TELEMETRY_PATH"); v != "" {
		cfg.TelemetryPath = v
	}
	if v := os.Getenv("JANUS_VERIFY_PATH"); v != "" {
		cfg.VerifyPath = v
	}
	if v := os.Getenv("JANUS_SESSION_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SessionTimeoutSeconds = n
		}
	}
	if v := os.Getenv("JANUS_NONCE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.NonceTTLSeconds = n
		}
	}
	if v := os.Getenv("JANUS_RATE_RPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimitRPS = n
		}
	}
	if v := os.Getenv("JANUS_RATE_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimitBurst = n
		}
	}
	if v := os.Getenv("JANUS_INJECT_SCRIPT"); v != "" {
		cfg.InjectScriptPath = v
	}
	// Numeric floats: ban/verify thresholds
	if v := os.Getenv("JANUS_BAN_SCORE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.BanScoreThreshold = f
		}
	}
	if v := os.Getenv("JANUS_VERIFY_SCORE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.VerifyScoreThreshold = f
		}
	}
}

func validate(cfg *Config) error {
	if cfg.Backend == "" {
		return errors.New("backend must be set")
	}
	// simple listen address sanity check
	if cfg.ListenAddr == "" {
		return errors.New("listen_addr must be set")
	}
	// TTL sanity
	if cfg.NonceTTLSeconds < 1 || cfg.NonceTTLSeconds > 600 {
		return errors.New("nonce_ttl_seconds must be between 1 and 600")
	}
	if cfg.SessionTimeoutSeconds < 60 {
		// not fatal but require sensible setting
		return errors.New("session_timeout_seconds must be >= 60")
	}
	// Score thresholds sanity
	if cfg.BanScoreThreshold >= cfg.VerifyScoreThreshold {
		return errors.New("ban_score_threshold must be less than verify_score_threshold")
	}
	// Everything ok
	return nil
}

// Duration helpers used by other packages
func (c *Config) SessionTimeout() time.Duration {
	return time.Duration(c.SessionTimeoutSeconds) * time.Second
}
func (c *Config) NonceTTL() time.Duration {
	return time.Duration(c.NonceTTLSeconds) * time.Second
}
