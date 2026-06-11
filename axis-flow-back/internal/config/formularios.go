package config

import (
	"os"
	"strconv"
	"time"
)

// FormulariosConfig holds runtime settings specific to the Formularios
// microservice. It is loaded by Load() from environment variables prefixed
// with FORMULARIOS_ (or WKHTMLTOPDF_* for the legacy PDF binary settings,
// which are kept for backward compat with PR-1 env files but no longer
// read by the service — PR-6 uses Gotenberg HTTP instead).
//
// Spec reference: docs/services/10_Formularios_Service_Spec/specification.md §Configuración.
type FormulariosConfig struct {
	// Port is the HTTP server port for the formularios microservice.
	// Loaded from FORMULARIOS_PORT. Default: "8090".
	Port string

	// DatabaseURL is an optional per-service DSN override. If non-empty it
	// takes precedence over the global DB config. Loaded from
	// FORMULARIOS_DATABASE_URL. Default: "".
	DatabaseURL string

	// S3 holds the AWS S3 evidence/report bucket configuration.
	S3 FormulariosS3Config

	// Wkhtmltopdf holds the LEGACY PDF rendering subprocess
	// configuration. The struct and env vars are kept for
	// backward compatibility with the PR-1 env files, but the
	// service no longer reads them — PR-6 (PDF/S3 Hardening)
	// uses Gotenberg HTTP. The WkhtmltopdfConfig doc carries a
	// Deprecated notice.
	Wkhtmltopdf WkhtmltopdfConfig

	// LockTTL is the TTL for the Redis S3 report lock
	// (formularios:reporte:lock:<iniciadoID>). The PDF service
	// acquires the lock at the start of GenerateReporte and
	// releases it via a safe-release Lua script. PR-6 (6.3) —
	// replaces the previous hard-coded 300s.
	// Loaded from FORMULARIOS_LOCK_TTL. Default: 5m.
	LockTTL time.Duration
}

// FormulariosS3Config holds the S3 bucket, region, and credentials used
// by the formularios service to store evidence and generated reports.
type FormulariosS3Config struct {
	// Bucket is the S3 bucket name. Loaded from FORMULARIOS_S3_BUCKET.
	// Default: "".
	Bucket string
	// Region is the AWS region. Loaded from FORMULARIOS_S3_REGION.
	// Default: "us-east-1".
	Region string
	// AccessKeyID is the AWS access key. Loaded from
	// FORMULARIOS_AWS_ACCESS_KEY_ID. NEVER log this value.
	AccessKeyID string
	// SecretAccessKey is the AWS secret key. Loaded from
	// FORMULARIOS_AWS_SECRET_ACCESS_KEY. NEVER log this value.
	SecretAccessKey string
	// UploadTimeout bounds a single S3 PutObject. Loaded from
	// FORMULARIOS_S3_UPLOAD_TIMEOUT. Default: 30s.
	UploadTimeout time.Duration
}

// WkhtmltopdfConfig holds the LEGACY PDF rendering subprocess
// configuration.
//
// Deprecated: PR-6 (PDF/S3 Hardening) replaces wkhtmltopdf with
// Gotenberg HTTP. The struct and the env vars (WKHTMLTOPDF_PATH,
// WKHTMLTOPDF_TIMEOUT_SECONDS, WKHTMLTOPDF_MAX_CONCURRENT) are
// kept for backward compatibility with the PR-1 env files but
// the service no longer reads them. Operators should migrate to
// PDF_ENGINE_ENDPOINT / GOTENBERG_ENABLED (see config.PDF).
type WkhtmltopdfConfig struct {
	// Path is the absolute path to the wkhtmltopdf binary.
	// Loaded from WKHTMLTOPDF_PATH. Default: "/usr/bin/wkhtmltopdf".
	Path string
	// TimeoutSeconds is the timeout for a single render process.
	// Loaded from WKHTMLTOPDF_TIMEOUT_SECONDS. Default: 10.
	TimeoutSeconds int
	// MaxConcurrent caps the number of simultaneous wkhtmltopdf processes.
	// Loaded from WKHTMLTOPDF_MAX_CONCURRENT. Default: 2.
	MaxConcurrent int
}

// loadFormulariosConfig builds a FormulariosConfig from the current
// process environment. It never returns an error; missing env vars fall
// back to documented defaults. The caller is responsible for any
// "required" validation specific to a deployment.
func loadFormulariosConfig() FormulariosConfig {
	return FormulariosConfig{
		Port:        getenvDefault("FORMULARIOS_PORT", "8090"),
		DatabaseURL: os.Getenv("FORMULARIOS_DATABASE_URL"),
		S3: FormulariosS3Config{
			Bucket:          os.Getenv("FORMULARIOS_S3_BUCKET"),
			Region:          getenvDefault("FORMULARIOS_S3_REGION", "us-east-1"),
			AccessKeyID:     os.Getenv("FORMULARIOS_AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("FORMULARIOS_AWS_SECRET_ACCESS_KEY"),
			UploadTimeout:   parseDurationDefault("FORMULARIOS_S3_UPLOAD_TIMEOUT", 30*time.Second),
		},
		Wkhtmltopdf: WkhtmltopdfConfig{
			Path:           getenvDefault("WKHTMLTOPDF_PATH", "/usr/bin/wkhtmltopdf"),
			TimeoutSeconds: parseIntDefault("WKHTMLTOPDF_TIMEOUT_SECONDS", 10),
			MaxConcurrent:  parseIntDefault("WKHTMLTOPDF_MAX_CONCURRENT", 2),
		},
		// PR-6 (6.3): FORMULARIOS_LOCK_TTL — the Redis SET-NX-with-TTL
		// lock on formularios:reporte:lock:<iniciadoID>. Default
		// 5 minutes matches the previous hard-coded value.
		LockTTL: parseDurationDefault("FORMULARIOS_LOCK_TTL", 5*time.Minute),
	}
}

// getenvDefault returns the value of key, or fallback if unset/empty.
func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseIntDefault returns the int value of key, or fallback if unset,
// empty, or unparsable.
func parseIntDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// parseDurationDefault returns the time.Duration value of key, or fallback
// if unset, empty, or unparsable.
func parseDurationDefault(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
