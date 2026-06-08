package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"axis-flow-back/internal/clientes"
	"axis-flow-back/internal/clientes/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ClienteServicer is the interface ClienteHandler depends on.
type ClienteServicer interface {
	CreateCliente(ctx context.Context, req service.CreateClienteRequest) (*clientes.Cliente, error)
	GetCliente(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	ListClientes(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error)
	UpdateCliente(ctx context.Context, id uuid.UUID, empresaID int64, req service.UpdateClienteRequest) (*clientes.Cliente, error)
	DeleteCliente(ctx context.Context, id uuid.UUID, empresaID int64) error
	PatchEstatus(ctx context.Context, id uuid.UUID, empresaID int64, estatus int) (httpStatus int, c *clientes.Cliente, gate *clientes.QualityGateStatus, err error)
	GetClienteByUser(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	GetStats(ctx context.Context, empresaID int64) (active, inactive int, err error)
}

// ClienteHandler handles HTTP requests for the clientes resource.
type ClienteHandler struct {
	svc ClienteServicer
}

// NewClienteHandler constructs a ClienteHandler.
func NewClienteHandler(svc ClienteServicer) *ClienteHandler {
	return &ClienteHandler{svc: svc}
}

// CreateCliente handles POST /cliente
func (h *ClienteHandler) CreateCliente(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EmpresaID           int64     `json:"empresa_id"`
		RepresentanteID     uuid.UUID `json:"representante_id"`
		NombreComercial     string    `json:"nombre_comercial"`
		RazonSocial         string    `json:"razon_social"`
		FechaInicioContrato *string   `json:"fecha_inicio_contrato"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	if body.EmpresaID == 0 {
		writeError(w, "UNAUTHORIZED", "empresa_id is required", http.StatusUnauthorized)
		return
	}

	req := service.CreateClienteRequest{
		EmpresaID:       body.EmpresaID,
		RepresentanteID: body.RepresentanteID,
		NombreComercial: body.NombreComercial,
		RazonSocial:     body.RazonSocial,
	}
	if body.FechaInicioContrato != nil {
		t, err := time.Parse(time.RFC3339, *body.FechaInicioContrato)
		if err != nil {
			writeError(w, "BAD_REQUEST", "invalid fecha_inicio_contrato format", http.StatusBadRequest)
			return
		}
		req.FechaInicioContrato = t
	}

	c, err := h.svc.CreateCliente(r.Context(), req)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": c})
}

// GetCliente handles GET /cliente/{id}
func (h *ClienteHandler) GetCliente(w http.ResponseWriter, r *http.Request) {
	id, empresaID, ok := parseClienteParams(w, r)
	if !ok {
		return
	}

	c, err := h.svc.GetCliente(r.Context(), id, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c})
}

// UpdateCliente handles PATCH /cliente/{id}
func (h *ClienteHandler) UpdateCliente(w http.ResponseWriter, r *http.Request) {
	id, empresaID, ok := parseClienteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		NombreComercial     *string `json:"nombre_comercial"`
		RazonSocial         *string `json:"razon_social"`
		FechaInicioContrato *string `json:"fecha_inicio_contrato"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	req := service.UpdateClienteRequest{
		NombreComercial: body.NombreComercial,
		RazonSocial:     body.RazonSocial,
	}
	if body.FechaInicioContrato != nil {
		t, err := time.Parse(time.RFC3339, *body.FechaInicioContrato)
		if err != nil {
			writeError(w, "BAD_REQUEST", "invalid fecha_inicio_contrato format", http.StatusBadRequest)
			return
		}
		req.FechaInicioContrato = &t
	}

	c, err := h.svc.UpdateCliente(r.Context(), id, empresaID, req)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c})
}

// PatchEstatus handles PATCH /cliente/{id}/estatus
func (h *ClienteHandler) PatchEstatus(w http.ResponseWriter, r *http.Request) {
	id, empresaID, ok := parseClienteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		Estatus int `json:"estatus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	httpStatus, c, gate, err := h.svc.PatchEstatus(r.Context(), id, empresaID, body.Estatus)
	if err != nil {
		writeClienteError(w, err)
		return
	}

	if httpStatus == 206 {
		missing := []string{}
		if gate != nil {
			if !gate.FacturaOK {
				missing = append(missing, "factura")
			}
			if !gate.PresupuestoOK {
				missing = append(missing, "presupuesto")
			}
			if !gate.CalendarioOK {
				missing = append(missing, "calendario")
			}
		}
		writeJSON(w, http.StatusPartialContent, map[string]any{
			"data": c,
			"warnings": map[string]any{
				"message":          "activation blocked: missing required sections",
				"missing_sections": missing,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": c})
}

// DeleteCliente handles DELETE /cliente/{id}
func (h *ClienteHandler) DeleteCliente(w http.ResponseWriter, r *http.Request) {
	id, empresaID, ok := parseClienteParams(w, r)
	if !ok {
		return
	}

	if err := h.svc.DeleteCliente(r.Context(), id, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetClienteByUser handles GET /cliente/by-user/{user_id}
func (h *ClienteHandler) GetClienteByUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid user_id", http.StatusBadRequest)
		return
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}

	c, err := h.svc.GetClienteByUser(r.Context(), userID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c})
}

// ListClientes handles GET /empresa/{empresa_id}/clientes
func (h *ClienteHandler) ListClientes(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaID(w, r)
	if !ok {
		return
	}

	page := parseQueryInt(r, "page", 1)
	size := parseQueryInt(r, "size", 20)

	list, total, err := h.svc.ListClientes(r.Context(), empresaID, page, size)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetStats handles GET /empresa/{empresa_id}/clientes/stats
func (h *ClienteHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	empresaID, ok := parseEmpresaID(w, r)
	if !ok {
		return
	}

	active, inactive, err := h.svc.GetStats(r.Context(), empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"active":   active,
			"inactive": inactive,
		},
	})
}

// ── local helpers ─────────────────────────────────────────────────────────────

// parseClienteParams extracts {id} UUID and empresa_id query param.
func parseClienteParams(w http.ResponseWriter, r *http.Request) (uuid.UUID, int64, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid id", http.StatusBadRequest)
		return uuid.Nil, 0, false
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return uuid.Nil, 0, false
	}

	return id, empresaID, true
}

// parseEmpresaID extracts {empresa_id} from URL path.
func parseEmpresaID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "empresa_id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "BAD_REQUEST", "invalid empresa_id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// parseQueryInt parses a query param as int, returning defaultVal on missing/invalid.
func parseQueryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}
