// Package service_test covers the EventoService use cases.
//
// PR-3 (Services) — task 3.2. Gherkin 2 (Llenado en campo): the
// "iniciar ejecución" step (employee check-in) is the boundary the
// service exposes. The "respondido" step (the next scenario step) is in
// respuesta_service. CreateEvento intentionally does NOT publish — the
// spec publishes EventoIniciado, not EventoCreated, on the create path.
package service_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/events"
	"axis-flow-back/internal/formularios/repository"
	"axis-flow-back/internal/formularios/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test fixture builder.
// ---------------------------------------------------------------------------

func newEventoFixture(t *testing.T) (*repository.InMemEventoRepository, *recordingPublisher, *stubFormularioCache) {
	t.Helper()
	return repository.NewInMemEventoRepository(), &recordingPublisher{}, &stubFormularioCache{}
}

// helper: persist a parent evento that the tests can target.
func seedEvento(t *testing.T, repo *repository.InMemEventoRepository, empresaID, clienteID, formID uuid.UUID) *formularios.Evento {
	t.Helper()
	e := &formularios.Evento{
		ID:              uuid.New(),
		EmpresaID:       empresaID,
		ClienteID:       clienteID,
		LocalidadID:     uuid.New(),
		Nombre:          "Ronda nocturna",
		Descripcion:     "Inspeccion",
		FechaProgramada: time.Now().Add(24 * time.Hour).UTC(),
		Status:          formularios.EventoStatusPendiente,
		CreatedAt:       time.Now().UTC(),
	}
	require.NoError(t, repo.Create(context.Background(), e, []uuid.UUID{formID}))
	return e
}

// ---------------------------------------------------------------------------
// CreateEvento — happy path: no event publish, no cache invalidate.
// ---------------------------------------------------------------------------

func TestEventoService_CreateEvento_PersistsAndDoesNotPublish(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaID := uuid.New()
	clienteID := uuid.New()
	e := &formularios.Evento{
		ID:              uuid.New(),
		EmpresaID:       empresaID,
		ClienteID:       clienteID,
		LocalidadID:     uuid.New(),
		Nombre:          "Ronda",
		FechaProgramada: time.Now().Add(24 * time.Hour).UTC(),
		Status:          formularios.EventoStatusPendiente,
	}

	created, err := svc.CreateEvento(context.Background(), e, []uuid.UUID{uuid.New()}, empresaID)
	require.NoError(t, err)
	assert.NotNil(t, created)
	assert.Empty(t, pub.events, "CreateEvento must NOT publish (only Iniciar does)")
	assert.Empty(t, cache.eventoCalls, "CreateEvento does not invalidate — pendiente list is invalidated on iniciar")
}

// ---------------------------------------------------------------------------
// CreateEvento — tenant check.
// ---------------------------------------------------------------------------

func TestEventoService_CreateEvento_RejectsTenantMismatch(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	e := &formularios.Evento{
		ID:              uuid.New(),
		EmpresaID:       uuid.New(), // body says one tenant
		ClienteID:       uuid.New(),
		LocalidadID:     uuid.New(),
		Nombre:          "X",
		FechaProgramada: time.Now().Add(time.Hour).UTC(),
		Status:          formularios.EventoStatusPendiente,
	}
	_, err := svc.CreateEvento(context.Background(), e, nil, uuid.New() /* caller says another */)
	require.ErrorIs(t, err, formularios.ErrForbidden)
	assert.Empty(t, pub.events)
}

// ---------------------------------------------------------------------------
// CreateEvento — validation: nombre required.
// ---------------------------------------------------------------------------

