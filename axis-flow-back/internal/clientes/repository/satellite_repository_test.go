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
// SatelliteRepository interface
// ---------------------------------------------------------------------------

type SatelliteRepositorier interface {
	CreateFactura(ctx context.Context, f *clientes.Factura) error
	GetFactura(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Factura, error)
	UpdateFactura(ctx context.Context, f *clientes.Factura) error
	CreatePresupuesto(ctx context.Context, p *clientes.Presupuesto) error
	GetPresupuesto(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error)
	UpdatePresupuesto(ctx context.Context, p *clientes.Presupuesto) error
	CreateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral) error
	GetCalendario(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error)
	UpdateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral) error
}

// ---------------------------------------------------------------------------
// In-memory mock
// ---------------------------------------------------------------------------

type mockSatelliteRepo struct {
	facturas     map[string]*clientes.Factura
	presupuestos map[string]*clientes.Presupuesto
	calendarios  map[string]*clientes.CalendarioLaboral
	// empresa mapping for tenant isolation checks
	empresaByCliente map[string]int64
}

func newMockSatelliteRepo() *mockSatelliteRepo {
	return &mockSatelliteRepo{
		facturas:         make(map[string]*clientes.Factura),
		presupuestos:     make(map[string]*clientes.Presupuesto),
		calendarios:      make(map[string]*clientes.CalendarioLaboral),
		empresaByCliente: make(map[string]int64),
	}
}

func (m *mockSatelliteRepo) registerCliente(clienteID uuid.UUID, empresaID int64) {
	m.empresaByCliente[clienteID.String()] = empresaID
}

func (m *mockSatelliteRepo) checkTenant(clienteID uuid.UUID, empresaID int64) bool {
	eid, ok := m.empresaByCliente[clienteID.String()]
	return ok && eid == empresaID
}

func (m *mockSatelliteRepo) CreateFactura(_ context.Context, f *clientes.Factura) error {
	key := f.ClienteID.String()
	if _, exists := m.facturas[key]; exists {
		return clientes.ErrFacturaExists
	}
	f.ID = uuid.New()
	f.CreatedAt = time.Now()
	f.UpdatedAt = time.Now()
	cp := *f
	m.facturas[key] = &cp
	return nil
}

func (m *mockSatelliteRepo) GetFactura(_ context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Factura, error) {
	if !m.checkTenant(clienteID, empresaID) {
		return nil, clientes.ErrClienteNotFound
	}
	f, ok := m.facturas[clienteID.String()]
	if !ok {
		return nil, clientes.ErrClienteNotFound
	}
	cp := *f
	return &cp, nil
}

func (m *mockSatelliteRepo) UpdateFactura(_ context.Context, f *clientes.Factura) error {
	key := f.ClienteID.String()
	existing, ok := m.facturas[key]
	if !ok || existing.ID != f.ID {
		return clientes.ErrClienteNotFound
	}
	cp := *f
	m.facturas[key] = &cp
	return nil
}

func (m *mockSatelliteRepo) CreatePresupuesto(_ context.Context, p *clientes.Presupuesto) error {
	key := p.ClienteID.String()
	if _, exists := m.presupuestos[key]; exists {
		return clientes.ErrPresupuestoExists
	}
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	cp := *p
	m.presupuestos[key] = &cp
	return nil
}

func (m *mockSatelliteRepo) GetPresupuesto(_ context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error) {
	if !m.checkTenant(clienteID, empresaID) {
		return nil, clientes.ErrClienteNotFound
	}
	p, ok := m.presupuestos[clienteID.String()]
	if !ok {
		return nil, clientes.ErrClienteNotFound
	}
	cp := *p
	return &cp, nil
}

func (m *mockSatelliteRepo) UpdatePresupuesto(_ context.Context, p *clientes.Presupuesto) error {
	key := p.ClienteID.String()
	existing, ok := m.presupuestos[key]
	if !ok || existing.ID != p.ID {
		return clientes.ErrClienteNotFound
	}
	cp := *p
	m.presupuestos[key] = &cp
	return nil
}

