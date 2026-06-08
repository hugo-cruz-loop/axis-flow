package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"axis-flow-back/internal/clientes"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// LocalidadServicer is the interface LocalidadHandler depends on.
type LocalidadServicer interface {
	CreateLocalidad(ctx context.Context, l *clientes.Localidad, empresaID int64) (*clientes.Localidad, error)
	GetLocalidad(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error)
	ListLocalidades(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error)
	UpdateLocalidad(ctx context.Context, l *clientes.Localidad, empresaID int64) (*clientes.Localidad, error)
	DeleteLocalidad(ctx context.Context, id uuid.UUID, empresaID int64) error
}

// LocalidadHandler handles HTTP requests for localidades.
type LocalidadHandler struct {
	svc LocalidadServicer
}

// NewLocalidadHandler constructs a LocalidadHandler.
func NewLocalidadHandler(svc LocalidadServicer) *LocalidadHandler {
	return &LocalidadHandler{svc: svc}
}

// CreateLocalidad handles POST /cliente/{id}/localidades
func (h *LocalidadHandler) CreateLocalidad(w http.ResponseWriter, r *http.Request) {
	clienteIDStr := chi.URLParam(r, "id")
	clienteID, err := uuid.Parse(clienteIDStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid id", http.StatusBadRequest)
		return
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}

	var body struct {
		Nombre          string   `json:"nombre"`
		Direccion       string   `json:"direccion"`
		TipoLocalidadID int64    `json:"tipo_localidad_id"`
		Latitud         *float64 `json:"latitud"`
		Longitud        *float64 `json:"longitud"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	l := &clientes.Localidad{
		ClienteID:       clienteID,
		Nombre:          body.Nombre,
		Direccion:       body.Direccion,
		TipoLocalidadID: body.TipoLocalidadID,
		Latitud:         body.Latitud,
		Longitud:        body.Longitud,
	}
	created, err := h.svc.CreateLocalidad(r.Context(), l, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

// ListLocalidades handles GET /cliente/{id}/localidades
func (h *LocalidadHandler) ListLocalidades(w http.ResponseWriter, r *http.Request) {
	clienteIDStr := chi.URLParam(r, "id")
	clienteID, err := uuid.Parse(clienteIDStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid id", http.StatusBadRequest)
		return
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}

	list, err := h.svc.ListLocalidades(r.Context(), clienteID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// GetLocalidad handles GET /localidad/{localidad_id}
func (h *LocalidadHandler) GetLocalidad(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "localidad_id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid localidad_id", http.StatusBadRequest)
		return
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}

	l, err := h.svc.GetLocalidad(r.Context(), id, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": l})
}

// UpdateLocalidad handles PUT /localidad/{localidad_id}
func (h *LocalidadHandler) UpdateLocalidad(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "localidad_id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid localidad_id", http.StatusBadRequest)
		return
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}

	var body struct {
		Nombre          string   `json:"nombre"`
		Direccion       string   `json:"direccion"`
		TipoLocalidadID int64    `json:"tipo_localidad_id"`
		Latitud         *float64 `json:"latitud"`
		Longitud        *float64 `json:"longitud"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	l := &clientes.Localidad{
		ID:              id,
		Nombre:          body.Nombre,
		Direccion:       body.Direccion,
		TipoLocalidadID: body.TipoLocalidadID,
		Latitud:         body.Latitud,
		Longitud:        body.Longitud,
	}
	updated, err := h.svc.UpdateLocalidad(r.Context(), l, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

// DeleteLocalidad handles DELETE /localidad/{localidad_id}
func (h *LocalidadHandler) DeleteLocalidad(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "localidad_id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid localidad_id", http.StatusBadRequest)
		return
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}

	if err := h.svc.DeleteLocalidad(r.Context(), id, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
