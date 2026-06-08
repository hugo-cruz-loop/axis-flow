// Package handler provides HTTP handlers for the clientes module.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"axis-flow-back/internal/clientes"
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

// parseEmpresaIDQuery parses empresa_id from query string.
func parseEmpresaIDQuery(w http.ResponseWriter, r *http.Request) (int64, bool) {
	s := r.URL.Query().Get("empresa_id")
	if s == "" {
		writeError(w, "BAD_REQUEST", "empresa_id query param is required", http.StatusBadRequest)
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id == 0 {
		writeError(w, "BAD_REQUEST", "invalid empresa_id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func writeClienteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, clientes.ErrClienteNotFound):
		writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
	case errors.Is(err, clientes.ErrLocalidadNotFound):
		writeError(w, "NOT_FOUND", err.Error(), http.StatusNotFound)
	case errors.Is(err, clientes.ErrTenantMismatch):
		writeError(w, "FORBIDDEN", err.Error(), http.StatusForbidden)
	case errors.Is(err, clientes.ErrClienteHasDependents):
		writeError(w, "HAS_DEPENDENTS", err.Error(), http.StatusConflict)
	case errors.Is(err, clientes.ErrFacturaExists):
		writeError(w, "CONFLICT", err.Error(), http.StatusConflict)
	case errors.Is(err, clientes.ErrPresupuestoExists):
		writeError(w, "CONFLICT", err.Error(), http.StatusConflict)
	case errors.Is(err, clientes.ErrCalendarioExists):
		writeError(w, "CONFLICT", err.Error(), http.StatusConflict)
	case errors.Is(err, clientes.ErrActivationBlocked):
		writeError(w, "ACTIVATION_BLOCKED", err.Error(), http.StatusUnprocessableEntity)
	default:
		writeError(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError)
	}
}
