// Package repository_test covers the in-memory pregunta repository behaviour.
//
// PR-2 (Repositories) — task 2.2.
package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestInMemPreguntaRepo(t *testing.T) *repository.InMemPreguntaRepository {
	t.Helper()
	return repository.NewInMemPreguntaRepository()
}

func samplePregunta(formularioID uuid.UUID, orden int, tipo int) *formularios.Pregunta {
	return &formularios.Pregunta{
		ID:            uuid.New(),
		FormularioID:  formularioID,
		Orden:         orden,
		TipoPregunta:  tipo,
		TextoPregunta: "Texto de la pregunta",
		Obligatoria:   false,
	}
}

// ---------------------------------------------------------------------------
// Create + ListByFormulario happy path
// ---------------------------------------------------------------------------

func TestInMemPreguntaRepositoryCreateAndListByFormulario(t *testing.T) {
	repo := newTestInMemPreguntaRepo(t)
	ctx := context.Background()

	formularioID := uuid.New()
	p := samplePregunta(formularioID, 1, formularios.TipoPreguntaTexto)
	require.NoError(t, repo.Create(ctx, p))

	results, err := repo.ListByFormulario(ctx, formularioID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, p.ID, results[0].ID)
	assert.Equal(t, p.FormularioID, results[0].FormularioID)
	assert.Equal(t, p.TextoPregunta, results[0].TextoPregunta)
}

// ---------------------------------------------------------------------------
// ListByFormulario returns rows ordered by `orden` ASC
// ---------------------------------------------------------------------------

func TestInMemPreguntaRepositoryListByFormularioOrdersByOrden(t *testing.T) {
	repo := newTestInMemPreguntaRepo(t)
	ctx := context.Background()

	formularioID := uuid.New()
	// Insert in non-sequential orden
	require.NoError(t, repo.Create(ctx, samplePregunta(formularioID, 3, formularios.TipoPreguntaMatriz)))
	require.NoError(t, repo.Create(ctx, samplePregunta(formularioID, 1, formularios.TipoPreguntaTexto)))
	require.NoError(t, repo.Create(ctx, samplePregunta(formularioID, 2, formularios.TipoPreguntaCheckbox)))

	results, err := repo.ListByFormulario(ctx, formularioID)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, 1, results[0].Orden)
	assert.Equal(t, 2, results[1].Orden)
	assert.Equal(t, 3, results[2].Orden)
}

// ---------------------------------------------------------------------------
// ListByFormulario excludes preguntas belonging to a different formulario.
// (Implicit IDOR: the parent formulario check is the service layer's job;
// here we just verify that the FK scope is honoured.)
// ---------------------------------------------------------------------------

func TestInMemPreguntaRepositoryListByFormularioExcludesOtherFormularios(t *testing.T) {
	repo := newTestInMemPreguntaRepo(t)
	ctx := context.Background()

	formA := uuid.New()
	formB := uuid.New()
	require.NoError(t, repo.Create(ctx, samplePregunta(formA, 1, formularios.TipoPreguntaTexto)))
	require.NoError(t, repo.Create(ctx, samplePregunta(formA, 2, formularios.TipoPreguntaCheckbox)))
	require.NoError(t, repo.Create(ctx, samplePregunta(formB, 1, formularios.TipoPreguntaRating)))

	results, err := repo.ListByFormulario(ctx, formA)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, p := range results {
		assert.Equal(t, formA, p.FormularioID)
	}
}

func TestInMemPreguntaRepositoryListByFormularioEmptyForUnknownFormulario(t *testing.T) {
	repo := newTestInMemPreguntaRepo(t)
	ctx := context.Background()

	// Insert something so the repo isn't empty in a trivial way
	require.NoError(t, repo.Create(ctx, samplePregunta(uuid.New(), 1, formularios.TipoPreguntaTexto)))

	results, err := repo.ListByFormulario(ctx, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// CHECK constraint on tipo_pregunta: 99 is not in {1,2,3,5,8,11}.
// The InMem adapter mirrors the SQL CHECK so the constraint is testable
// without a live database.
// ---------------------------------------------------------------------------

func TestInMemPreguntaRepositoryCreateRejectsInvalidTipoPregunta(t *testing.T) {
	repo := newTestInMemPreguntaRepo(t)
	ctx := context.Background()

	invalid := samplePregunta(uuid.New(), 1, 99) // not in {1,2,3,5,8,11}
	err := repo.Create(ctx, invalid)
	require.Error(t, err)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

func TestInMemPreguntaRepositoryCreateAcceptsAllValidTipoPreguntaValues(t *testing.T) {
	repo := newTestInMemPreguntaRepo(t)
	ctx := context.Background()

	for _, tipo := range []int{
		formularios.TipoPreguntaTexto,    // 1
		formularios.TipoPreguntaCheckbox, // 2
		formularios.TipoPreguntaRating,   // 3
		formularios.TipoPreguntaMatriz,   // 5
		formularios.TipoPreguntaFoto,     // 8
		formularios.TipoPreguntaFirma,    // 11
	} {
		require.NoError(t, repo.Create(ctx, samplePregunta(uuid.New(), 1, tipo)),
			"expected to accept tipo_pregunta=%d", tipo)
	}
}

// ---------------------------------------------------------------------------
// JSONB field round-trip: respuesta_predefinida must survive Create+List.
// ---------------------------------------------------------------------------

func TestInMemPreguntaRepositoryCreatePersistsRespuestaPredefinidaJSON(t *testing.T) {
	repo := newTestInMemPreguntaRepo(t)
	ctx := context.Background()

	predef := json.RawMessage(`{"filas":["Piso","Espejos"],"columnas":["Limpio","Sucio"]}`)
	p := samplePregunta(uuid.New(), 1, formularios.TipoPreguntaMatriz)
	p.RespuestaPredefinida = predef
	require.NoError(t, repo.Create(ctx, p))

	results, err := repo.ListByFormulario(ctx, p.FormularioID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.JSONEq(t, string(predef), string(results[0].RespuestaPredefinida))
}

// ---------------------------------------------------------------------------
// Compile-time port satisfaction.
// ---------------------------------------------------------------------------

var _ formularios.PreguntaRepository = (*repository.InMemPreguntaRepository)(nil)
