// Package service — storage_s3.go: the S3ReportStorage transport
// that satisfies the service.ReportStorage port using the AWS
// SDK for Go v2 (already wired in go.mod: aws-sdk-go-v2/service/s3
// v1.103.3).
//
// PR-6 (6.2) implementation. The storage is intentionally
// transport-only — the upload-timeouts, key validation, and
// error-surfacing conventions are the only logic that lives here;
// the S3 client construction (and the IAM-default credential
// chain) lives in NewS3ClientFromConfig so a future change to the
// auth path does not require touching the upload path.
//
// Security guarantees (spec §Hardening the wkhtmltopdf PDF
// Generator, mirrored in the S3 transport per PR-6's "PDF/S3
// Hardening" scope):
//
//   - Keys are validated against path-traversal (`..`), absolute
//     paths (leading `/`), and `file://` schemes BEFORE the S3
//     call. Any of these returns formularios.ErrInvalidInput and
//     the S3 API is never invoked.
//   - All PutObject calls include:
//     * ContentType=application/pdf
//     * ServerSideEncryption=AES256
//     * Metadata["source"]="formularios" (SRE cost attribution)
//   - The returned URL is the canonical s3:// form (NOT a
//     presigned URL). The handler can layer presigning on top
//     for downloads if/when needed.
//   - Any AWS error is surfaced as formularios.ErrInternal with
//     a generic message — NO bucket name, NO key, NO AWS error
//     code in the surfaced error.
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"axis-flow-back/internal/formularios"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ---------------------------------------------------------------------------
// FormulariosS3ConfigForTest is the shape the helper
// NewS3ClientFromConfig takes. It is a thin DTO decoupled from
// the config.FormulariosS3Config type (which lives in
// internal/config — the service package cannot import config
// without creating a cycle via main.go). The two are
// mirror-shaped: PR-6 step 6.4 wires them at the main.go boundary.
// ---------------------------------------------------------------------------

// FormulariosS3ConfigForTest is the parameter shape for
// NewS3ClientFromConfig. In production it is constructed from
// config.FormulariosS3Config in cmd/server/formularios_module.go.
// The "_ForTest" suffix signals "DTO" — the test in
// storage_s3_test.go uses it directly to assert the helper's
// behavior without spinning up a real config.Load.
type FormulariosS3ConfigForTest struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	UploadTimeout   time.Duration
}

// ---------------------------------------------------------------------------
// S3ReportStorage
// ---------------------------------------------------------------------------

// S3ReportStorage uploads PDF bytes to S3 and returns the
// canonical s3:// URL. The transport uses a real *s3.Client
// (injected via the constructor) so tests can point at a fake
// httptest server.
type S3ReportStorage struct {
	client    s3APIClient
	bucket    string
	keyPrefix string
	timeout   time.Duration
}

// s3APIClient is the minimal S3 surface we use; lets tests inject
// a real *s3.Client (production) or a fake (tests). The interface
// matches the one in internal/asignacion/storage to keep the
// patterns aligned.
type s3APIClient interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// NewS3ReportStorage constructs an S3-backed report storage.
// timeout is the per-upload deadline; the underlying AWS SDK
// call wraps the caller's context with this timeout to bound
// the latency of a single PutObject.
func NewS3ReportStorage(client *s3.Client, bucket, keyPrefix string, timeout time.Duration) *S3ReportStorage {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &S3ReportStorage{
		client:    client,
		bucket:    bucket,
		keyPrefix: keyPrefix,
		timeout:   timeout,
	}
}

