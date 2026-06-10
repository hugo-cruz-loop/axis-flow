// Package storage provides file upload/delete abstractions for the empleados module.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"axis-flow-back/internal/empleados"
)

// FileStorage is the port for uploading and deleting employee files.
type FileStorage interface {
	// Upload stores the content from r using the given bucket and key.
	// Returns the URL where the file can be accessed.
	Upload(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) (url string, err error)
	// Delete removes the file identified by bucket and key.
	Delete(ctx context.Context, bucket, key string) error
}

// ValidateMIME inspects the magic bytes of a file header and returns an error
// if the type is not PDF, PNG, or JPG.
//
//	PDF: %PDF   (25 50 44 46)
//	PNG: \x89PNG (89 50 4E 47)
//	JPG: \xFF\xD8\xFF (FF D8 FF)
func ValidateMIME(header []byte) error {
	if len(header) >= 4 {
		if header[0] == 0x25 && header[1] == 0x50 && header[2] == 0x44 && header[3] == 0x46 {
			return nil // PDF
		}
		if header[0] == 0x89 && header[1] == 0x50 && header[2] == 0x4E && header[3] == 0x47 {
			return nil // PNG
		}
	}
	if len(header) >= 3 {
		if header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF {
			return nil // JPG
		}
	}
	return empleados.ErrInvalidMIMEType
}

// ValidateSize returns ErrFileTooLarge when size exceeds maxBytes.
func ValidateSize(size int64, maxBytes int64) error {
	if size > maxBytes {
		return empleados.ErrFileTooLarge
	}
	return nil
}

// LocalStorage stores files on disk under BaseDir. Intended for dev and test only.
// Layout: {BaseDir}/{bucket}/{key}
type LocalStorage struct {
	BaseDir string
}

// Upload writes r to disk at {BaseDir}/{bucket}/{key} and returns a file:// URL.
func (l *LocalStorage) Upload(_ context.Context, bucket, key string, r io.Reader, _ int64, _ string) (string, error) {
	dest := filepath.Join(l.BaseDir, bucket, key)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("storage: create dirs: %w", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("storage: create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("storage: write file: %w", err)
	}
	return "file://" + dest, nil
}

// Delete removes the file at {BaseDir}/{bucket}/{key}.
// If the file does not exist, Delete returns nil (idempotent).
func (l *LocalStorage) Delete(_ context.Context, bucket, key string) error {
	dest := filepath.Join(l.BaseDir, bucket, key)
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: delete file: %w", err)
	}
	return nil
}
