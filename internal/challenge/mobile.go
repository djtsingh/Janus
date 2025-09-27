package challenge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"janus/internal/config"
	"janus/internal/types"
)

// GenerateMobileChallenge creates a new sensor-based challenge
func GenerateMobileChallenge(cfg *config.JanusConfig) (*types.Challenge, string) {
	nonceBytes := make([]byte, 16)
	_, err := rand.Read(nonceBytes)
	if err != nil {
		return nil, ""
	}
	nonce := base64.StdEncoding.EncodeToString(nonceBytes)

	seedBytes := make([]byte, 8)
	_, err = rand.Read(seedBytes)
	if err != nil {
		return nil, ""
	}
	seed := base64.StdEncoding.EncodeToString(seedBytes)

	iterations := 100
	switch cfg.Difficulty {
	case "medium":
		iterations = 500
	case "high":
		iterations = 1000
	}

	expected := sha256.Sum256([]byte(nonce + seed + fmt.Sprintf("%d", iterations)))
	expectedHash := base64.StdEncoding.EncodeToString(expected[:])

	return &types.Challenge{
		Nonce:      nonce,
		Iterations: iterations,
		Seed:       seed,
	}, expectedHash
}
