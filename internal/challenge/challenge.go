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

func VerifyChallenge(proof, expectedNonce, expectedClientIP, expectedSeed string, isMobile bool, canvasHash string, cfg *config.JanusConfig, challengeDifficulty int, challengeIterations int) bool {
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
	maxIter := challengeIterations
	if maxIter <= 0 {
		maxIter = cfg.MobileIterations
		if !isMobile {
			maxIter = cfg.DesktopIterations
		}
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

// GenerateInteractiveChallenge creates a grid-click challenge for fallback verification.
func GenerateInteractiveChallenge() (*types.InteractiveChallenge, error) {
	nonce, err := generateNonce()
	if err != nil {
		return nil, err
	}
	gridSize := 4
	totalCells := gridSize * gridSize
	seqLen := 5

	used := make(map[int]bool)
	seq := make([]int, 0, seqLen)
	for len(seq) < seqLen {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		cell := int(b[0]) % totalCells
		if !used[cell] {
			used[cell] = true
			seq = append(seq, cell)
		}
	}

	return &types.InteractiveChallenge{
		Nonce:    nonce,
		Sequence: seq,
		GridSize: gridSize,
		IssuedAt: time.Now(),
	}, nil
}

// VerifyInteractiveChallenge checks click sequence, timing, and variance.
func VerifyInteractiveChallenge(clicks []int, clickTimes []int64, expected []int) bool {
	if len(clicks) != len(expected) {
		log.Printf("VerifyInteractive: Click count mismatch: got %d, expected %d", len(clicks), len(expected))
		return false
	}
	for i, c := range clicks {
		if c != expected[i] {
			log.Printf("VerifyInteractive: Wrong cell at position %d: got %d, expected %d", i, c, expected[i])
			return false
		}
	}
	// Each click must be within human reaction range
	for i := 1; i < len(clickTimes); i++ {
		gap := clickTimes[i] - clickTimes[i-1]
		if gap < 200 || gap > 10000 {
			log.Printf("VerifyInteractive: Timing gap %dms at index %d out of range", gap, i)
			return false
		}
	}
	// Detect robotic regularity — all identical gaps
	if len(clickTimes) >= 3 {
		var gaps []int64
		for i := 1; i < len(clickTimes); i++ {
			gaps = append(gaps, clickTimes[i]-clickTimes[i-1])
		}
		allSame := true
		for i := 1; i < len(gaps); i++ {
			diff := gaps[i] - gaps[0]
			if diff < 0 {
				diff = -diff
			}
			if diff > 50 {
				allSame = false
				break
			}
		}
		if allSame {
			log.Printf("VerifyInteractive: Timing variance too low (robotic)")
			return false
		}
	}
	return true
}
