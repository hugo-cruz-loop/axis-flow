package repository_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidenciasRepoListTenantIsolation(t *testing.T) {
	repo := repository.NewInMemEvidenciasRepo()
	ctx := context.Background()

	_ = repo.Seed(reports.Evidencia{
		AsignacionID:   "a1",
		ActividadID:    "act1",
		EmpleadoNombre: "Juan Pérez",
		Evidencias:     []string{"https://s3.example.com/photo1.jpg"},
	}, "empresa-A")

	_ = repo.Seed(reports.Evidencia{
		AsignacionID:   "a2",
		ActividadID:    "act2",
		EmpleadoNombre: "María López",
		Evidencias:     []string{"https://s3.example.com/photo2.jpg"},
	}, "empresa-B")

	items, total, err := repo.List(ctx, "empresa-A", nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, items, 1)
	assert.Equal(t, "a1", items[0].AsignacionID)
}

func TestEvidenciasRepoListFilterByCliente(t *testing.T) {
	repo := repository.NewInMemEvidenciasRepo()
	ctx := context.Background()

	clienteA := "cliente-A"
	clienteB := "cliente-B"

	_ = repo.SeedWithCliente(reports.Evidencia{AsignacionID: "a1"}, "empresa-X", clienteA)
	_ = repo.SeedWithCliente(reports.Evidencia{AsignacionID: "a2"}, "empresa-X", clienteB)

	items, total, err := repo.List(ctx, "empresa-X", &clienteA, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "a1", items[0].AsignacionID)
}

func TestEvidenciasRepoListFilterByFecha(t *testing.T) {
	repo := repository.NewInMemEvidenciasRepo()
	ctx := context.Background()

	fecha := "2024-01-15"
	_ = repo.SeedWithFecha(reports.Evidencia{AsignacionID: "a1"}, "empresa-Y", fecha)
	_ = repo.SeedWithFecha(reports.Evidencia{AsignacionID: "a2"}, "empresa-Y", "2024-02-20")

	items, total, err := repo.List(ctx, "empresa-Y", nil, &fecha, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "a1", items[0].AsignacionID)
}

func TestEvidenciasRepoListPagination(t *testing.T) {
	repo := repository.NewInMemEvidenciasRepo()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = repo.Seed(reports.Evidencia{AsignacionID: string(rune('a' + i))}, "emp")
	}

	items, total, err := repo.List(ctx, "emp", nil, nil, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, items, 2)
}
