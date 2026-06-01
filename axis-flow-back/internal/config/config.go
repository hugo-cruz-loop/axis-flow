// Package config loads and validates application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the service.
type Config struct {
	AppEnv        string
	Server        ServerConfig
	DB            DBConfig
	JWT           JWTConfig
	Redis         RedisConfig
	Feature       FeatureConfig
	Observability ObservabilityConfig
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port string
}

// DBConfig contains PostgreSQL connection settings.
type DBConfig struct {
	Host         string
	Port         string
	Name         string
	User         string
	Password     string
	MaxOpenConns int
	MaxIdleConns int
}

// JWTConfig contains JWT signing and TTL settings.
type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// RedisConfig contains Redis connection settings.
type RedisConfig struct {
	URL string
}

// FeatureConfig holds feature-flag settings.
type FeatureConfig struct {
	DeleteAllData bool
}

// ObservabilityConfig holds telemetry and observability settings.
type ObservabilityConfig struct {
	OTELExporterEndpoint string
	MetricsEnabled       bool
	LogLevel             string
	LogFormat            string
	SentryDSN            string
}

// Load reads all required environment variables and returns a validated Config.
// It returns an error listing every missing or invalid variable.
// It first attempts to load environment variables from .env.local file if it exists.
func Load() (*Config, error) {
	// Load from .env.local if it exists
	envPath := filepath.Join(".", ".env.local")
	if _, err := os.Stat(envPath); err == nil {
		// File exists, try to load it
		if err := godotenv.Load(envPath); err != nil {
			// Log warning but don't fail if .env.local can't be read
			fmt.Printf("WARNING: Could not load .env.local: %v\n", err)
		}
	}

	var errs []string

	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Sprintf("%s is required", key))
		}
		return v
	}

	requireDuration := func(key string) time.Duration {
		raw := require(key)
		if raw == "" {
			return 0
		}
		d, err := time.ParseDuration(raw)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s must be a valid duration (e.g. 15m): %v", key, err))
		}
		return d
	}

	requireInt := func(key string, fallback int) int {
		raw := os.Getenv(key)
		if raw == "" {
			return fallback
		}
		v, err := strconv.Atoi(raw)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s must be an integer: %v", key, err))
			return fallback
		}
		return v
	}

	cfg := &Config{
		AppEnv: require("APP_ENV"),
		Server: ServerConfig{
			Port: func() string {
				if p := os.Getenv("SERVER_PORT"); p != "" {
					return p
				}
				return "8080"
			}(),
		},
		DB: DBConfig{
			Host:         require("DB_HOST"),
			Port:         require("DB_PORT"),
			Name:         require("DB_NAME"),
			User:         require("DB_USER"),
			Password:     require("DB_PASSWORD"),
			MaxOpenConns: requireInt("DB_MAX_OPEN_CONNS", 10),
			MaxIdleConns: requireInt("DB_MAX_IDLE_CONNS", 5),
		},
		JWT: JWTConfig{
			Secret:     require("JWT_SECRET"),
			AccessTTL:  requireDuration("JWT_ACCESS_TTL"),
			RefreshTTL: requireDuration("JWT_REFRESH_TTL"),
		},
		Redis: RedisConfig{
			URL: require("REDIS_URL"),
		},
		Feature: FeatureConfig{
			DeleteAllData: os.Getenv("FEATURE_DELETE_ALL_DATA") == "true",
		},
		Observability: ObservabilityConfig{
			OTELExporterEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
			MetricsEnabled: func() bool {
				if v := os.Getenv("METRICS_ENABLED"); v == "false" {
					return false
				}
				return true
			}(),
			LogLevel: func() string {
				if v := os.Getenv("LOG_LEVEL"); v != "" {
					return v
				}
				return "info"
			}(),
			LogFormat: func() string {
				if v := os.Getenv("LOG_FORMAT"); v != "" {
					return v
				}
				return "logfmt"
			}(),
			SentryDSN: os.Getenv("SENTRY_DSN"),
		},
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration errors: %v", errs)
	}

	return cfg, nil
}

// DSN returns a pgx-compatible connection string.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		d.Host, d.Port, d.Name, d.User, d.Password,
	)
}
