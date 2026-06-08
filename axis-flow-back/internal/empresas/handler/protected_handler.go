package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"axis-flow-back/internal/empresas"

	"github.com/go-chi/chi/v5"
)

// EmpresaServicer is the interface the ProtectedHandler depends on for empresa operations.
type EmpresaServicer interface {
	GetByID(ctx context.Context, id int64) (*empresas.Empresa, error)
	Update(ctx context.Context, e *empresas.Empresa) error
	Delete(ctx context.Context, id int64) error
}

// DatosFiscalesRepo is the interface for datos fiscales persistence.
type DatosFiscalesRepo interface {
	FindByEmpresaID(ctx context.Context, empresaID int64) (*empresas.DatosFiscales, error)
	Create(ctx context.Context, d *empresas.DatosFiscales) error
	Update(ctx context.Context, d *empresas.DatosFiscales) error
}

// ApoderadoRepo is the interface for apoderado persistence.
type ApoderadoRepo interface {
	ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Apoderado, error)
	Create(ctx context.Context, a *empresas.Apoderado) error
	Update(ctx context.Context, a *empresas.Apoderado) error
	Delete(ctx context.Context, id int64) error
}

// ServicioRepo is the interface for servicio persistence.
type ServicioRepo interface {
	ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Servicio, error)
	Create(ctx context.Context, s *empresas.Servicio) error
	Update(ctx context.Context, s *empresas.Servicio) error
	Delete(ctx context.Context, id int64) error
}

// ProtectedHandler handles JWT-protected empresa endpoints.
type ProtectedHandler struct {
	empresa    EmpresaServicer
	fiscal     DatosFiscalesRepo
	apoderados ApoderadoRepo
	servicios  ServicioRepo
}

// NewProtectedHandler creates a new ProtectedHandler.
func NewProtectedHandler(
	empresa EmpresaServicer,
	fiscal DatosFiscalesRepo,
	apoderados ApoderadoRepo,
	servicios ServicioRepo,
) *ProtectedHandler {
	return &ProtectedHandler{
		empresa:    empresa,
		fiscal:     fiscal,
		apoderados: apoderados,
		servicios:  servicios,
	}
}

// ── empresa CRUD ──────────────────────────────────────────────────────────────

// GetEmpresa handles GET /api/v1/empresa/{id}
func (h *ProtectedHandler) GetEmpresa(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	e, err := h.empresa.GetByID(r.Context(), id)
	if err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": e})
}

