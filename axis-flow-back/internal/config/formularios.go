package config

import (
	"os"
	"strconv"
	"time"
)

// FormulariosConfig holds runtime settings specific to the Formularios
// microservice. It is loaded by Load() from environment variables prefixed
// with FORMULARIOS_ (or WKHTMLTOPDF_* for the PDF binary settings, per the
// service specification).
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

	// Wkhtmltopdf holds the PDF rendering subprocess configuration.
	Wkhtmltopdf WkhtmltopdfConfig
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

// WkhtmltopdfConfig holds the PDF rendering subprocess configuration.
// All env var names match the spec verbatim (no FORMULARIOS_ prefix) so
// operators can reuse the values defined in the spec config table.
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
