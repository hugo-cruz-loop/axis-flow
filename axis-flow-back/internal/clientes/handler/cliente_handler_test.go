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
	"axis-flow-back/internal/clientes/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stub ──────────────────────────────────────────────────────────────────────

type stubClienteService struct {
	createFn       func(ctx context.Context, req service.CreateClienteRequest) (*clientes.Cliente, error)
	getFn          func(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	listFn         func(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error)
	updateFn       func(ctx context.Context, id uuid.UUID, empresaID int64, req service.UpdateClienteRequest) (*clientes.Cliente, error)
	deleteFn       func(ctx context.Context, id uuid.UUID, empresaID int64) error
	patchEstatusFn func(ctx context.Context, id uuid.UUID, empresaID int64, estatus int) (int, *clientes.Cliente, *clientes.QualityGateStatus, error)
	getByUserFn    func(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	getStatsFn     func(ctx context.Context, empresaID int64) (int, int, error)
}

func (s *stubClienteService) CreateCliente(ctx context.Context, req service.CreateClienteRequest) (*clientes.Cliente, error) {
	return s.createFn(ctx, req)
}
func (s *stubClienteService) GetCliente(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	return s.getFn(ctx, id, empresaID)
}
func (s *stubClienteService) ListClientes(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error) {
	return s.listFn(ctx, empresaID, page, size)
}
func (s *stubClienteService) UpdateCliente(ctx context.Context, id uuid.UUID, empresaID int64, req service.UpdateClienteRequest) (*clientes.Cliente, error) {
	return s.updateFn(ctx, id, empresaID, req)
}
func (s *stubClienteService) DeleteCliente(ctx context.Context, id uuid.UUID, empresaID int64) error {
	return s.deleteFn(ctx, id, empresaID)
}
func (s *stubClienteService) PatchEstatus(ctx context.Context, id uuid.UUID, empresaID int64, estatus int) (int, *clientes.Cliente, *clientes.QualityGateStatus, error) {
	return s.patchEstatusFn(ctx, id, empresaID, estatus)
}
func (s *stubClienteService) GetClienteByUser(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	return s.getByUserFn(ctx, userID, empresaID)
}
func (s *stubClienteService) GetStats(ctx context.Context, empresaID int64) (int, int, error) {
	return s.getStatsFn(ctx, empresaID)
}

// ── helper ────────────────────────────────────────────────────────────────────

func newChiCtxWithParam(key, val string) context.Context {
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add(key, val)
	return context.WithValue(context.Background(), chi.RouteCtxKey, chiCtx)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCreateCliente_Happy_201(t *testing.T) {
	svc := &stubClienteService{
		createFn: func(_ context.Context, req service.CreateClienteRequest) (*clientes.Cliente, error) {
			return &clientes.Cliente{
				ID:              uuid.New(),
				EmpresaID:       req.EmpresaID,
				NombreComercial: req.NombreComercial,
			}, nil
		},
	}
	h := handler.NewClienteHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"empresa_id":       int64(1),
		"representante_id": uuid.New().String(),
		"nombre_comercial": "Acme",
		"razon_social":     "Acme SA",
	})
	r := httptest.NewRequest(http.MethodPost, "/cliente", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateCliente(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestPatchEstatus_QualityGate_206(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubClienteService{
		patchEstatusFn: func(_ context.Context, id uuid.UUID, _ int64, _ int) (int, *clientes.Cliente, *clientes.QualityGateStatus, error) {
			return 206, nil, &clientes.QualityGateStatus{
				ClienteID:     id,
				FacturaOK:     false,
				PresupuestoOK: false,
				CalendarioOK:  true,
				CanActivate:   false,
			}, nil
		},
	}
	h := handler.NewClienteHandler(svc)

	body, _ := json.Marshal(map[string]any{"estatus": 1})
	r := httptest.NewRequest(http.MethodPatch, "/cliente/"+clienteID.String()+"/estatus?empresa_id=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	ctx := newChiCtxWithParam("id", clienteID.String())
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.PatchEstatus(w, r)

	require.Equal(t, http.StatusPartialContent, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	warnings, ok := resp["warnings"].(map[string]any)
	require.True(t, ok)
	missing, ok := warnings["missing_sections"].([]any)
	require.True(t, ok)
	assert.Contains(t, missing, "factura")
	assert.Contains(t, missing, "presupuesto")
}

func TestPatchEstatus_Success_200(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubClienteService{
		patchEstatusFn: func(_ context.Context, id uuid.UUID, _ int64, _ int) (int, *clientes.Cliente, *clientes.QualityGateStatus, error) {
			return 200, &clientes.Cliente{ID: id, Estatus: 1}, nil, nil
		},
	}
	h := handler.NewClienteHandler(svc)

	body, _ := json.Marshal(map[string]any{"estatus": 1})
	r := httptest.NewRequest(http.MethodPatch, "/cliente/"+clienteID.String()+"/estatus?empresa_id=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	ctx := newChiCtxWithParam("id", clienteID.String())
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.PatchEstatus(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetCliente_NotFound_404(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubClienteService{
		getFn: func(_ context.Context, _ uuid.UUID, _ int64) (*clientes.Cliente, error) {
			return nil, clientes.ErrClienteNotFound
		},
	}
	h := handler.NewClienteHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/cliente/"+clienteID.String()+"?empresa_id=1", nil)
	ctx := newChiCtxWithParam("id", clienteID.String())
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetCliente(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteCliente_Happy_204(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubClienteService{
		deleteFn: func(_ context.Context, _ uuid.UUID, _ int64) error {
			return nil
		},
	}
	h := handler.NewClienteHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/cliente/"+clienteID.String()+"?empresa_id=1", nil)
	ctx := newChiCtxWithParam("id", clienteID.String())
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.DeleteCliente(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}
