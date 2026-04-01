package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type JanusConfig struct {
	DesktopIterations  int            `yaml:"desktop_iterations"`
	MobileIterations   int            `yaml:"mobile_iterations"`
	DesktopDifficulty  int            `yaml:"desktop_difficulty"`
	MobileDifficulty   int            `yaml:"mobile_difficulty"`
	WhitelistUA        []string       `yaml:"whitelist_ua"`
	WhitelistIPs       []string       `yaml:"whitelist_ips"`
	BlacklistedIPs     []string       `yaml:"blacklisted_ips"`
	BannedGeoLocations []string       `yaml:"banned_geo_locations"`
	SuspicionThreshold int            `yaml:"suspicion_threshold"`
	SuspicionWeights   map[string]int `yaml:"suspicion_weights"`
	RedisAddr          string         `yaml:"redis_addr"`
	RateLimit          struct {
		RequestsPerMinute int `yaml:"requests_per_minute"`
		Burst             int `yaml:"burst"`
	} `yaml:"rate_limit"`
	// When true every request (except /health, /janus/*, /sensor.js) is challenged
	// regardless of whether the user already holds a valid janus_token.
	ChallengeAll bool `yaml:"challenge_all"`

	// Tarpit settings for bot mitigation
	Tarpit struct {
		Enabled              bool `yaml:"enabled"`
		DelayPerScoreMs      int  `yaml:"delay_per_score_ms"`    // ms delay per suspicion point
		MaxDelayMs           int  `yaml:"max_delay_ms"`          // cap on delay
		DifficultyMultiplier int  `yaml:"difficulty_multiplier"` // extra difficulty per risk tier
		OffenderMemoryMins   int  `yaml:"offender_memory_mins"`  // how long to remember offenders
		RepeatPenalty        int  `yaml:"repeat_penalty"`        // extra difficulty per repeat offense
	} `yaml:"tarpit"`
}

func DefaultConfig() *JanusConfig {
	cfg := &JanusConfig{
		DesktopIterations:  5000,
		MobileIterations:   5000,
		DesktopDifficulty:  8,
		MobileDifficulty:   6,
		WhitelistUA:        []string{"chrome", "firefox", "safari", "edge"},
		WhitelistIPs:       []string{"127.0.0.1", "::1"},
		BlacklistedIPs:     []string{},
		BannedGeoLocations: []string{},
		ChallengeAll:       false,
		SuspicionThreshold: 50,
		SuspicionWeights: map[string]int{
			"blacklisted_ip":        100,
			"banned_geo":            100,
			"tls_mismatch":          30,
			"no_user_agent":         40,
			"headless_browser":      50,
			"missing_headers":       20,
			"header_order_mismatch": 20,
			"no_fingerprint":        30,
		},
		RedisAddr: "localhost:6379",
	}
	cfg.RateLimit.RequestsPerMinute = 60
	cfg.RateLimit.Burst = 10
	// Tarpit defaults - aggressive bot mitigation
	cfg.Tarpit.Enabled = true
	cfg.Tarpit.DelayPerScoreMs = 50     // 50ms per suspicion point
	cfg.Tarpit.MaxDelayMs = 10000       // max 10 second delay
	cfg.Tarpit.DifficultyMultiplier = 4 // +4 difficulty per risk tier
	cfg.Tarpit.OffenderMemoryMins = 30  // remember offenders for 30 mins
	cfg.Tarpit.RepeatPenalty = 2        // +2 difficulty per repeat offense
	return cfg
}

func LoadConfig(path string) (*JanusConfig, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("LoadConfig: Failed to read config file %s: %v", path, err)
		return cfg, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		log.Printf("LoadConfig: Failed to unmarshal config: %v", err)
		return cfg, err
	}
	return cfg, nil
}