func (m *mockSatelliteRepo) CreateCalendario(_ context.Context, cal *clientes.CalendarioLaboral) error {
	key := cal.ClienteID.String()
	if _, exists := m.calendarios[key]; exists {
		return clientes.ErrCalendarioExists
	}
	cal.CreatedAt = time.Now()
	cal.UpdatedAt = time.Now()
	cp := *cal
	m.calendarios[key] = &cp
	return nil
}

func (m *mockSatelliteRepo) GetCalendario(_ context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error) {
	if !m.checkTenant(clienteID, empresaID) {
		return nil, clientes.ErrClienteNotFound
	}
	cal, ok := m.calendarios[clienteID.String()]
	if !ok {
		return nil, clientes.ErrClienteNotFound
	}
	cp := *cal
	return &cp, nil
}

func (m *mockSatelliteRepo) UpdateCalendario(_ context.Context, cal *clientes.CalendarioLaboral) error {
	key := cal.ClienteID.String()
	if _, ok := m.calendarios[key]; !ok {
		return clientes.ErrClienteNotFound
	}
	cp := *cal
	m.calendarios[key] = &cp
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSatelliteRepo_CreateFactura_HappyPath(t *testing.T) {
	repo := newMockSatelliteRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	f := &clientes.Factura{
		ClienteID:       clienteID,
		RFC:             "ENCRYPTED_RFC_OPAQUE_STRING",
		RazonSocial:     "Test SA",
		DomicilioFiscal: "Av. Test 123",
	}
	err := repo.CreateFactura(context.Background(), f)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, f.ID)
}

func TestSatelliteRepo_CreateFactura_Duplicate(t *testing.T) {
	repo := newMockSatelliteRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	f := &clientes.Factura{ClienteID: clienteID, RFC: "ENC", RazonSocial: "X", DomicilioFiscal: "Y"}
	require.NoError(t, repo.CreateFactura(context.Background(), f))

	f2 := &clientes.Factura{ClienteID: clienteID, RFC: "ENC2", RazonSocial: "X2", DomicilioFiscal: "Y2"}
	err := repo.CreateFactura(context.Background(), f2)
	require.Error(t, err)
	assert.True(t, errors.Is(err, clientes.ErrFacturaExists))
}

func TestSatelliteRepo_CreatePresupuesto_Duplicate(t *testing.T) {
	repo := newMockSatelliteRepo()
	clienteID := uuid.New()

	p := &clientes.Presupuesto{ClienteID: clienteID}
	require.NoError(t, repo.CreatePresupuesto(context.Background(), p))

	p2 := &clientes.Presupuesto{ClienteID: clienteID}
	err := repo.CreatePresupuesto(context.Background(), p2)
	require.Error(t, err)
	assert.True(t, errors.Is(err, clientes.ErrPresupuestoExists))
}

func TestSatelliteRepo_GetCalendario_HappyPath_WithJSONB(t *testing.T) {
	repo := newMockSatelliteRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	semana := map[string]any{"monday": true, "tuesday": true}
	dias := map[string]any{"2024-01-01": "New Year"}

	cal := &clientes.CalendarioLaboral{
		ClienteID:     clienteID,
		SemanaLaboral: semana,
		DiasInhabiles: dias,
	}
	require.NoError(t, repo.CreateCalendario(context.Background(), cal))

	got, err := repo.GetCalendario(context.Background(), clienteID, 1)
	require.NoError(t, err)
	assert.Equal(t, true, got.SemanaLaboral["monday"])
}

func TestSatelliteRepo_GetFactura_TenantIsolation(t *testing.T) {
	repo := newMockSatelliteRepo()
	clienteID := uuid.New()
	repo.registerCliente(clienteID, 1)

	f := &clientes.Factura{ClienteID: clienteID, RFC: "ENC", RazonSocial: "X", DomicilioFiscal: "Y"}
	require.NoError(t, repo.CreateFactura(context.Background(), f))

	// Wrong empresa_id
	_, err := repo.GetFactura(context.Background(), clienteID, 999)
	assert.True(t, errors.Is(err, clientes.ErrClienteNotFound))
}

// Compile-time interface check
var _ SatelliteRepositorier = (*mockSatelliteRepo)(nil)
