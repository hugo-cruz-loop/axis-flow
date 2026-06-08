package service_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/clientes"
	"axis-flow-back/internal/clientes/service"

	"github.com/google/uuid"
)

// ---- Mock SatelliteRepository -----------------------------------------------

type mockSatelliteRepo struct {
	createFacturaFn func(ctx context.Context, f *clientes.Factura) error
	getFacturaFn    func(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Factura, error)
	updateFacturaFn func(ctx context.Context, f *clientes.Factura) error

	createPresupuestoFn func(ctx context.Context, p *clientes.Presupuesto) error
	getPresupuestoFn    func(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error)
	updatePresupuestoFn func(ctx context.Context, p *clientes.Presupuesto) error

	createCalendarioFn func(ctx context.Context, cal *clientes.CalendarioLaboral) error
	getCalendarioFn    func(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error)
	updateCalendarioFn func(ctx context.Context, cal *clientes.CalendarioLaboral) error
}

func (m *mockSatelliteRepo) CreateFactura(ctx context.Context, f *clientes.Factura) error {
	return m.createFacturaFn(ctx, f)
}
func (m *mockSatelliteRepo) GetFactura(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Factura, error) {
	return m.getFacturaFn(ctx, clienteID, empresaID)
}
func (m *mockSatelliteRepo) UpdateFactura(ctx context.Context, f *clientes.Factura) error {
	return m.updateFacturaFn(ctx, f)
}
func (m *mockSatelliteRepo) CreatePresupuesto(ctx context.Context, p *clientes.Presupuesto) error {
	return m.createPresupuestoFn(ctx, p)
}
func (m *mockSatelliteRepo) GetPresupuesto(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error) {
	return m.getPresupuestoFn(ctx, clienteID, empresaID)
}
func (m *mockSatelliteRepo) UpdatePresupuesto(ctx context.Context, p *clientes.Presupuesto) error {
	return m.updatePresupuestoFn(ctx, p)
}
func (m *mockSatelliteRepo) CreateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral) error {
	return m.createCalendarioFn(ctx, cal)
}
func (m *mockSatelliteRepo) GetCalendario(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error) {
	return m.getCalendarioFn(ctx, clienteID, empresaID)
}
func (m *mockSatelliteRepo) UpdateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral) error {
	return m.updateCalendarioFn(ctx, cal)
}

// valid 64-char hex = 32 bytes
const testEncKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

func TestSatelliteService_CreateFactura_EncryptsRFC(t *testing.T) {
	ctx := context.Background()
	clienteID := uuid.New()
	plainRFC := "ABCD123456EFG"

	var storedRFC string
	repo := &mockSatelliteRepo{
		createFacturaFn: func(_ context.Context, f *clientes.Factura) error {
			storedRFC = f.RFC
			f.ID = uuid.New()
			f.CreatedAt = time.Now()
			f.UpdatedAt = time.Now()
			return nil
		},
	}

	svc := service.NewSatelliteService(repo)
	f := &clientes.Factura{
		ClienteID:       clienteID,
		RFC:             plainRFC,
		RazonSocial:     "Acme SA",
		DomicilioFiscal: "Calle Falsa 123",
	}

	err := svc.CreateFactura(ctx, f, 1, testEncKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storedRFC == "" {
		t.Fatal("expected RFC to be stored")
	}
	if storedRFC == plainRFC {
		t.Error("RFC must be encrypted before storing — plaintext detected")
	}
	// Verify f.RFC was restored to plaintext after storing
	if f.RFC != plainRFC {
		t.Errorf("f.RFC = %q after create, want plaintext %q", f.RFC, plainRFC)
	}
}

func TestSatelliteService_GetFactura_DecryptsRFC(t *testing.T) {
	ctx := context.Background()
	clienteID := uuid.New()
	plainRFC := "ABCD123456EFG"

	// Encrypt first to store as we would
	encrypted, err := service.EncryptForTest(plainRFC, testEncKey)
	if err != nil {
		t.Fatalf("setup encrypt: %v", err)
	}

	repo := &mockSatelliteRepo{
		getFacturaFn: func(_ context.Context, _ uuid.UUID, _ int64) (*clientes.Factura, error) {
			return &clientes.Factura{
				ID:              uuid.New(),
				ClienteID:       clienteID,
				RFC:             encrypted, // stored encrypted
				RazonSocial:     "Acme",
				DomicilioFiscal: "Calle 1",
			}, nil
		},
	}

	svc := service.NewSatelliteService(repo)
	got, err := svc.GetFactura(ctx, clienteID, 1, testEncKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RFC != plainRFC {
		t.Errorf("RFC = %q, want %q (decrypted)", got.RFC, plainRFC)
	}
}
