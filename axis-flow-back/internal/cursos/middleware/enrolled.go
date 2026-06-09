// Package middleware provides HTTP middleware for the cursos module.
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"axis-flow-back/internal/cursos"
	appmiddleware "axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
)

// EnrollmentRepository is the minimal interface required by IsEnrolled middleware.
type EnrollmentRepository interface {
	GetEnrollment(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error)
}

// IsEnrolled returns a chi middleware that validates the requesting empleado
// has an active enrollment in the curso_id from the URL param before proceeding.
func IsEnrolled(enrollRepo EnrollmentRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract curso_id from URL params.
			cursoIDStr := chi.URLParam(r, "curso_id")
			cursoID, err := strconv.ParseInt(cursoIDStr, 10, 64)
			if err != nil || cursoID <= 0 {
				writeError(w, "BAD_REQUEST", "invalid curso_id", http.StatusBadRequest)
				return
			}

			// Extract empleadoID from context (set by upstream JWT middleware or explicit).
			empleadoID, ok := r.Context().Value(appmiddleware.ContextKeyEmpleadoID).(int64)
			if !ok || empleadoID <= 0 {
				writeError(w, "UNAUTHORIZED", "missing empleado identity", http.StatusUnauthorized)
				return
			}

			// Validate enrollment.
			if _, err := enrollRepo.GetEnrollment(r.Context(), cursoID, empleadoID); err != nil {
				writeError(w, "FORBIDDEN", "not enrolled in this course", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeError(w http.ResponseWriter, code, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}