// Upload satisfies the ReportStorage port. Validates the key,
// wraps the context with the configured timeout, calls
// PutObject with the spec's required args, and returns the
// canonical s3:// URL.
//
// Error surfacing:
//   - Invalid key (path traversal, absolute, file://, empty)
//     → formularios.ErrInvalidInput (no S3 call).
//   - AWS error → formularios.ErrInternal with a generic
//     "pdf: upload failed" prefix; the underlying error is
//     captured in a slog.Error record at the service layer
//     (this transport is intentionally log-free to keep the
//     surfacing convention single-sourced).
func (s *S3ReportStorage) Upload(ctx context.Context, key string, body []byte) (string, error) {
	if err := validateS3Key(key); err != nil {
		return "", err
	}

	opCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_, err := s.client.PutObject(opCtx, &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		Key:                  aws.String(s.keyPrefix + key),
		Body:                 bytes.NewReader(body),
		ContentType:          aws.String("application/pdf"),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		Metadata: map[string]string{
			"source": "formularios",
		},
	})
	if err != nil {
		return "", fmt.Errorf("%w: %s", formularios.ErrInternal, "pdf: upload failed")
	}

	return fmt.Sprintf("s3://%s/%s%s", s.bucket, s.keyPrefix, key), nil
}

// ---------------------------------------------------------------------------
// validateS3Key
// ---------------------------------------------------------------------------

// validateS3Key enforces the spec's "no PII / no path traversal /
// no file://" guarantees for the S3 key. Returns
// formularios.ErrInvalidInput on rejection.
//
// Defense-in-depth: even though the key is constructed in-process
// (from `buildReporteS3Key(iniciadoID)`, which uses a fixed
// "formularios/reports/<uuid>.pdf" shape), a future call site
// (e.g. a presigned download handler) might accept user input
// for the key; this validator is the last line of defense.
func validateS3Key(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty key", formularios.ErrInvalidInput)
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: absolute key", formularios.ErrInvalidInput)
	}
	if strings.Contains(key, "..") {
		return fmt.Errorf("%w: path traversal", formularios.ErrInvalidInput)
	}
	if strings.Contains(strings.ToLower(key), "file://") {
		return fmt.Errorf("%w: file scheme", formularios.ErrInvalidInput)
	}
	// Reject any URL-encoded variant of the above (%2e%2e for ..,
	// %2f for /, %3a for :, etc.). A real attacker who controls
	// the key would normalize the encoding before sending the
	// S3 API call, so we normalize here.
	decoded, err := url.QueryUnescape(key)
	if err == nil && decoded != key {
		if strings.Contains(decoded, "..") || strings.Contains(decoded, "://") {
			return fmt.Errorf("%w: encoded traversal", formularios.ErrInvalidInput)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// NewS3ClientFromConfig — AWS client construction
// ---------------------------------------------------------------------------

// NewS3ClientFromConfig builds a *s3.Client from the formularios
// DTO. The credential chain is:
//
//   1. Static AccessKeyID/SecretAccessKey if both are set (local
//      dev / MinIO). The credentials are passed via
//      awsconfig.WithCredentialsProvider — they are NEVER logged.
//   2. The default AWS credential chain (IAM role attached to
//      the EC2 instance / ECS task / EKS pod) when no static
//      keys are set. This is the production path; deployment
//      MUST use IAM role + bucket policy.
//
// The function never returns an error at construction time
// (the IAM chain resolves lazily on the first API call). It
// returns an error only if the static credentials are
// malformed (empty AccessKeyID with a non-empty
// SecretAccessKey, or vice versa).
func NewS3ClientFromConfig(ctx context.Context, cfg FormulariosS3ConfigForTest) (*s3.Client, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}

	// Static-key path (dev / MinIO / staging without IAM).
	hasAccess := cfg.AccessKeyID != ""
	hasSecret := cfg.SecretAccessKey != ""
	if hasAccess != hasSecret {
		return nil, errors.New("s3: AccessKeyID and SecretAccessKey must be set together")
	}
	if hasAccess && hasSecret {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}
	// If neither is set, fall through to the default credential
	// chain (env vars, ~/.aws/credentials, EC2/ECS IAM role).
	// The AWS SDK resolves lazily on the first API call.

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}
	return s3.NewFromConfig(awsCfg), nil
}
