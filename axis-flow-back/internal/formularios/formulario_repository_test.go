// Package formularios_test covers the in-memory formulario repository behaviour.
//
// PR-2 (Repositories) — task 2.1. Mirrors the InMem* pattern from
// internal/atencionseguimiento/repository/ so the pgx adapter can be swapped in
// later by the integration test layer without changing service code.
package formularios_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/formularios"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestInMemFormularioRepo(t *testing.T) *formularios.InMemFormularioRepository {
	t.Helper()
	return formularios.NewInMemFormularioRepository()
}

func sampleFormulario(empresaID uuid.UUID) *formularios.Formulario {
	now := time.Now().UTC()
	return &formularios.Formulario{
		ID:          uuid.New(),
		EmpresaID:   empresaID,
		Nombre:      "Control Higienico",
		Descripcion: "Limpieza diaria",
		Activo:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ---------------------------------------------------------------------------
// Create + GetByID happy path
// ---------------------------------------------------------------------------

func TestInMemFormularioRepositoryCreateAndGetByIDRoundTrip(t *testing.T) {
	repo := newTestInMemFormularioRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	f := sampleFormulario(empresaID)
	require.NoError(t, repo.Create(ctx, f))

	got, err := repo.GetByID(ctx, f.ID, empresaID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, f.ID, got.ID)
	assert.Equal(t, f.EmpresaID, got.EmpresaID)
	assert.Equal(t, f.Nombre, got.Nombre)
	assert.Equal(t, f.Descripcion, got.Descripcion)
	assert.Equal(t, f.Activo, got.Activo)
}

// ---------------------------------------------------------------------------
// IDOR: GetByID with the wrong empresa_id must return ErrNotFound (no leak).
// ---------------------------------------------------------------------------

func TestInMemFormularioRepositoryGetByIDRejectsForeignEmpresa(t *testing.T) {
	repo := newTestInMemFormularioRepo(t)
	ctx := context.Background()

	owner := uuid.New()
	intruder := uuid.New()
	f := sampleFormulario(owner)
	require.NoError(t, repo.Create(ctx, f))

	_, err := repo.GetByID(ctx, f.ID, intruder)
	require.ErrorIs(t, err, formularios.ErrNotFound)
}

// ---------------------------------------------------------------------------
// GetByID missing record
// ---------------------------------------------------------------------------

func TestInMemFormularioRepositoryGetByIDReturnsNotFoundForMissingID(t *testing.T) {
	repo := newTestInMemFormularioRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New(), uuid.New())
	require.ErrorIs(t, err, formularios.ErrNotFound)
}

// ---------------------------------------------------------------------------
// ListByEmpresa: happy path, IDOR guard, activo filter
// ---------------------------------------------------------------------------

func TestInMemFormularioRepositoryListByEmpresaReturnsAllWhenActivoNil(t *testing.T) {
	repo := newTestInMemFormularioRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	f1 := sampleFormulario(empresaID)
	f1.Activo = true
	f2 := sampleFormulario(empresaID)
	f2.Activo = false
	other := sampleFormulario(uuid.New())
	other.Activo = true

	require.NoError(t, repo.Create(ctx, f1))
	require.NoError(t, repo.Create(ctx, f2))
	require.NoError(t, repo.Create(ctx, other))

	results, total, err := repo.ListByEmpresa(ctx, empresaID, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, results, 2)
}

func TestInMemFormularioRepositoryListByEmpresaFiltersByActivo(t *testing.T) {
	repo := newTestInMemFormularioRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	f1 := sampleFormulario(empresaID)
	f1.Activo = true
	f2 := sampleFormulario(empresaID)
	f2.Activo = false
	f3 := sampleFormulario(empresaID)
	f3.Activo = true

	require.NoError(t, repo.Create(ctx, f1))
	require.NoError(t, repo.Create(ctx, f2))
	require.NoError(t, repo.Create(ctx, f3))

	activo := true
	results, total, err := repo.ListByEmpresa(ctx, empresaID, &activo, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Activo)
	}
}

func TestInMemFormularioRepositoryListByEmpresaExcludesOtherEmpresas(t *testing.T) {
	repo := newTestInMemFormularioRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	other := uuid.New()
	require.NoError(t, repo.Create(ctx, sampleFormulario(empresaID)))
	require.NoError(t, repo.Create(ctx, sampleFormulario(other)))

	results, total, err := repo.ListByEmpresa(ctx, empresaID, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, results, 1)
	assert.Equal(t, empresaID, results[0].EmpresaID)
}

func TestInMemFormularioRepositoryListByEmpresaEmptyForUnknownEmpresa(t *testing.T) {
	repo := newTestInMemFormularioRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, sampleFormulario(uuid.New())))

	results, total, err := repo.ListByEmpresa(ctx, uuid.New(), nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, results)
}

func TestInMemFormularioRepositoryListByEmpresaPaginates(t *testing.T) {
	repo := newTestInMemFormularioRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Create(ctx, sampleFormulario(empresaID)))
	}

	page1, total, err := repo.ListByEmpresa(ctx, empresaID, nil, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, page1, 3)

	page2, total2, err := repo.ListByEmpresa(ctx, empresaID, nil, 2, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total2)
	assert.Len(t, page2, 2)
}

// ---------------------------------------------------------------------------
// Compile-time port satisfaction: the InMem repo must satisfy the same port
// the pgx adapter satisfies, so services can swap at wiring time.
// ---------------------------------------------------------------------------

var _ formularios.FormularioRepository = (*formularios.InMemFormularioRepository)(nil)
