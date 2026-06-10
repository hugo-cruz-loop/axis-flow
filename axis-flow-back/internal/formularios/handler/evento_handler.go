// Package handler — evento_handler.go: HTTP handlers for Evento headers
// and their check-in (EventoIniciado) lifecycle.
//
// PR-4 (REST/HTTP) — task 4.2. Mirrors the structure of
// internal/atencionseguimiento/handler/queja_handler.go (PR-4 of
// 09_AtencionSeguimiento_Service_Spec).
//
// Routes:
//
//	POST /evento            — CreateEvento
//	POST /evento_iniciado   — IniciarEvento
//	GET  /evento/byEmpId/{empId}/{cteId} — GetEventosByEmpCte
//
// IDOR rules enforced at the handler boundary:
//
//   - CreateEvento: the body empresa_id MUST equal the JWT tenant.
//     Foreign-empresa body → 403.
//   - IniciarEvento: parent evento is re-validated by the service
//     under the JWT tenant. Foreign-tenant parent surfaces as
//     formularios.ErrNotFound (no leak) → 404.
//   - GetEventosByEmpCte: the path {empId} MUST equal the JWT tenant.
//     Foreign-emp path → 403.
package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Response DTOs (openapi snake_case contract).
// ---------------------------------------------------------------------------

type eventoResponseDTO struct {
	ID                  uuid.UUID   `json:"id"`
	EmpresaID           uuid.UUID   `json:"empresa_id"`
	ClienteID           uuid.UUID   `json:"cliente_id"`
	LocalidadID         uuid.UUID   `json:"localidad_id"`
	Nombre              string      `json:"nombre"`
	Descripcion         string      `json:"descripcion"`
	FechaProgramada     time.Time   `json:"fecha_programada"`
	Estatus             string      `json:"estatus"`
	FormulariosAsociados []uuid.UUID `json:"formularios_asociados,omitempty"`
	CreatedAt           time.Time   `json:"created_at"`
}

type eventoIniciadoResponseDTO struct {
	ID                    uuid.UUID `json:"id"`
	EventoID              uuid.UUID `json:"evento_id"`
	EmpleadoID            int64     `json:"empleado_id"`
	FechaInicio           time.Time `json:"fecha_inicio"`
	Estatus               string    `json:"estatus"`
	GeolocalizacionInicio *geoDTO   `json:"geolocalizacion_inicio,omitempty"`
}

type geoDTO struct {
	Latitud  float64 `json:"latitud"`
	Longitud float64 `json:"longitud"`
}

// ---------------------------------------------------------------------------
// EventoHandler.
// ---------------------------------------------------------------------------

// EventoHandler handles HTTP requests for Evento headers and their
// check-in (EventoIniciado) lifecycle.
type EventoHandler struct {
	svc service.EventoService
}

// NewEventoHandler constructs an EventoHandler.
func NewEventoHandler(svc service.EventoService) *EventoHandler {
	return &EventoHandler{svc: svc}
}

// ---------------------------------------------------------------------------
// POST /evento — CreateEvento.
// ---------------------------------------------------------------------------

// createEventoRequest is the JSON body for POST /evento. Mirrors the
// openapi `application/json` schema for /evento POST.
type createEventoRequest struct {
	EmpresaID            uuid.UUID   `json:"empresa_id"`
	ClienteID            uuid.UUID   `json:"cliente_id"`
	LocalidadID          uuid.UUID   `json:"localidad_id"`
	Nombre               string      `json:"nombre"`
	Descripcion          string      `json:"descripcion"`
	FechaProgramada      time.Time   `json:"fecha_programada"`
	FormulariosAsociados []uuid.UUID `json:"formularios_asociados"`
}

// CreateEvento handles POST /evento. Requires JWT with role that matches
// the openapi `Manager` allow-list (enforced upstream).
func (h *EventoHandler) CreateEvento(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant")
		return
	}

	var req createEventoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if strings.TrimSpace(req.Nombre) == "" {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "nombre is required")
		return
	}
	if req.FechaProgramada.IsZero() {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "fecha_programada is required")
		return
	}
	if req.ClienteID == uuid.Nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "cliente_id is required")
		return
	}
	// IDOR: the body's empresa_id MUST equal the JWT tenant.
	if req.EmpresaID != tenantID {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "tenant mismatch")
		return
	}

	e := &formularios.Evento{
		EmpresaID:       tenantID,
		ClienteID:       req.ClienteID,
		LocalidadID:     req.LocalidadID,
		Nombre:          req.Nombre,
		Descripcion:     req.Descripcion,
		FechaProgramada: req.FechaProgramada,
		Status:          formularios.EventoStatusPendiente,
	}

	created, err := h.svc.CreateEvento(r.Context(), e, req.FormulariosAsociados, tenantID)
	if err != nil {
		mapFormulariosError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, toEventoResponseDTO(created, req.FormulariosAsociados))
}

// ---------------------------------------------------------------------------
// POST /evento_iniciado — IniciarEvento.
// ---------------------------------------------------------------------------

// iniciarEventoRequest is the JSON body for POST /evento_iniciado.
// empleado_id is accepted in the body for backwards compatibility with
// the spec's example (the openapi sample shows 5f3a1e0b as a number).
// The handler ALSO pulls EmpleadoID from the JWT context and uses that
// when the body field is zero — defence in depth against body-spoofing.
type iniciarEventoRequest struct {
	EventoID             uuid.UUID `json:"evento_id"`
	EmpleadoID           int64     `json:"empleado_id"`
	GeolocalizacionInicio *geoDTO  `json:"geolocalizacion_inicio,omitempty"`
}

