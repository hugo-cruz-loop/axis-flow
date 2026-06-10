// Package handler provides HTTP handlers for the Formularios module.
//
// Mirrors the structure of internal/atencionseguimiento/handler/ (PR-4 of
// 09_AtencionSeguimiento_Service_Spec): helpers in helpers.go, one
// handler struct per aggregate root (formulario, evento, respuesta), and
// a *_test.go in the handler_test package per handler.
//
// PR-4 (REST/HTTP) — task 4.1 helpers. The error mapping, response
// shapes, and JWT-context extractors match the 09 patterns verbatim
// (with formularios.ErrInvalidInput mapped to 422 per the openapi
// contract, instead of 400).
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/middleware"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Response writers.
// ---------------------------------------------------------------------------

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
// The message is intentionally generic — the handler MUST never echo
// user-supplied strings (PII) into the error envelope.
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
// JWT-context extractors.
// ---------------------------------------------------------------------------

// extractTenantID reads the tenant UUID from the JWT context. Mirrors
// atencionseguimiento/handler/helpers.go.
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
// PR-4 AMEND (FIX 5): a zero value is treated the same as a missing
// key — the handler refuses to fall back to the request body's
// empleado_id (which would let a client spoof the identity of
// another employee). The handler is the trust boundary; the JWT
// issuer MUST put a non-zero empleado id in the claim. PR-5 will
// wire the claim into the production JWT schema (see Deviation #3
// in apply-progress).
func extractEmpleadoID(r *http.Request) (int64, error) {
	v, ok := r.Context().Value(middleware.ContextKeyEmpleadoID).(int64)
	if !ok {
		return 0, errors.New("missing empleado id")
	}
	if v == 0 {
		return 0, errors.New("missing empleado id")
	}
	return v, nil
}

// extractRole reads the role string from the JWT context. Returns "" if
// the key is missing — role checks are role-aware, not role-required.
func extractRole(r *http.Request) string {
	v, _ := r.Context().Value(middleware.ContextKeyRole).(string)
	return v
}

// ---------------------------------------------------------------------------
// Error mapping.
// ---------------------------------------------------------------------------

// mapFormulariosError maps formularios sentinel errors to HTTP status +
// short code pairs, and writes the error response. Mirrors
// atencionseguimiento/handler/helpers.go:mapAtencionError but maps
// formularios.ErrInvalidInput to 422 (per the openapi contract) instead
// of 400.
//
// Status / code table (PR-4 AMEND — FIX 4, SCREAMING_SNAKE_CASE codes
// matching 09's atencionseguimiento constants verbatim):
//
//	ErrNotFound     → 404 / "NOT_FOUND"
//	ErrForbidden    → 403 / "FORBIDDEN"
//	ErrInvalidInput → 422 / "VALIDATION_ERROR"   (09 uses 400; formularios
//	                                            openapi requires 422)
//	ErrConflict     → 409 / "CONFLICT"
//	default         → 500 / "INTERNAL_ERROR"     (generic message)
//
// On the default branch, the original error is intentionally NOT echoed
// into the response body — the original error may contain file paths,
// SQL fragments, or vendor detail. The structured logger (PR-5) persists
// the full error for SRE.
func mapFormulariosError(w http.ResponseWriter, err error) {
	status, code := formulariosErrorCode(err)
	var message string
	switch {
	case errors.Is(err, formularios.ErrNotFound):
		message = "resource not found"
	case errors.Is(err, formularios.ErrForbidden):
		message = "forbidden"
	case errors.Is(err, formularios.ErrInvalidInput):
		message = "invalid payload"
	case errors.Is(err, formularios.ErrConflict):
		message = "conflict"
	default:
		message = "internal error"
	}
	respondError(w, status, code, message)
}

func formulariosErrorCode(err error) (int, string) {
	switch {
	case errors.Is(err, formularios.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, formularios.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, formularios.ErrInvalidInput):
		return http.StatusUnprocessableEntity, "VALIDATION_ERROR"
	case errors.Is(err, formularios.ErrConflict):
		return http.StatusConflict, "CONFLICT"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

// ---------------------------------------------------------------------------
// Pagination.
// ---------------------------------------------------------------------------

// parsePagination extracts page and limit from the URL query with safe
// defaults. limit is capped at 100 to prevent a single client from
// requesting an unbounded page. Non-numeric or non-positive values fall
// back to the defaults (1 / 20).
func parsePagination(r *http.Request) (page, limit int) {
	page = 1
	limit = 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return
}
