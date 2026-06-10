// Package formularios_test covers the in-memory respuesta repository behaviour.
//
// PR-2 (Repositories) — task 2.4.
//
// RespuestaRepository handles answers captured in the field during a running
// EventoIniciado. The repository stores VARCHAR paths for evidence URLs only —
// the S3 upload itself is a service-layer concern (PR-3).
package formularios_test

import (
	"context"
	"encoding/json"
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

func newTestInMemRespuestaRepo(t *testing.T) *formularios.InMemRespuestaRepository {
	t.Helper()
	return formularios.NewInMemRespuestaRepository()
}

func sampleRespuesta(iniciadoID, preguntaID uuid.UUID) *formularios.Respuesta {
	now := time.Now().UTC()
	return &formularios.Respuesta{
		ID:               uuid.New(),
		EventoIniciadoID: iniciadoID,
		PreguntaID:       preguntaID,
		RespuestaTexto:   "Limpio",
		RespuestaLista:   json.RawMessage(`["opcion_a"]`),
		Evidencia1:       "s3://bucket/evidence/photo1.jpg",
		Evidencia2:       "s3://bucket/evidence/photo2.jpg",
		Evidencia3:       "s3://bucket/evidence/photo3.jpg",
		DocumentoURL:     "s3://bucket/docs/firma.png",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// ---------------------------------------------------------------------------
// Create + ListByIniciado happy path
// ---------------------------------------------------------------------------

func TestInMemRespuestaRepositoryCreateAndListByIniciado(t *testing.T) {
	repo := newTestInMemRespuestaRepo(t)
	ctx := context.Background()

	iniciadoID := uuid.New()
	preguntaID := uuid.New()
	r := sampleRespuesta(iniciadoID, preguntaID)
	require.NoError(t, repo.Create(ctx, r))

	results, err := repo.ListByIniciado(ctx, iniciadoID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, r.ID, results[0].ID)
	assert.Equal(t, r.PreguntaID, results[0].PreguntaID)
	assert.Equal(t, r.RespuestaTexto, results[0].RespuestaTexto)
	assert.Equal(t, r.Evidencia1, results[0].Evidencia1)
	assert.Equal(t, r.Evidencia2, results[0].Evidencia2)
	assert.Equal(t, r.Evidencia3, results[0].Evidencia3)
	assert.Equal(t, r.DocumentoURL, results[0].DocumentoURL)
}

// ---------------------------------------------------------------------------
// Create with JSON-only payload (no multipart): evidencia fields empty
// ---------------------------------------------------------------------------

func TestInMemRespuestaRepositoryCreateJSONOnly(t *testing.T) {
	repo := newTestInMemRespuestaRepo(t)
	ctx := context.Background()

	iniciadoID := uuid.New()
	r := sampleRespuesta(iniciadoID, uuid.New())
	// JSON path: no evidence uploaded yet
	r.Evidencia1 = ""
	r.Evidencia2 = ""
	r.Evidencia3 = ""
	r.DocumentoURL = ""
	r.RespuestaLista = json.RawMessage(`{"fila":"Piso","columna":"Limpio"}`)
	require.NoError(t, repo.Create(ctx, r))

	results, err := repo.ListByIniciado(ctx, iniciadoID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Empty(t, results[0].Evidencia1)
	assert.Empty(t, results[0].Evidencia2)
	assert.Empty(t, results[0].Evidencia3)
	assert.Empty(t, results[0].DocumentoURL)
}

// ---------------------------------------------------------------------------
// Create with multipart-style payload (all evidence fields populated)
// ---------------------------------------------------------------------------

func TestInMemRespuestaRepositoryCreateMultipartFullPayload(t *testing.T) {
	repo := newTestInMemRespuestaRepo(t)
	ctx := context.Background()

	iniciadoID := uuid.New()
	r := sampleRespuesta(iniciadoID, uuid.New())
	r.RespuestaTexto = "" // matrix answer uses respuesta_lista, not texto
	r.RespuestaLista = json.RawMessage(`{"valor":"sucio"}`)
	require.NoError(t, repo.Create(ctx, r))

	results, err := repo.ListByIniciado(ctx, iniciadoID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.NotEmpty(t, results[0].Evidencia1)
	assert.NotEmpty(t, results[0].Evidencia2)
	assert.NotEmpty(t, results[0].Evidencia3)
	assert.NotEmpty(t, results[0].DocumentoURL)
}

// ---------------------------------------------------------------------------
// ListByIniciado: IDOR-by-FK scope — only respuestas for the given iniciado
// are returned.
// ---------------------------------------------------------------------------

func TestInMemRespuestaRepositoryListByIniciadoExcludesOtherIniciados(t *testing.T) {
	repo := newTestInMemRespuestaRepo(t)
	ctx := context.Background()

	iniciadoA := uuid.New()
	iniciadoB := uuid.New()
	preguntaID := uuid.New()
	require.NoError(t, repo.Create(ctx, sampleRespuesta(iniciadoA, preguntaID)))
	require.NoError(t, repo.Create(ctx, sampleRespuesta(iniciadoA, preguntaID)))
	require.NoError(t, repo.Create(ctx, sampleRespuesta(iniciadoB, preguntaID)))

	results, err := repo.ListByIniciado(ctx, iniciadoA)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, iniciadoA, r.EventoIniciadoID)
	}
}

func TestInMemRespuestaRepositoryListByIniciadoEmptyForUnknownIniciado(t *testing.T) {
	repo := newTestInMemRespuestaRepo(t)
	ctx := context.Background()

	// Pre-populate so the empty list is meaningful
	require.NoError(t, repo.Create(ctx, sampleRespuesta(uuid.New(), uuid.New())))

	results, err := repo.ListByIniciado(ctx, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// Multiple respuestas for the same iniciado — list returns all, ordered by
// created_at ASC (insertion order).
// ---------------------------------------------------------------------------

func TestInMemRespuestaRepositoryListByIniciadoPreservesInsertionOrder(t *testing.T) {
	repo := newTestInMemRespuestaRepo(t)
	ctx := context.Background()

	iniciadoID := uuid.New()
	first := sampleRespuesta(iniciadoID, uuid.New())
	first.RespuestaTexto = "primera"
	first.CreatedAt = time.Now().UTC().Add(-2 * time.Second)
	second := sampleRespuesta(iniciadoID, uuid.New())
	second.RespuestaTexto = "segunda"
	second.CreatedAt = time.Now().UTC().Add(-1 * time.Second)
	third := sampleRespuesta(iniciadoID, uuid.New())
	third.RespuestaTexto = "tercera"
	third.CreatedAt = time.Now().UTC()

	// Insert out of order to prove ListByIniciado re-orders
	require.NoError(t, repo.Create(ctx, third))
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, second))

	results, err := repo.ListByIniciado(ctx, iniciadoID)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "primera", results[0].RespuestaTexto)
	assert.Equal(t, "segunda", results[1].RespuestaTexto)
	assert.Equal(t, "tercera", results[2].RespuestaTexto)
}

// ---------------------------------------------------------------------------
// Geolocation CHECK: lat=91.0 must be rejected.
// ---------------------------------------------------------------------------

func TestInMemRespuestaRepositoryCreateRejectsInvalidLatitude(t *testing.T) {
	repo := newTestInMemRespuestaRepo(t)
	ctx := context.Background()

	r := sampleRespuesta(uuid.New(), uuid.New())
	lat, lon := 91.0, 0.0
	r.GeolocalizacionRespuestaLat = &lat
	r.GeolocalizacionRespuestaLon = &lon
	err := repo.Create(ctx, r)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

func TestInMemRespuestaRepositoryCreateAcceptsNullGeolocation(t *testing.T) {
	repo := newTestInMemRespuestaRepo(t)
	ctx := context.Background()

	r := sampleRespuesta(uuid.New(), uuid.New())
	r.GeolocalizacionRespuestaLat = nil
	r.GeolocalizacionRespuestaLon = nil
	require.NoError(t, repo.Create(ctx, r))
}

// ---------------------------------------------------------------------------
// Compile-time port satisfaction.
// ---------------------------------------------------------------------------

var _ formularios.RespuestaRepository = (*formularios.InMemRespuestaRepository)(nil)
