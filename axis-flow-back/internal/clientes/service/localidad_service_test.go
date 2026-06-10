package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"axis-flow-back/internal/clientes"
	clicache "axis-flow-back/internal/clientes/cache"
	"axis-flow-back/internal/clientes/service"

	"github.com/google/uuid"
)

// ---- Mock LocalidadRepository -----------------------------------------------

type mockLocalidadRepo struct {
	createFn   func(ctx context.Context, l *clientes.Localidad) error
	findByIDFn func(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error)
	listFn     func(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error)
	updateFn   func(ctx context.Context, l *clientes.Localidad) error
	deleteFn   func(ctx context.Context, id uuid.UUID, empresaID int64) error
}

func (m *mockLocalidadRepo) Create(ctx context.Context, l *clientes.Localidad) error {
	return m.createFn(ctx, l)
}
func (m *mockLocalidadRepo) FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error) {
	return m.findByIDFn(ctx, id, empresaID)
}
func (m *mockLocalidadRepo) ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error) {
	return m.listFn(ctx, clienteID, empresaID)
}
func (m *mockLocalidadRepo) Update(ctx context.Context, l *clientes.Localidad) error {
	return m.updateFn(ctx, l)
}
func (m *mockLocalidadRepo) Delete(ctx context.Context, id uuid.UUID, empresaID int64) error {
	return m.deleteFn(ctx, id, empresaID)
}

// ---- Mock SiteConfigRepository ----------------------------------------------

type mockSiteConfigRepo struct{}

func (m *mockSiteConfigRepo) AddServicio(_ context.Context, _ uuid.UUID, _ int64) error {
	return nil
}
func (m *mockSiteConfigRepo) RemoveServicio(_ context.Context, _ uuid.UUID, _ int64) error {
	return nil
}
func (m *mockSiteConfigRepo) ListServicios(_ context.Context, _ uuid.UUID, _ int64) ([]clientes.ServiciosLocalidad, error) {
	return nil, nil
}
func (m *mockSiteConfigRepo) CreateHorario(_ context.Context, h *clientes.Horario) error {
	return nil
}
func (m *mockSiteConfigRepo) ListHorarios(_ context.Context, _ uuid.UUID, _ int64) ([]clientes.Horario, error) {
	return nil, nil
}
func (m *mockSiteConfigRepo) UpdateHorario(_ context.Context, _ *clientes.Horario) error {
	return nil
}
func (m *mockSiteConfigRepo) DeleteHorario(_ context.Context, _ uuid.UUID, _ int64) error {
	return nil
}
func (m *mockSiteConfigRepo) CreateHerramienta(_ context.Context, h *clientes.Herramienta) error {
	return nil
}
func (m *mockSiteConfigRepo) ListHerramientas(_ context.Context, _ uuid.UUID, _ int64) ([]clientes.Herramienta, error) {
	return nil, nil
}
func (m *mockSiteConfigRepo) UpdateHerramienta(_ context.Context, _ *clientes.Herramienta) error {
	return nil
}
func (m *mockSiteConfigRepo) DeleteHerramienta(_ context.Context, _ uuid.UUID, _ int64) error {
	return nil
}
func (m *mockSiteConfigRepo) CreateActividad(_ context.Context, a *clientes.Actividad) error {
	return nil
}
func (m *mockSiteConfigRepo) ListActividades(_ context.Context, _ uuid.UUID, _ int64) ([]clientes.Actividad, error) {
	return nil, nil
}
func (m *mockSiteConfigRepo) UpdateActividad(_ context.Context, _ *clientes.Actividad) error {
	return nil
}
func (m *mockSiteConfigRepo) DeleteActividad(_ context.Context, _ uuid.UUID, _ int64) error {
	return nil
}

// ---- Tests ------------------------------------------------------------------

func TestLocalidadService_GetLocalidad_CacheHit(t *testing.T) {
	mr, rdb := newTestRedis(t)
	ctx := context.Background()
	id := uuid.New()
	empresaID := int64(1)

	loc := clientes.Localidad{ID: id, ClienteID: uuid.New(), Nombre: "Cached Site"}
	data, _ := json.Marshal(loc)
	mr.Set(clicache.LocalidadCacheKey(id), string(data))

	repoCalled := false
	repo := &mockLocalidadRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID, _ int64) (*clientes.Localidad, error) {
			repoCalled = true
			return nil, errors.New("should not be called")
		},
	}

	svc := service.NewLocalidadService(repo, &mockSiteConfigRepo{}, rdb)
	got, err := svc.GetLocalidad(ctx, id, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repoCalled {
		t.Fatal("repo should not be called on cache hit")
	}
	if got.ID != id {
		t.Errorf("ID = %v, want %v", got.ID, id)
	}
}

func TestLocalidadService_CreateLocalidad_InvalidatesCache(t *testing.T) {
	mr, rdb := newTestRedis(t)
	ctx := context.Background()
	id := uuid.New()
	empresaID := int64(1)

	// Pre-populate cache
	key := clicache.LocalidadCacheKey(id)
	mr.Set(key, `{"id":"`+id.String()+`"}`)

	repo := &mockLocalidadRepo{
		createFn: func(_ context.Context, l *clientes.Localidad) error {
			l.ID = id
			return nil
		},
	}

	svc := service.NewLocalidadService(repo, &mockSiteConfigRepo{}, rdb)
	l := &clientes.Localidad{ClienteID: uuid.New(), Nombre: "New Site", TipoLocalidadID: 1}
	got, err := svc.CreateLocalidad(ctx, l, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil localidad")
	}

	// Cache should be invalidated
	if mr.Exists(key) {
		t.Error("expected cache key to be invalidated after create")
	}
}
