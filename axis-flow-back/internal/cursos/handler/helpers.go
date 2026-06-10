// Package handler provides HTTP handlers for the cursos module.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// problemDetail is the RFC 7807-style error envelope.
type problemDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code, msg string, status int) {
	writeJSON(w, status, map[string]any{"error": problemDetail{Code: code, Message: msg}})
}

// tenantFromCtx extracts the tenant UUID from JWT context.
// Returns uuid.Nil and writes a 401 if missing or malformed.
func tenantFromCtx(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw, ok := r.Context().Value(middleware.ContextKeyTenantID).(string)
	if !ok || raw == "" {
		writeError(w, "UNAUTHORIZED", "missing tenant", http.StatusUnauthorized)
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, "UNAUTHORIZED", "invalid tenant id", http.StatusUnauthorized)
		return uuid.Nil, false
	}
	return id, true
}

// parsePathInt64 reads a chi URL param as int64.
func parsePathInt64(w http.ResponseWriter, r *http.Request, param string) (int64, bool) {
	raw := chi.URLParam(r, param)
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		writeError(w, "BAD_REQUEST", "invalid "+param, http.StatusBadRequest)
		return 0, false
	}
	return v, true
}

// parseQueryInt64 reads a query param as int64.
func parseQueryInt64(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	s := r.URL.Query().Get(key)
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		writeError(w, "BAD_REQUEST", "invalid "+key, http.StatusBadRequest)
		return 0, false
	}
	return v, true
}

// writeCursosError maps cursos sentinel errors to HTTP status codes.
func writeCursosError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, cursos.ErrCursoNotFound):
		writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
	case errors.Is(err, cursos.ErrNotEnrolled):
		writeError(w, "FORBIDDEN", err.Error(), http.StatusForbidden)
	case errors.Is(err, cursos.ErrTenantMismatch):
		writeError(w, "FORBIDDEN", err.Error(), http.StatusForbidden)
	case errors.Is(err, cursos.ErrExamExceededAttempts):
		writeError(w, "CONFLICT", err.Error(), http.StatusConflict)
	case errors.Is(err, cursos.ErrExamNotApproved):
		writeError(w, "FORBIDDEN", err.Error(), http.StatusForbidden)
	case errors.Is(err, cursos.ErrCursoDeleted):
		writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
	default:
		writeError(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError)
	}
}
