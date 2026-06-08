package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/empresas"

	"github.com/google/uuid"
)

// OnboardingRequest holds all data needed to register a new company.
type OnboardingRequest struct {
	Nombre    string
	Direccion string
	Telefono  string
	PlanID    int64
	PlanMonto float64 // fetched from catalog before calling service
	// Representante data — used to create the admin user for this empresa
	RepresentanteEmail      string
	RepresentanteNombre     string
	RepresentanteApPaterno  string
	RepresentanteApMaterno  string
}

// OnboardingResult is returned on successful onboarding.
type OnboardingResult struct {
	EmpresaID  int64
	UserID     uuid.UUID
	EmpleadoID int64 // placeholder = EmpresaID until the Empleados module is implemented
	ClavePago  string
}

// EmpresaCreatorRepo is the minimum interface the onboarding service needs from the empresa repository.
type EmpresaCreatorRepo interface {
	Create(ctx context.Context, e *empresas.Empresa) error
	UpdateStatus(ctx context.Context, id int64, status empresas.EmpresaStatus, vigencia *time.Time) error
}

// PagoCreatorRepo is the minimum interface the onboarding service needs from the pago repository.
type PagoCreatorRepo interface {
	Create(ctx context.Context, p *empresas.Pago) error
	UpdateStatus(ctx context.Context, id int64, status empresas.PagoStatus, stripeSessionID string) error
	FindByToken(ctx context.Context, token string) (*empresas.Pago, error)
}

// UserCreatorRepo is the minimum interface the onboarding service needs from the user repository.
type UserCreatorRepo interface {
	CreateUser(ctx context.Context, u *domain.User) error
}

// OnboardingService orchestrates atomic company registration.
type OnboardingService struct {
	empresaRepo EmpresaCreatorRepo
	pagoRepo    PagoCreatorRepo
	userRepo    UserCreatorRepo
}

// NewOnboardingService creates a new OnboardingService.
func NewOnboardingService(empresaRepo EmpresaCreatorRepo, pagoRepo PagoCreatorRepo, userRepo UserCreatorRepo) *OnboardingService {
	return &OnboardingService{
		empresaRepo: empresaRepo,
		pagoRepo:    pagoRepo,
		userRepo:    userRepo,
	}
}

// Register atomically creates a user (PENDING_ACTIVATION), empresa (PENDING_PAYMENT), and pago (PENDING).
func (s *OnboardingService) Register(ctx context.Context, req OnboardingRequest) (OnboardingResult, error) {
	clavePago, err := generateClavePago()
	if err != nil {
		return OnboardingResult{}, fmt.Errorf("onboarding.Register: %w", err)
	}

	u := &domain.User{
		ID:        uuid.New(),
		Email:     req.RepresentanteEmail,
		FirstName: req.RepresentanteNombre,
		LastName:  req.RepresentanteApPaterno + " " + req.RepresentanteApMaterno,
		Status:    domain.StatusPendingActivation,
	}
	if err := s.userRepo.CreateUser(ctx, u); err != nil {
		return OnboardingResult{}, fmt.Errorf("onboarding.Register create user: %w", err)
	}

	e := &empresas.Empresa{
		Nombre:          req.Nombre,
		Direccion:       req.Direccion,
		Telefono:        req.Telefono,
		PlanID:          req.PlanID,
		Status:          empresas.EmpresaStatusPendingPayment,
		RepresentanteID: u.ID,
	}
	if err := s.empresaRepo.Create(ctx, e); err != nil {
		return OnboardingResult{}, fmt.Errorf("onboarding.Register create empresa: %w", err)
	}

	p := &empresas.Pago{
		EmpresaID:   e.ID,
		TokenPago:   clavePago,
		EstatusPago: empresas.PagoStatusPending,
		Monto:       req.PlanMonto,
	}
	if err := s.pagoRepo.Create(ctx, p); err != nil {
		return OnboardingResult{}, fmt.Errorf("onboarding.Register create pago: %w", err)
	}

	return OnboardingResult{
		EmpresaID:  e.ID,
		UserID:     u.ID,
		EmpleadoID: e.ID, // placeholder until the Empleados module is implemented
		ClavePago:  clavePago,
	}, nil
}

// generateClavePago returns a URL-safe base64 random token.
func generateClavePago() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generateClavePago: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
