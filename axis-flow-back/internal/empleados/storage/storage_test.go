package storage_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"axis-flow-back/internal/empleados/storage"
)

// --- ValidateMIME ---

var pdfMagic = []byte{0x25, 0x50, 0x44, 0x46} // %PDF
var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47} // \x89PNG
var jpgMagic = []byte{0xFF, 0xD8, 0xFF}       // JPEG SOI marker
var exeMagic = []byte{0x4D, 0x5A}             // MZ (Windows PE)

func TestValidateMIME_PDF(t *testing.T) {
	if err := storage.ValidateMIME(pdfMagic); err != nil {
		t.Fatalf("expected PDF to pass, got: %v", err)
	}
}

func TestValidateMIME_PNG(t *testing.T) {
	if err := storage.ValidateMIME(pngMagic); err != nil {
		t.Fatalf("expected PNG to pass, got: %v", err)
	}
}

func TestValidateMIME_JPG(t *testing.T) {
	if err := storage.ValidateMIME(jpgMagic); err != nil {
		t.Fatalf("expected JPG to pass, got: %v", err)
	}
}

func TestValidateMIME_EXE_Rejected(t *testing.T) {
	err := storage.ValidateMIME(exeMagic)
	if err == nil {
		t.Fatal("expected EXE to be rejected, got nil error")
	}
}

func TestValidateMIME_Empty_Rejected(t *testing.T) {
	err := storage.ValidateMIME([]byte{})
	if err == nil {
		t.Fatal("expected empty header to be rejected, got nil error")
	}
}

// --- ValidateSize ---

func TestValidateSize_WithinLimit(t *testing.T) {
	if err := storage.ValidateSize(1024, 5*1024*1024); err != nil {
		t.Fatalf("expected size within limit to pass, got: %v", err)
	}
}

func TestValidateSize_ExactLimit(t *testing.T) {
	limit := int64(5 * 1024 * 1024)
	if err := storage.ValidateSize(limit, limit); err != nil {
		t.Fatalf("expected exact limit to pass, got: %v", err)
	}
}

func TestValidateSize_OverLimit(t *testing.T) {
	limit := int64(5 * 1024 * 1024)
	err := storage.ValidateSize(limit+1, limit)
	if err == nil {
		t.Fatal("expected over-limit to be rejected, got nil error")
	}
}

// --- LocalStorage ---

func TestLocalStorage_Upload_CreateFile(t *testing.T) {
	dir := t.TempDir()
	ls := &storage.LocalStorage{BaseDir: dir}

	content := []byte("hello world")
	url, err := ls.Upload(context.Background(), "bucket", "test-key.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if url == "" {
		t.Fatal("Upload returned empty URL")
	}

	// File must exist on disk.
	dest := filepath.Join(dir, "bucket", "test-key.txt")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Fatalf("expected file at %s to exist", dest)
	}
}

func TestLocalStorage_Delete_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	ls := &storage.LocalStorage{BaseDir: dir}

	// First upload.
	_, err := ls.Upload(context.Background(), "bucket", "delete-me.txt", bytes.NewReader([]byte("x")), 1, "text/plain")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Then delete.
	if err := ls.Delete(context.Background(), "bucket", "delete-me.txt"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	dest := filepath.Join(dir, "bucket", "delete-me.txt")
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected file at %s to be removed", dest)
	}
}

func TestLocalStorage_Delete_NonExistentFile_NoError(t *testing.T) {
	dir := t.TempDir()
	ls := &storage.LocalStorage{BaseDir: dir}

	// Deleting a file that does not exist should not error.
	if err := ls.Delete(context.Background(), "bucket", "ghost.txt"); err != nil {
		t.Fatalf("expected no error deleting non-existent file, got: %v", err)
	}
}

// Compile-time interface assertion.
var _ storage.FileStorage = (*storage.LocalStorage)(nil)
