package unit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"axis-flow-back/internal/logging"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerMiddleware_LogfmtRedactsAuthorizationHeaderAndAppendsRequiredAttrs(t *testing.T) {
	secretToken := "Bearer super-secret-token-12345"

	var buf bytes.Buffer
	mw := middleware.NewLoggerWithWriter(&buf, "debug", "logfmt")

	r := chi.NewRouter()
	r.Use(mw)
	r.Get("/api/auth/me/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me/", nil)
	req.Header.Set("Authorization", secretToken)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	logOutput := strings.TrimSpace(buf.String())
	require.NotEmpty(t, logOutput, "expected log output")
	assert.Contains(t, logOutput, "] [INFO ] [http request]")
	assert.NotContains(t, logOutput, "super-secret-token-12345")
	assert.Contains(t, logOutput, "auth_header_present=true")
	assert.Contains(t, logOutput, "route=/api/auth/me")
	assert.Regexp(t, `trace_id=[^ ]+ request_id=[^ ]+ user_id="" role="" route=/api/auth/me status=200 latency_ms=[0-9]+$`, logOutput)
}

func TestLoggerMiddleware_JSONFormatCanBeSelectedByConfig(t *testing.T) {
	var buf bytes.Buffer
	mw := middleware.NewLoggerWithWriter(&buf, "debug", "json")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "http request", entry["msg"])
	assert.Equal(t, float64(http.StatusCreated), entry["status"])
	assert.Contains(t, entry, "trace_id")
	assert.Contains(t, entry, "request_id")
	assert.Contains(t, entry, "user_id")
	assert.Contains(t, entry, "role")
	assert.Contains(t, entry, "route")
	assert.Contains(t, entry, "latency_ms")
}

func TestApplicationLoggerRedactsSensitiveAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf, "logfmt", slog.LevelDebug)

	logger.InfoContext(context.Background(), "credentials seen",
		slog.String("password", "plain-text"),
		slog.String("access_token", "abc123"),
		slog.String("safe", "ok"),
	)

	logOutput := buf.String()
	assert.NotContains(t, logOutput, "plain-text")
	assert.NotContains(t, logOutput, "abc123")
	assert.Contains(t, logOutput, "password=[REDACTED]")
	assert.Contains(t, logOutput, "access_token=[REDACTED]")
	assert.Contains(t, logOutput, "safe=ok")
}
