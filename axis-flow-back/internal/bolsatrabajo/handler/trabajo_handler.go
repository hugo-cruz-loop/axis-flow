package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TrabajoServicer is the interface TrabajoHandler depends on.
type TrabajoServicer interface {
	Create(ctx context.Context, t *bolsatrabajo.Trabajo, empresaID uuid.UUID) (*bolsatrabajo.Trabajo, error)
	GetActiveJobs(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	GetByEmpresa(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	GetRecent(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	SwitchEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error)
	CloseAllByEmpresa(ctx context.Context, empresaID uuid.UUID) error
}

// Compile-time check that service.TrabajoService satisfies TrabajoServicer.
var _ TrabajoServicer = (service.TrabajoService)(nil)

// TrabajoHandler handles HTTP requests for job postings.
type TrabajoHandler struct {
	svc TrabajoServicer
}

// NewTrabajoHandler constructs a TrabajoHandler.
func NewTrabajoHandler(svc TrabajoServicer) *TrabajoHandler {
	return &TrabajoHandler{svc: svc}
}

// Create handles POST /bolsa-trabajo/trabajo — JWT required.
func (h *TrabajoHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var body struct {
		EmpresaID      string   `json:"empresaId"`
		Titulo         string   `json:"titulo"`
		Descripcion    string   `json:"descripcion"`
		FechaCaducar   string   `json:"fecha_caducar"`
		Requisitos     []string `json:"requisitos"`
		EstatusVacante int      `json:"estatus_vacante"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	var fecha time.Time
	if body.FechaCaducar != "" {
		t, err := time.Parse(time.RFC3339, body.FechaCaducar)
		if err != nil {
			respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid fechaCaducar format (RFC3339 required)")
			return
		}
		fecha = t
	}

	if !fecha.After(time.Now()) {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "fechaCaducar must be a future date")
		return
	}

	t := &bolsatrabajo.Trabajo{
		Titulo:         body.Titulo,
		Descripcion:    body.Descripcion,
		FechaCaducar:   fecha,
		Requisitos:     body.Requisitos,
		EstatusVacante: body.EstatusVacante,
	}

	created, err := h.svc.Create(r.Context(), t, tenantID)
	if err != nil {
		mapBolsaError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, created, newRequestID())
}

// ActiveJobs handles GET /bolsa-trabajo/trabajo/activeJobs — Public.
func (h *TrabajoHandler) ActiveJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")
	page := parseQueryIntDefault(q.Get("page"), 1)
	pageSize := parseQueryIntDefault(q.Get("pageSize"), 20)

	var empresaID *uuid.UUID
	if raw := q.Get("empresaId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid empresaId")
			return
		}
		empresaID = &id
	}

	list, total, err := h.svc.GetActiveJobs(r.Context(), search, empresaID, page, pageSize)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondPaginated(w, http.StatusOK, list, newRequestID(), page, pageSize, total)
}

// ByEmpresa handles GET /bolsa-trabajo/trabajo/by-empresa/{id} — JWT required.
func (h *TrabajoHandler) ByEmpresa(w http.ResponseWriter, r *http.Request) {
	if _, err := extractTenantID(r); err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	idStr := chi.URLParam(r, "id")
	empresaID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid empresa id")
		return
	}

	q := r.URL.Query()
	filter := parseQueryIntDefault(q.Get("filter"), 0)
	page := parseQueryIntDefault(q.Get("page"), 1)
	pageSize := parseQueryIntDefault(q.Get("pageSize"), 20)

	list, total, err := h.svc.GetByEmpresa(r.Context(), empresaID, filter, page, pageSize)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondPaginated(w, http.StatusOK, list, newRequestID(), page, pageSize, total)
}

// Recent handles GET /bolsa-trabajo/trabajo/recent — Public.
func (h *TrabajoHandler) Recent(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := parseQueryIntDefault(q.Get("page"), 1)
	pageSize := parseQueryIntDefault(q.Get("pageSize"), 20)

	list, total, err := h.svc.GetRecent(r.Context(), page, pageSize)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondPaginated(w, http.StatusOK, list, newRequestID(), page, pageSize, total)
}

// SwitchEstatus handles PATCH /bolsa-trabajo/trabajo/switch/{id} — JWT required.
func (h *TrabajoHandler) SwitchEstatus(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid trabajo id")
		return
	}

	var body struct {
		EstatusVacante int `json:"estatusVacante"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	updated, err := h.svc.SwitchEstatus(r.Context(), id, tenantID, body.EstatusVacante)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, updated, newRequestID())
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseQueryIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
