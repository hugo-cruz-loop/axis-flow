package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"axis-flow-back/internal/clientes"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// SiteConfigServicer is the interface SiteConfigHandler depends on.
type SiteConfigServicer interface {
	AddServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64, empresaID int64) error
	RemoveServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64, empresaID int64) error
	ListServicios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.ServiciosLocalidad, error)

	CreateHorario(ctx context.Context, h *clientes.Horario, empresaID int64) (*clientes.Horario, error)
	ListHorarios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Horario, error)
	UpdateHorario(ctx context.Context, h *clientes.Horario, empresaID int64) (*clientes.Horario, error)
	DeleteHorario(ctx context.Context, id uuid.UUID, empresaID int64) error

	CreateHerramienta(ctx context.Context, h *clientes.Herramienta, empresaID int64) (*clientes.Herramienta, error)
	ListHerramientas(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Herramienta, error)
	UpdateHerramienta(ctx context.Context, h *clientes.Herramienta, empresaID int64) (*clientes.Herramienta, error)
	DeleteHerramienta(ctx context.Context, id uuid.UUID, empresaID int64) error

	CreateActividad(ctx context.Context, a *clientes.Actividad, empresaID int64) (*clientes.Actividad, error)
	ListActividades(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Actividad, error)
	UpdateActividad(ctx context.Context, a *clientes.Actividad, empresaID int64) (*clientes.Actividad, error)
	DeleteActividad(ctx context.Context, id uuid.UUID, empresaID int64) error
}

// SiteConfigHandler handles site configuration endpoints for a localidad.
type SiteConfigHandler struct {
	svc SiteConfigServicer
}

// NewSiteConfigHandler constructs a SiteConfigHandler.
func NewSiteConfigHandler(svc SiteConfigServicer) *SiteConfigHandler {
	return &SiteConfigHandler{svc: svc}
}

// ── Servicios ─────────────────────────────────────────────────────────────────

