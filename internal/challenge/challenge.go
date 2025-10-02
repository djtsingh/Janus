// internal/challenge/Challenge.go
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

// GenerateChallenge creates a challenge for desktop (PoR) or mobile (PoW) clients.
// Returns a Challenge struct and the number of leading zero bits required.
func GenerateChallenge(cfg *config.JanusConfig, isMobile bool) (*types.Challenge, int) {
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
	iterations := cfg.DesktopIterations
	zeroBits := cfg.DesktopDifficulty
	if isMobile {
		iterations = cfg.MobileIterations
		zeroBits = cfg.MobileDifficulty
	}
	return &types.Challenge{
		Nonce:      nonce,
		Iterations: iterations,
		Seed:       seed,
	}, zeroBits
}

// VerifyChallenge validates the client's proof for PoR (desktop) or PoW (mobile).
// Proof format: nonce:iteration:timestamp:clientIP:seed[:canvasHash] (desktop only).
func VerifyChallenge(proof, expectedNonce, expectedClientIP, expectedSeed string, isMobile bool, canvasHash string, cfg *config.JanusConfig) bool {
	parts := strings.Split(proof, "|")
	if isMobile {
		// Mobile clients send a 5-part proof
		if len(parts) != 5 {
			log.Printf("VerifyChallenge: Invalid proof length for mobile: got %d, expected 5", len(parts))
			return false
		}
	} else {
		// Desktop clients send a 6-part proof (including canvas hash)
		if len(parts) != 6 {
			log.Printf("VerifyChallenge: Invalid proof length for desktop: got %d, expected 6", len(parts))
			return false
		}
		// Only check the canvas hash for desktop clients
		if parts[5] != canvasHash {
			log.Printf("VerifyChallenge: Canvas hash mismatch")
			return false
		}
	}
	nonce, iteration, timestamp, clientIP, seed := parts[0], parts[1], parts[2], parts[3], parts[4]

	// Validate components
	if nonce != expectedNonce || clientIP != expectedClientIP || seed != expectedSeed {
		log.Printf("VerifyChallenge: Component mismatch")
		return false
	}
	log.Printf("DEBUG: Canvas hash from PROOF  : %s", parts[5])
	log.Printf("DEBUG: Canvas hash from STORAGE: %s", canvasHash)
	log.Printf("DEBUG: Are they equal? %v", parts[5] == canvasHash)

	if !isMobile && parts[5] != canvasHash {
		log.Printf("VerifyChallenge: Canvas hash mismatch")
		return false
	}

	// Validate iteration
	iter, err := strconv.Atoi(iteration)
	maxIter := cfg.MobileIterations
	if !isMobile {
		maxIter = cfg.DesktopIterations
	}
	if err != nil || iter < 0 || iter > maxIter {
		log.Printf("VerifyChallenge: Invalid iteration %s", iteration)
		return false
	}

	// Validate timestamp (within 5 minutes)
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil || time.Since(ts) > 5*time.Minute || ts.After(time.Now().Add(1*time.Minute)) {
		log.Printf("VerifyChallenge: Invalid timestamp: %s", timestamp)
		return false
	}

	// --- CRITICAL FIX ---
	// Get difficulty from config instead of hardcoding it
	zeroBits := cfg.MobileDifficulty
	if !isMobile {
		zeroBits = cfg.DesktopDifficulty
	}
	// --- END OF FIX ---

	hash := sha256.Sum256([]byte(proof))
	if !hasLeadingZeroBits(hash[:], zeroBits) {
		log.Printf("VerifyChallenge: Hash does not have %d leading zero bits", zeroBits)
		return false
	}

	return true
}

// hasLeadingZeroBits checks if the hash has at least the specified number of leading zero bits.
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

// generateNonce creates a cryptographically secure 16-byte nonce.
func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// generateSeed creates a cryptographically secure 8-byte seed.
func generateSeed() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
