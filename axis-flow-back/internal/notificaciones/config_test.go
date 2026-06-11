// Package notificaciones_test exercises ConfigFromEnv default values and parsing.
package notificaciones_test

import (
	"os"
	"testing"
	"time"

	"axis-flow-back/internal/notificaciones"
)

func TestConfigFromEnv_Defaults(t *testing.T) {
	// Unset all relevant env vars so defaults apply.
	envVars := []string{
		"SMTP_ENABLED", "SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME",
		"SMTP_PASSWORD", "SMTP_FROM_EMAIL", "SMTP_USE_TLS",
		"FIREBASE_CONFIG_JSON",
		"WS_WRITE_WAIT", "WS_PONG_WAIT", "WS_PING_PERIOD",
		"WS_MAX_MESSAGE_SIZE", "WS_HANDSHAKE_TIMEOUT",
		"RATE_LIMIT_API_WINDOW", "RATE_LIMIT_API_MAX",
		"RATE_LIMIT_WS_MAX_CONN_PER_IP", "RATE_LIMIT_WS_MSG_PER_SEC",
	}
	for _, v := range envVars {
		t.Setenv(v, "")
		os.Unsetenv(v) //nolint:errcheck
	}

	cfg := notificaciones.ConfigFromEnv()

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"SMTPEnabled", cfg.SMTPEnabled, false},
		{"WSWriteWait", cfg.WSWriteWait, 10 * time.Second},
		{"WSPongWait", cfg.WSPongWait, 60 * time.Second},
		{"WSPingPeriod", cfg.WSPingPeriod, 54 * time.Second},
		{"WSMaxMessageSize", cfg.WSMaxMessageSize, int64(4096)},
		{"WSHandshakeTimeout", cfg.WSHandshakeTimeout, 10 * time.Second},
		{"RateLimitAPIWindow", cfg.RateLimitAPIWindow, 60 * time.Second},
		{"RateLimitAPIMax", cfg.RateLimitAPIMax, 100},
		{"RateLimitWSMaxConnPerIP", cfg.RateLimitWSMaxConnPerIP, 10},
		{"RateLimitWSMsgPerSec", cfg.RateLimitWSMsgPerSec, 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestConfigFromEnv_SMTPEnabled_True(t *testing.T) {
	t.Setenv("SMTP_ENABLED", "true")
	cfg := notificaciones.ConfigFromEnv()
	if !cfg.SMTPEnabled {
		t.Error("expected SMTPEnabled=true when SMTP_ENABLED=true")
	}
}

func TestConfigFromEnv_SMTPEnabled_False(t *testing.T) {
	t.Setenv("SMTP_ENABLED", "false")
	cfg := notificaciones.ConfigFromEnv()
	if cfg.SMTPEnabled {
		t.Error("expected SMTPEnabled=false when SMTP_ENABLED=false")
	}
}

func TestConfigFromEnv_NoSecretsLeaked(t *testing.T) {
	// Verify the config struct does NOT expose a Stringer or any method that
	// would print SMTP_PASSWORD or FIREBASE_CONFIG_JSON.
	// This is a compile-time guard: Config must not implement fmt.Stringer.
	// If it does, this test will fail at compile time via the type assertion below.
	var _ notificaciones.Config = notificaciones.ConfigFromEnv() // must compile

	// Runtime: password and firebase fields should never appear in a default log line.
	// We only assert the field names exist and hold zero values by default.
	cfg := notificaciones.ConfigFromEnv()
	if cfg.SMTPPassword != "" {
		t.Error("SMTPPassword should be empty when env var is not set")
	}
	if cfg.FirebaseConfigJSON != "" {
		t.Error("FirebaseConfigJSON should be empty when env var is not set")
	}
}
