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
}

func DefaultConfig() *JanusConfig {
	return &JanusConfig{
		DesktopIterations:  5000,
		MobileIterations:   5000,
		DesktopDifficulty:  8,
		MobileDifficulty:   6,
		WhitelistUA:        []string{"chrome", "firefox", "safari", "edge"},
		WhitelistIPs:       []string{"127.0.0.1", "::1"},
		BlacklistedIPs:     []string{},
		BannedGeoLocations: []string{},
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
	}
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
