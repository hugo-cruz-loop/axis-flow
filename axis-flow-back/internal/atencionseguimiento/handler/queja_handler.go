package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// QuejaHandler handles HTTP requests for labor complaint (queja) operations.
type QuejaHandler struct {
	svc service.QuejaService
}

// NewQuejaHandler constructs a QuejaHandler.
func NewQuejaHandler(svc service.QuejaService) *QuejaHandler {
	return &QuejaHandler{svc: svc}
}

// createQuejaRequest is the request body for POST /queja.
type createQuejaRequest struct {
	TipoQuejaID       uuid.UUID `json:"tipo_queja_id"`
	Titulo            string    `json:"titulo"`
	Contenido         string    `json:"contenido"`
	ArchivoAdjuntoURL string    `json:"archivo_adjunto_url"`
}

// CreateQueja handles POST /queja.
// Requires JWT with Empleado role.
func (h *QuejaHandler) CreateQueja(w http.ResponseWriter, r *http.Request) {
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
	empleadoID, err := extractEmpleadoID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing empleado id")
		return
	}

	var req createQuejaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.Contenido == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "contenido is required")
		return
	}

	titulo := req.Titulo
	if titulo == "" {
		titulo = "Queja"
	}

	q := &atencionseguimiento.SolicitudQueja{
		TipoQuejaID: req.TipoQuejaID,
		Titulo:      titulo,
		Descripcion: req.Contenido,
	}

	// Use userID as RemitenteID proxy — set on initial message implicitly through service.
	_ = userID

	created, msg, err := h.svc.CreateQueja(r.Context(), q, empleadoID, tenantID)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"queja":           created,
		"mensaje_inicial": msg,
	})
}

// GetQueja handles GET /queja/{id}.
// Requires JWT Empleado/RH/Admin.
func (h *QuejaHandler) GetQueja(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}

	empleadoID, _ := extractEmpleadoID(r)
	role := extractRole(r)

	queja, err := h.svc.GetQueja(r.Context(), id, empleadoID, role)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, queja)
}

// GetMensajes handles GET /queja/{id}/mensajes.
func (h *QuejaHandler) GetMensajes(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}

	page, limit := parsePagination(r)
	empleadoID, _ := extractEmpleadoID(r)
	role := extractRole(r)

	msgs, total, err := h.svc.GetMensajes(r.Context(), id, empleadoID, role, page, limit)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondPaginated(w, http.StatusOK, msgs, page, limit, total)
}

// createMensajeRequest is the request body for POST /queja/{id}/mensaje.
type createMensajeRequest struct {
	Mensaje           string `json:"mensaje"`
	ArchivoAdjuntoURL string `json:"archivo_adjunto_url"`
}

// CreateMensaje handles POST /queja/{id}/mensaje.
func (h *QuejaHandler) CreateMensaje(w http.ResponseWriter, r *http.Request) {
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
	empleadoID, _ := extractEmpleadoID(r)
	role := extractRole(r)

	var req createMensajeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	msg := &atencionseguimiento.RespuestaQueja{
		RemitenteID:       userID,
		Mensaje:           req.Mensaje,
		ArchivoAdjuntoURL: req.ArchivoAdjuntoURL,
	}

	created, err := h.svc.CreateMensaje(r.Context(), id, msg, empleadoID, role)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, created)
}

// GetQuejasByEmpresa handles GET /empresa/{empresa_id}/quejas.
// Requires JWT RH/Admin.
func (h *QuejaHandler) GetQuejasByEmpresa(w http.ResponseWriter, r *http.Request) {
	empresaID, err := uuid.Parse(chi.URLParam(r, "empresa_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid empresa_id")
		return
	}

	page, limit := parsePagination(r)

	var estatus *int
	if s := r.URL.Query().Get("estatus"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid estatus")
			return
		}
		estatus = &v
	}

	quejas, total, err := h.svc.GetQuejasByEmpresa(r.Context(), empresaID, estatus, page, limit)
	if err != nil {
		mapAtencionError(w, err)
		return
	}

	respondPaginated(w, http.StatusOK, quejas, page, limit, total)
}

// parsePagination extracts page and limit from query params with safe defaults.
func parsePagination(r *http.Request) (page, limit int) {
	page = 1
	limit = 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return
}
