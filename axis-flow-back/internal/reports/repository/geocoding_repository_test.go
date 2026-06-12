// Package repository_test covers the reports repository layer with in-memory stubs.
package repository_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// GeocodingRepo tests (in-memory stub)
// ---------------------------------------------------------------------------

func TestGeocodingRepoGetByCoordsHit(t *testing.T) {
	repo := repository.NewInMemGeocodingRepo()
	ctx := context.Background()

	d := reports.DireccionCache{
		Latitud:   19.432608,
		Longitud:  -99.133209,
		Direccion: "Zócalo, CDMX",
	}
	require.NoError(t, repo.Store(ctx, d))

	got, err := repo.GetByCoords(ctx, 19.432608, -99.133209)
	require.NoError(t, err)
	assert.Equal(t, "Zócalo, CDMX", got.Direccion)
}

func TestGeocodingRepoGetByCoordsMiss(t *testing.T) {
	repo := repository.NewInMemGeocodingRepo()
	ctx := context.Background()

	_, err := repo.GetByCoords(ctx, 0.0, 0.0)
	assert.ErrorIs(t, err, reports.ErrNotFound)
}

func TestGeocodingRepoStoreUpserts(t *testing.T) {
	repo := repository.NewInMemGeocodingRepo()
	ctx := context.Background()

	first := reports.DireccionCache{Latitud: 1.0, Longitud: 2.0, Direccion: "first"}
	require.NoError(t, repo.Store(ctx, first))

	updated := reports.DireccionCache{Latitud: 1.0, Longitud: 2.0, Direccion: "updated"}
	require.NoError(t, repo.Store(ctx, updated))

	got, err := repo.GetByCoords(ctx, 1.0, 2.0)
	require.NoError(t, err)
	assert.Equal(t, "updated", got.Direccion)
}

func TestGeocodingRepoDeleteByTenantIsNoop(t *testing.T) {
	repo := repository.NewInMemGeocodingRepo()
	ctx := context.Background()

	d := reports.DireccionCache{Latitud: 5.0, Longitud: 6.0, Direccion: "stays"}
	require.NoError(t, repo.Store(ctx, d))

	// DeleteByTenant must NOT delete global geocoding entries.
	require.NoError(t, repo.DeleteByTenant(ctx, "some-empresa-id"))

	got, err := repo.GetByCoords(ctx, 5.0, 6.0)
	require.NoError(t, err)
	assert.Equal(t, "stays", got.Direccion)
}
