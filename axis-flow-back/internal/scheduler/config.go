// Package scheduler provides background job orchestration for the axis-flow
// platform. The package owns the scheduler_jobs and scheduler_executions
// persistence under the scheduler schema and exposes domain contracts that
// other modules can depend on.
package scheduler

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the Scheduler service.
// Sensitive fields (FirebaseCredentialsJSON, AWSSecretAccessKey) must NEVER
// be logged.
type Config struct {
	// Env is the deployment stage (production, staging, development).
	Env string
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// RedisURL is the connection string for the distributed lock store.
	RedisURL string
	// FirebaseCredentialsJSON is the full service-account JSON payload. Never log.
	FirebaseCredentialsJSON string
	// ParametrizacionServiceURL is the base URL of the internal Parametrizacion
	// service used for company-specific threshold lookups.
	ParametrizacionServiceURL string
	// SQSQueueURL is the target AWS SQS queue used by the consumer integration.
	SQSQueueURL string
	// AWSAccessKeyID is the AWS API access key. Never log.
	AWSAccessKeyID string
	// AWSSecretAccessKey is the AWS API secret access key. Never log.
	AWSSecretAccessKey string
	// AWSRegion is the AWS region hosting the SQS queue.
	AWSRegion string
	// LogFormat controls structured log output (logfmt or json).
	LogFormat string
	// SchedulerLockTTLSec is the TTL for Redis distributed job locks.
	SchedulerLockTTLSec int
	// M2MSigningKey is the shared HMAC secret used to sign M2M JWTs
	// minted by the scheduler. It must be at least 32 bytes; the spec
	// recommends 64 random bytes. Never log.
	M2MSigningKey string
	// M2MKeyID is an optional kid header value emitted with every
	// M2M token. Receiving services that resolve keys by id use it to
	// pick the right secret. Empty is allowed; the header is omitted.
	M2MKeyID string
	// M2MAudience is the aud claim placed on every M2M token. Defaults
	// to "axis-flow-internal" when left blank in the environment.
	M2MAudience string
}

// SchedulerLockTTL returns the configured lock TTL as a time.Duration.
func (c Config) SchedulerLockTTL() time.Duration {
	return time.Duration(c.SchedulerLockTTLSec) * time.Second
}

// LoadConfig reads environment variables and returns a validated Config.
// Returns an error when a required variable is missing or carries an invalid
// value. Defaults are applied for variables that have documented fallbacks.
func LoadConfig() (Config, error) {
	cfg := Config{
		Env:                       getenvDefault("ENV", "production"),
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		RedisURL:                  getenvDefault("REDIS_URL", "redis://127.0.0.1:6379/1"),
		FirebaseCredentialsJSON:   os.Getenv("FIREBASE_CREDENTIALS_JSON"),
		ParametrizacionServiceURL: getenvDefault("PARAMETRIZACION_SERVICE_URL", "http://parametrizacion-service:8080"),
		SQSQueueURL:               os.Getenv("SQS_QUEUE_URL"),
		AWSAccessKeyID:            os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:        os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSRegion:                 getenvDefault("AWS_REGION", "us-east-1"),
		LogFormat:                 getenvDefault("LOG_FORMAT", "logfmt"),
		SchedulerLockTTLSec:       parseIntDefault("SCHEDULER_LOCK_TTL_SEC", 600),
		M2MSigningKey:             os.Getenv("SCHEDULER_M2M_SIGNING_KEY"),
		M2MKeyID:                  os.Getenv("SCHEDULER_M2M_KEY_ID"),
		M2MAudience:               getenvDefault("SCHEDULER_M2M_AUDIENCE", "axis-flow-internal"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate ensures every required field carries a non-empty value and that
// the lock TTL is a positive integer. Callers should invoke this before
// relying on a Config instance.
func (c Config) Validate() error {
	switch c.Env {
	case "production", "staging", "development":
	default:
		return fmt.Errorf("scheduler: invalid ENV %q (allowed: production, staging, development)", c.Env)
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("scheduler: DATABASE_URL is required")
	}
	if c.FirebaseCredentialsJSON == "" {
		return fmt.Errorf("scheduler: FIREBASE_CREDENTIALS_JSON is required")
	}
	if c.SQSQueueURL == "" {
		return fmt.Errorf("scheduler: SQS_QUEUE_URL is required")
	}
	if c.AWSAccessKeyID == "" {
		return fmt.Errorf("scheduler: AWS_ACCESS_KEY_ID is required")
	}
	if c.AWSSecretAccessKey == "" {
		return fmt.Errorf("scheduler: AWS_SECRET_ACCESS_KEY is required")
	}
	if c.SchedulerLockTTLSec <= 0 {
		return fmt.Errorf("scheduler: SCHEDULER_LOCK_TTL_SEC must be > 0 (got %d)", c.SchedulerLockTTLSec)
	}
	if c.M2MSigningKey == "" {
		return fmt.Errorf("scheduler: SCHEDULER_M2M_SIGNING_KEY is required")
	}
	if len(c.M2MSigningKey) < 32 {
		return fmt.Errorf("scheduler: SCHEDULER_M2M_SIGNING_KEY must be at least 32 bytes (got %d)", len(c.M2MSigningKey))
	}
	switch c.LogFormat {
	case "logfmt", "json":
	default:
		return fmt.Errorf("scheduler: invalid LOG_FORMAT %q (allowed: logfmt, json)", c.LogFormat)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers — not exported; no logging of values.
// ---------------------------------------------------------------------------

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseIntDefault(key string, def int) int {
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
