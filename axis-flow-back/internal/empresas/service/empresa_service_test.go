package service_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/empresas"
	"axis-flow-back/internal/empresas/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type fullEmpresaRepo struct {
	empresa     *empresas.Empresa
	findErr     error
	deleteErr   error
	deleteCalls int
	dbCalls     int
}

func (r *fullEmpresaRepo) Create(ctx context.Context, e *empresas.Empresa) error { return nil }
func (r *fullEmpresaRepo) UpdateStatus(ctx context.Context, id int64, status empresas.EmpresaStatus, vigencia *time.Time) error {
	return nil
}
func (r *fullEmpresaRepo) FindByID(ctx context.Context, id int64) (*empresas.Empresa, error) {
	r.dbCalls++
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.empresa != nil {
		return r.empresa, nil
	}
	return &empresas.Empresa{ID: id, Nombre: "Test SA", Status: empresas.EmpresaStatusActive}, nil
}
func (r *fullEmpresaRepo) Update(ctx context.Context, e *empresas.Empresa) error { return nil }
func (r *fullEmpresaRepo) Delete(ctx context.Context, id int64) error {
	r.deleteCalls++
	return r.deleteErr
}

// ── tests ─────────────────────────────────────────────────────────────────────

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestEmpresaService_GetByID_OnCacheMiss_QueriesDB(t *testing.T) {
	repo := &fullEmpresaRepo{}
	rdb := newTestRedis(t)
	svc := service.NewEmpresaService(repo, rdb)

	e, err := svc.GetByID(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, int64(1), e.ID)
	assert.Equal(t, 1, repo.dbCalls, "should hit DB on cache miss")
}

func TestEmpresaService_GetByID_OnCacheHit_SkipsDB(t *testing.T) {
	repo := &fullEmpresaRepo{}
	rdb := newTestRedis(t)
	svc := service.NewEmpresaService(repo, rdb)

	// First call populates cache
	_, err := svc.GetByID(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.dbCalls)

	// Second call should hit cache
	_, err = svc.GetByID(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.dbCalls, "should NOT hit DB on cache hit")
}

func TestEmpresaService_Delete_CascadesViaRepo(t *testing.T) {
	repo := &fullEmpresaRepo{}
	rdb := newTestRedis(t)
	svc := service.NewEmpresaService(repo, rdb)

	err := svc.Delete(context.Background(), 5)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.deleteCalls)
}
