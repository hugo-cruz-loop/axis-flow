package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/clientes"
	"axis-flow-back/internal/clientes/cache"
	"axis-flow-back/internal/clientes/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ---- Mock ClienteRepository -------------------------------------------------

type mockClienteRepo struct {
	createFn       func(ctx context.Context, c *clientes.Cliente) error
	findByIDFn     func(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	findByUserFn   func(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	listFn         func(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error)
	updateFn       func(ctx context.Context, c *clientes.Cliente) error
	deleteFn       func(ctx context.Context, id uuid.UUID, empresaID int64) error
	patchEstatusFn func(ctx context.Context, id uuid.UUID, empresaID int64, estatus int) (int, *clientes.QualityGateStatus, error)
}

func (m *mockClienteRepo) Create(ctx context.Context, c *clientes.Cliente) error {
	return m.createFn(ctx, c)
}
func (m *mockClienteRepo) FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	return m.findByIDFn(ctx, id, empresaID)
}
func (m *mockClienteRepo) FindByUserID(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	return m.findByUserFn(ctx, userID, empresaID)
}
func (m *mockClienteRepo) ListByEmpresa(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error) {
	return m.listFn(ctx, empresaID, page, size)
}
func (m *mockClienteRepo) Update(ctx context.Context, c *clientes.Cliente) error {
	return m.updateFn(ctx, c)
}
func (m *mockClienteRepo) Delete(ctx context.Context, id uuid.UUID, empresaID int64) error {
	return m.deleteFn(ctx, id, empresaID)
}
func (m *mockClienteRepo) CheckQualityGate(ctx context.Context, clienteID uuid.UUID) (*clientes.QualityGateStatus, error) {
	return nil, nil
}
func (m *mockClienteRepo) PatchEstatus(ctx context.Context, id uuid.UUID, empresaID int64, estatus int) (int, *clientes.QualityGateStatus, error) {
	return m.patchEstatusFn(ctx, id, empresaID, estatus)
}

// ---- Helpers ----------------------------------------------------------------

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

// ---- Tests ------------------------------------------------------------------

func TestClienteService_CreateCliente_HappyPath(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()
	empresaID := int64(1)

	repo := &mockClienteRepo{
		createFn: func(_ context.Context, c *clientes.Cliente) error {
			c.ID = uuid.New()
			c.CreatedAt = time.Now()
			c.UpdatedAt = time.Now()
			return nil
		},
	}

	svc := service.NewClienteService(repo, rdb)
	req := service.CreateClienteRequest{
		EmpresaID:       empresaID,
		RepresentanteID: uuid.New(),
		NombreComercial: "Acme",
		RazonSocial:     "Acme SA",
	}

	got, err := svc.CreateCliente(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == uuid.Nil {
		t.Fatal("expected non-nil ID after create")
	}
	if got.NombreComercial != "Acme" {
		t.Errorf("NombreComercial = %q, want %q", got.NombreComercial, "Acme")
	}
}

func TestClienteService_GetCliente_CacheHit(t *testing.T) {
	mr, rdb := newTestRedis(t)
	ctx := context.Background()
	empresaID := int64(1)
	id := uuid.New()

	// Pre-populate cache
	c := clientes.Cliente{
		ID:              id,
		EmpresaID:       empresaID,
		NombreComercial: "Cached Co",
		Estatus:         clientes.ClienteStatusActivo,
	}
	data, _ := json.Marshal(c)
	mr.Set(cache.ClienteCacheKey(empresaID, id), string(data))

	repoCalled := false
	repo := &mockClienteRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID, _ int64) (*clientes.Cliente, error) {
			repoCalled = true
			return nil, errors.New("should not be called")
		},
	}

	svc := service.NewClienteService(repo, rdb)
	got, err := svc.GetCliente(ctx, id, empresaID)
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

func TestClienteService_GetCliente_CacheMiss_CallsRepo(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()
	empresaID := int64(1)
	id := uuid.New()

	expected := &clientes.Cliente{
		ID:              id,
		EmpresaID:       empresaID,
		NombreComercial: "DB Co",
		Estatus:         clientes.ClienteStatusIncompleto,
	}

	repoCalled := false
	repo := &mockClienteRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID, _ int64) (*clientes.Cliente, error) {
			repoCalled = true
			return expected, nil
		},
	}

	svc := service.NewClienteService(repo, rdb)
	got, err := svc.GetCliente(ctx, id, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repoCalled {
		t.Fatal("repo should be called on cache miss")
	}
	if got.NombreComercial != "DB Co" {
		t.Errorf("NombreComercial = %q, want %q", got.NombreComercial, "DB Co")
	}

	// Cache should now be populated
	key := cache.ClienteCacheKey(empresaID, id)
	val, err := rdb.Get(ctx, key).Result()
	if err != nil || val == "" {
		t.Error("expected cache to be populated after miss")
	}
}

func TestClienteService_PatchEstatus_Blocked_Returns206(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()
	empresaID := int64(1)
	id := uuid.New()

	gate := &clientes.QualityGateStatus{
		ClienteID:     id,
		FacturaOK:     false,
		PresupuestoOK: false,
		CalendarioOK:  false,
		CanActivate:   false,
	}

	repo := &mockClienteRepo{
		patchEstatusFn: func(_ context.Context, _ uuid.UUID, _ int64, _ int) (int, *clientes.QualityGateStatus, error) {
			return 206, gate, nil
		},
	}

	svc := service.NewClienteService(repo, rdb)
	httpStatus, c, gotGate, err := svc.PatchEstatus(ctx, id, empresaID, clientes.ClienteStatusActivo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpStatus != 206 {
		t.Errorf("httpStatus = %d, want 206", httpStatus)
	}
	if c != nil {
		t.Error("expected nil cliente on 206")
	}
	if gotGate == nil {
		t.Fatal("expected gate status on 206")
	}
	if gotGate.CanActivate {
		t.Error("expected CanActivate=false on blocked 206")
	}
}

func TestClienteService_PatchEstatus_Success_InvalidatesCache(t *testing.T) {
	mr, rdb := newTestRedis(t)
	ctx := context.Background()
	empresaID := int64(1)
	id := uuid.New()

	// Pre-populate cache to verify invalidation
	key := cache.ClienteCacheKey(empresaID, id)
	mr.Set(key, `{"id":"`+id.String()+`"}`)

	repo := &mockClienteRepo{
		patchEstatusFn: func(_ context.Context, _ uuid.UUID, _ int64, _ int) (int, *clientes.QualityGateStatus, error) {
			return 200, nil, nil
		},
		findByIDFn: func(_ context.Context, _ uuid.UUID, _ int64) (*clientes.Cliente, error) {
			return &clientes.Cliente{ID: id, EmpresaID: empresaID, Estatus: clientes.ClienteStatusActivo}, nil
		},
	}

	svc := service.NewClienteService(repo, rdb)
	httpStatus, got, gotGate, err := svc.PatchEstatus(ctx, id, empresaID, clientes.ClienteStatusActivo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpStatus != 200 {
		t.Errorf("httpStatus = %d, want 200", httpStatus)
	}
	if got == nil {
		t.Fatal("expected cliente returned on 200")
	}
	if gotGate != nil {
		t.Error("expected nil gate on 200")
	}

	// Cache should be invalidated
	if mr.Exists(key) {
		t.Error("expected cache key to be invalidated after 200")
	}
}
