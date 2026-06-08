package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/empresas"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EmpresaRepositorier is the interface under test.
type EmpresaRepositorier interface {
	Create(ctx context.Context, e *empresas.Empresa) error
	FindByID(ctx context.Context, id int64) (*empresas.Empresa, error)
	Update(ctx context.Context, e *empresas.Empresa) error
	UpdateStatus(ctx context.Context, id int64, status empresas.EmpresaStatus, vigencia *time.Time) error
	Delete(ctx context.Context, id int64) error
	ListAll(ctx context.Context) ([]empresas.Empresa, error)
}

// mockEmpresaRepo is a simple in-memory mock for unit testing.
type mockEmpresaRepo struct {
	store  map[int64]*empresas.Empresa
	nextID int64
}

func newMockEmpresaRepo() *mockEmpresaRepo {
	return &mockEmpresaRepo{store: make(map[int64]*empresas.Empresa), nextID: 1}
}

func (m *mockEmpresaRepo) Create(ctx context.Context, e *empresas.Empresa) error {
	e.ID = m.nextID
	m.nextID++
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	copy := *e
	m.store[e.ID] = &copy
	return nil
}

func (m *mockEmpresaRepo) FindByID(ctx context.Context, id int64) (*empresas.Empresa, error) {
	e, ok := m.store[id]
	if !ok {
		return nil, empresas.ErrEmpresaNotFound
	}
	copy := *e
	return &copy, nil
}

func (m *mockEmpresaRepo) Update(ctx context.Context, e *empresas.Empresa) error {
	if _, ok := m.store[e.ID]; !ok {
		return empresas.ErrEmpresaNotFound
	}
	e.UpdatedAt = time.Now()
	copy := *e
	m.store[e.ID] = &copy
	return nil
}

func (m *mockEmpresaRepo) UpdateStatus(ctx context.Context, id int64, status empresas.EmpresaStatus, vigencia *time.Time) error {
	e, ok := m.store[id]
	if !ok {
		return empresas.ErrEmpresaNotFound
	}
	e.Status = status
	e.Vigencia = vigencia
	e.UpdatedAt = time.Now()
	return nil
}

func (m *mockEmpresaRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.store[id]; !ok {
		return empresas.ErrEmpresaNotFound
	}
	delete(m.store, id)
	return nil
}

func (m *mockEmpresaRepo) ListAll(ctx context.Context) ([]empresas.Empresa, error) {
	out := make([]empresas.Empresa, 0, len(m.store))
	for _, e := range m.store {
		out = append(out, *e)
	}
	return out, nil
}

// deleteOrder records the sequence of entity types deleted (for cascade verification).
type mockCascadeRepo struct {
	mockEmpresaRepo
	deleteOrder []string
}

func newMockCascadeRepo() *mockCascadeRepo {
	return &mockCascadeRepo{
		mockEmpresaRepo: mockEmpresaRepo{store: make(map[int64]*empresas.Empresa), nextID: 1},
	}
}

func (m *mockCascadeRepo) Delete(ctx context.Context, id int64) error {
	// Simulate programmatic cascade order
	m.deleteOrder = append(m.deleteOrder,
		"servicios", "apoderados", "datosfiscales", "pagos", "empresa",
	)
	delete(m.mockEmpresaRepo.store, id)
	return nil
}

// ---- Tests ----

func TestEmpresaRepo_Create_SetsIDOnSuccess(t *testing.T) {
	repo := newMockEmpresaRepo()
	e := &empresas.Empresa{
		Nombre:          "ACME Corp",
		RepresentanteID: uuid.New(),
		PlanID:          1,
		Status:          empresas.EmpresaStatusPendingPayment,
	}

	err := repo.Create(context.Background(), e)
	require.NoError(t, err)
	assert.Greater(t, e.ID, int64(0))
	assert.False(t, e.CreatedAt.IsZero())
	assert.False(t, e.UpdatedAt.IsZero())
}

func TestEmpresaRepo_FindByID_WithNonExistentID_ReturnsErrNotFound(t *testing.T) {
	repo := newMockEmpresaRepo()

	_, err := repo.FindByID(context.Background(), 9999)
	assert.ErrorIs(t, err, empresas.ErrEmpresaNotFound)
}

func TestEmpresaRepo_UpdateStatus_ChangesToActive(t *testing.T) {
	repo := newMockEmpresaRepo()
	e := &empresas.Empresa{
		Nombre:          "Test SA",
		RepresentanteID: uuid.New(),
		PlanID:          1,
		Status:          empresas.EmpresaStatusPendingPayment,
	}
	require.NoError(t, repo.Create(context.Background(), e))

	vigencia := time.Now().AddDate(1, 0, 0)
	err := repo.UpdateStatus(context.Background(), e.ID, empresas.EmpresaStatusActive, &vigencia)
	require.NoError(t, err)

	got, err := repo.FindByID(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, empresas.EmpresaStatusActive, got.Status)
	assert.NotNil(t, got.Vigencia)
}

func TestEmpresaRepo_Delete_CascadesInOrder(t *testing.T) {
	repo := newMockCascadeRepo()
	e := &empresas.Empresa{
		Nombre:          "Cascade Corp",
		RepresentanteID: uuid.New(),
		PlanID:          1,
		Status:          empresas.EmpresaStatusActive,
	}
	require.NoError(t, repo.Create(context.Background(), e))

	err := repo.Delete(context.Background(), e.ID)
	require.NoError(t, err)

	expectedOrder := []string{"servicios", "apoderados", "datosfiscales", "pagos", "empresa"}
	assert.Equal(t, expectedOrder, repo.deleteOrder)

	_, findErr := repo.FindByID(context.Background(), e.ID)
	assert.ErrorIs(t, findErr, empresas.ErrEmpresaNotFound)
}
