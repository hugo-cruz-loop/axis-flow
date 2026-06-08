package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"axis-flow-back/internal/clientes"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// SatelliteServicer is the interface SatelliteHandler depends on.
type SatelliteServicer interface {
	CreateFactura(ctx context.Context, f *clientes.Factura, empresaID int64, encKey string) error
	GetFactura(ctx context.Context, clienteID uuid.UUID, empresaID int64, encKey string) (*clientes.Factura, error)
	UpdateFactura(ctx context.Context, f *clientes.Factura, empresaID int64, encKey string) error

	CreatePresupuesto(ctx context.Context, p *clientes.Presupuesto, empresaID int64) error
	GetPresupuesto(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error)
	UpdatePresupuesto(ctx context.Context, p *clientes.Presupuesto, empresaID int64) error

	CreateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral, empresaID int64) error
	GetCalendario(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error)
	UpdateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral, empresaID int64) error
}

// SatelliteHandler handles satellite sub-resources (factura, presupuesto, calendario).
type SatelliteHandler struct {
	svc    SatelliteServicer
	encKey string // NEVER logged
}

// NewSatelliteHandler constructs a SatelliteHandler.
func NewSatelliteHandler(svc SatelliteServicer, encKey string) *SatelliteHandler {
	return &SatelliteHandler{svc: svc, encKey: encKey}
}

// ── Factura ───────────────────────────────────────────────────────────────────

// CreateFactura handles POST /cliente/{id}/factura
func (h *SatelliteHandler) CreateFactura(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		RFC             string `json:"rfc"`
		RazonSocial     string `json:"razon_social"`
		DomicilioFiscal string `json:"domicilio_fiscal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	f := &clientes.Factura{
		ClienteID:       clienteID,
		RFC:             body.RFC,
		RazonSocial:     body.RazonSocial,
		DomicilioFiscal: body.DomicilioFiscal,
	}
	if err := h.svc.CreateFactura(r.Context(), f, empresaID, h.encKey); err != nil {
		writeClienteError(w, err)
		return
	}
	// RFC in f is plaintext (service restores it); safe to return
	writeJSON(w, http.StatusCreated, map[string]any{"data": f})
}

// GetFactura handles GET /cliente/{id}/factura
func (h *SatelliteHandler) GetFactura(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	f, err := h.svc.GetFactura(r.Context(), clienteID, empresaID, h.encKey)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	// RFC is decrypted by service — never log it
	writeJSON(w, http.StatusOK, map[string]any{"data": f})
}

// UpdateFactura handles PUT /cliente/{id}/factura
func (h *SatelliteHandler) UpdateFactura(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		RFC             string `json:"rfc"`
		RazonSocial     string `json:"razon_social"`
		DomicilioFiscal string `json:"domicilio_fiscal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	f := &clientes.Factura{
		ClienteID:       clienteID,
		RFC:             body.RFC,
		RazonSocial:     body.RazonSocial,
		DomicilioFiscal: body.DomicilioFiscal,
	}
	if err := h.svc.UpdateFactura(r.Context(), f, empresaID, h.encKey); err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": f})
}

// ── Presupuesto ───────────────────────────────────────────────────────────────

// CreatePresupuesto handles POST /cliente/{id}/presupuesto
func (h *SatelliteHandler) CreatePresupuesto(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		PersonalRequerido *int     `json:"personal_requerido"`
		MaterialEstimado  *string  `json:"material_estimado"`
		CostoMensual      *float64 `json:"costo_mensual"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	p := &clientes.Presupuesto{
		ClienteID:         clienteID,
		PersonalRequerido: body.PersonalRequerido,
		MaterialEstimado:  body.MaterialEstimado,
		CostoMensual:      body.CostoMensual,
	}
	if err := h.svc.CreatePresupuesto(r.Context(), p, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": p})
}

// GetPresupuesto handles GET /cliente/{id}/presupuesto
func (h *SatelliteHandler) GetPresupuesto(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	p, err := h.svc.GetPresupuesto(r.Context(), clienteID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

// UpdatePresupuesto handles PUT /cliente/{id}/presupuesto
func (h *SatelliteHandler) UpdatePresupuesto(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		PersonalRequerido *int     `json:"personal_requerido"`
		MaterialEstimado  *string  `json:"material_estimado"`
		CostoMensual      *float64 `json:"costo_mensual"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	p := &clientes.Presupuesto{
		ClienteID:         clienteID,
		PersonalRequerido: body.PersonalRequerido,
		MaterialEstimado:  body.MaterialEstimado,
		CostoMensual:      body.CostoMensual,
	}
	if err := h.svc.UpdatePresupuesto(r.Context(), p, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

// ── CalendarioLaboral ─────────────────────────────────────────────────────────

// CreateCalendario handles POST /cliente/{id}/calendario
func (h *SatelliteHandler) CreateCalendario(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		SemanaLaboral map[string]any `json:"semana_laboral"`
		DiasInhabiles map[string]any `json:"dias_inhabiles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	cal := &clientes.CalendarioLaboral{
		ClienteID:     clienteID,
		SemanaLaboral: body.SemanaLaboral,
		DiasInhabiles: body.DiasInhabiles,
	}
	if err := h.svc.CreateCalendario(r.Context(), cal, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": cal})
}

// GetCalendario handles GET /cliente/{id}/calendario
func (h *SatelliteHandler) GetCalendario(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	cal, err := h.svc.GetCalendario(r.Context(), clienteID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": cal})
}

// UpdateCalendario handles PUT /cliente/{id}/calendario
func (h *SatelliteHandler) UpdateCalendario(w http.ResponseWriter, r *http.Request) {
	clienteID, empresaID, ok := parseSatelliteParams(w, r)
	if !ok {
		return
	}

	var body struct {
		SemanaLaboral map[string]any `json:"semana_laboral"`
		DiasInhabiles map[string]any `json:"dias_inhabiles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	cal := &clientes.CalendarioLaboral{
		ClienteID:     clienteID,
		SemanaLaboral: body.SemanaLaboral,
		DiasInhabiles: body.DiasInhabiles,
	}
	if err := h.svc.UpdateCalendario(r.Context(), cal, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": cal})
}

// ── local helper ──────────────────────────────────────────────────────────────

func parseSatelliteParams(w http.ResponseWriter, r *http.Request) (uuid.UUID, int64, bool) {
	idStr := chi.URLParam(r, "id")
	clienteID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid id", http.StatusBadRequest)
		return uuid.Nil, 0, false
	}
	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return uuid.Nil, 0, false
	}
	return clienteID, empresaID, true
}
