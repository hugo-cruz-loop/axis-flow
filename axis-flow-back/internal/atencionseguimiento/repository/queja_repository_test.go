// Package repository_test covers the in-memory queja repository behaviour.
package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func newTestInMemQuejaRepo(t *testing.T) *repository.InMemQuejaRepository {
	t.Helper()
	return repository.NewInMemQuejaRepository()
}

func sampleQueja(empresaID uuid.UUID, empleadoID int64, estatus int) *atencionseguimiento.SolicitudQueja {
	return &atencionseguimiento.SolicitudQueja{
		ID:          uuid.New(),
		EmpresaID:   empresaID,
		EmpleadoID:  empleadoID,
		TipoQuejaID: uuid.New(),
		Titulo:      "test queja",
		Descripcion: "descripcion",
		Estatus:     estatus,
		UltimaResp:  atencionseguimiento.RolEmpleadoCliente,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

// ---------------------------------------------------------------------------
// GetQuejasByEmpresa: filter by estatus
// ---------------------------------------------------------------------------

func TestInMemQuejaRepositoryListByEmpresaReturnsAllWhenEstatusNil(t *testing.T) {
	repo := newTestInMemQuejaRepo(t)
	ctx := context.Background()
	empresaID := uuid.New()

	q1 := sampleQueja(empresaID, 1, atencionseguimiento.EstatusPendiente)
	q2 := sampleQueja(empresaID, 2, atencionseguimiento.EstatusEnProceso)
	q3 := sampleQueja(empresaID, 3, atencionseguimiento.EstatusFinalizado)

	require.NoError(t, repo.Create(ctx, q1))
	require.NoError(t, repo.Create(ctx, q2))
	require.NoError(t, repo.Create(ctx, q3))

	results, total, err := repo.ListByEmpresa(ctx, empresaID, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, results, 3)
}

func TestInMemQuejaRepositoryListByEmpresaFiltersWhenEstatusProvided(t *testing.T) {
	repo := newTestInMemQuejaRepo(t)
	ctx := context.Background()
	empresaID := uuid.New()

	q1 := sampleQueja(empresaID, 1, atencionseguimiento.EstatusPendiente)
	q2 := sampleQueja(empresaID, 2, atencionseguimiento.EstatusPendiente)
	q3 := sampleQueja(empresaID, 3, atencionseguimiento.EstatusEnProceso)

	require.NoError(t, repo.Create(ctx, q1))
	require.NoError(t, repo.Create(ctx, q2))
	require.NoError(t, repo.Create(ctx, q3))

	estatus := atencionseguimiento.EstatusPendiente
	results, total, err := repo.ListByEmpresa(ctx, empresaID, &estatus, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, atencionseguimiento.EstatusPendiente, r.Estatus)
	}
}

func TestInMemQuejaRepositoryListByEmpresaExcludesOtherEmpresas(t *testing.T) {
	repo := newTestInMemQuejaRepo(t)
	ctx := context.Background()
	empresaID := uuid.New()
	otherEmpresaID := uuid.New()

	require.NoError(t, repo.Create(ctx, sampleQueja(empresaID, 1, atencionseguimiento.EstatusPendiente)))
	require.NoError(t, repo.Create(ctx, sampleQueja(otherEmpresaID, 2, atencionseguimiento.EstatusPendiente)))

	results, total, err := repo.ListByEmpresa(ctx, empresaID, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, results, 1)
}

// ---------------------------------------------------------------------------
// GetByID / not found
// ---------------------------------------------------------------------------

func TestInMemQuejaRepositoryGetByIDReturnsNotFoundForMissingRecord(t *testing.T) {
	repo := newTestInMemQuejaRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New(), 0)
	require.ErrorIs(t, err, atencionseguimiento.ErrNotFound)
}

func TestInMemQuejaRepositoryGetByIDReturnsForbiddenForWrongEmpleado(t *testing.T) {
	repo := newTestInMemQuejaRepo(t)
	ctx := context.Background()

	queja := sampleQueja(uuid.New(), 42, atencionseguimiento.EstatusPendiente)
	require.NoError(t, repo.Create(ctx, queja))

	// empleadoID 0 means "no check" — must succeed
	result, err := repo.GetByID(ctx, queja.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, queja.ID, result.ID)

	// wrong empleadoID must return ErrForbidden
	_, err = repo.GetByID(ctx, queja.ID, 99)
	require.ErrorIs(t, err, atencionseguimiento.ErrForbidden)
}

// ---------------------------------------------------------------------------
// CreateRespuesta: Redis UNLINK called
// ---------------------------------------------------------------------------

func TestCreateRespuestaQuejaInvalidatesRedisKeys(t *testing.T) {
	mr, client := newTestRedis(t)
	ctx := context.Background()

	// Pre-set keys that should be unlinked
	empleadoID := uuid.New()
	empresaID := uuid.New()
	mr.Set("atencion_seguimiento:empleado:"+empleadoID.String()+":unread_complaints", "5")
	mr.Set("atencion_seguimiento:empresa:"+empresaID.String()+":unread_complaints", "3")

	invalidator := repository.NewRedisAtencionCacheInvalidator(client)

	require.NoError(t, invalidator.UnlinkQuejaRespuestaKeys(ctx, empleadoID, empresaID))

	assert.False(t, mr.Exists("atencion_seguimiento:empleado:"+empleadoID.String()+":unread_complaints"))
	assert.False(t, mr.Exists("atencion_seguimiento:empresa:"+empresaID.String()+":unread_complaints"))
}

// ---------------------------------------------------------------------------
// SuspendQuejasByEmpleado
// ---------------------------------------------------------------------------

func TestInMemQuejaRepositorySuspendQuejasByEmpleadoUpdatesOnlyNonFinalized(t *testing.T) {
	repo := newTestInMemQuejaRepo(t)
	ctx := context.Background()
	empresaID := uuid.New()
	const empleadoID int64 = 10

	q1 := sampleQueja(empresaID, empleadoID, atencionseguimiento.EstatusPendiente)
	q2 := sampleQueja(empresaID, empleadoID, atencionseguimiento.EstatusEnProceso)
	q3 := sampleQueja(empresaID, empleadoID, atencionseguimiento.EstatusFinalizado) // already closed
	otherQ := sampleQueja(empresaID, 99, atencionseguimiento.EstatusPendiente)      // other employee

	require.NoError(t, repo.Create(ctx, q1))
	require.NoError(t, repo.Create(ctx, q2))
	require.NoError(t, repo.Create(ctx, q3))
	require.NoError(t, repo.Create(ctx, otherQ))

	require.NoError(t, repo.SuspendQuejasByEmpleado(ctx, empleadoID))

	got1, err := repo.GetByID(ctx, q1.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, atencionseguimiento.EstatusFinalizado, got1.Estatus)

	got2, err := repo.GetByID(ctx, q2.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, atencionseguimiento.EstatusFinalizado, got2.Estatus)

	got3, err := repo.GetByID(ctx, q3.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, atencionseguimiento.EstatusFinalizado, got3.Estatus) // unchanged

	gotOther, err := repo.GetByID(ctx, otherQ.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, atencionseguimiento.EstatusPendiente, gotOther.Estatus) // not affected
}
