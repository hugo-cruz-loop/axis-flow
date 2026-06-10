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

// PostulacionRepositorier is the interface under test.
type PostulacionRepositorier interface {
	Create(ctx context.Context, p *bolsatrabajo.Postulacion) error
	GetStatsByTrabajo(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error)
	UpdateEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error)
}

// mockPostulacionRepo is an in-memory implementation for unit tests.
type mockPostulacionRepo struct {
	store    map[uuid.UUID]*bolsatrabajo.Postulacion
	trabajos map[uuid.UUID]*bolsatrabajo.Trabajo // needed for tenant check
}

func newMockPostulacionRepo() *mockPostulacionRepo {
	return &mockPostulacionRepo{
		store:    make(map[uuid.UUID]*bolsatrabajo.Postulacion),
		trabajos: make(map[uuid.UUID]*bolsatrabajo.Trabajo),
	}
}

func (m *mockPostulacionRepo) seedTrabajo(t *bolsatrabajo.Trabajo) {
	m.trabajos[t.ID] = t
}

func (m *mockPostulacionRepo) Create(_ context.Context, p *bolsatrabajo.Postulacion) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	cp := *p
	m.store[p.ID] = &cp
	return nil
}

func (m *mockPostulacionRepo) GetStatsByTrabajo(_ context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error) {
	// Tenant check
	t, ok := m.trabajos[trabajoID]
	if !ok || t.EmpresaID != empresaID {
		return nil, bolsatrabajo.ErrForbidden
	}

	counts := map[int]int{}
	for _, p := range m.store {
		if p.TrabajoID == trabajoID {
			counts[p.Estatus]++
		}
	}

	stageNames := map[int]string{
		bolsatrabajo.PostulacionPendiente:  "Pendiente",
		bolsatrabajo.PostulacionEntrevista: "Entrevista",
		bolsatrabajo.PostulacionOferta:     "Oferta",
		bolsatrabajo.PostulacionContratado: "Contratado",
		bolsatrabajo.PostulacionRechazado:  "Rechazado",
	}

	stages := make([]bolsatrabajo.StageStat, 0, 5)
	for _, stage := range []int{
		bolsatrabajo.PostulacionPendiente,
		bolsatrabajo.PostulacionEntrevista,
		bolsatrabajo.PostulacionOferta,
		bolsatrabajo.PostulacionContratado,
		bolsatrabajo.PostulacionRechazado,
	} {
		stages = append(stages, bolsatrabajo.StageStat{
			Stage:     stage,
			StageName: stageNames[stage],
			Count:     counts[stage],
		})
	}

	return &bolsatrabajo.PipelineStats{TrabajoID: trabajoID, Stats: stages}, nil
}

func (m *mockPostulacionRepo) UpdateEstatus(_ context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error) {
	p, ok := m.store[id]
	if !ok {
		return nil, bolsatrabajo.ErrNotFound
	}
	// Tenant check via trabajo
	t, tOk := m.trabajos[p.TrabajoID]
	if !tOk || t.EmpresaID != empresaID {
		return nil, bolsatrabajo.ErrForbidden
	}
	p.Estatus = newEstatus
	p.UpdatedAt = time.Now()
	cp := *p
	return &cp, nil
}

// ---- Tests ----

func TestPostulacionRepo_Create_AssignsIDAndTimestamps(t *testing.T) {
	repo := newMockPostulacionRepo()
	trabajoID := uuid.New()
	p := &bolsatrabajo.Postulacion{
		TrabajoID:      trabajoID,
		NombreCompleto: "Jane Doe",
		Email:          "jane@example.com",
		Estatus:        bolsatrabajo.PostulacionPendiente,
	}
	require.NoError(t, repo.Create(context.Background(), p))
	assert.NotEqual(t, uuid.Nil, p.ID)
	assert.False(t, p.CreatedAt.IsZero())
}

func TestPostulacionRepo_GetStatsByTrabajo_ReturnsAllFiveStages(t *testing.T) {
	repo := newMockPostulacionRepo()
	empresaID := uuid.New()
	trabajoID := uuid.New()

	// Seed the trabajo for tenant check
	repo.seedTrabajo(&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: empresaID})

	// Add one application in Pendiente only — others should be 0
	require.NoError(t, repo.Create(context.Background(), &bolsatrabajo.Postulacion{
		TrabajoID: trabajoID,
		Estatus:   bolsatrabajo.PostulacionPendiente,
	}))

	stats, err := repo.GetStatsByTrabajo(context.Background(), trabajoID, empresaID)
	require.NoError(t, err)
	assert.Equal(t, trabajoID, stats.TrabajoID)
	assert.Len(t, stats.Stats, 5, "must return all 5 stages even if some have 0 count")

	stageMap := make(map[int]int)
	for _, s := range stats.Stats {
		stageMap[s.Stage] = s.Count
	}
	assert.Equal(t, 1, stageMap[bolsatrabajo.PostulacionPendiente])
	assert.Equal(t, 0, stageMap[bolsatrabajo.PostulacionEntrevista])
	assert.Equal(t, 0, stageMap[bolsatrabajo.PostulacionOferta])
	assert.Equal(t, 0, stageMap[bolsatrabajo.PostulacionContratado])
	assert.Equal(t, 0, stageMap[bolsatrabajo.PostulacionRechazado])
}

func TestPostulacionRepo_GetStatsByTrabajo_TenantMismatch_ReturnsErrForbidden(t *testing.T) {
	repo := newMockPostulacionRepo()
	trabajoID := uuid.New()
	ownerID := uuid.New()
	otherID := uuid.New()

	repo.seedTrabajo(&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: ownerID})

	_, err := repo.GetStatsByTrabajo(context.Background(), trabajoID, otherID)
	assert.ErrorIs(t, err, bolsatrabajo.ErrForbidden)
}

func TestPostulacionRepo_UpdateEstatus_HappyPath(t *testing.T) {
	repo := newMockPostulacionRepo()
	empresaID := uuid.New()
	trabajoID := uuid.New()

	repo.seedTrabajo(&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: empresaID})

	p := &bolsatrabajo.Postulacion{TrabajoID: trabajoID, Estatus: bolsatrabajo.PostulacionPendiente}
	require.NoError(t, repo.Create(context.Background(), p))

	updated, err := repo.UpdateEstatus(context.Background(), p.ID, empresaID, bolsatrabajo.PostulacionEntrevista)
	require.NoError(t, err)
	assert.Equal(t, bolsatrabajo.PostulacionEntrevista, updated.Estatus)
}

func TestPostulacionRepo_UpdateEstatus_TenantMismatch_ReturnsErrForbidden(t *testing.T) {
	repo := newMockPostulacionRepo()
	ownerID := uuid.New()
	otherID := uuid.New()
	trabajoID := uuid.New()

	repo.seedTrabajo(&bolsatrabajo.Trabajo{ID: trabajoID, EmpresaID: ownerID})

	p := &bolsatrabajo.Postulacion{TrabajoID: trabajoID, Estatus: bolsatrabajo.PostulacionPendiente}
	require.NoError(t, repo.Create(context.Background(), p))

	_, err := repo.UpdateEstatus(context.Background(), p.ID, otherID, bolsatrabajo.PostulacionEntrevista)
	assert.ErrorIs(t, err, bolsatrabajo.ErrForbidden)
}
