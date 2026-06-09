package storage_test

import (
	"bytes"
	"testing"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMagicBytes_ValidPDF(t *testing.T) {
	// PDF magic: %PDF (hex 25 50 44 46) + padding to 8 bytes.
	data := []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E, 0x34}
	err := storage.ValidateMagicBytes(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
}

func TestValidateMagicBytes_ValidDOCX(t *testing.T) {
	// DOCX/ZIP magic: PK 0x50 0x4B 0x03 0x04 + padding.
	data := []byte{0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00}
	err := storage.ValidateMagicBytes(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
}

func TestValidateMagicBytes_InvalidPNG(t *testing.T) {
	// PNG magic: 0x89 0x50 0x4E 0x47 0x0D 0x0A 0x1A 0x0A
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	err := storage.ValidateMagicBytes(bytes.NewReader(data), int64(len(data)))
	assert.ErrorIs(t, err, bolsatrabajo.ErrInvalidFile)
}

func TestValidateMagicBytes_SizeExceeds5MB(t *testing.T) {
	// Size > 5MB should fail immediately without reading bytes.
	data := []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E, 0x34} // valid PDF bytes
	const fiveMBPlusOne = 5_242_881
	err := storage.ValidateMagicBytes(bytes.NewReader(data), fiveMBPlusOne)
	assert.ErrorIs(t, err, bolsatrabajo.ErrInvalidFile)
}
