// Package repository_test covers the in-memory evento repository behaviour.
//
// PR-2 (Repositories) — task 2.3.
//
// Covers Evento, EventoFormulario, and EventoIniciado. The Create method
// inserts both the evento header and the M:N association rows in a single
// logical operation; the InMem adapter wraps this with the same UNIQUE
// (evento_id, formulario_id) constraint the SQL schema enforces.
package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestInMemEventoRepo(t *testing.T) *repository.InMemEventoRepository {
	t.Helper()
	return repository.NewInMemEventoRepository()
}

func sampleEvento(empresaID, clienteID, localidadID uuid.UUID) *formularios.Evento {
	now := time.Now().UTC()
	return &formularios.Evento{
		ID:              uuid.New(),
		EmpresaID:       empresaID,
		ClienteID:       clienteID,
		LocalidadID:     localidadID,
		Nombre:          "Inspeccion sede central",
		Descripcion:     "Recorrido mensual",
		FechaProgramada: now.Add(24 * time.Hour),
		Status:          formularios.EventoStatusPendiente,
		CreatedAt:       now,
	}
}

func sampleIniciado(eventoID uuid.UUID, empleadoID int64) *formularios.EventoIniciado {
	lat, lon := -34.6037, -58.3816
	return &formularios.EventoIniciado{
		ID:                       uuid.New(),
		EventoID:                 eventoID,
		EmpleadoID:               empleadoID,
		GeolocalizacionInicioLat: &lat,
		GeolocalizacionInicioLon: &lon,
		CheckInTime:              time.Now().UTC(),
		Status:                   formularios.IniciadoStatus,
	}
}

// ---------------------------------------------------------------------------
// Create + GetByID happy path with formularioIDs (M:N association rows)
// ---------------------------------------------------------------------------

func TestInMemEventoRepositoryCreateAndGetByIDRoundTrip(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	clienteID := uuid.New()
	localidadID := uuid.New()
	e := sampleEvento(empresaID, clienteID, localidadID)
	formIDs := []uuid.UUID{uuid.New(), uuid.New()}

	require.NoError(t, repo.Create(ctx, e, formIDs))

	got, err := repo.GetByID(ctx, e.ID, empresaID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, e.ID, got.ID)
	assert.Equal(t, e.EmpresaID, got.EmpresaID)
	assert.Equal(t, e.Nombre, got.Nombre)
	assert.Equal(t, e.Status, got.Status)
}

// ---------------------------------------------------------------------------
// IDOR: GetByID with the wrong empresa_id must return formularios.ErrNotFound.
// ---------------------------------------------------------------------------

func TestInMemEventoRepositoryGetByIDRejectsForeignEmpresa(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	owner := uuid.New()
	intruder := uuid.New()
	e := sampleEvento(owner, uuid.New(), uuid.New())
	require.NoError(t, repo.Create(ctx, e, nil))

	_, err := repo.GetByID(ctx, e.ID, intruder)
	require.ErrorIs(t, err, formularios.ErrNotFound)
}

func TestInMemEventoRepositoryGetByIDReturnsNotFoundForMissingID(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New(), uuid.New())
	require.ErrorIs(t, err, formularios.ErrNotFound)
}

// ---------------------------------------------------------------------------
// UNIQUE (evento_id, formulario_id): second insert with the same pair must
// fail with formularios.ErrConflict.
// ---------------------------------------------------------------------------

func TestInMemEventoRepositoryCreateRejectsDuplicateEventoFormularioPair(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	e := sampleEvento(empresaID, uuid.New(), uuid.New())
	formID := uuid.New()

	require.NoError(t, repo.Create(ctx, e, []uuid.UUID{formID}))

	// Same (evento_id, formulario_id) must be rejected.
	err := repo.Create(ctx, e, []uuid.UUID{formID})
	require.ErrorIs(t, err, formularios.ErrConflict)
}

// ---------------------------------------------------------------------------
// ListByEmpCte: filter by empresa AND cliente, optional status
// ---------------------------------------------------------------------------

func TestInMemEventoRepositoryListByEmpCteReturnsAllWhenStatusNil(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	clienteA := uuid.New()
	clienteB := uuid.New()

	e1 := sampleEvento(empresaID, clienteA, uuid.New())
	e1.Status = formularios.EventoStatusPendiente
	e2 := sampleEvento(empresaID, clienteA, uuid.New())
	e2.Status = formularios.EventoStatusCompletado
	e3 := sampleEvento(empresaID, clienteB, uuid.New()) // other cliente
	e4 := sampleEvento(uuid.New(), clienteA, uuid.New()) // other empresa

	require.NoError(t, repo.Create(ctx, e1, nil))
	require.NoError(t, repo.Create(ctx, e2, nil))
	require.NoError(t, repo.Create(ctx, e3, nil))
	require.NoError(t, repo.Create(ctx, e4, nil))

	results, total, err := repo.ListByEmpCte(ctx, empresaID, clienteA, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, results, 2)
}

