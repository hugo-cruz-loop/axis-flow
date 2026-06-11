// Package handler provides HTTP handlers for the Notificaciones module.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/notificaciones"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// JSON response helpers
// ---------------------------------------------------------------------------

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    payload,
	})
}

func respondPaginated(w http.ResponseWriter, status int, data any, page, pageSize, total int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    data,
		"meta": map[string]any{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
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
// Context extractors
// ---------------------------------------------------------------------------

func extractUserID(r *http.Request) (uuid.UUID, error) {
	raw, ok := r.Context().Value(middleware.ContextKeyUserID).(string)
	if !ok || raw == "" {
		return uuid.Nil, errors.New("missing user")
	}
	return uuid.Parse(raw)
}

func extractRole(r *http.Request) string {
	v, _ := r.Context().Value(middleware.ContextKeyRole).(string)
	return v
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

// mapNotifError maps notificaciones sentinel errors to (httpStatus, errorCode).
func mapNotifError(err error) (int, string) {
	switch {
	case errors.Is(err, notificaciones.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, notificaciones.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, notificaciones.ErrInvalidInput):
		return http.StatusBadRequest, "INVALID_INPUT"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}