// ListServicios handles GET /localidad/{localidad_id}/servicios
func (h *SiteConfigHandler) ListServicios(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	list, err := h.svc.ListServicios(r.Context(), localidadID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// AddServicio handles POST /localidad/{localidad_id}/servicios
func (h *SiteConfigHandler) AddServicio(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		ServicioID int64 `json:"servicio_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.svc.AddServicio(r.Context(), localidadID, body.ServicioID, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// RemoveServicio handles DELETE /localidad/{localidad_id}/servicios/{servicio_id}
func (h *SiteConfigHandler) RemoveServicio(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	servicioIDStr := chi.URLParam(r, "servicio_id")
	servicioID, err := strconv.ParseInt(servicioIDStr, 10, 64)
	if err != nil || servicioID <= 0 {
		writeError(w, "BAD_REQUEST", "invalid servicio_id", http.StatusBadRequest)
		return
	}

	if err := h.svc.RemoveServicio(r.Context(), localidadID, servicioID, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Horarios ──────────────────────────────────────────────────────────────────

// ListHorarios handles GET /localidad/{localidad_id}/horario
func (h *SiteConfigHandler) ListHorarios(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	list, err := h.svc.ListHorarios(r.Context(), localidadID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// CreateHorario handles POST /localidad/{localidad_id}/horario
func (h *SiteConfigHandler) CreateHorario(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		HoraEntrada      string  `json:"hora_entrada"`
		HoraSalida       string  `json:"hora_salida"`
		HoraComidaInicio *string `json:"hora_comida_inicio"`
		HoraComidaFin    *string `json:"hora_comida_fin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	hor := &clientes.Horario{
		LocalidadID:      localidadID,
		HoraEntrada:      body.HoraEntrada,
		HoraSalida:       body.HoraSalida,
		HoraComidaInicio: body.HoraComidaInicio,
		HoraComidaFin:    body.HoraComidaFin,
	}
	created, err := h.svc.CreateHorario(r.Context(), hor, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

// UpdateHorario handles PUT /localidad/{localidad_id}/horario/{horario_id}
func (h *SiteConfigHandler) UpdateHorario(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	horarioID, ok2 := parseSiteSubID(w, r, "horario_id")
	if !ok2 {
		return
	}

	var body struct {
		HoraEntrada      string  `json:"hora_entrada"`
		HoraSalida       string  `json:"hora_salida"`
		HoraComidaInicio *string `json:"hora_comida_inicio"`
		HoraComidaFin    *string `json:"hora_comida_fin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	hor := &clientes.Horario{
		ID:               horarioID,
		LocalidadID:      localidadID,
		HoraEntrada:      body.HoraEntrada,
		HoraSalida:       body.HoraSalida,
		HoraComidaInicio: body.HoraComidaInicio,
		HoraComidaFin:    body.HoraComidaFin,
	}
	updated, err := h.svc.UpdateHorario(r.Context(), hor, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

// DeleteHorario handles DELETE /localidad/{localidad_id}/horario/{horario_id}
func (h *SiteConfigHandler) DeleteHorario(w http.ResponseWriter, r *http.Request) {
	_, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	horarioID, ok2 := parseSiteSubID(w, r, "horario_id")
	if !ok2 {
		return
	}

	if err := h.svc.DeleteHorario(r.Context(), horarioID, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Herramientas ──────────────────────────────────────────────────────────────

// ListHerramientas handles GET /localidad/{localidad_id}/herramientas
func (h *SiteConfigHandler) ListHerramientas(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	list, err := h.svc.ListHerramientas(r.Context(), localidadID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// CreateHerramienta handles POST /localidad/{localidad_id}/herramientas
func (h *SiteConfigHandler) CreateHerramienta(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		Nombre           string  `json:"nombre"`
		Cantidad         int     `json:"cantidad"`
		Especificaciones *string `json:"especificaciones"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	her := &clientes.Herramienta{
		LocalidadID:      localidadID,
		Nombre:           body.Nombre,
		Cantidad:         body.Cantidad,
		Especificaciones: body.Especificaciones,
	}
	created, err := h.svc.CreateHerramienta(r.Context(), her, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

// UpdateHerramienta handles PUT /localidad/{localidad_id}/herramientas/{id}
func (h *SiteConfigHandler) UpdateHerramienta(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	subID, ok2 := parseSiteSubID(w, r, "id")
	if !ok2 {
		return
	}

	var body struct {
		Nombre           string  `json:"nombre"`
		Cantidad         int     `json:"cantidad"`
		Especificaciones *string `json:"especificaciones"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	her := &clientes.Herramienta{
		ID:               subID,
		LocalidadID:      localidadID,
		Nombre:           body.Nombre,
		Cantidad:         body.Cantidad,
		Especificaciones: body.Especificaciones,
	}
	updated, err := h.svc.UpdateHerramienta(r.Context(), her, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

// DeleteHerramienta handles DELETE /localidad/{localidad_id}/herramientas/{id}
func (h *SiteConfigHandler) DeleteHerramienta(w http.ResponseWriter, r *http.Request) {
	_, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	subID, ok2 := parseSiteSubID(w, r, "id")
	if !ok2 {
		return
	}

	if err := h.svc.DeleteHerramienta(r.Context(), subID, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Actividades ───────────────────────────────────────────────────────────────

// ListActividades handles GET /localidad/{localidad_id}/actividades
func (h *SiteConfigHandler) ListActividades(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	list, err := h.svc.ListActividades(r.Context(), localidadID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// CreateActividad handles POST /localidad/{localidad_id}/actividades
func (h *SiteConfigHandler) CreateActividad(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		Descripcion string `json:"descripcion"`
		Frecuencia  string `json:"frecuencia"`
		Orden       int    `json:"orden"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	act := &clientes.Actividad{
		LocalidadID: localidadID,
		Descripcion: body.Descripcion,
		Frecuencia:  body.Frecuencia,
		Orden:       body.Orden,
	}
	created, err := h.svc.CreateActividad(r.Context(), act, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

// UpdateActividad handles PUT /localidad/{localidad_id}/actividades/{id}
func (h *SiteConfigHandler) UpdateActividad(w http.ResponseWriter, r *http.Request) {
	localidadID, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	subID, ok2 := parseSiteSubID(w, r, "id")
	if !ok2 {
		return
	}

	var body struct {
		Descripcion string `json:"descripcion"`
		Frecuencia  string `json:"frecuencia"`
		Orden       int    `json:"orden"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	act := &clientes.Actividad{
		ID:          subID,
		LocalidadID: localidadID,
		Descripcion: body.Descripcion,
		Frecuencia:  body.Frecuencia,
		Orden:       body.Orden,
	}
	updated, err := h.svc.UpdateActividad(r.Context(), act, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

// DeleteActividad handles DELETE /localidad/{localidad_id}/actividades/{id}
func (h *SiteConfigHandler) DeleteActividad(w http.ResponseWriter, r *http.Request) {
	_, empresaID, ok := parseSiteParams(w, r)
	if !ok {
		return
	}

	subID, ok2 := parseSiteSubID(w, r, "id")
	if !ok2 {
		return
	}

	if err := h.svc.DeleteActividad(r.Context(), subID, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── local helpers ─────────────────────────────────────────────────────────────

func parseSiteParams(w http.ResponseWriter, r *http.Request) (uuid.UUID, int64, bool) {
	idStr := chi.URLParam(r, "localidad_id")
	localidadID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid localidad_id", http.StatusBadRequest)
		return uuid.Nil, 0, false
	}
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return uuid.Nil, 0, false
	}
	return localidadID, empresaID, true
}

func parseSiteSubID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	idStr := chi.URLParam(r, param)
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid "+param, http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

// ensure strconv is used (for RemoveServicio)
var _ = strconv.ParseInt
