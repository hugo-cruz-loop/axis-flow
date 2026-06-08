package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// LocalidadRepository interface
// ---------------------------------------------------------------------------

type LocalidadRepositorier interface {
	Create(ctx context.Context, l *clientes.Localidad) error
	FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error)
	ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error)
	Update(ctx context.Context, l *clientes.Localidad) error
	Delete(ctx context.Context, id uuid.UUID, empresaID int64) error
}

// ---------------------------------------------------------------------------
// In-memory mock
// ---------------------------------------------------------------------------

type mockLocalidadRepo struct {
	records          map[string]*clientes.Localidad
	empresaByCliente map[string]int64
}

func newMockLocalidadRepo() *mockLocalidadRepo {
	return &mockLocalidadRepo{
		records:          make(map[string]*clientes.Localidad),
		empresaByCliente: make(map[string]int64),
	}
}

func (m *mockLocalidadRepo) registerCliente(clienteID uuid.UUID, empresaID int64) {
	m.empresaByCliente[clienteID.String()] = empresaID
}

func (m *mockLocalidadRepo) resolveEmpresa(clienteID uuid.UUID) (int64, bool) {
	eid, ok := m.empresaByCliente[clienteID.String()]
	return eid, ok
}

func (m *mockLocalidadRepo) Create(_ context.Context, l *clientes.Localidad) error {
	l.ID = uuid.New()
	l.CreatedAt = time.Now()
	l.UpdatedAt = time.Now()
	cp := *l
	m.records[l.ID.String()] = &cp
	return nil
}

func (m *mockLocalidadRepo) FindByID(_ context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error) {
	l, ok := m.records[id.String()]
	if !ok {
		return nil, clientes.ErrLocalidadNotFound
	}
	eid, found := m.resolveEmpresa(l.ClienteID)
	if !found || eid != empresaID {
		return nil, clientes.ErrLocalidadNotFound
	}
	cp := *l
	return &cp, nil
}

func (m *mockLocalidadRepo) ListByCliente(_ context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error) {
	eid, found := m.resolveEmpresa(clienteID)
	if !found || eid != empresaID {
		return nil, nil
	}
	var out []clientes.Localidad
	for _, l := range m.records {
		if l.ClienteID == clienteID {
			out = append(out, *l)
		}
	}
	return out, nil
}

func (m *mockLocalidadRepo) Update(_ context.Context, l *clientes.Localidad) error {
	existing, ok := m.records[l.ID.String()]
	if !ok || existing.ClienteID != l.ClienteID {
		return clientes.ErrLocalidadNotFound
	}
	cp := *l
	m.records[l.ID.String()] = &cp
	return nil
}

func (m *mockLocalidadRepo) Delete(_ context.Context, id uuid.UUID, empresaID int64) error {
	l, ok := m.records[id.String()]
	if !ok {
		return clientes.ErrLocalidadNotFound
	}
	eid, found := m.resolveEmpresa(l.ClienteID)
	if !found || eid != empresaID {
		return clientes.ErrLocalidadNotFound
	}
	delete(m.records, id.String())
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLocalidadRepo_Create_HappyPath(t *testing.T) {
	repo := newMockLocalidadRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	l := &clientes.Localidad{
		ClienteID:       clienteID,
		Nombre:          "Sucursal Norte",
		Direccion:       "Av. Norte 100",
		TipoLocalidadID: 1,
	}
	err := repo.Create(context.Background(), l)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, l.ID)
}

func TestLocalidadRepo_FindByID_NotFound(t *testing.T) {
	repo := newMockLocalidadRepo()
	_, err := repo.FindByID(context.Background(), uuid.New(), 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, clientes.ErrLocalidadNotFound))
}

func TestLocalidadRepo_FindByID_TenantIsolation(t *testing.T) {
	repo := newMockLocalidadRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	l := &clientes.Localidad{ClienteID: clienteID, Nombre: "Test", Direccion: "X", TipoLocalidadID: 1}
	require.NoError(t, repo.Create(context.Background(), l))

	// Wrong empresa
	_, err := repo.FindByID(context.Background(), l.ID, 999)
	assert.True(t, errors.Is(err, clientes.ErrLocalidadNotFound))
}

func TestLocalidadRepo_Delete_HappyPath(t *testing.T) {
	repo := newMockLocalidadRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	l := &clientes.Localidad{ClienteID: clienteID, Nombre: "Test", Direccion: "X", TipoLocalidadID: 1}
	require.NoError(t, repo.Create(context.Background(), l))
	require.NoError(t, repo.Delete(context.Background(), l.ID, 1))

	_, err := repo.FindByID(context.Background(), l.ID, 1)
	assert.True(t, errors.Is(err, clientes.ErrLocalidadNotFound))
}

func TestLocalidadRepo_ListByCliente(t *testing.T) {
	repo := newMockLocalidadRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	for i := 0; i < 3; i++ {
		l := &clientes.Localidad{ClienteID: clienteID, Nombre: "Branch", Direccion: "Addr", TipoLocalidadID: 1}
		require.NoError(t, repo.Create(context.Background(), l))
	}

	list, err := repo.ListByCliente(context.Background(), clienteID, 1)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

// Compile-time interface check
var _ LocalidadRepositorier = (*mockLocalidadRepo)(nil)
