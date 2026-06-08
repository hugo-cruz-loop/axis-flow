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
)

// ── stub ──────────────────────────────────────────────────────────────────────

type stubLocalidadService struct {
	createFn func(ctx context.Context, l *clientes.Localidad, empresaID int64) (*clientes.Localidad, error)
	getFn    func(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error)
	listFn   func(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error)
	updateFn func(ctx context.Context, l *clientes.Localidad, empresaID int64) (*clientes.Localidad, error)
	deleteFn func(ctx context.Context, id uuid.UUID, empresaID int64) error
}

func (s *stubLocalidadService) CreateLocalidad(ctx context.Context, l *clientes.Localidad, empresaID int64) (*clientes.Localidad, error) {
	return s.createFn(ctx, l, empresaID)
}
func (s *stubLocalidadService) GetLocalidad(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error) {
	return s.getFn(ctx, id, empresaID)
}
func (s *stubLocalidadService) ListLocalidades(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error) {
	return s.listFn(ctx, clienteID, empresaID)
}
func (s *stubLocalidadService) UpdateLocalidad(ctx context.Context, l *clientes.Localidad, empresaID int64) (*clientes.Localidad, error) {
	return s.updateFn(ctx, l, empresaID)
}
func (s *stubLocalidadService) DeleteLocalidad(ctx context.Context, id uuid.UUID, empresaID int64) error {
	return s.deleteFn(ctx, id, empresaID)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCreateLocalidad_Happy_201(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubLocalidadService{
		createFn: func(_ context.Context, l *clientes.Localidad, _ int64) (*clientes.Localidad, error) {
			l.ID = uuid.New()
			return l, nil
		},
	}
	h := handler.NewLocalidadHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"nombre":            "Sucursal Norte",
		"direccion":         "Av. Principal 100",
		"tipo_localidad_id": 1,
	})
	r := httptest.NewRequest(http.MethodPost, "/cliente/"+clienteID.String()+"/localidades?empresa_id=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", clienteID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()
	h.CreateLocalidad(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGetLocalidad_NotFound_404(t *testing.T) {
	localidadID := uuid.New()
	svc := &stubLocalidadService{
		getFn: func(_ context.Context, _ uuid.UUID, _ int64) (*clientes.Localidad, error) {
			return nil, clientes.ErrLocalidadNotFound
		},
	}
	h := handler.NewLocalidadHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/localidad/"+localidadID.String()+"?empresa_id=1", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("localidad_id", localidadID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()
	h.GetLocalidad(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteLocalidad_Happy_204(t *testing.T) {
	localidadID := uuid.New()
	svc := &stubLocalidadService{
		deleteFn: func(_ context.Context, _ uuid.UUID, _ int64) error {
			return nil
		},
	}
	h := handler.NewLocalidadHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/localidad/"+localidadID.String()+"?empresa_id=1", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("localidad_id", localidadID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()
	h.DeleteLocalidad(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}
