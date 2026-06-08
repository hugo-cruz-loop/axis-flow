package service_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"axis-flow-back/internal/clientes/service"
)

// validKeyHex is a 32-byte key expressed as 64 hex chars.
const validKeyHex = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := "RFC-SAMPLE-12345"
	cipher, err := service.Encrypt(plaintext, validKeyHex)
	require.NoError(t, err)
	assert.NotEmpty(t, cipher)
	assert.NotEqual(t, plaintext, cipher)

	got, err := service.Decrypt(cipher, validKeyHex)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestEncryptProducesUniqueOutputs(t *testing.T) {
	// Each call must produce a different ciphertext (random nonce).
	c1, err := service.Encrypt("same-input", validKeyHex)
	require.NoError(t, err)
	c2, err := service.Encrypt("same-input", validKeyHex)
	require.NoError(t, err)
	assert.NotEqual(t, c1, c2, "ciphertext must be non-deterministic due to random nonce")
}

func TestEncryptInvalidKeyLength(t *testing.T) {
	_, err := service.Encrypt("hello", "deadbeef") // only 4 bytes — too short
	require.Error(t, err)
	assert.False(t, strings.Contains(err.Error(), "hello"), "error must not leak plaintext")
}

func TestDecryptInvalidKeyLength(t *testing.T) {
	_, err := service.Decrypt("anyciphertext", "deadbeef")
	require.Error(t, err)
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	cipher, err := service.Encrypt("original", validKeyHex)
	require.NoError(t, err)

	// Flip last character to corrupt the ciphertext.
	tampered := cipher[:len(cipher)-1] + "X"
	if tampered == cipher {
		tampered = cipher[:len(cipher)-1] + "Y"
	}
	_, err = service.Decrypt(tampered, validKeyHex)
	require.Error(t, err)
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	cipher, err := service.Encrypt("", validKeyHex)
	require.NoError(t, err)
	got, err := service.Decrypt(cipher, validKeyHex)
	require.NoError(t, err)
	assert.Equal(t, "", got)
}
