package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/bolsatrabajo"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EvaluacionRepositorier is the interface under test.
type EvaluacionRepositorier interface {
	Create(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) error
	GetByPostulacion(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
}

// mockEvaluacionRepo is an in-memory implementation for unit tests.
type mockEvaluacionRepo struct {
	store         map[uuid.UUID]*bolsatrabajo.Evaluacion // key: evaluacion.ID
	byPostulacion map[uuid.UUID]uuid.UUID                // postulacionID → evaluacionID
	postulaciones map[uuid.UUID]*bolsatrabajo.Postulacion
	trabajos      map[uuid.UUID]*bolsatrabajo.Trabajo
}

func newMockEvaluacionRepo() *mockEvaluacionRepo {
	return &mockEvaluacionRepo{
		store:         make(map[uuid.UUID]*bolsatrabajo.Evaluacion),
		byPostulacion: make(map[uuid.UUID]uuid.UUID),
		postulaciones: make(map[uuid.UUID]*bolsatrabajo.Postulacion),
		trabajos:      make(map[uuid.UUID]*bolsatrabajo.Trabajo),
	}
}

func (m *mockEvaluacionRepo) seedPostulacion(p *bolsatrabajo.Postulacion, t *bolsatrabajo.Trabajo) {
	m.trabajos[t.ID] = t
	m.postulaciones[p.ID] = p
}

func (m *mockEvaluacionRepo) Create(_ context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) error {
	// Tenant check: postulacion must exist and belong to a trabajo owned by empresaID
	p, ok := m.postulaciones[e.PostulacionID]
	if !ok {
		return bolsatrabajo.ErrForbidden
	}
	t, ok := m.trabajos[p.TrabajoID]
	if !ok || t.EmpresaID != empresaID {
		return bolsatrabajo.ErrForbidden
	}

	// Unique constraint check (one evaluacion per postulacion)
	if _, exists := m.byPostulacion[e.PostulacionID]; exists {
		return bolsatrabajo.ErrConflict
	}

	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	e.CreatedAt = time.Now()
	cp := *e
	m.store[e.ID] = &cp
	m.byPostulacion[e.PostulacionID] = e.ID
	return nil
}

func (m *mockEvaluacionRepo) GetByPostulacion(_ context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	// Triple-join tenant check
	p, ok := m.postulaciones[postulacionID]
	if !ok {
		return nil, bolsatrabajo.ErrNotFound
	}
	t, ok := m.trabajos[p.TrabajoID]
	if !ok || t.EmpresaID != empresaID {
		return nil, bolsatrabajo.ErrForbidden
	}

	evID, ok := m.byPostulacion[postulacionID]
	if !ok {
		return nil, bolsatrabajo.ErrNotFound
	}
	cp := *m.store[evID]
	return &cp, nil
}

// ---- Tests ----

func TestEvaluacionRepo_Create_HappyPath(t *testing.T) {
	repo := newMockEvaluacionRepo()
	empresaID := uuid.New()
	trabajoID := uuid.New()
	postulacionID := uuid.New()

	repo.seedPostulacion(
		&bolsatrabajo.Postulacion{ID: postulacionID, TrabajoID: trabajoID, Estatus: bolsatrabajo.PostulacionEntrevista},
		&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: empresaID},
	)

	ev := &bolsatrabajo.Evaluacion{
		PostulacionID: postulacionID,
		Puntualidad:   4,
		Cortesia:      5,
		SoftSkills:    3,
		Comentarios:   "Good candidate",
		EvaluatorID:   uuid.New(),
	}
	require.NoError(t, repo.Create(context.Background(), ev, empresaID))
	assert.NotEqual(t, uuid.Nil, ev.ID)
}

func TestEvaluacionRepo_Create_EmpresaIDMismatch_ReturnsErrForbidden(t *testing.T) {
	repo := newMockEvaluacionRepo()
	ownerID := uuid.New()
	otherID := uuid.New()
	trabajoID := uuid.New()
	postulacionID := uuid.New()

	repo.seedPostulacion(
		&bolsatrabajo.Postulacion{ID: postulacionID, TrabajoID: trabajoID},
		&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: ownerID},
	)

	ev := &bolsatrabajo.Evaluacion{PostulacionID: postulacionID}
	err := repo.Create(context.Background(), ev, otherID)
	assert.ErrorIs(t, err, bolsatrabajo.ErrForbidden)
}

func TestEvaluacionRepo_Create_DuplicatePostulacion_ReturnsErrConflict(t *testing.T) {
	repo := newMockEvaluacionRepo()
	empresaID := uuid.New()
	trabajoID := uuid.New()
	postulacionID := uuid.New()

	repo.seedPostulacion(
		&bolsatrabajo.Postulacion{ID: postulacionID, TrabajoID: trabajoID},
		&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: empresaID},
	)

	ev1 := &bolsatrabajo.Evaluacion{PostulacionID: postulacionID, EvaluatorID: uuid.New()}
	require.NoError(t, repo.Create(context.Background(), ev1, empresaID))

	ev2 := &bolsatrabajo.Evaluacion{PostulacionID: postulacionID, EvaluatorID: uuid.New()}
	err := repo.Create(context.Background(), ev2, empresaID)
	assert.ErrorIs(t, err, bolsatrabajo.ErrConflict)
}

func TestEvaluacionRepo_GetByPostulacion_HappyPath(t *testing.T) {
	repo := newMockEvaluacionRepo()
	empresaID := uuid.New()
	trabajoID := uuid.New()
	postulacionID := uuid.New()

	repo.seedPostulacion(
		&bolsatrabajo.Postulacion{ID: postulacionID, TrabajoID: trabajoID},
		&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: empresaID},
	)

	ev := &bolsatrabajo.Evaluacion{PostulacionID: postulacionID, Comentarios: "Great", EvaluatorID: uuid.New()}
	require.NoError(t, repo.Create(context.Background(), ev, empresaID))

	got, err := repo.GetByPostulacion(context.Background(), postulacionID, empresaID)
	require.NoError(t, err)
	assert.Equal(t, "Great", got.Comentarios)
}

func TestEvaluacionRepo_GetByPostulacion_TenantMismatch_ReturnsErrForbidden(t *testing.T) {
	repo := newMockEvaluacionRepo()
	ownerID := uuid.New()
	otherID := uuid.New()
	trabajoID := uuid.New()
	postulacionID := uuid.New()

	repo.seedPostulacion(
		&bolsatrabajo.Postulacion{ID: postulacionID, TrabajoID: trabajoID},
		&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: ownerID},
	)

	_, err := repo.GetByPostulacion(context.Background(), postulacionID, otherID)
	assert.ErrorIs(t, err, bolsatrabajo.ErrForbidden)
}
