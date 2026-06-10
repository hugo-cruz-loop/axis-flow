// Package handler provides HTTP handlers for the AtencionSeguimiento module.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/middleware"

	"github.com/google/uuid"
)

// respondJSON writes a JSON success response: {"success":true,"data":<payload>}.
func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    payload,
	})
}

// respondPaginated writes a JSON success response with pagination meta.
func respondPaginated(w http.ResponseWriter, status int, data any, page, limit, total int) {
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
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

// respondError writes a JSON error response: {"success":false,"error":{"code":"...","message":"..."}}.
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

// extractTenantID reads the tenant UUID from the JWT context.
func extractTenantID(r *http.Request) (uuid.UUID, error) {
	raw, ok := r.Context().Value(middleware.ContextKeyTenantID).(string)
	if !ok || raw == "" {
		return uuid.Nil, errors.New("missing tenant")
	}
	return uuid.Parse(raw)
}

// extractUserID reads the user UUID from the JWT context.
func extractUserID(r *http.Request) (uuid.UUID, error) {
	raw, ok := r.Context().Value(middleware.ContextKeyUserID).(string)
	if !ok || raw == "" {
		return uuid.Nil, errors.New("missing user")
	}
	return uuid.Parse(raw)
}

// extractEmpleadoID reads the empleado int64 from the JWT context.
func extractEmpleadoID(r *http.Request) (int64, error) {
	v, ok := r.Context().Value(middleware.ContextKeyEmpleadoID).(int64)
	if !ok {
		return 0, errors.New("missing empleado id")
	}
	return v, nil
}

// extractRole reads the role string from the JWT context.
func extractRole(r *http.Request) string {
	v, _ := r.Context().Value(middleware.ContextKeyRole).(string)
	return v
}

// mapAtencionError maps sentinel errors to HTTP status + code pairs and writes
// the error response.
func mapAtencionError(w http.ResponseWriter, err error) {
	status, code := atencionErrorCode(err)
	respondError(w, status, code, err.Error())
}

func atencionErrorCode(err error) (int, string) {
	switch {
	case errors.Is(err, atencionseguimiento.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, atencionseguimiento.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, atencionseguimiento.ErrConflict):
		return http.StatusConflict, "CONFLICT"
	case errors.Is(err, atencionseguimiento.ErrInvalidInput):
		return http.StatusBadRequest, "VALIDATION_ERROR"
	case errors.Is(err, atencionseguimiento.ErrSLAConfigMissing):
		return http.StatusUnprocessableEntity, "SLA_CONFIG_MISSING"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}
