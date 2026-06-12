// Package handler provides HTTP handlers for the ReportBro report service.
package handler

import (
	"errors"
	"net/http"

	report "axis-flow-back/internal/report"
)

// mapReportError maps sentinel domain errors to HTTP status + error code pairs.
func mapReportError(err error) (int, string) {
	switch {
	case errors.Is(err, report.ErrInvalidFormat):
		return http.StatusBadRequest, "INVALID_FORMAT"
	case errors.Is(err, report.ErrReportNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, report.ErrSignatureInvalid):
		return http.StatusForbidden, "FORBIDDEN"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}
