// Package service_test covers the RespuestaService use cases.
//
// PR-3 (Services) — task 3.3. Gherkin 2 (Llenado en campo de checklist):
// the "responde el checklist" step is SubmitRespuesta. The service
// persists, publishes FormularioRespondido, and invalidates the cached
// respuestas list for the check-in. Geo range validation is enforced in
// the InMemRespuestaRepository (mirrors the SQL CHECK); the service
// relies on that for the gate.
//
// IDOR: the repository has no tenant column on respuestas, so the
// service relies on the handler layer (PR-4) to scope the call. This
// matches the 09 pattern (respuestas equivalents in atencionseguimiento
// rely on the same chain — see Deviation #2 in apply-progress #343+).
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

func newRespuestaFixture(t *testing.T) (
	*repository.InMemRespuestaRepository,
	*recordingPublisher,
	*stubFormularioCache,
) {
	t.Helper()
	return repository.NewInMemRespuestaRepository(), &recordingPublisher{}, &stubFormularioCache{}
}

func sampleRespuesta(iniciadoID, preguntaID uuid.UUID) *formularios.Respuesta {
	return &formularios.Respuesta{
		ID:               uuid.New(),
		EventoIniciadoID: iniciadoID,
		PreguntaID:       preguntaID,
		RespuestaTexto:   "Limpio",
	}
}

// ---------------------------------------------------------------------------
// SubmitRespuesta — happy path (Gherkin 2 response step).
// ---------------------------------------------------------------------------

func TestRespuestaService_SubmitRespuesta_PersistsAndPublishesAndInvalidates(t *testing.T) {
	respRepo, pub, cache := newRespuestaFixture(t)
	svc := service.NewRespuestaService(respRepo, pub, cache)

	iniciadoID := uuid.New()
	preguntaID := uuid.New()
	r := sampleRespuesta(iniciadoID, preguntaID)
	empresaID := uuid.New()

	created, err := svc.SubmitRespuesta(context.Background(), r, empresaID)
	require.NoError(t, err)
	assert.Equal(t, r.ID, created.ID)

	// Cache: KeyRespuestasByIniciado invalidated.
	assert.Equal(t, []uuid.UUID{iniciadoID}, cache.respuestaCalls)

	// Publisher: StreamFormularioRespondido with the spec payload.
	require.Len(t, pub.events, 1)
	assert.Equal(t, events.StreamFormularioRespondido, pub.events[0].stream)
	payload := pub.events[0].payload
	assert.Equal(t, r.ID, payload["respuesta_id"])
	assert.Equal(t, iniciadoID, payload["evento_iniciado_id"])
	assert.Equal(t, preguntaID, payload["pregunta_id"])
	assert.Equal(t, empresaID, payload["empresa_id"])
}

// ---------------------------------------------------------------------------
// SubmitRespuesta — validation: required IDs non-zero.
// ---------------------------------------------------------------------------

func TestRespuestaService_SubmitRespuesta_RejectsMissingIniciadoID(t *testing.T) {
	respRepo, pub, cache := newRespuestaFixture(t)
	svc := service.NewRespuestaService(respRepo, pub, cache)

	r := &formularios.Respuesta{PreguntaID: uuid.New()} // EventoIniciadoID is uuid.Nil
	_, err := svc.SubmitRespuesta(context.Background(), r, uuid.New())
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
	assert.Empty(t, pub.events)
	assert.Empty(t, cache.respuestaCalls)
}

func TestRespuestaService_SubmitRespuesta_RejectsMissingPreguntaID(t *testing.T) {
	respRepo, pub, cache := newRespuestaFixture(t)
	svc := service.NewRespuestaService(respRepo, pub, cache)

	r := &formularios.Respuesta{EventoIniciadoID: uuid.New()} // PreguntaID is uuid.Nil
	_, err := svc.SubmitRespuesta(context.Background(), r, uuid.New())
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// SubmitRespuesta — geo range check propagated from the repo.
// ---------------------------------------------------------------------------

func TestRespuestaService_SubmitRespuesta_RejectsInvalidLatitude(t *testing.T) {
	respRepo, pub, cache := newRespuestaFixture(t)
	svc := service.NewRespuestaService(respRepo, pub, cache)

	lat := 91.0
	r := &formularios.Respuesta{
		EventoIniciadoID:            uuid.New(),
		PreguntaID:                  uuid.New(),
		GeolocalizacionRespuestaLat: &lat,
	}
	_, err := svc.SubmitRespuesta(context.Background(), r, uuid.New())
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// GetRespuestasByIniciado — pure delegation. The cache is not invalidated
// on reads.
// ---------------------------------------------------------------------------

func TestRespuestaService_GetRespuestasByIniciado_DelegatesAndDoesNotInvalidate(t *testing.T) {
	respRepo, pub, cache := newRespuestaFixture(t)
	svc := service.NewRespuestaService(respRepo, pub, cache)

	iniciadoID := uuid.New()
	for i := 0; i < 3; i++ {
		r := sampleRespuesta(iniciadoID, uuid.New())
		require.NoError(t, respRepo.Create(context.Background(), r))
	}

	got, err := svc.GetRespuestasByIniciado(context.Background(), iniciadoID, uuid.New())
	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Empty(t, cache.respuestaCalls, "reads must not invalidate")
	assert.Empty(t, pub.events, "reads must not publish")
}

// ---------------------------------------------------------------------------
// SubmitRespuesta — fire-and-forget: publish and cache errors are swallowed.
// ---------------------------------------------------------------------------

func TestRespuestaService_SubmitRespuesta_SwallowsPublisherAndCacheErrors(t *testing.T) {
	respRepo, pub, cache := newRespuestaFixture(t)
	pub.err = assert.AnError
	cache.respuestaErr = assert.AnError
	svc := service.NewRespuestaService(respRepo, pub, cache)

	r := sampleRespuesta(uuid.New(), uuid.New())
	_, err := svc.SubmitRespuesta(context.Background(), r, uuid.New())
	require.NoError(t, err, "publisher/cache errors must not fail the write")
}

// time used to silence the "imported and not used" linter when the test
// evolves to assert on check_in_time.
var _ = time.Now