func TestInMemEventoRepositoryListByEmpCteFiltersByStatus(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	clienteID := uuid.New()
	e1 := sampleEvento(empresaID, clienteID, uuid.New())
	e1.Status = formularios.EventoStatusPendiente
	e2 := sampleEvento(empresaID, clienteID, uuid.New())
	e2.Status = formularios.EventoStatusCompletado
	e3 := sampleEvento(empresaID, clienteID, uuid.New())
	e3.Status = formularios.EventoStatusPendiente

	require.NoError(t, repo.Create(ctx, e1, nil))
	require.NoError(t, repo.Create(ctx, e2, nil))
	require.NoError(t, repo.Create(ctx, e3, nil))

	status := formularios.EventoStatusPendiente
	results, total, err := repo.ListByEmpCte(ctx, empresaID, clienteID, &status, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, results, 2)
	for _, e := range results {
		assert.Equal(t, formularios.EventoStatusPendiente, e.Status)
	}
}

func TestInMemEventoRepositoryListByEmpCteExcludesOtherEmpresas(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	other := uuid.New()
	clienteID := uuid.New()
	require.NoError(t, repo.Create(ctx, sampleEvento(empresaID, clienteID, uuid.New()), nil))
	require.NoError(t, repo.Create(ctx, sampleEvento(other, clienteID, uuid.New()), nil))

	results, total, err := repo.ListByEmpCte(ctx, empresaID, clienteID, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, results, 1)
	assert.Equal(t, empresaID, results[0].EmpresaID)
}

func TestInMemEventoRepositoryListByEmpCteEmptyForUnknownPair(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	results, total, err := repo.ListByEmpCte(ctx, uuid.New(), uuid.New(), nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// CreateIniciado happy path
// ---------------------------------------------------------------------------

func TestInMemEventoRepositoryCreateIniciadoHappyPath(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	e := sampleEvento(empresaID, uuid.New(), uuid.New())
	require.NoError(t, repo.Create(ctx, e, nil))

	iniciado := sampleIniciado(e.ID, 42)
	require.NoError(t, repo.CreateIniciado(ctx, iniciado))
	assert.NotEqual(t, uuid.Nil, iniciado.ID)
}

// ---------------------------------------------------------------------------
// CreateIniciado IDOR: parent evento must exist AND belong to the calling
// empresa. The InMem adapter looks up the evento to enforce the cross-table
// IDOR since the iniciado row itself has no empresa_id column.
// ---------------------------------------------------------------------------

func TestInMemEventoRepositoryCreateIniciadoRejectsMissingEvento(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	iniciado := sampleIniciado(uuid.New(), 42)
	err := repo.CreateIniciado(ctx, iniciado)
	require.ErrorIs(t, err, formularios.ErrNotFound)
}

// ---------------------------------------------------------------------------
// CreateIniciado CHECK on geolocation: lat=91.0 must be rejected.
// ---------------------------------------------------------------------------

func TestInMemEventoRepositoryCreateIniciadoRejectsInvalidLatitude(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	e := sampleEvento(empresaID, uuid.New(), uuid.New())
	require.NoError(t, repo.Create(ctx, e, nil))

	lat, lon := 91.0, 0.0
	iniciado := sampleIniciado(e.ID, 42)
	iniciado.GeolocalizacionInicioLat = &lat
	iniciado.GeolocalizacionInicioLon = &lon
	err := repo.CreateIniciado(ctx, iniciado)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

func TestInMemEventoRepositoryCreateIniciadoRejectsInvalidLongitude(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	e := sampleEvento(empresaID, uuid.New(), uuid.New())
	require.NoError(t, repo.Create(ctx, e, nil))

	lat, lon := 0.0, 181.0
	iniciado := sampleIniciado(e.ID, 42)
	iniciado.GeolocalizacionInicioLat = &lat
	iniciado.GeolocalizacionInicioLon = &lon
	err := repo.CreateIniciado(ctx, iniciado)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

func TestInMemEventoRepositoryCreateIniciadoAcceptsNullGeolocation(t *testing.T) {
	repo := newTestInMemEventoRepo(t)
	ctx := context.Background()

	empresaID := uuid.New()
	e := sampleEvento(empresaID, uuid.New(), uuid.New())
	require.NoError(t, repo.Create(ctx, e, nil))

	iniciado := sampleIniciado(e.ID, 42)
	iniciado.GeolocalizacionInicioLat = nil
	iniciado.GeolocalizacionInicioLon = nil
	require.NoError(t, repo.CreateIniciado(ctx, iniciado))
}

// ---------------------------------------------------------------------------
// Compile-time port satisfaction.
// ---------------------------------------------------------------------------

var _ formularios.EventoRepository = (*repository.InMemEventoRepository)(nil)
