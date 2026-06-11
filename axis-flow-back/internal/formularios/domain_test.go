package formularios_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/formularios"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventoStatusConstants asserts the four eventos_evento.status values are
// exactly the strings enforced by the CHECK constraint in V13.
func TestEventoStatusConstants(t *testing.T) {
	assert.Equal(t, "pendiente", formularios.EventoStatusPendiente)
	assert.Equal(t, "iniciado", formularios.EventoStatusIniciado)
	assert.Equal(t, "completado", formularios.EventoStatusCompletado)
	assert.Equal(t, "cancelado", formularios.EventoStatusCancelado)

	// Triangulate: all four are distinct.
	assert.NotEqual(t, formularios.EventoStatusPendiente, formularios.EventoStatusIniciado)
	assert.NotEqual(t, formularios.EventoStatusIniciado, formularios.EventoStatusCompletado)
	assert.NotEqual(t, formularios.EventoStatusCompletado, formularios.EventoStatusCancelado)
}

// TestIniciadoStatusConstants asserts the three eventos_evento_iniciado.status
// values are distinct and match the CHECK constraint.
func TestIniciadoStatusConstants(t *testing.T) {
	assert.Equal(t, "iniciado", formularios.IniciadoStatus)
	assert.Equal(t, "completado", formularios.CompletadoStatus)
	assert.Equal(t, "cancelado", formularios.CanceladoStatus)

	// Triangulate: all three are distinct.
	assert.NotEqual(t, formularios.IniciadoStatus, formularios.CompletadoStatus)
	assert.NotEqual(t, formularios.CompletadoStatus, formularios.CanceladoStatus)
}

// TestTipoPreguntaConstants asserts the closed set of tipo_pregunta values
// matches the CHECK constraint in V13.
func TestTipoPreguntaConstants(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"Texto", formularios.TipoPreguntaTexto, 1},
		{"Checkbox", formularios.TipoPreguntaCheckbox, 2},
		{"Rating", formularios.TipoPreguntaRating, 3},
		{"Matriz", formularios.TipoPreguntaMatriz, 5},
		{"Foto", formularios.TipoPreguntaFoto, 8},
		{"Firma", formularios.TipoPreguntaFirma, 11},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.got)
		})
	}
}

// TestSentinelErrors asserts each sentinel error has a unique non-empty message.
func TestSentinelErrors(t *testing.T) {
	errs := []error{
		formularios.ErrNotFound,
		formularios.ErrForbidden,
		formularios.ErrConflict,
		formularios.ErrInvalidInput,
	}

	seen := make(map[string]bool)
	for _, e := range errs {
		require.NotNil(t, e)
		msg := e.Error()
		assert.NotEmpty(t, msg)
		assert.False(t, seen[msg], "duplicate error message: %s", msg)
		seen[msg] = true
	}
}

// TestRedisCacheKeyConstantsAreDefined asserts all five Redis key constants
// exist and are non-empty strings.
func TestRedisCacheKeyConstantsAreDefined(t *testing.T) {
	keys := []string{
		formularios.KeyFormularioByEmpresa,
		formularios.KeyEventoByEmpCte,
		formularios.KeyRespuestasByIniciado,
		formularios.KeyReportePending,
		formularios.KeyReporteS3Lock,
	}

	seen := make(map[string]bool)
	for _, k := range keys {
		assert.NotEmpty(t, k, "Redis key constant must not be empty")
		assert.False(t, seen[k], "duplicate Redis key constant: %s", k)
		seen[k] = true
	}
}

// TestRepositoryPortsCompile verifies that stub types satisfy the repository
// port interfaces declared in domain.go — compile-time contract.
func TestRepositoryPortsCompile(t *testing.T) {
	var _ formularios.FormularioRepository = (*formularioRepositoryStub)(nil)
	var _ formularios.PreguntaRepository = (*preguntaRepositoryStub)(nil)
	var _ formularios.EventoRepository = (*eventoRepositoryStub)(nil)
	var _ formularios.RespuestaRepository = (*respuestaRepositoryStub)(nil)
}

// ---------------------------------------------------------------------------
// Stubs — satisfy the port interfaces at compile time only.
// ---------------------------------------------------------------------------

type formularioRepositoryStub struct{}

func (formularioRepositoryStub) Create(_ context.Context, _ *formularios.Formulario) error {
	return nil
}
func (formularioRepositoryStub) GetByID(_ context.Context, _, _ uuid.UUID) (*formularios.Formulario, error) {
	return nil, nil
}
func (formularioRepositoryStub) ListByEmpresa(_ context.Context, _ uuid.UUID, _ *bool, _, _ int) ([]*formularios.Formulario, int, error) {
	return nil, 0, nil
}

type preguntaRepositoryStub struct{}

func (preguntaRepositoryStub) Create(_ context.Context, _ *formularios.Pregunta) error {
	return nil
}
func (preguntaRepositoryStub) ListByFormulario(_ context.Context, _ uuid.UUID) ([]*formularios.Pregunta, error) {
	return nil, nil
}

type eventoRepositoryStub struct{}

func (eventoRepositoryStub) Create(_ context.Context, _ *formularios.Evento, _ []uuid.UUID) error {
	return nil
}
func (eventoRepositoryStub) GetByID(_ context.Context, _, _ uuid.UUID) (*formularios.Evento, error) {
	return nil, nil
}
func (eventoRepositoryStub) ListByEmpCte(_ context.Context, _, _ uuid.UUID, _ *string, _, _ int) ([]*formularios.Evento, int, error) {
	return nil, 0, nil
}
func (eventoRepositoryStub) CreateIniciado(_ context.Context, _ *formularios.EventoIniciado) error {
	return nil
}
func (eventoRepositoryStub) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}

type respuestaRepositoryStub struct{}

func (respuestaRepositoryStub) Create(_ context.Context, _ *formularios.Respuesta) error {
	return nil
}
func (respuestaRepositoryStub) ListByIniciado(_ context.Context, _ uuid.UUID) ([]*formularios.Respuesta, error) {
	return nil, nil
}
