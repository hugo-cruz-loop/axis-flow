// Package service contains business-logic helpers for the Clientes module.
package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// keyHex must be a 64-character hex string (32 bytes).
// The output is a base64url-encoded string containing [nonce || ciphertext || tag].
// Security: plaintext and keyHex are never included in returned errors.
func Encrypt(plaintext, keyHex string) (string, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("clientes/encryption: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("clientes/encryption: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("clientes/encryption: generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.URLEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64url-encoded ciphertext produced by Encrypt.
// keyHex must be a 64-character hex string (32 bytes).
// Security: ciphertext and keyHex are never included in returned errors.
func Decrypt(ciphertext, keyHex string) (string, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return "", err
	}

	data, err := base64.URLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("clientes/encryption: decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("clientes/encryption: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("clientes/encryption: create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("clientes/encryption: ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("clientes/encryption: decrypt failed: %w", err)
	}

	return string(plaintext), nil
}

// decodeKey decodes a 64-char hex string into a 32-byte AES-256 key.
func decodeKey(keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("clientes/encryption: invalid key encoding: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("clientes/encryption: key must be exactly 32 bytes, got %d", len(key))
	}
	return key, nil
}
