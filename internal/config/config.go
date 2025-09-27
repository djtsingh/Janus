package config

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v3"
)

type JanusConfig struct {
	RateLimitRPS int      `yaml:"rate_limit_rps"`
	Difficulty   string   `yaml:"difficulty"`
	WhitelistUA  []string `yaml:"whitelist_ua"`
}

func LoadConfig(path string) (*JanusConfig, error) {
	cfg := &JanusConfig{
		RateLimitRPS: 10,
		Difficulty:   "medium",
		WhitelistUA:  []string{"Googlebot", "Twitterbot"},
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("Config file not found at %s, using defaults: %v", path, err)
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
