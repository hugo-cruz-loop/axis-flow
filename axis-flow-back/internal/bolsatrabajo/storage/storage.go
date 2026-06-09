// Package storage provides file-upload abstractions for BolsaDeTrabajo CVs.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BolsaTrabajoStorage is the port for CV file operations.
type BolsaTrabajoStorage interface {
	// UploadCV stores the CV file identified by key and returns a public URL.
	UploadCV(ctx context.Context, key string, r io.Reader, size int64, contentType string) (url string, err error)
	// DeleteCV removes the CV identified by key.
	DeleteCV(ctx context.Context, key string) error
}

// LocalStorage stores files on the local filesystem. Intended for development.
type LocalStorage struct {
	// BaseDir is the root directory for stored files.
	BaseDir string
}

// UploadCV writes the reader content to BaseDir/key and returns a local URL.
func (l LocalStorage) UploadCV(_ context.Context, key string, r io.Reader, _ int64, _ string) (string, error) {
	dest := filepath.Join(l.BaseDir, key)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("local storage mkdir: %w", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("local storage create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("local storage write: %w", err)
	}
	return "file://" + dest, nil
}

// DeleteCV removes the file at BaseDir/key.
func (l LocalStorage) DeleteCV(_ context.Context, key string) error {
	return os.Remove(filepath.Join(l.BaseDir, key))
}

// S3Storage is a stub implementation that always returns a mock URL.
// Real MinIO / AWS S3 wiring will be added in a later PR.
type S3Storage struct{}

// UploadCV returns a stub S3 URL without performing a real upload.
func (S3Storage) UploadCV(_ context.Context, key string, _ io.Reader, _ int64, _ string) (string, error) {
	return "s3://mock/" + key, nil
}

// DeleteCV is a no-op stub.
func (S3Storage) DeleteCV(_ context.Context, _ string) error {
	return nil
}

// Compile-time interface checks.
var (
	_ BolsaTrabajoStorage = LocalStorage{}
	_ BolsaTrabajoStorage = S3Storage{}
)
