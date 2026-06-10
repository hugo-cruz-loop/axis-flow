package repository_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestInMemIncidenciaRepo(t *testing.T) *repository.InMemIncidenciaRepository {
	t.Helper()
	return repository.NewInMemIncidenciaRepository()
}

func sampleIncidencia(empresaID uuid.UUID) *atencionseguimiento.IncidenciaSupervisor {
	return &atencionseguimiento.IncidenciaSupervisor{
		ID:               uuid.New(),
		EmpresaID:        empresaID,
		SupervisorID:     uuid.New(),
		EmpleadoID:       42,
		LocalidadID:      uuid.New(),
		TipoIncidenciaID: uuid.New(),
		Descripcion:      "incidencia de prueba",
	}
}

func TestInMemIncidenciaRepositoryCreateAndList(t *testing.T) {
	repo := newTestInMemIncidenciaRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	i1 := sampleIncidencia(empresaID)
	i2 := sampleIncidencia(empresaID)
	other := sampleIncidencia(uuid.New())

	require.NoError(t, repo.Create(ctx, i1))
	require.NoError(t, repo.Create(ctx, i2))
	require.NoError(t, repo.Create(ctx, other))

	results, total, err := repo.ListByEmpresa(ctx, empresaID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, results, 2)
}

func TestInMemIncidenciaRepositoryListByEmpresaExcludesOthers(t *testing.T) {
	repo := newTestInMemIncidenciaRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	require.NoError(t, repo.Create(ctx, sampleIncidencia(uuid.New())))

	results, total, err := repo.ListByEmpresa(ctx, empresaID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, results)
}

func TestInMemIncidenciaRepositoryPaginates(t *testing.T) {
	repo := newTestInMemIncidenciaRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Create(ctx, sampleIncidencia(empresaID)))
	}

	page1, total, err := repo.ListByEmpresa(ctx, empresaID, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, page1, 3)

	page2, total2, err := repo.ListByEmpresa(ctx, empresaID, 2, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total2)
	assert.Len(t, page2, 2)
}
