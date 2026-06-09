// Package handler provides HTTP handlers for the bolsatrabajo module.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/middleware"

	"github.com/google/uuid"
)

// respondJSON writes a JSON response with {"data": data, "meta": {"requestId": requestID}}.
func respondJSON(w http.ResponseWriter, statusCode int, data any, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": data,
		"meta": map[string]any{"requestId": requestID},
	})
}

// respondPaginated writes a JSON response with pagination metadata.
func respondPaginated(w http.ResponseWriter, statusCode int, data any, requestID string, page, pageSize, total int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": data,
		"meta": map[string]any{
			"requestId": requestID,
			"page":      page,
			"pageSize":  pageSize,
			"total":     total,
		},
	})
}

// respondError writes a JSON error response: {"error": {"code": "...", "message": "..."}}.
func respondError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
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

// newRequestID generates a short request-scoped UUID string.
func newRequestID() string {
	return uuid.New().String()
}

// jsonDecodeBody decodes the JSON request body into dst.
func jsonDecodeBody(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// mapBolsaError maps bolsatrabajo sentinel errors to HTTP status codes and writes the response.
func mapBolsaError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, bolsatrabajo.ErrNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, bolsatrabajo.ErrForbidden):
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, bolsatrabajo.ErrConflict):
		respondError(w, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, bolsatrabajo.ErrInvalidFile):
		respondError(w, http.StatusUnprocessableEntity, "INVALID_FILE", err.Error())
	case errors.Is(err, bolsatrabajo.ErrCaptchaFail):
		respondError(w, http.StatusBadRequest, "CAPTCHA_VALIDATION_FAILED", err.Error())
	case errors.Is(err, bolsatrabajo.ErrInvalidInput):
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
