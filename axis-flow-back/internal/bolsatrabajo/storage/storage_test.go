package storage_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"axis-flow-back/internal/bolsatrabajo/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLocalStorage_UploadCV stores a file and returns a non-empty URL.
func TestLocalStorage_UploadCV(t *testing.T) {
	dir := t.TempDir()
	ls := storage.LocalStorage{BaseDir: dir}

	data := []byte("fake cv content")
	url, err := ls.UploadCV(context.Background(), "candidate/test.pdf", bytes.NewReader(data), int64(len(data)), "application/pdf")

	require.NoError(t, err)
	assert.NotEmpty(t, url)

	// File must exist on disk.
	expectedPath := filepath.Join(dir, "candidate/test.pdf")
	_, statErr := os.Stat(expectedPath)
	assert.NoError(t, statErr)
}

// TestLocalStorage_DeleteCV removes the previously uploaded file.
func TestLocalStorage_DeleteCV(t *testing.T) {
	dir := t.TempDir()
	ls := storage.LocalStorage{BaseDir: dir}

	data := []byte("fake cv content")
	_, err := ls.UploadCV(context.Background(), "del/test.pdf", bytes.NewReader(data), int64(len(data)), "application/pdf")
	require.NoError(t, err)

	err = ls.DeleteCV(context.Background(), "del/test.pdf")
	require.NoError(t, err)

	expectedPath := filepath.Join(dir, "del/test.pdf")
	_, statErr := os.Stat(expectedPath)
	assert.True(t, os.IsNotExist(statErr))
}

// TestS3Storage_UploadCV_Stub returns a mock URL without error.
func TestS3Storage_UploadCV_Stub(t *testing.T) {
	s3 := storage.S3Storage{}
	url, err := s3.UploadCV(context.Background(), "company/cv.pdf", bytes.NewReader([]byte("x")), 1, "application/pdf")
	require.NoError(t, err)
	assert.Equal(t, "s3://mock/company/cv.pdf", url)
}

// TestBolsaTrabajoStorage_InterfaceCompliance ensures both implementations satisfy the interface.
func TestBolsaTrabajoStorage_InterfaceCompliance(t *testing.T) {
	var _ storage.BolsaTrabajoStorage = storage.LocalStorage{}
	var _ storage.BolsaTrabajoStorage = storage.S3Storage{}
}
