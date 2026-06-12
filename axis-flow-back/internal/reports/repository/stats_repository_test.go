package repository_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsRepoGraficaEvidenciaReturnsWeeklyStats(t *testing.T) {
	repo := repository.NewInMemStatsRepo()
	ctx := context.Background()

	repo.SeedGraficaEvidencia("empresa-A", []reports.GraficaEvidenciaStat{
		{Week: "2024-W01", Total: 10, Compliant: 8},
		{Week: "2024-W02", Total: 12, Compliant: 11},
	})

	stats, err := repo.GraficaEvidencia(ctx, "empresa-A", nil)
	require.NoError(t, err)
	assert.Len(t, stats, 2)
	assert.Equal(t, "2024-W01", stats[0].Week)
	assert.Equal(t, 8, stats[0].Compliant)
}

func TestStatsRepoGraficaEvidenciaFilterByCliente(t *testing.T) {
	repo := repository.NewInMemStatsRepo()
	ctx := context.Background()

	clienteA := "cliente-A"
	repo.SeedGraficaEvidenciaWithCliente("empresa-A", clienteA, []reports.GraficaEvidenciaStat{
		{Week: "2024-W01", Total: 5, Compliant: 4},
	})
	repo.SeedGraficaEvidencia("empresa-A", []reports.GraficaEvidenciaStat{
		{Week: "2024-W01", Total: 20, Compliant: 15},
	})

	stats, err := repo.GraficaEvidencia(ctx, "empresa-A", &clienteA)
	require.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Equal(t, 5, stats[0].Total)
}

func TestStatsRepoCountIncidentesReturnsGroupedCounts(t *testing.T) {
	repo := repository.NewInMemStatsRepo()
	ctx := context.Background()

	repo.SeedIncidenteCounts("empresa-A", []reports.IncidenteCount{
		{Status: "abierto", Count: 3},
		{Status: "cerrado", Count: 7},
	})

	counts, err := repo.CountIncidentes(ctx, "empresa-A")
	require.NoError(t, err)
	assert.Len(t, counts, 2)

	statusMap := make(map[string]int)
	for _, c := range counts {
		statusMap[c.Status] = c.Count
	}
	assert.Equal(t, 3, statusMap["abierto"])
	assert.Equal(t, 7, statusMap["cerrado"])
}

func TestStatsRepoEmptyEmpresaReturnsEmptySlice(t *testing.T) {
	repo := repository.NewInMemStatsRepo()
	ctx := context.Background()

	stats, err := repo.GraficaEvidencia(ctx, "unknown-empresa", nil)
	require.NoError(t, err)
	assert.Empty(t, stats)

	counts, err := repo.CountIncidentes(ctx, "unknown-empresa")
	require.NoError(t, err)
	assert.Empty(t, counts)
}
