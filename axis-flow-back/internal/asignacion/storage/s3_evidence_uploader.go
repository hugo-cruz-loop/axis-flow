// Package storage holds real storage transport implementations for
// Asignacion service artifacts. The current slice ships an S3-backed
// evidence uploader; future slices may add MinIO, GCS, or local-disk
// alternatives without changing the EvidenceUploader port.
package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"

	"axis-flow-back/internal/asignacion"
)

const (
	defaultEvidenceBucket    = "checkon-evidences"
	defaultEvidenceKeyPrefix = "evidencia"
	defaultPublicBaseURL     = "https://checkon-evidences.s3.amazonaws.com"
	defaultUploadTimeout     = 10 * time.Second
)

// S3EvidenceUploaderConfig tunes the S3 evidence uploader.
type S3EvidenceUploaderConfig struct {
	Bucket         string
	KeyPrefix      string
	PublicBaseURL  string
	UploadTimeout  time.Duration
	MaxObjectBytes int64
}

// s3APIClient is the minimal S3 surface we use; lets tests inject a fake.
type s3APIClient interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3EvidenceUploader uploads one evidence payload to S3 and returns the
// canonical public URL that callers persist alongside the activity.
type S3EvidenceUploader struct {
	client  s3APIClient
	cfg     S3EvidenceUploaderConfig
	keyFunc func(activityID uuid.UUID, slot int, ext string) string
}

// NewS3EvidenceUploader creates an S3-backed evidence uploader. Defaults
// are applied for empty configuration values.
func NewS3EvidenceUploader(client s3APIClient, cfg S3EvidenceUploaderConfig) *S3EvidenceUploader {
	if cfg.Bucket == "" {
		cfg.Bucket = defaultEvidenceBucket
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = defaultEvidenceKeyPrefix
	}
	if cfg.PublicBaseURL == "" {
		cfg.PublicBaseURL = defaultPublicBaseURL
	}
	if cfg.UploadTimeout <= 0 {
		cfg.UploadTimeout = defaultUploadTimeout
	}
	if cfg.MaxObjectBytes <= 0 {
		cfg.MaxObjectBytes = asignacion.MaxEvidenceFileSize
	}
	uploader := &S3EvidenceUploader{client: client, cfg: cfg}
	uploader.keyFunc = uploader.buildKey
	return uploader
}

// Upload reads up to MaxObjectBytes+1 from content; if the payload exceeds
// the cap it returns asignacion.ErrInvalidEvidenceFile. Otherwise it
// uploads to S3 with public-read ACL and returns the canonical URL.
func (u *S3EvidenceUploader) Upload(ctx context.Context, activityID uuid.UUID, slot int, content io.Reader, contentType, filename string) (string, error) {
	if content == nil {
		return "", fmt.Errorf("s3_uploader.Upload: content is nil")
	}

	body, err := io.ReadAll(io.LimitReader(content, u.cfg.MaxObjectBytes+1))
	if err != nil {
		return "", fmt.Errorf("s3_uploader.Upload read: %w", err)
	}
	if int64(len(body)) > u.cfg.MaxObjectBytes {
		return "", fmt.Errorf("%w: payload exceeds %d bytes", asignacion.ErrInvalidEvidenceFile, u.cfg.MaxObjectBytes)
	}

	ext := extensionFromFilename(filename)
	key := u.keyFunc(activityID, slot, ext)

	opCtx, cancel := context.WithTimeout(ctx, u.cfg.UploadTimeout)
	defer cancel()

	_, err = u.client.PutObject(opCtx, &s3.PutObjectInput{
		Bucket:      &u.cfg.Bucket,
		Key:         &key,
		Body:        bytesReader(body),
		ContentType: &contentType,
		ACL:         types.ObjectCannedACLPublicRead,
	})
	if err != nil {
		return "", fmt.Errorf("s3_uploader.Upload put: %w", err)
	}

	return strings.TrimRight(u.cfg.PublicBaseURL, "/") + "/" + key, nil
}

func (u *S3EvidenceUploader) buildKey(activityID uuid.UUID, slot int, ext string) string {
	return fmt.Sprintf("%s/%d_%s.%s", u.cfg.KeyPrefix, slot, activityID.String(), ext)
}

func extensionFromFilename(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "png"
	case strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".jpg"):
		return "jpg"
	default:
		return "jpg"
	}
}

// bytesReader is a tiny adapter so we don't pull in bytes in this file's import surface here.
type readerFromBytes []byte

func (b readerFromBytes) Read(p []byte) (int, error) {
	n := copy(p, b)
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func bytesReader(b []byte) io.Reader {
	return readerFromBytes(b)
}
