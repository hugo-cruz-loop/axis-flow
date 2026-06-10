package handler

import (
	"encoding/json"
	"net/http"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TicketHandler handles HTTP requests for client service ticket operations.
type TicketHandler struct {
	svc service.TicketService
}

// NewTicketHandler constructs a TicketHandler.
func NewTicketHandler(svc service.TicketService) *TicketHandler {
	return &TicketHandler{svc: svc}
}

// createTicketRequest is the request body for POST /ticket.
type createTicketRequest struct {
	LocalidadID       uuid.UUID `json:"localidad_id"`
	Asunto            string    `json:"asunto"`
	Descripcion       string    `json:"descripcion"`
	ArchivoAdjuntoURL string    `json:"archivo_adjunto_url"`
}

// CreateTicket handles POST /ticket.
// Requires JWT with Cliente role.
func (h *TicketHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
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

	var req createTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.Asunto == "" || req.Descripcion == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "asunto and descripcion are required")
		return
	}

	t := &atencionseguimiento.TicketServicio{
		LocalidadID: req.LocalidadID,
		Asunto:      req.Asunto,
		Descripcion: req.Descripcion,
	}

	created, msg, err := h.svc.CreateTicket(r.Context(), t, userID, tenantID)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"ticket":          created,
		"mensaje_inicial": msg,
	})
}

// GetMensajesServicio handles GET /ticket/{id}/mensajes.
func (h *TicketHandler) GetMensajesServicio(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}

	page, limit := parsePagination(r)
	userID, _ := extractUserID(r)
	role := extractRole(r)

	msgs, total, err := h.svc.GetMensajesServicio(r.Context(), id, userID, role, page, limit)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondPaginated(w, http.StatusOK, msgs, page, limit, total)
}

// createMensajeServicioRequest is the request body for POST /ticket/{id}/mensaje.
type createMensajeServicioRequest struct {
	Mensaje           string `json:"mensaje"`
	ArchivoAdjuntoURL string `json:"archivo_adjunto_url"`
}

// CreateMensajeServicio handles POST /ticket/{id}/mensaje.
func (h *TicketHandler) CreateMensajeServicio(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}

	userID, err := extractUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing user")
		return
	}
	role := extractRole(r)

	var req createMensajeServicioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	msg := &atencionseguimiento.RespuestaServicio{
		RemitenteID:       userID,
		Mensaje:           req.Mensaje,
		ArchivoAdjuntoURL: req.ArchivoAdjuntoURL,
	}

	created, err := h.svc.CreateMensajeServicio(r.Context(), id, msg, userID, role)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, created)
}

// updateTicketEstatusRequest is the request body for PATCH /ticket/{id}/status.
type updateTicketEstatusRequest struct {
	Estatus int `json:"estatus"`
}

// UpdateTicketEstatus handles PATCH /ticket/{id}/status.
// Requires JWT Gestor/Admin.
func (h *TicketHandler) UpdateTicketEstatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}

	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant")
		return
	}
	userID, err := extractUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing user")
		return
	}

	var req updateTicketEstatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	updated, err := h.svc.UpdateTicketEstatus(r.Context(), id, tenantID, req.Estatus, userID)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// GetTicketsByCliente handles GET /cliente/{cliente_id}/tickets.
func (h *TicketHandler) GetTicketsByCliente(w http.ResponseWriter, r *http.Request) {
	clienteID, err := uuid.Parse(chi.URLParam(r, "cliente_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid cliente_id")
		return
	}

	userID, _ := extractUserID(r)
	role := extractRole(r)
	page, limit := parsePagination(r)

	tickets, total, err := h.svc.GetTicketsByCliente(r.Context(), clienteID, userID, role, page, limit)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondPaginated(w, http.StatusOK, tickets, page, limit, total)
}

// GetTicketStats handles GET /empresa/{empresa_id}/tickets/stats.
func (h *TicketHandler) GetTicketStats(w http.ResponseWriter, r *http.Request) {
	empresaID, err := uuid.Parse(chi.URLParam(r, "empresa_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid empresa_id")
		return
	}

	stats, err := h.svc.GetTicketStats(r.Context(), empresaID)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"empresa_id": stats.EmpresaID,
		"pendiente":  stats.Pendiente,
		"en_proceso": stats.EnProceso,
		"finalizado": stats.Finalizado,
	})
}
