package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"axis-flow-back/internal/cursos"

	"github.com/go-chi/chi/v5"
)

// EnrollmentServicer is the interface EnrollmentHandler depends on.
type EnrollmentServicer interface {
	Enroll(ctx context.Context, e *cursos.Enrollment) error
	ListEnrollmentsByEmpleado(ctx context.Context, empleadoID int64) ([]*cursos.Enrollment, error)
	MarkLeccionCompleta(ctx context.Context, empleadoID, leccionID int64) error
	GetNota(ctx context.Context, leccionID, empleadoID int64) (*cursos.Nota, error)
	UpsertNota(ctx context.Context, n *cursos.Nota) error
}

// EnrollmentHandler handles HTTP requests for course enrollment and progress.
type EnrollmentHandler struct {
	svc EnrollmentServicer
}

// NewEnrollmentHandler constructs an EnrollmentHandler.
func NewEnrollmentHandler(svc EnrollmentServicer) *EnrollmentHandler {
	return &EnrollmentHandler{svc: svc}
}

// Enroll handles POST /enroll
// Body: {curso_id, empleado_id}; empresaID injected from JWT tenant.
func (h *EnrollmentHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	var body struct {
		CursoID    int64 `json:"curso_id"`
		EmpleadoID int64 `json:"empleado_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	if body.CursoID <= 0 || body.EmpleadoID <= 0 {
		writeError(w, "BAD_REQUEST", "curso_id and empleado_id are required", http.StatusBadRequest)
		return
	}

	e := &cursos.Enrollment{
		CursoID:    body.CursoID,
		EmpleadoID: body.EmpleadoID,
		EmpresaID:  tenantID,
	}
	if err := h.svc.Enroll(r.Context(), e); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": e})
}

// ListEnrollmentsByEmpleado handles GET /enroll/by-empleado/{empleado_id}
func (h *EnrollmentHandler) ListEnrollmentsByEmpleado(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	raw := chi.URLParam(r, "empleado_id")
	empleadoID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || empleadoID <= 0 {
		writeError(w, "BAD_REQUEST", "invalid empleado_id", http.StatusBadRequest)
		return
	}

	list, err := h.svc.ListEnrollmentsByEmpleado(r.Context(), empleadoID)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// MarkLeccionCompleta handles POST /avance-leccion
// Body: {empleado_id, leccion_id, curso_id}
func (h *EnrollmentHandler) MarkLeccionCompleta(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	var body struct {
		EmpleadoID int64 `json:"empleado_id"`
		LeccionID  int64 `json:"leccion_id"`
		CursoID    int64 `json:"curso_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	if body.EmpleadoID <= 0 || body.LeccionID <= 0 || body.CursoID <= 0 {
		writeError(w, "BAD_REQUEST", "empleado_id, leccion_id and curso_id are required", http.StatusBadRequest)
		return
	}

	if err := h.svc.MarkLeccionCompleta(r.Context(), body.EmpleadoID, body.LeccionID); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "leccion marked as complete"})
}

// GetNota handles GET /leccion/{id}/notas
func (h *EnrollmentHandler) GetNota(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	leccionID, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}
	empleadoID, ok := parseQueryInt64(w, r, "empleado_id")
	if !ok {
		return
	}

	nota, err := h.svc.GetNota(r.Context(), leccionID, empleadoID)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": nota})
}

// UpsertNota handles POST /leccion/{id}/notas
// Body: {empleado_id, contenido}
func (h *EnrollmentHandler) UpsertNota(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	leccionID, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	var body struct {
		EmpleadoID int64  `json:"empleado_id"`
		Contenido  string `json:"contenido"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	n := &cursos.Nota{
		LeccionID:  leccionID,
		EmpleadoID: body.EmpleadoID,
		Contenido:  body.Contenido,
	}
	if err := h.svc.UpsertNota(r.Context(), n); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": n})
}
