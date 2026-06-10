// Package handler — formulario_handler.go: HTTP handlers for Formulario
// template headers and their child Pregunta rows.
//
// PR-4 (REST/HTTP) — task 4.1. Mirrors the structure of
// internal/atencionseguimiento/handler/queja_handler.go (PR-4 of
// 09_AtencionSeguimiento_Service_Spec).
//
// Routes:
//
//	POST /formulario              — CreateFormulario
//	GET  /formulario/byempresa/{id} — GetFormulariosByEmpresa
//	POST /pregunta                — CreatePregunta
//
// IDOR rules enforced at the handler boundary:
//
//   - CreateFormulario: the body empresa_id MUST equal the JWT tenant.
//     Foreign-empresa body → 403.
//   - GetFormulariosByEmpresa: the path {id} MUST equal the JWT tenant.
//     Foreign-empresa path → 403.
//   - CreatePregunta: the parent formulario is re-validated by the
//     service under the JWT tenant; a foreign-tenant parent surfaces
//     as ErrNotFound (no leak) → 404.
//
// All validation errors are mapped to 422 per the openapi contract.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Response DTOs (openapi snake_case contract).
//
// The formularios.Formulario domain type uses Go field names (ID,
// EmpresaID, ...) and would marshal as such. The openapi contract uses
// snake_case (id, empresa_id, ...). These unexported DTOs bridge the two
// without polluting the domain type with transport-layer concerns.
// ---------------------------------------------------------------------------

type formularioResponseDTO struct {
	ID          uuid.UUID `json:"id"`
	EmpresaID   uuid.UUID `json:"empresa_id"`
	Nombre      string    `json:"nombre"`
	Descripcion string    `json:"descripcion"`
	Activo      bool      `json:"activo"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type preguntaResponseDTO struct {
	ID                   uuid.UUID       `json:"id"`
	FormularioID         uuid.UUID       `json:"formulario_id"`
	Orden                int             `json:"orden"`
	TipoPregunta         int             `json:"tipo_pregunta"`
	TextoPregunta        string          `json:"texto_pregunta"`
	Obligatoria          bool            `json:"obligatoria"`
	RespuestaPredefinida json.RawMessage `json:"respuesta_predefinida,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
}

// FormularioHandler handles HTTP requests for Formulario template headers
// and their child Pregunta rows.
type FormularioHandler struct {
	svc service.FormularioService
}

// NewFormularioHandler constructs a FormularioHandler.
func NewFormularioHandler(svc service.FormularioService) *FormularioHandler {
	return &FormularioHandler{svc: svc}
}

// ---------------------------------------------------------------------------
// POST /formulario — CreateFormulario.
// ---------------------------------------------------------------------------

// createFormularioRequest is the JSON body for POST /formulario. Mirrors
// the openapi `application/json` schema for /formulario POST.
type createFormularioRequest struct {
	EmpresaID   uuid.UUID `json:"empresa_id"`
	Nombre      string    `json:"nombre"`
	Descripcion string    `json:"descripcion"`
	Activo      *bool     `json:"activo,omitempty"`
}

// CreateFormulario handles POST /formulario. Requires JWT with role that
// matches the openapi `Admin` / `Manager` allow-list (enforced upstream
// by requireRoles in cmd/server/formularios_routes.go).
func (h *FormularioHandler) CreateFormulario(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing tenant")
		return
	}

	var req createFormularioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "invalid request body")
		return
	}
	if strings.TrimSpace(req.Nombre) == "" {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "nombre is required")
		return
	}
	if len(req.Nombre) > maxFormularioNombreHTTP {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "nombre exceeds max length")
		return
	}
	// IDOR: the body's empresa_id MUST equal the JWT tenant. A
	// foreign-tenant body is a tenant-spoofing attempt.
	if req.EmpresaID != tenantID {
		respondError(w, http.StatusForbidden, "forbidden", "tenant mismatch")
		return
	}

	activo := true
	if req.Activo != nil {
		activo = *req.Activo
	}
	f := &formularios.Formulario{
		EmpresaID:   tenantID,
		Nombre:      req.Nombre,
		Descripcion: req.Descripcion,
		Activo:      activo,
	}

	created, err := h.svc.CreateFormulario(r.Context(), f, tenantID)
	if err != nil {
		mapFormulariosError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, toFormularioResponseDTO(created))
}

// maxFormularioNombreHTTP mirrors the openapi `maxLength: 150` on
// /formulario POST body.nombre, kept as a constant so the openapi and
// the handler stay in lockstep on a future bump.
const maxFormularioNombreHTTP = 150

