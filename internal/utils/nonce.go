package utils

import (
	"crypto/rand"
	"encoding/base64"
)

func GenerateNonce() string {
	b := make([]byte, 16) // 16 bytes = 128 bits
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
