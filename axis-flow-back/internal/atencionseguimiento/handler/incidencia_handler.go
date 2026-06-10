package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// IncidenciaHandler handles HTTP requests for supervisor incident reports.
type IncidenciaHandler struct {
	svc service.IncidenciaService
}

// NewIncidenciaHandler constructs an IncidenciaHandler.
func NewIncidenciaHandler(svc service.IncidenciaService) *IncidenciaHandler {
	return &IncidenciaHandler{svc: svc}
}

// createIncidenciaRequest is the request body for POST /incidencia.
type createIncidenciaRequest struct {
	EmpresaID        uuid.UUID `json:"empresa_id"`
	EmpleadoID       int64     `json:"empleado_id"`
	TipoIncidenciaID uuid.UUID `json:"tipo_incidencia_id"`
	Descripcion      string    `json:"descripcion"`
	FechaIncidencia  string    `json:"fecha_incidencia"`
	SancionSugerida  string    `json:"sancion_sugerida"`
	EvidenciaURL     string    `json:"evidencia_url"`
}

// CreateIncidencia handles POST /incidencia.
// Requires JWT Supervisor/Admin.
func (h *IncidenciaHandler) CreateIncidencia(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid tenant")
		return
	}
	userID, err := extractUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid user")
		return
	}

	var req createIncidenciaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.Descripcion == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "descripcion is required")
		return
	}

	var fechaIncidencia time.Time
	if req.FechaIncidencia != "" {
		t, err := time.Parse(time.RFC3339, req.FechaIncidencia)
		if err != nil {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid fecha_incidencia format, use RFC3339")
			return
		}
		fechaIncidencia = t
	}
	_ = fechaIncidencia // stored via the domain struct when schema supports it

	inc := &atencionseguimiento.IncidenciaSupervisor{
		EmpresaID:        req.EmpresaID,
		EmpleadoID:       req.EmpleadoID,
		TipoIncidenciaID: req.TipoIncidenciaID,
		Descripcion:      req.Descripcion,
		SancionSugerida:  req.SancionSugerida,
		EvidenciaURL:     req.EvidenciaURL,
	}

	created, err := h.svc.CreateIncidencia(r.Context(), inc, userID, tenantID)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, created)
}

// GetIncidenciasByEmpresa handles GET /empresa/{empresa_id}/incidencias.
// Requires JWT Admin/RH.
func (h *IncidenciaHandler) GetIncidenciasByEmpresa(w http.ResponseWriter, r *http.Request) {
	empresaID, err := uuid.Parse(chi.URLParam(r, "empresa_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid empresa_id")
		return
	}

	page, limit := parsePagination(r)

	incidencias, total, err := h.svc.GetIncidenciasByEmpresa(r.Context(), empresaID, page, limit)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondPaginated(w, http.StatusOK, incidencias, page, limit, total)
}
