// Package handler provides HTTP handlers for the Reports module.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/reports"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    payload,
	})
}

func respondPaginated(w http.ResponseWriter, status int, data any, page, limit, total int) {
	totalPages := total / limit
	if total%limit != 0 || total == 0 {
		totalPages++
	}
	if total == 0 {
		totalPages = 0
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    data,
		"meta": map[string]any{
			"page":          page,
			"limit":         limit,
			"total_records": total,
			"total_pages":   totalPages,
		},
	})
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

// ---------------------------------------------------------------------------
// JWT claim extractors — same pattern as atencionseguimiento/handler.
// ---------------------------------------------------------------------------

func extractTenantID(r *http.Request) (string, error) {
	v, ok := r.Context().Value(middleware.ContextKeyTenantID).(string)
	if !ok || v == "" {
		return "", errors.New("missing tenant")
	}
	return v, nil
}

func extractRole(r *http.Request) string {
	v, _ := r.Context().Value(middleware.ContextKeyRole).(string)
	return v
}

// ---------------------------------------------------------------------------
// Query param helpers
// ---------------------------------------------------------------------------

func queryString(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

func queryInt(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return def
	}
	return v
}

// isValidUUID returns true when s is a well-formed UUID v4.
func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// hasSuspiciousPattern returns true when s contains common SQL injection
// probe characters. When detected, a security warning is logged.
func hasSuspiciousPattern(s, param string) bool {
	for _, ch := range []string{"'", "\"", ";", "--", "/*", "*/", "xp_", "DROP", "SELECT", "INSERT", "UPDATE", "DELETE", "UNION"} {
		for i := 0; i+len(ch) <= len(s); i++ {
			if s[i:i+len(ch)] == ch {
				slog.Warn("suspicious query parameter detected",
					"param", param,
					"level", "warn",
				)
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Error mapper
// ---------------------------------------------------------------------------

// mapReportsError converts sentinel errors from the reports domain to HTTP
// status code + error code pairs.
func mapReportsError(err error) (int, string) {
	switch {
	case errors.Is(err, reports.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, reports.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, reports.ErrRateLimitExceeded):
		return http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED"
	case errors.Is(err, reports.ErrGeocodingUnavailable):
		return http.StatusServiceUnavailable, "GEOCODING_UNAVAILABLE"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}
