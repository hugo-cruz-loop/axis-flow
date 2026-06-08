// Package crypto provides shared cryptographic helpers for the identity service.
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HashToken returns the SHA-256 hex digest of token.
// Used to store secrets (refresh tokens, activation tokens) without leaking raw values.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateRandomToken returns a cryptographically random 32-byte hex-encoded string.
func GenerateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read: %w", err)
	}
	return hex.EncodeToString(b), nil
}
