package service

import (
	"context"
	"fmt"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
)

// SatelliteRepository is the subset of repository.SatelliteRepository used by the service layer.
type SatelliteRepository interface {
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

// SatelliteServicer defines business operations for satellite records (Factura, Presupuesto, Calendario).
type SatelliteServicer interface {
	CreateFactura(ctx context.Context, f *clientes.Factura, empresaID int64, encKey string) error
	GetFactura(ctx context.Context, clienteID uuid.UUID, empresaID int64, encKey string) (*clientes.Factura, error)
	UpdateFactura(ctx context.Context, f *clientes.Factura, empresaID int64, encKey string) error

	CreatePresupuesto(ctx context.Context, p *clientes.Presupuesto, empresaID int64) error
	GetPresupuesto(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error)
	UpdatePresupuesto(ctx context.Context, p *clientes.Presupuesto, empresaID int64) error

	CreateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral, empresaID int64) error
	GetCalendario(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error)
	UpdateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral, empresaID int64) error
}

// SatelliteService implements SatelliteServicer.
type SatelliteService struct {
	repo SatelliteRepository
}

// NewSatelliteService constructs a SatelliteService.
func NewSatelliteService(repo SatelliteRepository) *SatelliteService {
	return &SatelliteService{repo: repo}
}

// ---- Factura ----------------------------------------------------------------

// CreateFactura encrypts f.RFC before persisting. f.RFC is restored to plaintext on return.
// Security: RFC and encKey are never logged or included in error messages.
func (s *SatelliteService) CreateFactura(ctx context.Context, f *clientes.Factura, _ int64, encKey string) error {
	encRFC, err := Encrypt(f.RFC, encKey)
	if err != nil {
		return fmt.Errorf("satelliteService.CreateFactura: encryption failed")
	}

	plainRFC := f.RFC
	f.RFC = encRFC
	repoErr := s.repo.CreateFactura(ctx, f)
	f.RFC = plainRFC // always restore so caller gets plaintext back

	if repoErr != nil {
		return fmt.Errorf("satelliteService.CreateFactura: %w", repoErr)
	}
	return nil
}

// GetFactura retrieves and decrypts the RFC.
// Security: RFC and encKey are never logged or included in error messages.
func (s *SatelliteService) GetFactura(ctx context.Context, clienteID uuid.UUID, empresaID int64, encKey string) (*clientes.Factura, error) {
	f, err := s.repo.GetFactura(ctx, clienteID, empresaID)
	if err != nil {
		return nil, err
	}

	plain, err := Decrypt(f.RFC, encKey)
	if err != nil {
		return nil, fmt.Errorf("satelliteService.GetFactura: decryption failed")
	}
	f.RFC = plain
	return f, nil
}

// UpdateFactura encrypts f.RFC if it is provided (non-empty plaintext).
// f.RFC is restored to plaintext on return.
// Security: RFC and encKey are never logged or included in error messages.
func (s *SatelliteService) UpdateFactura(ctx context.Context, f *clientes.Factura, _ int64, encKey string) error {
	encRFC, err := Encrypt(f.RFC, encKey)
	if err != nil {
		return fmt.Errorf("satelliteService.UpdateFactura: encryption failed")
	}

	plainRFC := f.RFC
	f.RFC = encRFC
	repoErr := s.repo.UpdateFactura(ctx, f)
	f.RFC = plainRFC

	if repoErr != nil {
		return fmt.Errorf("satelliteService.UpdateFactura: %w", repoErr)
	}
	return nil
}

// ---- Presupuesto ------------------------------------------------------------

func (s *SatelliteService) CreatePresupuesto(ctx context.Context, p *clientes.Presupuesto, _ int64) error {
	if err := s.repo.CreatePresupuesto(ctx, p); err != nil {
		return fmt.Errorf("satelliteService.CreatePresupuesto: %w", err)
	}
	return nil
}

func (s *SatelliteService) GetPresupuesto(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error) {
	return s.repo.GetPresupuesto(ctx, clienteID, empresaID)
}

func (s *SatelliteService) UpdatePresupuesto(ctx context.Context, p *clientes.Presupuesto, _ int64) error {
	if err := s.repo.UpdatePresupuesto(ctx, p); err != nil {
		return fmt.Errorf("satelliteService.UpdatePresupuesto: %w", err)
	}
	return nil
}

// ---- CalendarioLaboral ------------------------------------------------------

func (s *SatelliteService) CreateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral, _ int64) error {
	if err := s.repo.CreateCalendario(ctx, cal); err != nil {
		return fmt.Errorf("satelliteService.CreateCalendario: %w", err)
	}
	return nil
}

func (s *SatelliteService) GetCalendario(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error) {
	return s.repo.GetCalendario(ctx, clienteID, empresaID)
}

func (s *SatelliteService) UpdateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral, _ int64) error {
	if err := s.repo.UpdateCalendario(ctx, cal); err != nil {
		return fmt.Errorf("satelliteService.UpdateCalendario: %w", err)
	}
	return nil
}

// EncryptForTest exposes Encrypt for use in test packages only.
// It is NOT part of the public service API; only tests should call this.
func EncryptForTest(plaintext, keyHex string) (string, error) {
	return Encrypt(plaintext, keyHex)
}
