// Package reports — runtime configuration for the Reports service.
package reports

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds all runtime configuration consumed by the Reports service.
//
// Security contract:
//   - GCPGeocodingAPIKey MUST NEVER appear in logs or error strings.
//     Only log whether it is set (present/absent), never its value.
//   - AWSS3CustomDomain is a public CDN hostname; treat it as non-sensitive.
type Config struct {
	// AWSS3CustomDomain is the CDN base URL used to build S3 media URLs.
	// Defaults to "https://cdn.checkon.mx" when unset.
	AWSS3CustomDomain string
	// GCPGeocodingAPIKey is the Google Cloud Geocoding API key.
	// NEVER log this value.
	GCPGeocodingAPIKey string
	// GeocodingCacheTTLDays is the number of days a geocoded address is
	// kept in the PostgreSQL cache before being treated as stale.
	// Default: 30.
	GeocodingCacheTTLDays int
	// GeocodingRateLimitRPM is the maximum number of live GCP Geocoding
	// requests allowed per minute. Default: 60.
	GeocodingRateLimitRPM int
}

// ConfigFromEnv reads environment variables and returns a populated Config.
// Missing or unparseable values fall back to the documented defaults.
// A WARNING is logged (without the key value) when GCPGeocodingAPIKey is
// absent — the service will operate in cache-only geocoding mode.
func ConfigFromEnv() Config {
	cfg := Config{
		AWSS3CustomDomain:     envDefault("AWS_S3_CUSTOM_DOMAIN", "https://cdn.checkon.mx"),
		GCPGeocodingAPIKey:    os.Getenv("GCP_GEOCODING_API_KEY"),
		GeocodingCacheTTLDays: parseInt("GEOCODING_CACHE_TTL_DAYS", 30),
		GeocodingRateLimitRPM: parseInt("GEOCODING_RATE_LIMIT_RPM", 60),
	}

	if cfg.GCPGeocodingAPIKey == "" {
		slog.Warn("reports: GCP_GEOCODING_API_KEY is not set — geocoding will operate in cache-only mode")
	}

	return cfg
}

// ---------------------------------------------------------------------------
// Internal helpers.
// ---------------------------------------------------------------------------

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
