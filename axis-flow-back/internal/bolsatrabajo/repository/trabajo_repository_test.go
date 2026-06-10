package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"axis-flow-back/internal/bolsatrabajo"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TrabajoRepositorier is the interface under test (mirrors trabajo_repository.go).
type TrabajoRepositorier interface {
	Create(ctx context.Context, t *bolsatrabajo.Trabajo) error
	GetActiveJobs(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	GetByEmpresa(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	GetRecent(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	SwitchEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error)
}

// mockTrabajoRepo is an in-memory implementation for unit tests.
type mockTrabajoRepo struct {
	store map[uuid.UUID]*bolsatrabajo.Trabajo
}

func newMockTrabajoRepo() *mockTrabajoRepo {
	return &mockTrabajoRepo{store: make(map[uuid.UUID]*bolsatrabajo.Trabajo)}
}

func (m *mockTrabajoRepo) Create(_ context.Context, t *bolsatrabajo.Trabajo) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	cp := *t
	m.store[t.ID] = &cp
	return nil
}

func (m *mockTrabajoRepo) GetActiveJobs(_ context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	now := time.Now()
	var results []*bolsatrabajo.Trabajo
	for _, t := range m.store {
		if t.EstatusVacante != bolsatrabajo.VacanteActivo {
			continue
		}
		if t.FechaCaducar.Before(now) {
			continue
		}
		if empresaID != nil && t.EmpresaID != *empresaID {
			continue
		}
		if search != "" {
			lower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(t.Titulo), lower) &&
				!strings.Contains(strings.ToLower(t.Descripcion), lower) {
				continue
			}
		}
		cp := *t
		results = append(results, &cp)
	}
	total := len(results)
	start := (page - 1) * pageSize
	if start >= total {
		return []*bolsatrabajo.Trabajo{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return results[start:end], total, nil
}

func (m *mockTrabajoRepo) GetByEmpresa(_ context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	var results []*bolsatrabajo.Trabajo
	for _, t := range m.store {
		if t.EmpresaID != empresaID {
			continue
		}
		switch filter {
		case 1:
			if t.EstatusVacante != bolsatrabajo.VacanteActivo {
				continue
			}
		case 2:
			if t.EstatusVacante != bolsatrabajo.VacantePausa {
				continue
			}
		}
		cp := *t
		results = append(results, &cp)
	}
	total := len(results)
	start := (page - 1) * pageSize
	if start >= total {
		return []*bolsatrabajo.Trabajo{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return results[start:end], total, nil
}

func (m *mockTrabajoRepo) GetRecent(_ context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var results []*bolsatrabajo.Trabajo
	for _, t := range m.store {
		if t.CreatedAt.After(cutoff) {
			cp := *t
			results = append(results, &cp)
		}
	}
	total := len(results)
	start := (page - 1) * pageSize
	if start >= total {
		return []*bolsatrabajo.Trabajo{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return results[start:end], total, nil
}

func (m *mockTrabajoRepo) SwitchEstatus(_ context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error) {
	t, ok := m.store[id]
	if !ok {
		return nil, bolsatrabajo.ErrNotFound
	}
	if t.EmpresaID != empresaID {
		return nil, bolsatrabajo.ErrForbidden
	}
	t.EstatusVacante = newEstatus
	t.UpdatedAt = time.Now()
	cp := *t
	return &cp, nil
}

// ---- Tests ----

func TestTrabajoRepo_Create_AssignsIDAndTimestamps(t *testing.T) {
	repo := newMockTrabajoRepo()
	empresaID := uuid.New()
	job := &bolsatrabajo.Trabajo{
		EmpresaID:      empresaID,
		Titulo:         "Go Developer",
		Descripcion:    "Build APIs",
		FechaCaducar:   time.Now().Add(30 * 24 * time.Hour),
		EstatusVacante: bolsatrabajo.VacanteActivo,
	}
	require.NoError(t, repo.Create(context.Background(), job))
	assert.NotEqual(t, uuid.Nil, job.ID)
	assert.False(t, job.CreatedAt.IsZero())
}

func TestTrabajoRepo_GetActiveJobs_TextSearchHitsTitulo(t *testing.T) {
	repo := newMockTrabajoRepo()
	empresaID := uuid.New()

	require.NoError(t, repo.Create(context.Background(), &bolsatrabajo.Trabajo{
		EmpresaID:      empresaID,
		Titulo:         "Senior Golang Engineer",
		Descripcion:    "Build microservices",
		FechaCaducar:   time.Now().Add(30 * 24 * time.Hour),
		EstatusVacante: bolsatrabajo.VacanteActivo,
	}))
	require.NoError(t, repo.Create(context.Background(), &bolsatrabajo.Trabajo{
		EmpresaID:      empresaID,
		Titulo:         "Frontend React Developer",
		Descripcion:    "Build UIs",
		FechaCaducar:   time.Now().Add(30 * 24 * time.Hour),
		EstatusVacante: bolsatrabajo.VacanteActivo,
	}))

	results, total, err := repo.GetActiveJobs(context.Background(), "golang", nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, results, 1)
	assert.Contains(t, strings.ToLower(results[0].Titulo), "golang")
}

func TestTrabajoRepo_GetActiveJobs_ExpiredJobsExcluded(t *testing.T) {
	repo := newMockTrabajoRepo()
	empresaID := uuid.New()

	// Create an expired job by inserting then manually updating FechaCaducar
	job := &bolsatrabajo.Trabajo{
		EmpresaID:      empresaID,
		Titulo:         "Expired Role",
		Descripcion:    "Old job",
		FechaCaducar:   time.Now().Add(30 * 24 * time.Hour),
		EstatusVacante: bolsatrabajo.VacanteActivo,
	}
	require.NoError(t, repo.Create(context.Background(), job))
	// Back-date it
	repo.store[job.ID].FechaCaducar = time.Now().Add(-1 * time.Hour)

	results, total, err := repo.GetActiveJobs(context.Background(), "", nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, results)
}

func TestTrabajoRepo_SwitchEstatus_EmpresaIDMismatch_ReturnsErrForbidden(t *testing.T) {
	repo := newMockTrabajoRepo()
	ownerID := uuid.New()
	otherID := uuid.New()

	job := &bolsatrabajo.Trabajo{
		EmpresaID:      ownerID,
		Titulo:         "Job",
		FechaCaducar:   time.Now().Add(30 * 24 * time.Hour),
		EstatusVacante: bolsatrabajo.VacanteActivo,
	}
	require.NoError(t, repo.Create(context.Background(), job))

	_, err := repo.SwitchEstatus(context.Background(), job.ID, otherID, bolsatrabajo.VacantePausa)
	assert.ErrorIs(t, err, bolsatrabajo.ErrForbidden)
}

func TestTrabajoRepo_SwitchEstatus_NotFound_ReturnsErrNotFound(t *testing.T) {
	repo := newMockTrabajoRepo()

	_, err := repo.SwitchEstatus(context.Background(), uuid.New(), uuid.New(), bolsatrabajo.VacantePausa)
	assert.ErrorIs(t, err, bolsatrabajo.ErrNotFound)
}

func TestTrabajoRepo_SwitchEstatus_HappyPath(t *testing.T) {
	repo := newMockTrabajoRepo()
	empresaID := uuid.New()

	job := &bolsatrabajo.Trabajo{
		EmpresaID:      empresaID,
		Titulo:         "Job",
		FechaCaducar:   time.Now().Add(30 * 24 * time.Hour),
		EstatusVacante: bolsatrabajo.VacanteActivo,
	}
	require.NoError(t, repo.Create(context.Background(), job))

	updated, err := repo.SwitchEstatus(context.Background(), job.ID, empresaID, bolsatrabajo.VacantePausa)
	require.NoError(t, err)
	assert.Equal(t, bolsatrabajo.VacantePausa, updated.EstatusVacante)
}
