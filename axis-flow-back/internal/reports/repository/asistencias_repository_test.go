package repository_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsistenciasRepoListTenantIsolation(t *testing.T) {
	repo := repository.NewInMemAsistenciasRepo()
	ctx := context.Background()

	_ = repo.Seed(reports.AsistenciaRecord{ID: "r1", Status: "on_time"}, "empresa-A")
	_ = repo.Seed(reports.AsistenciaRecord{ID: "r2", Status: "late"}, "empresa-B")

	items, total, err := repo.List(ctx, "empresa-A", nil, nil, nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "r1", items[0].ID)
}

func TestAsistenciasRepoListFilterByEmployee(t *testing.T) {
	repo := repository.NewInMemAsistenciasRepo()
	ctx := context.Background()

	empID := "emp-1"
	_ = repo.SeedWithEmployee(reports.AsistenciaRecord{ID: "r1"}, "empresa-A", empID)
	_ = repo.SeedWithEmployee(reports.AsistenciaRecord{ID: "r2"}, "empresa-A", "emp-2")

	items, total, err := repo.List(ctx, "empresa-A", &empID, nil, nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "r1", items[0].ID)
}

func TestAsistenciasRepoListFilterByStatus(t *testing.T) {
	repo := repository.NewInMemAsistenciasRepo()
	ctx := context.Background()

	status := "late"
	_ = repo.Seed(reports.AsistenciaRecord{ID: "r1", Status: "on_time"}, "empresa-A")
	_ = repo.Seed(reports.AsistenciaRecord{ID: "r2", Status: "late"}, "empresa-A")

	items, total, err := repo.List(ctx, "empresa-A", nil, &status, nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "r2", items[0].ID)
}

func TestAsistenciasRepoListDateRangeFilter(t *testing.T) {
	repo := repository.NewInMemAsistenciasRepo()
	ctx := context.Background()

	from := "2024-01-01"
	to := "2024-01-31"

	_ = repo.SeedWithDate(reports.AsistenciaRecord{ID: "r1"}, "empresa-A", "2024-01-15")
	_ = repo.SeedWithDate(reports.AsistenciaRecord{ID: "r2"}, "empresa-A", "2024-02-15")

	items, total, err := repo.List(ctx, "empresa-A", nil, nil, &from, &to, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "r1", items[0].ID)
}

func TestAsistenciasRepoListPagination(t *testing.T) {
	repo := repository.NewInMemAsistenciasRepo()
	ctx := context.Background()

	for i := 0; i < 6; i++ {
		_ = repo.Seed(reports.AsistenciaRecord{ID: string(rune('a' + i))}, "empresa-A")
	}

	items, total, err := repo.List(ctx, "empresa-A", nil, nil, nil, nil, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 6, total)
	assert.Len(t, items, 3)
}
