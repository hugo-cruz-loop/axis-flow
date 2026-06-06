// Package handler provides HTTP handlers for the catalogos module.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"axis-flow-back/internal/catalogos/domain"
)

type problemDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, status, problemDetail{Code: "ERROR", Message: msg})
}

func writeCatalogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, problemDetail{Code: "RESOURCE_NOT_FOUND", Message: err.Error()})
	case errors.Is(err, domain.ErrDuplicateCode):
		writeJSON(w, http.StatusConflict, problemDetail{Code: "DUPLICATE_ENTRY", Message: err.Error()})
	case errors.Is(err, domain.ErrHasDependents):
		writeJSON(w, http.StatusConflict, problemDetail{Code: "HAS_DEPENDENTS", Message: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, problemDetail{Code: "INTERNAL_ERROR", Message: "internal server error"})
	}
}
