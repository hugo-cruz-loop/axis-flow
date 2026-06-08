package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// EvaluacionRepository interface
// ---------------------------------------------------------------------------

type EvaluacionRepositorier interface {
	Create(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error
	ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error)
}

// ---------------------------------------------------------------------------
// In-memory mock
// ---------------------------------------------------------------------------

type mockEvaluacionRepo struct {
	records          []*clientes.EvaluacionServicio
	empresaByCliente map[string]int64
}

func newMockEvaluacionRepo() *mockEvaluacionRepo {
	return &mockEvaluacionRepo{
		empresaByCliente: make(map[string]int64),
	}
}

func (m *mockEvaluacionRepo) registerCliente(clienteID uuid.UUID, empresaID int64) {
	m.empresaByCliente[clienteID.String()] = empresaID
}

func (m *mockEvaluacionRepo) Create(_ context.Context, e *clientes.EvaluacionServicio, empresaID int64) error {
	eid, ok := m.empresaByCliente[e.ClienteID.String()]
	if !ok || eid != empresaID {
		return clientes.ErrClienteNotFound
	}
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	cp := *e
	m.records = append(m.records, &cp)
	return nil
}

func (m *mockEvaluacionRepo) ListByCliente(_ context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error) {
	eid, ok := m.empresaByCliente[clienteID.String()]
	if !ok || eid != empresaID {
		return nil, nil
	}
	var out []clientes.EvaluacionServicio
	for _, e := range m.records {
		if e.ClienteID == clienteID {
			out = append(out, *e)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestEvaluacionRepo_Create_HappyPath(t *testing.T) {
	repo := newMockEvaluacionRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	puntuacion := 5
	e := &clientes.EvaluacionServicio{
		ClienteID:   clienteID,
		Puntuacion:  puntuacion,
		DeUsuarioID: uuid.New(),
		Fecha:       time.Now(),
	}
	err := repo.Create(context.Background(), e, 1)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e.ID)
}

func TestEvaluacionRepo_Create_TenantIsolation(t *testing.T) {
	repo := newMockEvaluacionRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	e := &clientes.EvaluacionServicio{
		ClienteID:   clienteID,
		Puntuacion:  3,
		DeUsuarioID: uuid.New(),
		Fecha:       time.Now(),
	}
	// Wrong empresa
	err := repo.Create(context.Background(), e, 999)
	require.Error(t, err)
}

func TestEvaluacionRepo_ListByCliente_ReturnsCorrectEvaluaciones(t *testing.T) {
	repo := newMockEvaluacionRepo()
	clienteID := uuid.New()
	otherClienteID := uuid.New()
	repo.registerCliente(clienteID, 1)
	repo.registerCliente(otherClienteID, 1)

	for i := 0; i < 3; i++ {
		e := &clientes.EvaluacionServicio{
			ClienteID:   clienteID,
			Puntuacion:  i + 1,
			DeUsuarioID: uuid.New(),
			Fecha:       time.Now(),
		}
		require.NoError(t, repo.Create(context.Background(), e, 1))
	}
	// Evaluacion for a different cliente
	other := &clientes.EvaluacionServicio{
		ClienteID:   otherClienteID,
		Puntuacion:  4,
		DeUsuarioID: uuid.New(),
		Fecha:       time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), other, 1))

	list, err := repo.ListByCliente(context.Background(), clienteID, 1)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

// Compile-time interface check
var _ EvaluacionRepositorier = (*mockEvaluacionRepo)(nil)
