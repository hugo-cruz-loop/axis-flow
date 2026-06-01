package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"axis-flow-back/internal/logging"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type requestLogContextKey struct{}

// RequestLogContext carries log attributes populated across middleware layers.
type RequestLogContext struct {
	UserID string
	Role   string
}

// SetLogIdentity stores authenticated identity attributes for the request log.
func SetLogIdentity(ctx context.Context, userID, role string) {
	logCtx, ok := ctx.Value(requestLogContextKey{}).(*RequestLogContext)
	if !ok || logCtx == nil {
		return
	}
	logCtx.UserID = userID
	logCtx.Role = role
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// ParseSlogLevel converts a string log level to slog.Level.
func ParseSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger creates a chi-compatible structured logging middleware.
// It logs each request with required observability attributes.
// The Authorization header value is NEVER logged — only its presence.
func NewLogger(logLevel, logFormat string) func(http.Handler) http.Handler {
	return NewLoggerWithWriter(os.Stdout, logLevel, logFormat)
}

// NewLoggerWithWriter creates the logging middleware using a caller-provided writer.
// It is primarily useful for tests and local file targets.
func NewLoggerWithWriter(w io.Writer, logLevel, logFormat string) func(http.Handler) http.Handler {
	level := ParseSlogLevel(logLevel)
	logger := logging.New(w, logFormat, level)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestID := chimw.GetReqID(r.Context())
			if requestID == "" {
				requestID = uuid.New().String()
			}
			logCtx := &RequestLogContext{}
			r = r.WithContext(context.WithValue(r.Context(), requestLogContextKey{}, logCtx))

			// Derive or generate trace_id from context (otelhttp sets span; fallback to UUID).
			traceID := TraceIDFromContext(r.Context())
			if traceID == "" {
				traceID = uuid.New().String()
			}

			// Capture response status.
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			latencyMs := time.Since(start).Milliseconds()

			route := r.URL.Path
			if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
				if pattern := routeCtx.RoutePattern(); pattern != "" {
					route = pattern
				}
			}

			// Redact Authorization header: only log whether it was present.
			authPresent := r.Header.Get("Authorization") != ""

			logger.InfoContext(r.Context(), "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Bool("auth_header_present", authPresent),
				slog.String("trace_id", traceID),
				slog.String("request_id", requestID),
				slog.String("user_id", logCtx.UserID),
				slog.String("role", logCtx.Role),
				slog.String("route", route),
				slog.Int("status", wrapped.status),
				slog.Int64("latency_ms", latencyMs),
			)
		})
	}
}
