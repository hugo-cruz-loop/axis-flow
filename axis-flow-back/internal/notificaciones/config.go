package notificaciones

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the Notificaciones module.
// Sensitive fields (SMTPPassword, FirebaseConfigJSON) must NEVER be logged.
type Config struct {
	// SMTP settings.
	SMTPEnabled   bool
	SMTPHost      string
	SMTPPort      int
	SMTPUsername  string
	SMTPPassword  string // NEVER log this field
	SMTPFromEmail string
	SMTPUseTLS    bool

	// Firebase Cloud Messaging settings.
	FirebaseConfigJSON string // NEVER log this field

	// WebSocket tuning.
	WSWriteWait        time.Duration
	WSPongWait         time.Duration
	WSPingPeriod       time.Duration
	WSMaxMessageSize   int64
	WSHandshakeTimeout time.Duration

	// Rate limiting.
	RateLimitAPIWindow      time.Duration
	RateLimitAPIMax         int
	RateLimitWSMaxConnPerIP int
	RateLimitWSMsgPerSec    int
}

// ConfigFromEnv reads environment variables and returns a Config with sensible
// defaults. Missing or invalid values fall back to the documented defaults.
// Sensitive fields are read but must never be logged by callers.
func ConfigFromEnv() Config {
	return Config{
		SMTPEnabled:   parseBool(os.Getenv("SMTP_ENABLED"), false),
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      parseInt(os.Getenv("SMTP_PORT"), 587),
		SMTPUsername:  os.Getenv("SMTP_USERNAME"),
		SMTPPassword:  os.Getenv("SMTP_PASSWORD"),
		SMTPFromEmail: os.Getenv("SMTP_FROM_EMAIL"),
		SMTPUseTLS:    parseBool(os.Getenv("SMTP_USE_TLS"), false),

		FirebaseConfigJSON: os.Getenv("FIREBASE_CONFIG_JSON"),

		WSWriteWait:        parseDuration(os.Getenv("WS_WRITE_WAIT"), 10*time.Second),
		WSPongWait:         parseDuration(os.Getenv("WS_PONG_WAIT"), 60*time.Second),
		WSPingPeriod:       parseDuration(os.Getenv("WS_PING_PERIOD"), 54*time.Second),
		WSMaxMessageSize:   parseInt64(os.Getenv("WS_MAX_MESSAGE_SIZE"), 4096),
		WSHandshakeTimeout: parseDuration(os.Getenv("WS_HANDSHAKE_TIMEOUT"), 10*time.Second),

		RateLimitAPIWindow:      parseDuration(os.Getenv("RATE_LIMIT_API_WINDOW"), 60*time.Second),
		RateLimitAPIMax:         parseInt(os.Getenv("RATE_LIMIT_API_MAX"), 100),
		RateLimitWSMaxConnPerIP: parseInt(os.Getenv("RATE_LIMIT_WS_MAX_CONN_PER_IP"), 10),
		RateLimitWSMsgPerSec:    parseInt(os.Getenv("RATE_LIMIT_WS_MSG_PER_SEC"), 5),
	}
}

// ---------------------------------------------------------------------------
// Internal helpers — not exported; no logging of values.
// ---------------------------------------------------------------------------

func parseBool(s string, def bool) bool {
	if s == "" {
		return def
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return def
	}
	return v
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseInt64(s string, def int64) int64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return v
}
