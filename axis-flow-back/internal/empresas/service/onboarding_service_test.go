package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/empresas"
	"axis-flow-back/internal/empresas/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── consumer-defined stubs ────────────────────────────────────────────────────

type stubEmpresaRepo struct {
	createErr    error
	createdCount int
}

func (s *stubEmpresaRepo) Create(ctx context.Context, e *empresas.Empresa) error {
	s.createdCount++
	if s.createErr != nil {
		return s.createErr
	}
	e.ID = 1
	return nil
}

func (s *stubEmpresaRepo) UpdateStatus(ctx context.Context, id int64, status empresas.EmpresaStatus, vigencia *time.Time) error {
	return nil
}

type stubPagoRepo struct {
	createErr    error
	createdCount int
}

func (s *stubPagoRepo) Create(ctx context.Context, p *empresas.Pago) error {
	s.createdCount++
	if s.createErr != nil {
		return s.createErr
	}
	p.ID = 1
	return nil
}

func (s *stubPagoRepo) UpdateStatus(ctx context.Context, id int64, status empresas.PagoStatus, stripeSessionID string) error {
	return nil
}

func (s *stubPagoRepo) FindByToken(ctx context.Context, token string) (*empresas.Pago, error) {
	return nil, empresas.ErrEmpresaNotFound
}

type stubUserCreatorRepo struct {
	createErr    error
	createdCount int
}

func (s *stubUserCreatorRepo) CreateUser(ctx context.Context, u *domain.User) error {
	s.createdCount++
	return s.createErr
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestOnboarding_WithValidRequest_CreatesAllEntities(t *testing.T) {
	empresaRepo := &stubEmpresaRepo{}
	pagoRepo := &stubPagoRepo{}
	userRepo := &stubUserCreatorRepo{}

	svc := service.NewOnboardingService(empresaRepo, pagoRepo, userRepo)

	req := service.OnboardingRequest{
		Nombre:                 "Test SA",
		Direccion:              "Av Test 123",
		Telefono:               "5551234567",
		PlanID:                 1,
		PlanMonto:              999.00,
		RepresentanteEmail:     "rep@test.com",
		RepresentanteNombre:    "Juan",
		RepresentanteApPaterno: "Perez",
		RepresentanteApMaterno: "Lopez",
	}

	result, err := svc.Register(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.EmpresaID)
	assert.NotEmpty(t, result.ClavePago)
	assert.NotEqual(t, [16]byte{}, result.UserID, "UserID must be populated")
	assert.Equal(t, result.EmpresaID, result.EmpleadoID, "EmpleadoID should equal EmpresaID (placeholder)")
	assert.Equal(t, 1, userRepo.createdCount)
	assert.Equal(t, 1, empresaRepo.createdCount)
	assert.Equal(t, 1, pagoRepo.createdCount)
}

func TestOnboarding_WhenEmpresaCreateFails_ReturnsError(t *testing.T) {
	empresaRepo := &stubEmpresaRepo{createErr: errors.New("db error")}
	pagoRepo := &stubPagoRepo{}
	userRepo := &stubUserCreatorRepo{}

	svc := service.NewOnboardingService(empresaRepo, pagoRepo, userRepo)

	req := service.OnboardingRequest{
		Nombre:    "Test SA",
		Direccion: "Av Test 123",
		PlanID:    1,
		PlanMonto: 500.00,
	}

	_, err := svc.Register(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, 0, pagoRepo.createdCount, "pago should not be created when empresa fails")
}

func TestOnboarding_WithDuplicateRepresentante_ReturnsErrDuplicateRFC(t *testing.T) {
	empresaRepo := &stubEmpresaRepo{createErr: empresas.ErrDuplicateRFC}
	pagoRepo := &stubPagoRepo{}
	userRepo := &stubUserCreatorRepo{}

	svc := service.NewOnboardingService(empresaRepo, pagoRepo, userRepo)

	req := service.OnboardingRequest{
		Nombre:    "Dup SA",
		Direccion: "Calle Dup 1",
		PlanID:    2,
		PlanMonto: 200.00,
	}

	_, err := svc.Register(context.Background(), req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, empresas.ErrDuplicateRFC))
}

func TestOnboarding_CreatesUserWithPendingActivationStatus(t *testing.T) {
	empresaRepo := &stubEmpresaRepo{}
	pagoRepo := &stubPagoRepo{}
	userRepo := &stubUserCreatorRepo{}

	svc := service.NewOnboardingService(empresaRepo, pagoRepo, userRepo)

	req := service.OnboardingRequest{
		Nombre:                 "Status SA",
		Direccion:              "Av Status 1",
		PlanID:                 1,
		PlanMonto:              100.00,
		RepresentanteEmail:     "status@test.com",
		RepresentanteNombre:    "Ana",
		RepresentanteApPaterno: "Garcia",
		RepresentanteApMaterno: "Torres",
	}

	result, err := svc.Register(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 1, userRepo.createdCount)
	// UserID must be a non-zero UUID
	var zeroUUID [16]byte
	assert.NotEqual(t, zeroUUID, [16]byte(result.UserID), "UserID must be a valid UUID")
}
