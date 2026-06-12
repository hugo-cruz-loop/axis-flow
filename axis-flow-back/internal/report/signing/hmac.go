// Package signing provides HMAC-SHA256 helpers for signed download URLs.
package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Sign returns a hex-encoded HMAC-SHA256 of "key|expires|userID" using secret.
// NEVER log the secret parameter.
func Sign(key, expires, userID, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%s|%s|%s", key, expires, userID)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify returns true iff the provided sig matches Sign(key, expires, userID, secret).
// Uses constant-time comparison to prevent timing attacks.
func Verify(key, expires, userID, sig, secret string) bool {
	expected := Sign(key, expires, userID, secret)
	return hmac.Equal([]byte(expected), []byte(sig))
}
