package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateCredentials generates a new key/secret pair
// Key format: MUXI_{16 hex chars}
// Secret format: sk_{64 hex chars}
func GenerateCredentials() (key, secret string, err error) {
	// Generate random bytes
	keyBytes := make([]byte, 8)     // 16 hex chars
	secretBytes := make([]byte, 32) // 64 hex chars

	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate key: %w", err)
	}

	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate secret: %w", err)
	}

	key = "MUXI_" + hex.EncodeToString(keyBytes)
	secret = "sk_" + hex.EncodeToString(secretBytes)

	return key, secret, nil
}
