// Package service_test covers the FormularioService use cases.
//
// PR-3 (Services) — task 3.1. Gherkin 1 (Creación de plantilla de formulario
// por un Administrador): a Formulario header is validated, persisted, and
// FormularioCreado is published; on the next read the cached list is
// invalidated. AddPregunta enforces IDOR via the parent formulario's tenant.
package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/events"
	"axis-flow-back/internal/formularios/repository"
	"axis-flow-back/internal/formularios/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Inline mocks for the FormularioService collaborators.
// ---------------------------------------------------------------------------

// stubFormularioCache is a service.CacheInvalidator that records each
// invalidation call. Implements the full 3-method surface so it can stand
// in for *repository.RedisFormulariosCacheInvalidator in the service
// constructor.
type stubFormularioCache struct {
	formularioCalls []uuid.UUID
	eventoCalls     []struct{ Empresa, Cliente uuid.UUID }
	respuestaCalls  []uuid.UUID
	formularioErr   error
	respuestaErr    error
}

func (s *stubFormularioCache) UnlinkFormulariosByEmpresa(_ context.Context, empresaID uuid.UUID) error {
	s.formularioCalls = append(s.formularioCalls, empresaID)
	return s.formularioErr
}
func (s *stubFormularioCache) UnlinkEventosByEmpCte(_ context.Context, empresaID, clienteID uuid.UUID) error {
	s.eventoCalls = append(s.eventoCalls, struct{ Empresa, Cliente uuid.UUID }{empresaID, clienteID})
	return nil
}
func (s *stubFormularioCache) UnlinkRespuestasByIniciado(_ context.Context, iniciadoID uuid.UUID) error {
	s.respuestaCalls = append(s.respuestaCalls, iniciadoID)
	return s.respuestaErr
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

func newFormularioFixture(t *testing.T) (*repository.InMemFormularioRepository, *repository.InMemPreguntaRepository, *recordingPublisher, *stubFormularioCache) {
	t.Helper()
	return repository.NewInMemFormularioRepository(),
		repository.NewInMemPreguntaRepository(),
		&recordingPublisher{},
		&stubFormularioCache{}
}

// ---------------------------------------------------------------------------
// CreateFormulario — happy path (Gherkin 1).
// ---------------------------------------------------------------------------

func TestFormularioService_CreateFormulario_PersistsAndPublishesAndInvalidates(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()
	ownerID := uuid.New()
	f := &formularios.Formulario{
		ID:          ownerID,
		EmpresaID:   empresaID,
		Nombre:      "Control Higienico",
		Descripcion: "Limpieza diaria",
		Activo:      true,
	}

	created, err := svc.CreateFormulario(context.Background(), f, empresaID)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "Control Higienico", created.Nombre)
	assert.Equal(t, empresaID, created.EmpresaID)
	assert.True(t, created.Activo)
	assert.Equal(t, ownerID, created.ID) // service must not overwrite a caller-supplied ID

	// Cache: KeyFormularioByEmpresa must have been invalidated.
	assert.Equal(t, []uuid.UUID{empresaID}, cache.formularioCalls)

	// Publisher: StreamFormularioCreado with the spec payload.
	require.Len(t, pub.events, 1)
	assert.Equal(t, events.StreamFormularioCreado, pub.events[0].stream)
	assert.Equal(t, ownerID, pub.events[0].payload["formulario_id"])
	assert.Equal(t, empresaID, pub.events[0].payload["empresa_id"])
	assert.Equal(t, "Control Higienico", pub.events[0].payload["nombre"])
}

// ---------------------------------------------------------------------------
// CreateFormulario — validation: nombre required.
// ---------------------------------------------------------------------------

func TestFormularioService_CreateFormulario_RejectsEmptyNombre(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()
	f := &formularios.Formulario{EmpresaID: empresaID, Nombre: "", Activo: true}

	_, err := svc.CreateFormulario(context.Background(), f, empresaID)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
	assert.Empty(t, pub.events, "must not publish on validation failure")
	assert.Empty(t, cache.formularioCalls, "must not invalidate cache on validation failure")
}

// ---------------------------------------------------------------------------
// CreateFormulario — validation: nombre length cap (mirrors openapi maxLength: 150).
// ---------------------------------------------------------------------------

