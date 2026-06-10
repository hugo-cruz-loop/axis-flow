package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"axis-flow-back/internal/cursos"

	"github.com/google/uuid"
)

// CatalogServicer is the interface CatalogHandler depends on.
type CatalogServicer interface {
	CreateCategoria(ctx context.Context, c *cursos.Categoria, empresaID uuid.UUID) error
	ListCategorias(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error)
	UpdateCategoria(ctx context.Context, c *cursos.Categoria) error
	DeleteCategoria(ctx context.Context, id int64, empresaID uuid.UUID) error
	CreateModulo(ctx context.Context, m *cursos.Modulo, empresaID uuid.UUID) error
	ListModulos(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error)
}

// CatalogHandler handles HTTP requests for curso catalog resources (categorias, modulos).
type CatalogHandler struct {
	svc CatalogServicer
}

// NewCatalogHandler constructs a CatalogHandler.
func NewCatalogHandler(svc CatalogServicer) *CatalogHandler {
	return &CatalogHandler{svc: svc}
}

// CreateCategoria handles POST /categoria
func (h *CatalogHandler) CreateCategoria(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	var body struct {
		Nombre      string `json:"nombre"`
		Descripcion string `json:"descripcion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Nombre == "" {
		writeError(w, "BAD_REQUEST", "nombre is required", http.StatusBadRequest)
		return
	}

	c := &cursos.Categoria{
		Nombre:      body.Nombre,
		Descripcion: body.Descripcion,
	}
	if err := h.svc.CreateCategoria(r.Context(), c, tenantID); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": c})
}

// ListCategorias handles GET /categorias
func (h *CatalogHandler) ListCategorias(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	list, err := h.svc.ListCategorias(r.Context(), tenantID)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// UpdateCategoria handles PUT /categoria/{id}
func (h *CatalogHandler) UpdateCategoria(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	var body struct {
		Nombre      string `json:"nombre"`
		Descripcion string `json:"descripcion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	c := &cursos.Categoria{
		ID:          id,
		Nombre:      body.Nombre,
		Descripcion: body.Descripcion,
		EmpresaID:   tenantID,
	}
	if err := h.svc.UpdateCategoria(r.Context(), c); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c})
}

// DeleteCategoria handles DELETE /categoria/{id}
func (h *CatalogHandler) DeleteCategoria(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	id, ok := parsePathInt64(w, r, "id")
	if !ok {
		return
	}

	if err := h.svc.DeleteCategoria(r.Context(), id, tenantID); err != nil {
		writeCursosError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateModulo handles POST /modulo
func (h *CatalogHandler) CreateModulo(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	var body struct {
		Nombre string `json:"nombre"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Nombre == "" {
		writeError(w, "BAD_REQUEST", "nombre is required", http.StatusBadRequest)
		return
	}

	m := &cursos.Modulo{
		Nombre:    body.Nombre,
		EmpresaID: tenantID,
	}
	if err := h.svc.CreateModulo(r.Context(), m, tenantID); err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": m})
}

// ListModulos handles GET /modulos
func (h *CatalogHandler) ListModulos(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	list, err := h.svc.ListModulos(r.Context(), tenantID)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}
