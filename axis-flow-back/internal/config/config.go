// Package config loads and validates application configuration from environment variables.
package config

import (
	"encoding/hex"
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
	Stripe        StripeConfig
	Observability ObservabilityConfig
	// EncryptionKey is a 32-byte AES-256 key hex-encoded (64 hex chars).
	// Loaded from ENCRYPTION_KEY env var. Required. Never logged.
	EncryptionKey string
	Storage       StorageConfig
	// PDF holds configuration for the PDF generation service (Gotenberg).
	PDF PDFConfig
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

// StripeConfig holds Stripe payment integration settings.
type StripeConfig struct {
	SecretKey     string // STRIPE_SECRET_KEY, required when StripeEnabled=true
	WebhookSecret string // STRIPE_WEBHOOK_SECRET, required when StripeEnabled=true
	Enabled       bool   // STRIPE_ENABLED default false (allows dev without Stripe)
}

// ObservabilityConfig holds telemetry and observability settings.
type ObservabilityConfig struct {
	OTELExporterEndpoint string
	MetricsEnabled       bool
	LogLevel             string
	LogFormat            string
	SentryDSN            string
}

// PDFConfig holds settings for the Gotenberg PDF generation service.
type PDFConfig struct {
	// Endpoint is the base URL of the Gotenberg / pdf-engine service.
	// Loaded from PDF_ENGINE_ENDPOINT. Default: http://pdf-engine:3000.
	Endpoint string
	// GotenbergEnabled controls whether real PDF generation is used.
	// When false a mock (static bytes) is returned. Default: false.
	// Loaded from GOTENBERG_ENABLED.
	GotenbergEnabled bool
}

// StorageConfig holds file storage and biometric service settings.
type StorageConfig struct {
	// MinIOEndpoint is the MinIO server address. Empty = use LocalStorage.
	MinIOEndpoint string
	// MinIOBucket is the target bucket for employee files.
	MinIOBucket string
	// AWSRegion is the AWS region for Rekognition calls.
	AWSRegion string
	// AWSAccessKeyID is required when RekognitionEnabled is true. NEVER LOG.
	AWSAccessKeyID string
	// AWSSecretAccessKey is required when RekognitionEnabled is true. NEVER LOG.
	AWSSecretAccessKey string
	// RekognitionEnabled enables AWS Rekognition face comparison. Default false.
	RekognitionEnabled bool
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

	// Stripe config
	stripeEnabled := os.Getenv("STRIPE_ENABLED") == "true"
	cfg.Stripe = StripeConfig{
		Enabled:       stripeEnabled,
		SecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		WebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
	if stripeEnabled {
		if cfg.Stripe.SecretKey == "" {
			errs = append(errs, "STRIPE_SECRET_KEY is required when STRIPE_ENABLED=true")
		}
		if cfg.Stripe.WebhookSecret == "" {
			errs = append(errs, "STRIPE_WEBHOOK_SECRET is required when STRIPE_ENABLED=true")
		}
	}

	// Storage / file upload / biometric config.
	rekognitionEnabled := os.Getenv("AWS_REKOGNITION_ENABLED") == "true"
	cfg.Storage = StorageConfig{
		MinIOEndpoint: os.Getenv("MINIO_ENDPOINT"),
		MinIOBucket: func() string {
			if v := os.Getenv("MINIO_BUCKET"); v != "" {
				return v
			}
			return "checkon-empleados"
		}(),
		AWSRegion: func() string {
			if v := os.Getenv("AWS_REGION"); v != "" {
				return v
			}
			return "us-east-1"
		}(),
		// AWSAccessKeyID and AWSSecretAccessKey are sensitive — NEVER LOG.
		AWSAccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		RekognitionEnabled: rekognitionEnabled,
	}
	if rekognitionEnabled {
		if cfg.Storage.AWSAccessKeyID == "" {
			errs = append(errs, "AWS_ACCESS_KEY_ID is required when AWS_REKOGNITION_ENABLED=true")
		}
		if cfg.Storage.AWSSecretAccessKey == "" {
			errs = append(errs, "AWS_SECRET_ACCESS_KEY is required when AWS_REKOGNITION_ENABLED=true")
		}
	}

	// PDF / Gotenberg config.
	cfg.PDF = PDFConfig{
		Endpoint: func() string {
			if v := os.Getenv("PDF_ENGINE_ENDPOINT"); v != "" {
				return v
			}
			return "http://pdf-engine:3000"
		}(),
		GotenbergEnabled: os.Getenv("GOTENBERG_ENABLED") == "true",
	}

	// EncryptionKey: required, must decode to exactly 32 bytes.
	// NEVER log the value.
	encKeyHex := os.Getenv("ENCRYPTION_KEY")
	if encKeyHex == "" {
		errs = append(errs, "ENCRYPTION_KEY is required")
	} else {
		decoded, decErr := hex.DecodeString(encKeyHex)
		if decErr != nil {
			errs = append(errs, "ENCRYPTION_KEY must be a valid hex string")
		} else if len(decoded) != 32 {
			errs = append(errs, fmt.Sprintf("ENCRYPTION_KEY must decode to exactly 32 bytes, got %d", len(decoded)))
		} else {
			cfg.EncryptionKey = encKeyHex
		}
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