func TestFormularioService_CreateFormulario_RejectsNombreOverLimit(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()
	long := make([]byte, 151)
	for i := range long {
		long[i] = 'a'
	}
	f := &formularios.Formulario{EmpresaID: empresaID, Nombre: string(long), Activo: true}

	_, err := svc.CreateFormulario(context.Background(), f, empresaID)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// CreateFormulario — tenant check: body EmpresaID must match the caller's
// JWT-empresaID (passed in as empresaID arg).
// ---------------------------------------------------------------------------

func TestFormularioService_CreateFormulario_RejectsTenantMismatch(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	body := uuid.New()
	caller := uuid.New() // different from body
	f := &formularios.Formulario{EmpresaID: body, Nombre: "X", Activo: true}

	_, err := svc.CreateFormulario(context.Background(), f, caller)
	require.ErrorIs(t, err, formularios.ErrForbidden)
	assert.Empty(t, pub.events)
	assert.Empty(t, cache.formularioCalls)
}

// ---------------------------------------------------------------------------
// GetFormulariosByEmpresa — pure delegation. The cache invalidator is
// NOT touched on reads (only on writes).
// ---------------------------------------------------------------------------

func TestFormularioService_GetFormulariosByEmpresa_DelegatesAndDoesNotInvalidate(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()
	for i := 0; i < 3; i++ {
		require.NoError(t, formRepo.Create(context.Background(), &formularios.Formulario{
			ID: uuid.New(), EmpresaID: empresaID, Nombre: "F", Activo: true,
		}))
	}

	got, total, err := svc.GetFormulariosByEmpresa(context.Background(), empresaID, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, got, 3)
	assert.Empty(t, cache.formularioCalls, "reads must not invalidate the cache")
	assert.Empty(t, pub.events, "reads must not publish events")
}

// ---------------------------------------------------------------------------
// AddPregunta — happy path: parent formulario owned by caller.
// ---------------------------------------------------------------------------

func TestFormularioService_AddPregunta_PersistsAndDoesNotInvalidate(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()
	formID := uuid.New()
	require.NoError(t, formRepo.Create(context.Background(), &formularios.Formulario{
		ID: formID, EmpresaID: empresaID, Nombre: "F", Activo: true,
	}))

	p := &formularios.Pregunta{
		FormularioID:  formID,
		Orden:         1,
		TipoPregunta:  formularios.TipoPreguntaMatriz,
		TextoPregunta: "Limpieza",
		Obligatoria:   true,
	}

	created, err := svc.AddPregunta(context.Background(), p, empresaID)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, formID, created.FormularioID)
	assert.Empty(t, pub.events, "AddPregunta does not publish — only CreateFormulario does")
	assert.Empty(t, cache.formularioCalls, "AddPregunta does not invalidate — no cached list keyed by pregunta")
}

// ---------------------------------------------------------------------------
// AddPregunta — IDOR: parent formulario belongs to a different empresa.
// ---------------------------------------------------------------------------

func TestFormularioService_AddPregunta_RejectsForeignParent(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	owner := uuid.New()
	intruder := uuid.New()
	formID := uuid.New()
	require.NoError(t, formRepo.Create(context.Background(), &formularios.Formulario{
		ID: formID, EmpresaID: owner, Nombre: "F", Activo: true,
	}))

	p := &formularios.Pregunta{
		FormularioID:  formID,
		Orden:         1,
		TipoPregunta:  formularios.TipoPreguntaTexto,
		TextoPregunta: "Algo",
		Obligatoria:   false,
	}

	_, err := svc.AddPregunta(context.Background(), p, intruder)
	require.ErrorIs(t, err, formularios.ErrNotFound, "IDOR violation must look like a not-found (no leak)")
}

// ---------------------------------------------------------------------------
// AddPregunta — validation: texto_pregunta must not be empty.
// ---------------------------------------------------------------------------

