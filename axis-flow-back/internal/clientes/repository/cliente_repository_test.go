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
// ClienteRepository interface (for mock compliance)
// ---------------------------------------------------------------------------

type ClienteRepositorier interface {
	Create(ctx context.Context, c *clientes.Cliente) error
	FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	ListByEmpresa(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error)
	Update(ctx context.Context, c *clientes.Cliente) error
	Delete(ctx context.Context, id uuid.UUID, empresaID int64) error
	CheckQualityGate(ctx context.Context, clienteID uuid.UUID) (*clientes.QualityGateStatus, error)
	PatchEstatus(ctx context.Context, clienteID uuid.UUID, empresaID int64, estatus int) (int, *clientes.QualityGateStatus, error)
}

// ---------------------------------------------------------------------------
// In-memory mock
// ---------------------------------------------------------------------------

type mockClienteRepo struct {
	records      map[string]*clientes.Cliente // key: id
	facturas     map[string]bool
	presupuestos map[string]bool
	calendarios  map[string]bool
	// If true, PatchEstatus to Activo always triggers quality gate violation
	blockActivation bool
}

func newMockClienteRepo() *mockClienteRepo {
	return &mockClienteRepo{
		records:      make(map[string]*clientes.Cliente),
		facturas:     make(map[string]bool),
		presupuestos: make(map[string]bool),
		calendarios:  make(map[string]bool),
	}
}

func (m *mockClienteRepo) Create(_ context.Context, c *clientes.Cliente) error {
	c.ID = uuid.New()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	cp := *c
	m.records[c.ID.String()] = &cp
	return nil
}

func (m *mockClienteRepo) FindByID(_ context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	c, ok := m.records[id.String()]
	if !ok || c.EmpresaID != empresaID {
		return nil, clientes.ErrClienteNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *mockClienteRepo) FindByUserID(_ context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	for _, c := range m.records {
		if c.RepresentanteID == userID && c.EmpresaID == empresaID {
			cp := *c
			return &cp, nil
		}
	}
	return nil, clientes.ErrClienteNotFound
}

func (m *mockClienteRepo) ListByEmpresa(_ context.Context, empresaID int64, _, _ int) ([]clientes.Cliente, int, error) {
	var out []clientes.Cliente
	for _, c := range m.records {
		if c.EmpresaID == empresaID {
			out = append(out, *c)
		}
	}
	return out, len(out), nil
}

func (m *mockClienteRepo) Update(_ context.Context, c *clientes.Cliente) error {
	existing, ok := m.records[c.ID.String()]
	if !ok || existing.EmpresaID != c.EmpresaID {
		return clientes.ErrClienteNotFound
	}
	cp := *c
	m.records[c.ID.String()] = &cp
	return nil
}

func (m *mockClienteRepo) Delete(_ context.Context, id uuid.UUID, empresaID int64) error {
	c, ok := m.records[id.String()]
	if !ok || c.EmpresaID != empresaID {
		return clientes.ErrClienteNotFound
	}
	delete(m.records, id.String())
	return nil
}

func (m *mockClienteRepo) CheckQualityGate(_ context.Context, clienteID uuid.UUID) (*clientes.QualityGateStatus, error) {
	key := clienteID.String()
	gate := &clientes.QualityGateStatus{
		ClienteID:     clienteID,
		FacturaOK:     m.facturas[key],
		PresupuestoOK: m.presupuestos[key],
		CalendarioOK:  m.calendarios[key],
	}
	gate.CanActivate = gate.FacturaOK && gate.PresupuestoOK && gate.CalendarioOK
	return gate, nil
}

func (m *mockClienteRepo) PatchEstatus(_ context.Context, clienteID uuid.UUID, empresaID int64, estatus int) (int, *clientes.QualityGateStatus, error) {
	c, ok := m.records[clienteID.String()]
	if !ok || c.EmpresaID != empresaID {
		return 0, nil, clientes.ErrClienteNotFound
	}

	// Simulate quality gate check when activating
	if estatus == clientes.ClienteStatusActivo && m.blockActivation {
		key := clienteID.String()
		gate := &clientes.QualityGateStatus{
			ClienteID:     clienteID,
			FacturaOK:     m.facturas[key],
			PresupuestoOK: m.presupuestos[key],
			CalendarioOK:  m.calendarios[key],
		}
		gate.CanActivate = gate.FacturaOK && gate.PresupuestoOK && gate.CalendarioOK
		return 206, gate, nil
	}

	c.Estatus = estatus
	return 200, nil, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestClienteRepo_Create_HappyPath(t *testing.T) {
	repo := newMockClienteRepo()
	c := &clientes.Cliente{
		EmpresaID:       1,
		RepresentanteID: uuid.New(),
		NombreComercial: "Acme Corp",
		RazonSocial:     "Acme SA de CV",
		Estatus:         clientes.ClienteStatusIncompleto,
	}
	err := repo.Create(context.Background(), c)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, c.ID)
	assert.False(t, c.CreatedAt.IsZero())
}

func TestClienteRepo_FindByID_NotFound(t *testing.T) {
	repo := newMockClienteRepo()
	_, err := repo.FindByID(context.Background(), uuid.New(), 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, clientes.ErrClienteNotFound))
}

func TestClienteRepo_FindByID_TenantIsolation(t *testing.T) {
	repo := newMockClienteRepo()
	c := &clientes.Cliente{EmpresaID: 1, NombreComercial: "Tenant A", Estatus: clientes.ClienteStatusIncompleto}
	require.NoError(t, repo.Create(context.Background(), c))

	// Same ID but wrong empresa_id → not found
	_, err := repo.FindByID(context.Background(), c.ID, 999)
	assert.True(t, errors.Is(err, clientes.ErrClienteNotFound))
}

func TestClienteRepo_PatchEstatus_BlockedByQualityGate(t *testing.T) {
	repo := newMockClienteRepo()
	repo.blockActivation = true

	c := &clientes.Cliente{EmpresaID: 1, NombreComercial: "Incomplete", Estatus: clientes.ClienteStatusIncompleto}
	require.NoError(t, repo.Create(context.Background(), c))

	code, gate, err := repo.PatchEstatus(context.Background(), c.ID, 1, clientes.ClienteStatusActivo)
	require.NoError(t, err)
	assert.Equal(t, 206, code)
	require.NotNil(t, gate)
	assert.False(t, gate.CanActivate)
}

func TestClienteRepo_PatchEstatus_Success(t *testing.T) {
	repo := newMockClienteRepo()

	c := &clientes.Cliente{EmpresaID: 1, NombreComercial: "Complete", Estatus: clientes.ClienteStatusIncompleto}
	require.NoError(t, repo.Create(context.Background(), c))

	code, gate, err := repo.PatchEstatus(context.Background(), c.ID, 1, clientes.ClienteStatusActivo)
	require.NoError(t, err)
	assert.Equal(t, 200, code)
	assert.Nil(t, gate)
}

func TestClienteRepo_Delete_NotFound(t *testing.T) {
	repo := newMockClienteRepo()
	err := repo.Delete(context.Background(), uuid.New(), 1)
	assert.True(t, errors.Is(err, clientes.ErrClienteNotFound))
}

// Compile-time interface check
var _ ClienteRepositorier = (*mockClienteRepo)(nil)
