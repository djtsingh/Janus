package challenge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log"
	"strconv"
	"strings"
	"time"

	"janus/internal/config"
	"janus/internal/types"
)

func GenerateChallenge(cfg *config.JanusConfig, isMobile bool, riskScore int, history int) (*types.Challenge, int) {
	nonce, err := generateNonce()
	if err != nil {
		log.Printf("GenerateChallenge: Failed to generate nonce: %v", err)
		return nil, 0
	}
	seed, err := generateSeed()
	if err != nil {
		log.Printf("GenerateChallenge: Failed to generate seed: %v", err)
		return nil, 0
	}
	baseIterations := cfg.DesktopIterations
	baseDifficulty := cfg.DesktopDifficulty
	if isMobile {
		baseIterations = cfg.MobileIterations
		baseDifficulty = cfg.MobileDifficulty
	}
	difficulty := baseDifficulty
	if riskScore > 80 {
		difficulty = baseDifficulty + 2
	}
	return &types.Challenge{
		Nonce:      nonce,
		Iterations: baseIterations,
		Seed:       seed,
		Type:       "pow",
		Difficulty: difficulty,
		IssuedAt:   time.Now(),
	}, difficulty
}

func VerifyChallenge(proof, expectedNonce, expectedClientIP, expectedSeed string, isMobile bool, canvasHash string, cfg *config.JanusConfig, challengeDifficulty int) bool {
	parts := strings.Split(proof, "|")
	if isMobile {
		if len(parts) != 5 {
			log.Printf("VerifyChallenge: Invalid proof length for mobile: got %d, expected 5", len(parts))
			return false
		}
	} else {
		if len(parts) != 6 {
			log.Printf("VerifyChallenge: Invalid proof length for desktop: got %d, expected 6", len(parts))
			return false
		}
		if parts[5] != canvasHash {
			log.Printf("VerifyChallenge: Canvas hash mismatch")
			return false
		}
	}
	nonce, iteration, timestamp, clientIP, seed := parts[0], parts[1], parts[2], parts[3], parts[4]

	if nonce != expectedNonce || clientIP != expectedClientIP || seed != expectedSeed {
		log.Printf("VerifyChallenge: Component mismatch")
		return false
	}

	iter, err := strconv.Atoi(iteration)
	maxIter := cfg.MobileIterations
	if !isMobile {
		maxIter = cfg.DesktopIterations
	}
	if err != nil || iter < 0 || iter > maxIter {
		log.Printf("VerifyChallenge: Invalid iteration %s", iteration)
		return false
	}

	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil || time.Since(ts) > 5*time.Minute || ts.After(time.Now().Add(1*time.Minute)) {
		log.Printf("VerifyChallenge: Invalid timestamp")
		return false
	}

	// Use the actual challenge difficulty (includes tarpit adjustments)
	zeroBits := challengeDifficulty
	if zeroBits <= 0 {
		zeroBits = cfg.MobileDifficulty
		if !isMobile {
			zeroBits = cfg.DesktopDifficulty
		}
	}

	hash := sha256.Sum256([]byte(proof))
	if !hasLeadingZeroBits(hash[:], zeroBits) {
		log.Printf("VerifyChallenge: Hash does not meet difficulty requirement (%d bits)", zeroBits)
		return false
	}

	return true
}

func hasLeadingZeroBits(hash []byte, zeroBits int) bool {
	fullBytes := zeroBits / 8
	extraBits := zeroBits % 8
	for i := 0; i < fullBytes; i++ {
		if hash[i] != 0 {
			return false
		}
	}
	if extraBits > 0 {
		mask := byte(0xFF << (8 - extraBits))
		return (hash[fullBytes] & mask) == 0
	}
	return true
}

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func generateSeed() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
