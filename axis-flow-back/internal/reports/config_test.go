// Package reports_test contains black-box tests for reports config loading.
package reports_test

import (
	"testing"

	"axis-flow-back/internal/reports"

	"github.com/stretchr/testify/assert"
)

// TestConfigFromEnv_Defaults verifies that ConfigFromEnv applies documented
// defaults when no environment variables are set.
func TestConfigFromEnv_Defaults(t *testing.T) {
	// Clear all relevant env vars before each subtest.
	t.Setenv("AWS_S3_CUSTOM_DOMAIN", "")
	t.Setenv("GCP_GEOCODING_API_KEY", "")
	t.Setenv("GEOCODING_CACHE_TTL_DAYS", "")
	t.Setenv("GEOCODING_RATE_LIMIT_RPM", "")

	cfg := reports.ConfigFromEnv()

	assert.Equal(t, "https://cdn.checkon.mx", cfg.AWSS3CustomDomain)
	assert.Equal(t, 30, cfg.GeocodingCacheTTLDays)
	assert.Equal(t, 60, cfg.GeocodingRateLimitRPM)
	// Key should be empty when env var is not set.
	assert.Equal(t, "", cfg.GCPGeocodingAPIKey)
}

// TestConfigFromEnv_CustomDomain verifies that AWS_S3_CUSTOM_DOMAIN overrides
// the built-in default.
func TestConfigFromEnv_CustomDomain(t *testing.T) {
	t.Setenv("AWS_S3_CUSTOM_DOMAIN", "https://custom.example.com")
	t.Setenv("GCP_GEOCODING_API_KEY", "")
	t.Setenv("GEOCODING_CACHE_TTL_DAYS", "")
	t.Setenv("GEOCODING_RATE_LIMIT_RPM", "")

	cfg := reports.ConfigFromEnv()
	assert.Equal(t, "https://custom.example.com", cfg.AWSS3CustomDomain)
}

// TestConfigFromEnv_GCPKeyPresence verifies that GCPGeocodingAPIKey is loaded
// from the environment. The test confirms presence without ever printing the
// value to test output.
func TestConfigFromEnv_GCPKeyPresence(t *testing.T) {
	const testKey = "test-api-key-sentinel"
	t.Setenv("GCP_GEOCODING_API_KEY", testKey)
	t.Setenv("AWS_S3_CUSTOM_DOMAIN", "")
	t.Setenv("GEOCODING_CACHE_TTL_DAYS", "")
	t.Setenv("GEOCODING_RATE_LIMIT_RPM", "")

	cfg := reports.ConfigFromEnv()

	// We assert the key is present (non-empty) without logging its value.
	assert.NotEmpty(t, cfg.GCPGeocodingAPIKey, "GCPGeocodingAPIKey should be set when env var is present")
}

// TestConfigFromEnv_IntParsing verifies that integer env vars are parsed and
// applied correctly.
func TestConfigFromEnv_IntParsing(t *testing.T) {
	t.Setenv("AWS_S3_CUSTOM_DOMAIN", "")
	t.Setenv("GCP_GEOCODING_API_KEY", "")
	t.Setenv("GEOCODING_CACHE_TTL_DAYS", "90")
	t.Setenv("GEOCODING_RATE_LIMIT_RPM", "120")

	cfg := reports.ConfigFromEnv()

	assert.Equal(t, 90, cfg.GeocodingCacheTTLDays)
	assert.Equal(t, 120, cfg.GeocodingRateLimitRPM)
}

// TestConfigFromEnv_InvalidIntFallsBackToDefault verifies that an unparseable
// integer env var causes ConfigFromEnv to apply the documented default instead
// of crashing.
func TestConfigFromEnv_InvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("AWS_S3_CUSTOM_DOMAIN", "")
	t.Setenv("GCP_GEOCODING_API_KEY", "")
	t.Setenv("GEOCODING_CACHE_TTL_DAYS", "not-a-number")
	t.Setenv("GEOCODING_RATE_LIMIT_RPM", "")

	cfg := reports.ConfigFromEnv()

	assert.Equal(t, 30, cfg.GeocodingCacheTTLDays)
}