// ---------------------------------------------------------------------------
// GET /formulario/byempresa/{id} — GetFormulariosByEmpresa.
// ---------------------------------------------------------------------------

// GetFormulariosByEmpresa handles GET /formulario/byempresa/{id}.
// Requires JWT (any role). IDOR: the path {id} MUST equal the JWT tenant.
func (h *FormularioHandler) GetFormulariosByEmpresa(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing tenant")
		return
	}
	empresaID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "invalid empresa id")
		return
	}
	if empresaID != tenantID {
		respondError(w, http.StatusForbidden, "forbidden", "tenant mismatch")
		return
	}

	page, limit := parsePagination(r)

	var activo *bool
	if a := r.URL.Query().Get("activo"); a != "" {
		v, err := strconv.ParseBool(a)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "invalid activo flag")
			return
		}
		activo = &v
	}

	items, total, err := h.svc.GetFormulariosByEmpresa(r.Context(), empresaID, activo, page, limit)
	if err != nil {
		mapFormulariosError(w, err)
		return
	}

	dtos := make([]formularioResponseDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toFormularioResponseDTO(it))
	}
	respondPaginated(w, http.StatusOK, dtos, page, limit, total)
}

// ---------------------------------------------------------------------------
// POST /pregunta — CreatePregunta.
// ---------------------------------------------------------------------------

// createPreguntaRequest is the JSON body for POST /pregunta. Mirrors the
// openapi `application/json` schema for /pregunta POST.
type createPreguntaRequest struct {
	FormularioID        uuid.UUID       `json:"formulario_id"`
	Orden               int             `json:"orden"`
	TipoPregunta        int             `json:"tipo_pregunta"`
	TextoPregunta       string          `json:"texto_pregunta"`
	Obligatoria         bool            `json:"obligatoria"`
	RespuestaPredefinida json.RawMessage `json:"respuesta_predefinida,omitempty"`
}

// validTipoPreguntaHTTP returns true for the closed set enforced by the
// formularios_pregunta.tipo_pregunta SQL CHECK. Mirrors the constants in
// internal/formularios/domain.go.
func validTipoPreguntaHTTP(t int) bool {
	switch t {
	case formularios.TipoPreguntaTexto,
		formularios.TipoPreguntaCheckbox,
		formularios.TipoPreguntaRating,
		formularios.TipoPreguntaMatriz,
		formularios.TipoPreguntaFoto,
		formularios.TipoPreguntaFirma:
		return true
	}
	return false
}

// CreatePregunta handles POST /pregunta. Requires JWT with role that
// matches the openapi `Admin` allow-list (enforced upstream).
//
// IDOR is delegated to the service: AddPregunta calls
// FormularioRepository.GetByID(formularioID, tenantID) which returns
// formularios.ErrNotFound for foreign-tenant parents (no leak).
func (h *FormularioHandler) CreatePregunta(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing tenant")
		return
	}

	var req createPreguntaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "invalid request body")
		return
	}
	if req.Orden < 1 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "orden must be ≥ 1")
		return
	}
	if strings.TrimSpace(req.TextoPregunta) == "" {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "texto_pregunta is required")
		return
	}
	if !validTipoPreguntaHTTP(req.TipoPregunta) {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "invalid tipo_pregunta")
		return
	}

	p := &formularios.Pregunta{
		FormularioID:         req.FormularioID,
		Orden:                req.Orden,
		TipoPregunta:         req.TipoPregunta,
		TextoPregunta:        req.TextoPregunta,
		Obligatoria:          req.Obligatoria,
		RespuestaPredefinida: req.RespuestaPredefinida,
	}

	created, err := h.svc.AddPregunta(r.Context(), p, tenantID)
	if err != nil {
		mapFormulariosError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, toPreguntaResponseDTO(created))
}

// ---------------------------------------------------------------------------
// DTO conversions.
// ---------------------------------------------------------------------------

func toFormularioResponseDTO(f *formularios.Formulario) formularioResponseDTO {
	return formularioResponseDTO{
		ID:          f.ID,
		EmpresaID:   f.EmpresaID,
		Nombre:      f.Nombre,
		Descripcion: f.Descripcion,
		Activo:      f.Activo,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}
}

func toPreguntaResponseDTO(p *formularios.Pregunta) preguntaResponseDTO {
	return preguntaResponseDTO{
		ID:                   p.ID,
		FormularioID:         p.FormularioID,
		Orden:                p.Orden,
		TipoPregunta:         p.TipoPregunta,
		TextoPregunta:        p.TextoPregunta,
		Obligatoria:          p.Obligatoria,
		RespuestaPredefinida: p.RespuestaPredefinida,
		CreatedAt:            p.CreatedAt,
	}
}
