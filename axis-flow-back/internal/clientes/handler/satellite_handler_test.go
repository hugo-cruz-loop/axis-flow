package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/clientes"
	"axis-flow-back/internal/clientes/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stub ──────────────────────────────────────────────────────────────────────

type stubSatelliteService struct {
	createFacturaFn    func(ctx context.Context, f *clientes.Factura, empresaID int64, encKey string) error
	getFacturaFn       func(ctx context.Context, clienteID uuid.UUID, empresaID int64, encKey string) (*clientes.Factura, error)
	updateFacturaFn    func(ctx context.Context, f *clientes.Factura, empresaID int64, encKey string) error
	createPresupuestoFn func(ctx context.Context, p *clientes.Presupuesto, empresaID int64) error
	getPresupuestoFn   func(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error)
	updatePresupuestoFn func(ctx context.Context, p *clientes.Presupuesto, empresaID int64) error
	createCalendarioFn func(ctx context.Context, cal *clientes.CalendarioLaboral, empresaID int64) error
	getCalendarioFn    func(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error)
	updateCalendarioFn func(ctx context.Context, cal *clientes.CalendarioLaboral, empresaID int64) error
	// track whether decrypt was called (via encKey presence)
	encKeyUsed string
}

func (s *stubSatelliteService) CreateFactura(ctx context.Context, f *clientes.Factura, empresaID int64, encKey string) error {
	s.encKeyUsed = encKey
	return s.createFacturaFn(ctx, f, empresaID, encKey)
}
func (s *stubSatelliteService) GetFactura(ctx context.Context, clienteID uuid.UUID, empresaID int64, encKey string) (*clientes.Factura, error) {
	s.encKeyUsed = encKey
	return s.getFacturaFn(ctx, clienteID, empresaID, encKey)
}
func (s *stubSatelliteService) UpdateFactura(ctx context.Context, f *clientes.Factura, empresaID int64, encKey string) error {
	s.encKeyUsed = encKey
	return s.updateFacturaFn(ctx, f, empresaID, encKey)
}
func (s *stubSatelliteService) CreatePresupuesto(ctx context.Context, p *clientes.Presupuesto, empresaID int64) error {
	return s.createPresupuestoFn(ctx, p, empresaID)
}
func (s *stubSatelliteService) GetPresupuesto(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error) {
	return s.getPresupuestoFn(ctx, clienteID, empresaID)
}
func (s *stubSatelliteService) UpdatePresupuesto(ctx context.Context, p *clientes.Presupuesto, empresaID int64) error {
	return s.updatePresupuestoFn(ctx, p, empresaID)
}
func (s *stubSatelliteService) CreateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral, empresaID int64) error {
	return s.createCalendarioFn(ctx, cal, empresaID)
}
func (s *stubSatelliteService) GetCalendario(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error) {
	return s.getCalendarioFn(ctx, clienteID, empresaID)
}
func (s *stubSatelliteService) UpdateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral, empresaID int64) error {
	return s.updateCalendarioFn(ctx, cal, empresaID)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCreateFactura_Happy_201(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubSatelliteService{
		createFacturaFn: func(_ context.Context, f *clientes.Factura, _ int64, _ string) error {
			f.ID = uuid.New()
			return nil
		},
	}
	h := handler.NewSatelliteHandler(svc, "test-key")

	body, _ := json.Marshal(map[string]any{
		"rfc":              "ABC123456789",
		"razon_social":     "Test SA",
		"domicilio_fiscal": "Calle 1",
	})
	r := httptest.NewRequest(http.MethodPost, "/cliente/"+clienteID.String()+"/factura?empresa_id=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", clienteID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()
	h.CreateFactura(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
	// Verify encKey was passed to service (decrypt delegation)
	assert.Equal(t, "test-key", svc.encKeyUsed)
}

func TestGetFactura_DecryptCalled(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubSatelliteService{
		getFacturaFn: func(_ context.Context, id uuid.UUID, _ int64, encKey string) (*clientes.Factura, error) {
			// Simulate service decrypting and returning plaintext RFC
			return &clientes.Factura{
				ID:        uuid.New(),
				ClienteID: id,
				RFC:       "DECRYPTED_RFC",
			}, nil
		},
	}
	h := handler.NewSatelliteHandler(svc, "my-enc-key")

	r := httptest.NewRequest(http.MethodGet, "/cliente/"+clienteID.String()+"/factura?empresa_id=1", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", clienteID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()
	h.GetFactura(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	// Verify the enc key was passed to the service (which handles decrypt)
	assert.Equal(t, "my-enc-key", svc.encKeyUsed)
	// Verify decrypted RFC is in response
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "DECRYPTED_RFC", data["RFC"])
}

func TestCreatePresupuesto_Duplicate_409(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubSatelliteService{
		createPresupuestoFn: func(_ context.Context, _ *clientes.Presupuesto, _ int64) error {
			return clientes.ErrPresupuestoExists
		},
	}
	h := handler.NewSatelliteHandler(svc, "key")

	body, _ := json.Marshal(map[string]any{"costo_mensual": 1000.0})
	r := httptest.NewRequest(http.MethodPost, "/cliente/"+clienteID.String()+"/presupuesto?empresa_id=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", clienteID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()
	h.CreatePresupuesto(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}
