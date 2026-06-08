package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/empresas"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- PagoRepository mock ----

type PagoRepositorier interface {
	Create(ctx context.Context, p *empresas.Pago) error
	FindByToken(ctx context.Context, token string) (*empresas.Pago, error)
	FindByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Pago, error)
	UpdateStatus(ctx context.Context, id int64, status empresas.PagoStatus, stripeSessionID string) error
}

type mockPagoRepo struct {
	store  map[int64]*empresas.Pago
	tokens map[string]int64
	nextID int64
}

func newMockPagoRepo() *mockPagoRepo {
	return &mockPagoRepo{
		store:  make(map[int64]*empresas.Pago),
		tokens: make(map[string]int64),
		nextID: 1,
	}
}

func (m *mockPagoRepo) Create(ctx context.Context, p *empresas.Pago) error {
	if _, dup := m.tokens[p.TokenPago]; dup {
		return empresas.ErrDuplicateTokenPago
	}
	p.ID = m.nextID
	m.nextID++
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	cp := *p
	m.store[p.ID] = &cp
	m.tokens[p.TokenPago] = p.ID
	return nil
}

func (m *mockPagoRepo) FindByToken(ctx context.Context, token string) (*empresas.Pago, error) {
	id, ok := m.tokens[token]
	if !ok {
		return nil, empresas.ErrEmpresaNotFound
	}
	cp := *m.store[id]
	return &cp, nil
}

