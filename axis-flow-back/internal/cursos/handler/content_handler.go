package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/middleware"

	"github.com/google/uuid"
)

// ContentServicer is the interface ContentHandler depends on.
type ContentServicer interface {
	CreateCurso(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error
	ListCursos(ctx context.Context, tenantID uuid.UUID, isAdmin bool) ([]*cursos.Curso, error)
	GetCurso(ctx context.Context, id int64, tenantID uuid.UUID) (*cursos.Curso, error)
	GetCursoContenido(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)
	UpdateCurso(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error
	DeleteCurso(ctx context.Context, id int64, empresaID uuid.UUID) error
	CreateUnidad(ctx context.Context, u *cursos.Unidad) error
	UpdateUnidad(ctx context.Context, u *cursos.Unidad) error
	DeleteUnidad(ctx context.Context, id int64) error
	CreateLeccion(ctx context.Context, l *cursos.Leccion) error
	UpdateLeccion(ctx context.Context, l *cursos.Leccion) error
	DeleteLeccion(ctx context.Context, id int64) error
}

// ContentHandler handles HTTP requests for curso content (cursos, unidades, lecciones).
type ContentHandler struct {
	svc ContentServicer
}

// NewContentHandler constructs a ContentHandler.
func NewContentHandler(svc ContentServicer) *ContentHandler {
	return &ContentHandler{svc: svc}
}

// CreateCurso handles POST /curso
func (h *ContentHandler) CreateCurso(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	var body cursos.Curso
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.svc.CreateCurso(r.Context(), &body, tenantID); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": body})
}

// ListCursos handles GET /cursos
// If the caller has role=admin AND include_private=true, private courses are included.
func (h *ContentHandler) ListCursos(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	role, _ := r.Context().Value(middleware.ContextKeyRole).(string)
	includePrivate := r.URL.Query().Get("include_private") == "true"
	isAdmin := role == "admin" && includePrivate

	list, err := h.svc.ListCursos(r.Context(), tenantID, isAdmin)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// GetCurso handles GET /curso/{id}
func (h *ContentHandler) GetCurso(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	c, err := h.svc.GetCurso(r.Context(), id, tenantID)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c})
}

// GetCursoContenido handles GET /curso/{id}/contenido
func (h *ContentHandler) GetCursoContenido(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	unidades, err := h.svc.GetCursoContenido(r.Context(), id)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": unidades})
}

// UpdateCurso handles PUT /curso/{id}
func (h *ContentHandler) UpdateCurso(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	var body cursos.Curso
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	body.ID = id

	if err := h.svc.UpdateCurso(r.Context(), &body, tenantID); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": body})
}

// DeleteCurso handles DELETE /curso/{id} — soft delete
func (h *ContentHandler) DeleteCurso(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	if err := h.svc.DeleteCurso(r.Context(), id, tenantID); err != nil {
		writeCursosError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateUnidad handles POST /unidad
func (h *ContentHandler) CreateUnidad(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	var body cursos.Unidad
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.svc.CreateUnidad(r.Context(), &body); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": body})
}

// UpdateUnidad handles PUT /unidad/{id}
func (h *ContentHandler) UpdateUnidad(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	var body cursos.Unidad
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	body.ID = id

	if err := h.svc.UpdateUnidad(r.Context(), &body); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": body})
}

// DeleteUnidad handles DELETE /unidad/{id}
func (h *ContentHandler) DeleteUnidad(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	if err := h.svc.DeleteUnidad(r.Context(), id); err != nil {
		writeCursosError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateLeccion handles POST /leccion
func (h *ContentHandler) CreateLeccion(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	var body cursos.Leccion
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.svc.CreateLeccion(r.Context(), &body); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": body})
}

// UpdateLeccion handles PUT /leccion/{id}
func (h *ContentHandler) UpdateLeccion(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	var body cursos.Leccion
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	body.ID = id

	if err := h.svc.UpdateLeccion(r.Context(), &body); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": body})
}

// DeleteLeccion handles DELETE /leccion/{id}
func (h *ContentHandler) DeleteLeccion(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	if err := h.svc.DeleteLeccion(r.Context(), id); err != nil {
		writeCursosError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