// UpdateEmpresa handles PUT /api/v1/empresa/{id}
func (h *ProtectedHandler) UpdateEmpresa(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	var body struct {
		Nombre    string `json:"nombre"`
		Direccion string `json:"direccion"`
		Telefono  string `json:"telefono"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	e := &empresas.Empresa{
		ID:        id,
		Nombre:    body.Nombre,
		Direccion: body.Direccion,
		Telefono:  body.Telefono,
	}
	if err := h.empresa.Update(r.Context(), e); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "updated"}})
}

// DeleteEmpresa handles DELETE /api/v1/empresa/{id}
func (h *ProtectedHandler) DeleteEmpresa(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	if err := h.empresa.Delete(r.Context(), id); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "deleted"}})
}

// ── datos fiscales ────────────────────────────────────────────────────────────

// GetFiscal handles GET /api/v1/empresa/{id}/fiscal
func (h *ProtectedHandler) GetFiscal(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	d, err := h.fiscal.FindByEmpresaID(r.Context(), id)
	if err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": d})
}

// CreateFiscal handles POST /api/v1/empresa/{id}/fiscal
func (h *ProtectedHandler) CreateFiscal(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	var body struct {
		RFC          string `json:"rfc"`
		RazonSocial  string `json:"razon_social"`
		LogoURL      string `json:"logo_url"`
		IMSSPatronal string `json:"imss_patronal"`
		REPSE        string `json:"repse"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	d := &empresas.DatosFiscales{
		EmpresaID:    id,
		RFC:          body.RFC,
		RazonSocial:  body.RazonSocial,
		LogoURL:      body.LogoURL,
		IMSSPatronal: body.IMSSPatronal,
		REPSE:        body.REPSE,
	}
	if err := h.fiscal.Create(r.Context(), d); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": d})
}

// UpdateFiscal handles PUT /api/v1/empresa/{id}/fiscal
func (h *ProtectedHandler) UpdateFiscal(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	var body struct {
		RazonSocial  string `json:"razon_social"`
		LogoURL      string `json:"logo_url"`
		IMSSPatronal string `json:"imss_patronal"`
		REPSE        string `json:"repse"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	d := &empresas.DatosFiscales{
		EmpresaID:    id,
		RazonSocial:  body.RazonSocial,
		LogoURL:      body.LogoURL,
		IMSSPatronal: body.IMSSPatronal,
		REPSE:        body.REPSE,
	}
	if err := h.fiscal.Update(r.Context(), d); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "updated"}})
}

// ── apoderados ────────────────────────────────────────────────────────────────

// ListApoderados handles GET /api/v1/empresa/{id}/apoderados
func (h *ProtectedHandler) ListApoderados(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	list, err := h.apoderados.ListByEmpresaID(r.Context(), id)
	if err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// CreateApoderado handles POST /api/v1/empresa/{id}/apoderados
func (h *ProtectedHandler) CreateApoderado(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	var body struct {
		Nombre   string `json:"nombre"`
		CURP     string `json:"curp"`
		RFC      string `json:"rfc"`
		Email    string `json:"email"`
		Telefono string `json:"telefono"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	a := &empresas.Apoderado{
		EmpresaID: id,
		Nombre:    body.Nombre,
		CURP:      body.CURP,
		RFC:       body.RFC,
		Email:     body.Email,
		Telefono:  body.Telefono,
	}
	if err := h.apoderados.Create(r.Context(), a); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": a})
}

// UpdateApoderado handles PUT /api/v1/empresa/{id}/apoderados/{apoderado_id}
func (h *ProtectedHandler) UpdateApoderado(w http.ResponseWriter, r *http.Request) {
	_, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	apoderadoID, ok := parseID(w, r, "apoderado_id")
	if !ok {
		return
	}

	var body struct {
		Nombre   string `json:"nombre"`
		CURP     string `json:"curp"`
		RFC      string `json:"rfc"`
		Email    string `json:"email"`
		Telefono string `json:"telefono"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	a := &empresas.Apoderado{
		ID:       apoderadoID,
		Nombre:   body.Nombre,
		CURP:     body.CURP,
		RFC:      body.RFC,
		Email:    body.Email,
		Telefono: body.Telefono,
	}
	if err := h.apoderados.Update(r.Context(), a); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "updated"}})
}

// DeleteApoderado handles DELETE /api/v1/empresa/{id}/apoderados/{apoderado_id}
func (h *ProtectedHandler) DeleteApoderado(w http.ResponseWriter, r *http.Request) {
	apoderadoID, ok := parseID(w, r, "apoderado_id")
	if !ok {
		return
	}

	if err := h.apoderados.Delete(r.Context(), apoderadoID); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "deleted"}})
}

// ── servicios ─────────────────────────────────────────────────────────────────

// ListServicios handles GET /api/v1/empresa/{id}/servicios
func (h *ProtectedHandler) ListServicios(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	list, err := h.servicios.ListByEmpresaID(r.Context(), id)
	if err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// CreateServicio handles POST /api/v1/empresa/{id}/servicios
func (h *ProtectedHandler) CreateServicio(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	var body struct {
		Nombre      string  `json:"nombre"`
		Descripcion string  `json:"descripcion"`
		Precio      float64 `json:"precio"`
		StatusActivo bool   `json:"status_activo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s := &empresas.Servicio{
		EmpresaID:    id,
		Nombre:       body.Nombre,
		Descripcion:  body.Descripcion,
		Precio:       body.Precio,
		StatusActivo: body.StatusActivo,
	}
	if err := h.servicios.Create(r.Context(), s); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": s})
}

// UpdateServicio handles PUT /api/v1/empresa/{id}/servicios/{servicio_id}
func (h *ProtectedHandler) UpdateServicio(w http.ResponseWriter, r *http.Request) {
	_, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	servicioID, ok := parseID(w, r, "servicio_id")
	if !ok {
		return
	}

	var body struct {
		Nombre      string  `json:"nombre"`
		Descripcion string  `json:"descripcion"`
		Precio      float64 `json:"precio"`
		StatusActivo bool   `json:"status_activo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s := &empresas.Servicio{
		ID:           servicioID,
		Nombre:       body.Nombre,
		Descripcion:  body.Descripcion,
		Precio:       body.Precio,
		StatusActivo: body.StatusActivo,
	}
	if err := h.servicios.Update(r.Context(), s); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "updated"}})
}

// DeleteServicio handles DELETE /api/v1/empresa/{id}/servicios/{servicio_id}
func (h *ProtectedHandler) DeleteServicio(w http.ResponseWriter, r *http.Request) {
	servicioID, ok := parseID(w, r, "servicio_id")
	if !ok {
		return
	}

	if err := h.servicios.Delete(r.Context(), servicioID); err != nil {
		writeEmpresaError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "deleted"}})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseID(w http.ResponseWriter, r *http.Request, param string) (int64, bool) {
	raw := chi.URLParam(r, param)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid "+param, http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func writeEmpresaError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, empresas.ErrEmpresaNotFound):
		writeError(w, "empresa not found", http.StatusNotFound)
	case errors.Is(err, empresas.ErrDuplicateRFC):
		writeError(w, "RFC already registered", http.StatusConflict)
	case errors.Is(err, empresas.ErrDuplicateTokenPago):
		writeError(w, "payment token already used", http.StatusConflict)
	default:
		writeError(w, "internal server error", http.StatusInternalServerError)
	}
}