func (m *mockPagoRepo) FindByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Pago, error) {
	var out []empresas.Pago
	for _, p := range m.store {
		if p.EmpresaID == empresaID {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (m *mockPagoRepo) UpdateStatus(ctx context.Context, id int64, status empresas.PagoStatus, stripeSessionID string) error {
	p, ok := m.store[id]
	if !ok {
		return empresas.ErrEmpresaNotFound
	}
	p.EstatusPago = status
	p.StripeSessionID = stripeSessionID
	p.UpdatedAt = time.Now()
	return nil
}

// ---- DatosFiscalesRepository mock ----

type DatosFiscalesRepositorier interface {
	Create(ctx context.Context, d *empresas.DatosFiscales) error
	FindByEmpresaID(ctx context.Context, empresaID int64) (*empresas.DatosFiscales, error)
	Update(ctx context.Context, d *empresas.DatosFiscales) error
}

type mockDatosFiscalesRepo struct {
	store  map[int64]*empresas.DatosFiscales
	rfcs   map[string]int64
	nextID int64
}

func newMockDatosFiscalesRepo() *mockDatosFiscalesRepo {
	return &mockDatosFiscalesRepo{
		store:  make(map[int64]*empresas.DatosFiscales),
		rfcs:   make(map[string]int64),
		nextID: 1,
	}
}

func (m *mockDatosFiscalesRepo) Create(ctx context.Context, d *empresas.DatosFiscales) error {
	if _, dup := m.rfcs[d.RFC]; dup {
		return empresas.ErrDuplicateRFC
	}
	d.ID = m.nextID
	m.nextID++
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	cp := *d
	m.store[d.EmpresaID] = &cp
	m.rfcs[d.RFC] = d.ID
	return nil
}

func (m *mockDatosFiscalesRepo) FindByEmpresaID(ctx context.Context, empresaID int64) (*empresas.DatosFiscales, error) {
	d, ok := m.store[empresaID]
	if !ok {
		return nil, empresas.ErrEmpresaNotFound
	}
	cp := *d
	return &cp, nil
}

func (m *mockDatosFiscalesRepo) Update(ctx context.Context, d *empresas.DatosFiscales) error {
	if _, ok := m.store[d.EmpresaID]; !ok {
		return empresas.ErrEmpresaNotFound
	}
	d.UpdatedAt = time.Now()
	cp := *d
	m.store[d.EmpresaID] = &cp
	return nil
}

// ---- ApoderadoRepository mock ----

type ApoderadoRepositorier interface {
	Create(ctx context.Context, a *empresas.Apoderado) error
	FindByID(ctx context.Context, id int64) (*empresas.Apoderado, error)
	ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Apoderado, error)
	Update(ctx context.Context, a *empresas.Apoderado) error
	Delete(ctx context.Context, id int64) error
}

type mockApoderadoRepo struct {
	store  map[int64]*empresas.Apoderado
	nextID int64
}

func newMockApoderadoRepo() *mockApoderadoRepo {
	return &mockApoderadoRepo{store: make(map[int64]*empresas.Apoderado), nextID: 1}
}

func (m *mockApoderadoRepo) Create(ctx context.Context, a *empresas.Apoderado) error {
	a.ID = m.nextID
	m.nextID++
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	cp := *a
	m.store[a.ID] = &cp
	return nil
}

func (m *mockApoderadoRepo) FindByID(ctx context.Context, id int64) (*empresas.Apoderado, error) {
	a, ok := m.store[id]
	if !ok {
		return nil, empresas.ErrEmpresaNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *mockApoderadoRepo) ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Apoderado, error) {
	var out []empresas.Apoderado
	for _, a := range m.store {
		if a.EmpresaID == empresaID {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (m *mockApoderadoRepo) Update(ctx context.Context, a *empresas.Apoderado) error {
	if _, ok := m.store[a.ID]; !ok {
		return empresas.ErrEmpresaNotFound
	}
	a.UpdatedAt = time.Now()
	cp := *a
	m.store[a.ID] = &cp
	return nil
}

func (m *mockApoderadoRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.store[id]; !ok {
		return empresas.ErrEmpresaNotFound
	}
	delete(m.store, id)
	return nil
}

// ---- ServicioRepository mock ----

type ServicioRepositorier interface {
	Create(ctx context.Context, s *empresas.Servicio) error
	FindByID(ctx context.Context, id int64) (*empresas.Servicio, error)
	ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Servicio, error)
	Update(ctx context.Context, s *empresas.Servicio) error
	Delete(ctx context.Context, id int64) error
}

type mockServicioRepo struct {
	store  map[int64]*empresas.Servicio
	nextID int64
}

func newMockServicioRepo() *mockServicioRepo {
	return &mockServicioRepo{store: make(map[int64]*empresas.Servicio), nextID: 1}
}

func (m *mockServicioRepo) Create(ctx context.Context, s *empresas.Servicio) error {
	s.ID = m.nextID
	m.nextID++
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	cp := *s
	m.store[s.ID] = &cp
	return nil
}

func (m *mockServicioRepo) FindByID(ctx context.Context, id int64) (*empresas.Servicio, error) {
	s, ok := m.store[id]
	if !ok {
		return nil, empresas.ErrEmpresaNotFound
	}
	cp := *s
	return &cp, nil
}

func (m *mockServicioRepo) ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Servicio, error) {
	var out []empresas.Servicio
	for _, s := range m.store {
		if s.EmpresaID == empresaID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *mockServicioRepo) Update(ctx context.Context, s *empresas.Servicio) error {
	if _, ok := m.store[s.ID]; !ok {
		return empresas.ErrEmpresaNotFound
	}
	s.UpdatedAt = time.Now()
	cp := *s
	m.store[s.ID] = &cp
	return nil
}

func (m *mockServicioRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.store[id]; !ok {
		return empresas.ErrEmpresaNotFound
	}
	delete(m.store, id)
	return nil
}

// ---- Tests ----

func TestPagoRepo_Create_WithDuplicateToken_ReturnsErrDuplicateTokenPago(t *testing.T) {
	repo := newMockPagoRepo()
	p := &empresas.Pago{EmpresaID: 1, TokenPago: "tok_abc", Monto: 100, EstatusPago: empresas.PagoStatusPending}

	require.NoError(t, repo.Create(context.Background(), p))

	p2 := &empresas.Pago{EmpresaID: 1, TokenPago: "tok_abc", Monto: 50, EstatusPago: empresas.PagoStatusPending}
	err := repo.Create(context.Background(), p2)
	assert.ErrorIs(t, err, empresas.ErrDuplicateTokenPago)
}

func TestPagoRepo_FindByToken_ReturnsCorrectPago(t *testing.T) {
	repo := newMockPagoRepo()
	p := &empresas.Pago{EmpresaID: 2, TokenPago: "tok_xyz", Monto: 200, EstatusPago: empresas.PagoStatusPending}
	require.NoError(t, repo.Create(context.Background(), p))

	got, err := repo.FindByToken(context.Background(), "tok_xyz")
	require.NoError(t, err)
	assert.Equal(t, int64(2), got.EmpresaID)
	assert.Equal(t, 200.0, got.Monto)
}

func TestPagoRepo_UpdateStatus_ChangesToPaid(t *testing.T) {
	repo := newMockPagoRepo()
	p := &empresas.Pago{EmpresaID: 3, TokenPago: "tok_pay", Monto: 300, EstatusPago: empresas.PagoStatusPending}
	require.NoError(t, repo.Create(context.Background(), p))

	err := repo.UpdateStatus(context.Background(), p.ID, empresas.PagoStatusPaid, "cs_stripe_123")
	require.NoError(t, err)
	assert.Equal(t, empresas.PagoStatusPaid, repo.store[p.ID].EstatusPago)
	assert.Equal(t, "cs_stripe_123", repo.store[p.ID].StripeSessionID)
}

func TestDatosFiscalesRepo_Create_WithDuplicateRFC_ReturnsErrDuplicateRFC(t *testing.T) {
	repo := newMockDatosFiscalesRepo()
	d := &empresas.DatosFiscales{EmpresaID: 1, RFC: "XAXX010101000", RazonSocial: "Empresa A"}
	require.NoError(t, repo.Create(context.Background(), d))

	d2 := &empresas.DatosFiscales{EmpresaID: 2, RFC: "XAXX010101000", RazonSocial: "Empresa B"}
	err := repo.Create(context.Background(), d2)
	assert.ErrorIs(t, err, empresas.ErrDuplicateRFC)
}

func TestApoderadoRepo_ListByEmpresa_ReturnsAll(t *testing.T) {
	repo := newMockApoderadoRepo()
	empresaID := int64(10)

	for i := 0; i < 3; i++ {
		a := &empresas.Apoderado{
			EmpresaID: empresaID,
			Nombre:    "Rep",
			CURP:      "CURP123",
			RFC:       "RFC123",
			Email:     "rep@example.com",
		}
		require.NoError(t, repo.Create(context.Background(), a))
	}
	// Add one for a different empresa
	other := &empresas.Apoderado{EmpresaID: 99, Nombre: "Other", CURP: "X", RFC: "Y", Email: "z@z.com"}
	require.NoError(t, repo.Create(context.Background(), other))

	list, err := repo.ListByEmpresaID(context.Background(), empresaID)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestServicioRepo_Create_WithNegativePrice_StillSucceeds(t *testing.T) {
	// Price validation is at the service layer, not the repository.
	repo := newMockServicioRepo()
	s := &empresas.Servicio{
		EmpresaID:    1,
		Nombre:       "Servicio X",
		Precio:       -50.0,
		StatusActivo: true,
	}

	err := repo.Create(context.Background(), s)
	require.NoError(t, err)
	assert.Greater(t, s.ID, int64(0))
	assert.Equal(t, -50.0, s.Precio)
}