func TestFormularioService_AddPregunta_RejectsEmptyTexto(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()
	formID := uuid.New()
	require.NoError(t, formRepo.Create(context.Background(), &formularios.Formulario{
		ID: formID, EmpresaID: empresaID, Nombre: "F", Activo: true,
	}))

	p := &formularios.Pregunta{FormularioID: formID, Orden: 1, TipoPregunta: formularios.TipoPreguntaTexto, Obligatoria: true}
	_, err := svc.AddPregunta(context.Background(), p, empresaID)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// AddPregunta — repo validation propagates: invalid tipo_pregunta.
// ---------------------------------------------------------------------------

func TestFormularioService_AddPregunta_RepoInvalidTipoPropagates(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()
	formID := uuid.New()
	require.NoError(t, formRepo.Create(context.Background(), &formularios.Formulario{
		ID: formID, EmpresaID: empresaID, Nombre: "F", Activo: true,
	}))

	p := &formularios.Pregunta{
		FormularioID:  formID,
		Orden:         1,
		TipoPregunta:  99, // outside the SQL CHECK set
		TextoPregunta: "Algo",
		Obligatoria:   false,
	}
	_, err := svc.AddPregunta(context.Background(), p, empresaID)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// Cache invalidator error must NOT propagate (fire-and-forget).
// ---------------------------------------------------------------------------

func TestFormularioService_CreateFormulario_SwallowsCacheErrors(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	cache.formularioErr = errors.New("redis down")
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()
	f := &formularios.Formulario{ID: uuid.New(), EmpresaID: empresaID, Nombre: "X", Activo: true}

	_, err := svc.CreateFormulario(context.Background(), f, empresaID)
	require.NoError(t, err, "cache error must not fail the write")
	assert.Equal(t, []uuid.UUID{empresaID}, cache.formularioCalls)
}

// ---------------------------------------------------------------------------
// No-PII regression (PR-3 task 3.5 audit). Every error message across the
// 4 services must NOT include user-supplied strings (nombre,
// descripcion, texto_pregunta, evidencia URLs, file paths, geo coords).
// ---------------------------------------------------------------------------

func TestFormularioService_NoPIIInErrorMessages(t *testing.T) {
	formRepo, preguntaRepo, pub, cache := newFormularioFixture(t)
	svc := service.NewFormularioService(formRepo, preguntaRepo, pub, cache)

	empresaID := uuid.New()

	// CreateFormulario: a nombre over the 150-char cap with PII-shaped
	// content must not appear in the error message (the error is
	// ErrInvalidInput with a generic message; the offending value is
	// intentionally NOT echoed).
	piiNombre := strings.Repeat("Cliente VIP-DNI-4111-1111-1111-1111-", 6) // 40*6 = 240 chars
	f := &formularios.Formulario{EmpresaID: empresaID, Nombre: piiNombre, Activo: true}
	_, err := svc.CreateFormulario(context.Background(), f, empresaID)
	require.ErrorIs(t, err, formularios.ErrInvalidInput)
	assert.NotContains(t, err.Error(), piiNombre, "nombre must not be echoed in the error")
	assert.NotContains(t, err.Error(), "4111-1111", "PII must not be echoed in the error")

	// Tenant mismatch: the body EmpresaID (a UUID) is not PII, but the
	// test asserts the service returns ErrForbidden with a generic
	// message.
	body := uuid.New()
	_, err = svc.CreateFormulario(context.Background(), &formularios.Formulario{
		EmpresaID: body, Nombre: "X", Activo: true,
	}, uuid.New())
	require.ErrorIs(t, err, formularios.ErrForbidden)
	assert.Equal(t, formularios.ErrForbidden.Error(), err.Error(), "no extra context leaked")

	// AddPregunta: texto_pregunta with PII content must not leak via
	// the published payload (AddPregunta does not publish, so this is
	// a negative assertion on the events list).
	formID := uuid.New()
	require.NoError(t, formRepo.Create(context.Background(), &formularios.Formulario{
		ID: formID, EmpresaID: empresaID, Nombre: "F", Activo: true,
	}))
	piiTexto := "Confidencial: paciente John Doe, DNI 12345678"
	_, err = svc.AddPregunta(context.Background(), &formularios.Pregunta{
		FormularioID: formID, Orden: 1, TipoPregunta: formularios.TipoPreguntaTexto,
		TextoPregunta: piiTexto, Obligatoria: false,
	}, empresaID)
	require.NoError(t, err, "texto_pregunta is non-empty so it must succeed")
	assert.Empty(t, pub.events, "AddPregunta must not publish — and must NOT include texto_pregunta in any payload")
}