// IniciarEvento handles POST /evento_iniciado. Requires JWT with role
// that matches the openapi `Empleado` allow-list (enforced upstream).
//
// IDOR: parent evento is re-validated by the service under the JWT
// tenant. Foreign-tenant parent → formularios.ErrNotFound → 404.
//
// Geolocalizacion is validated at the handler for defense in depth (the
// service also enforces the lat/lon range).
func (h *EventoHandler) IniciarEvento(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant")
		return
	}
	_ = tenantID // tenant check happens at the service via the parent evento

	// Prefer the JWT-claim EmpleadoID; fall back to the body field if
	// the claim is missing (production TODO — see Deviation #3 in
	// apply-progress).
	empleadoID, _ := extractEmpleadoID(r)

	var req iniciarEventoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.EventoID == uuid.Nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "evento_id is required")
		return
	}
	if req.EmpleadoID <= 0 && empleadoID <= 0 {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "empleado_id is required")
		return
	}
	if empleadoID <= 0 {
		empleadoID = req.EmpleadoID
	}
	if req.GeolocalizacionInicio != nil {
		if !validLatLonHTTP(req.GeolocalizacionInicio.Latitud, req.GeolocalizacionInicio.Longitud) {
			respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid geolocalizacion")
			return
		}
	}

	ei := &formularios.EventoIniciado{
		EventoID:                 req.EventoID,
		EmpleadoID:               empleadoID,
		GeolocalizacionInicioLat: latPtrIfProvided(req.GeolocalizacionInicio),
		GeolocalizacionInicioLon: lonPtrIfProvided(req.GeolocalizacionInicio),
		Status:                   formularios.IniciadoStatus,
	}

	created, err := h.svc.IniciarEvento(r.Context(), ei, tenantID)
	if err != nil {
		mapFormulariosError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, toEventoIniciadoResponseDTO(created, req.GeolocalizacionInicio))
}

// ---------------------------------------------------------------------------
// GET /evento/byEmpId/{empId}/{cteId} — GetEventosByEmpCte.
// ---------------------------------------------------------------------------

// GetEventosByEmpCte handles GET /evento/byEmpId/{empId}/{cteId}.
// Requires JWT (any role). IDOR: path {empId} MUST equal JWT tenant.
func (h *EventoHandler) GetEventosByEmpCte(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant")
		return
	}
	empresaID, err := uuid.Parse(chi.URLParam(r, "empId"))
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid empresa id")
		return
	}
	clienteID, err := uuid.Parse(chi.URLParam(r, "cteId"))
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid cliente id")
		return
	}
	if empresaID != tenantID {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "tenant mismatch")
		return
	}

	page, limit := parsePagination(r)

	var status *string
	if s := r.URL.Query().Get("estatus"); s != "" {
		s2 := s
		status = &s2
	}

	items, total, err := h.svc.GetEventosByEmpCte(r.Context(), empresaID, clienteID, status, page, limit)
	if err != nil {
		mapFormulariosError(w, err)
		return
	}

	dtos := make([]eventoResponseDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toEventoResponseDTO(it, nil))
	}
	respondPaginated(w, http.StatusOK, dtos, page, limit, total)
}

// ---------------------------------------------------------------------------
// Geo validation helpers.
// ---------------------------------------------------------------------------

// validLatLonHTTP enforces the openapi lat/lon range (-90..90, -180..180).
func validLatLonHTTP(lat, lon float64) bool {
	if lat < -90 || lat > 90 {
		return false
	}
	if lon < -180 || lon > 180 {
		return false
	}
	return true
}

func latPtrIfProvided(g *geoDTO) *float64 {
	if g == nil {
		return nil
	}
	v := g.Latitud
	return &v
}

func lonPtrIfProvided(g *geoDTO) *float64 {
	if g == nil {
		return nil
	}
	v := g.Longitud
	return &v
}

// ---------------------------------------------------------------------------
// DTO conversions.
// ---------------------------------------------------------------------------

func toEventoResponseDTO(e *formularios.Evento, formulariosAsoc []uuid.UUID) eventoResponseDTO {
	dto := eventoResponseDTO{
		ID:              e.ID,
		EmpresaID:       e.EmpresaID,
		ClienteID:       e.ClienteID,
		LocalidadID:     e.LocalidadID,
		Nombre:          e.Nombre,
		Descripcion:     e.Descripcion,
		FechaProgramada: e.FechaProgramada,
		Estatus:         e.Status,
		CreatedAt:       e.CreatedAt,
	}
	if len(formulariosAsoc) > 0 {
		dto.FormulariosAsociados = formulariosAsoc
	}
	return dto
}

func toEventoIniciadoResponseDTO(ei *formularios.EventoIniciado, geo *geoDTO) eventoIniciadoResponseDTO {
	dto := eventoIniciadoResponseDTO{
		ID:          ei.ID,
		EventoID:    ei.EventoID,
		EmpleadoID:  ei.EmpleadoID,
		FechaInicio: ei.CheckInTime,
		Estatus:     ei.Status,
	}
	if geo != nil {
		dto.GeolocalizacionInicio = geo
	}
	return dto
}
