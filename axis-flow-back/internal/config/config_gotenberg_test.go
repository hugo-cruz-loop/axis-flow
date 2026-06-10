package config_test

import (
	"os"
	"testing"

	"axis-flow-back/internal/config"
)

func setRequiredEnv(t *testing.T) func() {
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
	return func() {
		for k := range vars {
			os.Unsetenv(k)
		}
	}
}

func TestGotenbergConfigDefaults(t *testing.T) {
	cleanup := setRequiredEnv(t)
	defer cleanup()
	os.Unsetenv("PDF_ENGINE_ENDPOINT")
	os.Unsetenv("GOTENBERG_ENABLED")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.PDF.Endpoint != "http://pdf-engine:3000" {
		t.Errorf("expected default Endpoint=http://pdf-engine:3000, got %q", cfg.PDF.Endpoint)
	}
	if cfg.PDF.GotenbergEnabled != false {
		t.Errorf("expected GotenbergEnabled=false by default")
	}
}

func TestGotenbergConfigExplicit(t *testing.T) {
	cleanup := setRequiredEnv(t)
	defer cleanup()
	os.Setenv("PDF_ENGINE_ENDPOINT", "http://gotenberg:3000")
	os.Setenv("GOTENBERG_ENABLED", "true")
	defer func() {
		os.Unsetenv("PDF_ENGINE_ENDPOINT")
		os.Unsetenv("GOTENBERG_ENABLED")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.PDF.Endpoint != "http://gotenberg:3000" {
		t.Errorf("expected Endpoint=http://gotenberg:3000, got %q", cfg.PDF.Endpoint)
	}
	if !cfg.PDF.GotenbergEnabled {
		t.Errorf("expected GotenbergEnabled=true")
	}
}
