package config_test

import (
	"os"
	"testing"
	"time"

	"axis-flow-back/internal/config"
)

// setFormulariosRequiredEnv sets only the global required env vars
// (so config.Load() does not return "required" errors), leaving the
// formularios-specific env vars free for the test to set or unset.
func setFormulariosRequiredEnv(t *testing.T) {
	t.Helper()
	vars := map[string]string{
		"APP_ENV":         "test",
		"DB_HOST":         "localhost",
		"DB_PORT":         "5432",
		"DB_NAME":         "testdb",
		"DB_USER":         "test",
		"DB_PASSWORD":     "test",
		"JWT_SECRET":      "secret",
		"JWT_ACCESS_TTL":  "15m",
		"JWT_REFRESH_TTL": "168h",
		"REDIS_URL":       "redis://localhost:6379",
		"ENCRYPTION_KEY":  "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
	}
	for k, v := range vars {
		os.Setenv(k, v)
	}
}

func unsetFormulariosEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"FORMULARIOS_PORT",
		"FORMULARIOS_DATABASE_URL",
		"FORMULARIOS_S3_BUCKET",
		"FORMULARIOS_S3_REGION",
		"FORMULARIOS_AWS_ACCESS_KEY_ID",
		"FORMULARIOS_AWS_SECRET_ACCESS_KEY",
		"FORMULARIOS_S3_UPLOAD_TIMEOUT",
		"WKHTMLTOPDF_PATH",
		"WKHTMLTOPDF_TIMEOUT_SECONDS",
		"WKHTMLTOPDF_MAX_CONCURRENT",
	} {
		os.Unsetenv(k)
	}
}

// TestFormulariosConfigDefaults asserts sensible defaults for every
// formularios-specific env var when none are set.
func TestFormulariosConfigDefaults(t *testing.T) {
	setFormulariosRequiredEnv(t)
	defer unsetFormulariosEnv(t)
	unsetFormulariosEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	f := cfg.Formularios

	if f.Port != "8090" {
		t.Errorf("expected default Port=8090, got %q", f.Port)
	}
	if f.S3.Bucket != "" {
		t.Errorf("expected default S3.Bucket=\"\", got %q", f.S3.Bucket)
	}
	if f.S3.Region != "us-east-1" {
		t.Errorf("expected default S3.Region=us-east-1, got %q", f.S3.Region)
	}
	if f.S3.AccessKeyID != "" {
		t.Errorf("expected default S3.AccessKeyID=\"\", got %q", f.S3.AccessKeyID)
	}
	if f.S3.SecretAccessKey != "" {
		t.Errorf("expected default S3.SecretAccessKey=\"\", got %q", f.S3.SecretAccessKey)
	}
	if f.S3.UploadTimeout != 30*time.Second {
		t.Errorf("expected default S3.UploadTimeout=30s, got %s", f.S3.UploadTimeout)
	}
	if f.Wkhtmltopdf.Path != "/usr/bin/wkhtmltopdf" {
		t.Errorf("expected default Wkhtmltopdf.Path=/usr/bin/wkhtmltopdf, got %q", f.Wkhtmltopdf.Path)
	}
	if f.Wkhtmltopdf.TimeoutSeconds != 10 {
		t.Errorf("expected default Wkhtmltopdf.TimeoutSeconds=10, got %d", f.Wkhtmltopdf.TimeoutSeconds)
	}
	if f.Wkhtmltopdf.MaxConcurrent != 2 {
		t.Errorf("expected default Wkhtmltopdf.MaxConcurrent=2, got %d", f.Wkhtmltopdf.MaxConcurrent)
	}
	if f.DatabaseURL != "" {
		t.Errorf("expected default DatabaseURL=\"\", got %q", f.DatabaseURL)
	}
}

// TestFormulariosConfigExplicit asserts every formularios-specific env var
// overrides the corresponding default.
func TestFormulariosConfigExplicit(t *testing.T) {
	setFormulariosRequiredEnv(t)
	defer unsetFormulariosEnv(t)
	unsetFormulariosEnv(t)

	os.Setenv("FORMULARIOS_PORT", "9090")
	os.Setenv("FORMULARIOS_DATABASE_URL", "postgresql://form:pass@db:5432/formularios?search_path=formularios")
	os.Setenv("FORMULARIOS_S3_BUCKET", "axis-flow-formularios")
	os.Setenv("FORMULARIOS_S3_REGION", "us-west-2")
	os.Setenv("FORMULARIOS_AWS_ACCESS_KEY_ID", "AKIA-FORM")
	os.Setenv("FORMULARIOS_AWS_SECRET_ACCESS_KEY", "SECRET-FORM")
	os.Setenv("FORMULARIOS_S3_UPLOAD_TIMEOUT", "45s")
	os.Setenv("WKHTMLTOPDF_PATH", "/opt/wkhtmltopdf/bin/wkhtmltopdf")
	os.Setenv("WKHTMLTOPDF_TIMEOUT_SECONDS", "30")
	os.Setenv("WKHTMLTOPDF_MAX_CONCURRENT", "4")
	defer unsetFormulariosEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	f := cfg.Formularios
	if f.Port != "9090" {
		t.Errorf("expected Port=9090, got %q", f.Port)
	}
	if f.DatabaseURL != "postgresql://form:pass@db:5432/formularios?search_path=formularios" {
		t.Errorf("DatabaseURL not loaded: %q", f.DatabaseURL)
	}
	if f.S3.Bucket != "axis-flow-formularios" {
		t.Errorf("expected S3.Bucket=axis-flow-formularios, got %q", f.S3.Bucket)
	}
	if f.S3.Region != "us-west-2" {
		t.Errorf("expected S3.Region=us-west-2, got %q", f.S3.Region)
	}
	if f.S3.AccessKeyID != "AKIA-FORM" {
		t.Errorf("expected S3.AccessKeyID=AKIA-FORM, got %q", f.S3.AccessKeyID)
	}
	if f.S3.SecretAccessKey != "SECRET-FORM" {
		t.Errorf("expected S3.SecretAccessKey=SECRET-FORM, got %q", f.S3.SecretAccessKey)
	}
	if f.S3.UploadTimeout != 45*time.Second {
		t.Errorf("expected S3.UploadTimeout=45s, got %s", f.S3.UploadTimeout)
	}
	if f.Wkhtmltopdf.Path != "/opt/wkhtmltopdf/bin/wkhtmltopdf" {
		t.Errorf("expected Wkhtmltopdf.Path override, got %q", f.Wkhtmltopdf.Path)
	}
	if f.Wkhtmltopdf.TimeoutSeconds != 30 {
		t.Errorf("expected Wkhtmltopdf.TimeoutSeconds=30, got %d", f.Wkhtmltopdf.TimeoutSeconds)
	}
	if f.Wkhtmltopdf.MaxConcurrent != 4 {
		t.Errorf("expected Wkhtmltopdf.MaxConcurrent=4, got %d", f.Wkhtmltopdf.MaxConcurrent)
	}
}