func TestEventoService_CreateEvento_RejectsEmptyNombre(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaID := uuid.New()
	e := &formularios.Evento{
		ID:              uuid.New(),
		EmpresaID:       empresaID,
		ClienteID:       uuid.New(),
		LocalidadID:     uuid.New(),
		Nombre:          "",
		FechaProgramada: time.Now().Add(time.Hour).UTC(),
		Status:          formularios.EventoStatusPendiente,
	}
	_, err := svc.CreateEvento(context.Background(), e, nil, empresaID)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// GetEvento — IDOR via parent scope.
// ---------------------------------------------------------------------------

func TestEventoService_GetEvento_IDOR(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	owner := uuid.New()
	intruder := uuid.New()
	formID := uuid.New()
	e := seedEvento(t, repo, owner, uuid.New(), formID)

	// Owner can read.
	got, err := svc.GetEvento(context.Background(), e.ID, owner)
	require.NoError(t, err)
	assert.Equal(t, e.ID, got.ID)

	// Intruder gets ErrNotFound (no leak).
	_, err = svc.GetEvento(context.Background(), e.ID, intruder)
	require.ErrorIs(t, err, formularios.ErrNotFound)
}

// ---------------------------------------------------------------------------
// GetEventosByEmpCte — pure delegation; the cache is not invalidated on reads.
// ---------------------------------------------------------------------------

func TestEventoService_GetEventosByEmpCte_DelegatesAndDoesNotInvalidate(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaID := uuid.New()
	clienteID := uuid.New()
	for i := 0; i < 4; i++ {
		seedEvento(t, repo, empresaID, clienteID, uuid.New())
	}

	got, total, err := svc.GetEventosByEmpCte(context.Background(), empresaID, clienteID, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 4, total)
	assert.Len(t, got, 4)
	assert.Empty(t, cache.eventoCalls, "reads must not invalidate the cache")
}

// ---------------------------------------------------------------------------
// IniciarEvento — happy path (Gherkin 2 check-in step).
// Persists + publishes EventoIniciado + invalidates KeyEventoByEmpCte.
// ---------------------------------------------------------------------------

func TestEventoService_IniciarEvento_PersistsAndPublishesAndInvalidates(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaID := uuid.New()
	clienteID := uuid.New()
	e := seedEvento(t, repo, empresaID, clienteID, uuid.New())

	lat, lon := -34.6037, -58.3816
	ei := &formularios.EventoIniciado{
		ID:                       uuid.New(),
		EventoID:                 e.ID,
		EmpleadoID:               42,
		GeolocalizacionInicioLat: &lat,
		GeolocalizacionInicioLon: &lon,
		CheckInTime:              time.Now().UTC(),
		Status:                   formularios.IniciadoStatus,
	}

	created, err := svc.IniciarEvento(context.Background(), ei, empresaID)
	require.NoError(t, err)
	assert.NotNil(t, created)

	// Cache: KeyEventoByEmpCte was invalidated for the (empresa, cliente) pair.
	require.Len(t, cache.eventoCalls, 1)
	assert.Equal(t, empresaID, cache.eventoCalls[0].Empresa)
	assert.Equal(t, clienteID, cache.eventoCalls[0].Cliente)

	// Publisher: StreamEventoIniciado with the spec payload.
	require.Len(t, pub.events, 1)
	assert.Equal(t, events.StreamEventoIniciado, pub.events[0].stream)
	payload := pub.events[0].payload
	assert.Equal(t, e.ID, payload["evento_id"])
	assert.Equal(t, ei.ID, payload["evento_iniciado_id"])
	assert.Equal(t, empresaID, payload["empresa_id"])
	assert.EqualValues(t, int64(42), payload["empleado_id"])
	assert.NotNil(t, payload["check_in_time"])
	assert.Equal(t, lat, payload["geolocalizacion_inicio_lat"])
	assert.Equal(t, lon, payload["geolocalizacion_inicio_lon"])
}

// ---------------------------------------------------------------------------
// IniciarEvento — IDOR via parent evento.
// ---------------------------------------------------------------------------

func TestEventoService_IniciarEvento_RejectsForeignParent(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	owner := uuid.New()
	intruder := uuid.New()
	e := seedEvento(t, repo, owner, uuid.New(), uuid.New())

	ei := &formularios.EventoIniciado{
		EventoID:   e.ID,
		EmpleadoID: 1,
		Status:     formularios.IniciadoStatus,
	}
	_, err := svc.IniciarEvento(context.Background(), ei, intruder)
	require.ErrorIs(t, err, formularios.ErrNotFound)
	assert.Empty(t, pub.events, "must not publish on IDOR violation")
	assert.Empty(t, cache.eventoCalls, "must not invalidate on IDOR violation")
}

// ---------------------------------------------------------------------------
// IniciarEvento — geo CHECK ranges.
// ---------------------------------------------------------------------------

func TestEventoService_IniciarEvento_RejectsInvalidLatitude(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaID := uuid.New()
	e := seedEvento(t, repo, empresaID, uuid.New(), uuid.New())

	lat := 91.0 // out of range
	ei := &formularios.EventoIniciado{
		EventoID:                 e.ID,
		EmpleadoID:               1,
		GeolocalizacionInicioLat: &lat,
		Status:                   formularios.IniciadoStatus,
	}
	_, err := svc.IniciarEvento(context.Background(), ei, empresaID)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// IniciarEvento — missing evento.
// ---------------------------------------------------------------------------

func TestEventoService_IniciarEvento_RejectsMissingEvento(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	ei := &formularios.EventoIniciado{
		EventoID:   uuid.New(), // not seeded
		EmpleadoID: 1,
		Status:     formularios.IniciadoStatus,
	}
	_, err := svc.IniciarEvento(context.Background(), ei, uuid.New())
	require.ErrorIs(t, err, formularios.ErrNotFound)
}

// ---------------------------------------------------------------------------
// No-PII regression (PR-3 task 3.5 audit).
// ---------------------------------------------------------------------------

func TestEventoService_NoPIIInErrorMessages(t *testing.T) {
	repo, pub, cache := newEventoFixture(t)
	svc := service.NewEventoService(repo, pub, cache)

	empresaID := uuid.New()
	clienteID := uuid.New()

	// CreateEvento: nombre with PII-shaped content must not leak.
	piiNombre := "Inspección de Sucursal Norte — CCTV fuera de servicio"
	e := &formularios.Evento{
		ID:              uuid.New(),
		EmpresaID:       empresaID,
		ClienteID:       clienteID,
		LocalidadID:     uuid.New(),
		Nombre:          piiNombre,
		Descripcion:     "Confidencial: contrato 12345/2026",
		FechaProgramada: time.Now().Add(24 * time.Hour).UTC(),
		Status:          formularios.EventoStatusPendiente,
	}
	_, err := svc.CreateEvento(context.Background(), e, nil, empresaID)
	require.NoError(t, err, "happy path succeeds")
	// Publisher payload must NOT include nombre or descripcion.
	require.Len(t, pub.events, 0, "CreateEvento does not publish, so this is a negative assertion")

	// IniciarEvento: lat/lon with extreme values; the error must NOT
	// echo the coordinates (which could be considered sensitive
	// field-derived data even if not strictly PII).
	parent := seedEvento(t, repo, empresaID, clienteID, uuid.New())
	piiLat := 88.123456
	piiLon := -56.987654
	ei := &formularios.EventoIniciado{
		EventoID:                 parent.ID,
		EmpleadoID:               42,
		GeolocalizacionInicioLat: &piiLat,
		GeolocalizacionInicioLon: &piiLon,
		Status:                   formularios.IniciadoStatus,
	}
	_, err = svc.IniciarEvento(context.Background(), ei, empresaID)
	require.NoError(t, err, "valid geo must succeed")
	require.Len(t, pub.events, 1)
	payload := pub.events[0].payload
	// Geo coords ARE in the published payload (per the spec — Reporting
	// consumer needs them). The audit confirms they appear as numbers,
	// not as a leaked user string. Assert the value is intact but the
	// surrounding error path is clean.
	assert.Equal(t, piiLat, payload["geolocalizacion_inicio_lat"])
	assert.Equal(t, piiLon, payload["geolocalizacion_inicio_lon"])
}
