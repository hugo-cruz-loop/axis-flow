package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	empleados "axis-flow-back/internal/empleados"
	"axis-flow-back/internal/empleados/repository"

	"github.com/google/uuid"
)

// ErrInvalidEmpleadoRequest is returned when an employee command is missing required data.
var ErrInvalidEmpleadoRequest = errors.New("invalid empleado request")

// TenantResolver maps an empresa ID to the identity tenant UUID required by users.identity_users.
type TenantResolver interface {
	ResolveTenantID(ctx context.Context, empresaID int64) (uuid.UUID, error)
}

// EmpleadoRepositoryPort is the subset of repository behavior used by EmpleadoService.
type EmpleadoRepositoryPort interface {
	Create(ctx context.Context, p repository.CreateEmpleadoParams) (*empleados.Empleado, error)
	GetByID(ctx context.Context, numEmpleado, empresaID int64) (*empleados.Empleado, error)
	GetByEmpresa(ctx context.Context, empresaID int64) ([]empleados.Empleado, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, empresaID int64) (*empleados.Empleado, error)
	Update(ctx context.Context, e *empleados.Empleado) error
	SoftDelete(ctx context.Context, numEmpleado, empresaID int64) error
}

// CreateEmpleadoRequest contains the data needed to create identity and business employee records atomically.
type CreateEmpleadoRequest struct {
	TenantID        uuid.UUID
	UsuarioID       uuid.UUID
	Email           string
	PasswordHash    string
	IDEmpleado      string
	EmpresaID       int64
	Nombre          string
	ApellidoPaterno string
	ApellidoMaterno string
	CreatedBy       uuid.UUID
}

// UpdateEmpleadoRequest holds partial employee profile changes.
type UpdateEmpleadoRequest struct {
	Nombre          *string
	ApellidoPaterno *string
	ApellidoMaterno *string
	Status          *int
}

// EmpleadoService orchestrates employee lifecycle operations.
type EmpleadoService struct {
	repo           EmpleadoRepositoryPort
	tenantResolver TenantResolver
}

// NewEmpleadoService creates an employee service using repository and tenant resolver ports.
func NewEmpleadoService(repo EmpleadoRepositoryPort, tenantResolver TenantResolver) *EmpleadoService {
	return &EmpleadoService{repo: repo, tenantResolver: tenantResolver}
}

// CreateEmpleado resolves or accepts TenantID, then delegates atomic creation to the repository.
func (s *EmpleadoService) CreateEmpleado(ctx context.Context, req CreateEmpleadoRequest) (*empleados.Empleado, error) {
	if err := validateCreateEmpleadoRequest(req); err != nil {
		return nil, err
	}

	tenantID := req.TenantID
	if tenantID == uuid.Nil {
		if s.tenantResolver == nil {
			return nil, fmt.Errorf("%w: tenant resolver is required when TenantID is empty", ErrInvalidEmpleadoRequest)
		}
		resolved, err := s.tenantResolver.ResolveTenantID(ctx, req.EmpresaID)
		if err != nil {
			return nil, fmt.Errorf("empleadoService resolve tenant: %w", err)
		}
		tenantID = resolved
	}

	empleado, err := s.repo.Create(ctx, repository.CreateEmpleadoParams{
		UsuarioID:       req.UsuarioID,
		TenantID:        tenantID,
		Email:           strings.TrimSpace(req.Email),
		PasswordHash:    req.PasswordHash,
		IDEmpleado:      strings.TrimSpace(req.IDEmpleado),
		EmpresaID:       req.EmpresaID,
		Nombre:          strings.TrimSpace(req.Nombre),
		ApellidoPaterno: strings.TrimSpace(req.ApellidoPaterno),
		ApellidoMaterno: strings.TrimSpace(req.ApellidoMaterno),
		CreatedBy:       req.CreatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("empleadoService.CreateEmpleado: %w", err)
	}
	return empleado, nil
}

// GetEmpleado returns an empresa-scoped employee.
func (s *EmpleadoService) GetEmpleado(ctx context.Context, numEmpleado, empresaID int64) (*empleados.Empleado, error) {
	return s.repo.GetByID(ctx, numEmpleado, empresaID)
}

// ListEmpleados returns employees scoped to one empresa.
func (s *EmpleadoService) ListEmpleados(ctx context.Context, empresaID int64) ([]empleados.Empleado, error) {
	return s.repo.GetByEmpresa(ctx, empresaID)
}

// GetEmpleadoByUser returns the employee attached to an identity user within an empresa.
func (s *EmpleadoService) GetEmpleadoByUser(ctx context.Context, userID uuid.UUID, empresaID int64) (*empleados.Empleado, error) {
	return s.repo.GetByUserID(ctx, userID, empresaID)
}

// UpdateEmpleado applies partial employee profile changes within empresa scope.
func (s *EmpleadoService) UpdateEmpleado(ctx context.Context, numEmpleado, empresaID int64, req UpdateEmpleadoRequest) (*empleados.Empleado, error) {
	empleado, err := s.repo.GetByID(ctx, numEmpleado, empresaID)
	if err != nil {
		return nil, err
	}
	if empleado == nil {
		return nil, empleados.ErrEmpleadoNotFound
	}

	if req.Nombre != nil {
		empleado.Nombre = strings.TrimSpace(*req.Nombre)
	}
	if req.ApellidoPaterno != nil {
		empleado.ApellidoPaterno = strings.TrimSpace(*req.ApellidoPaterno)
	}
	if req.ApellidoMaterno != nil {
		empleado.ApellidoMaterno = strings.TrimSpace(*req.ApellidoMaterno)
	}
	if req.Status != nil {
		empleado.Status = *req.Status
	}

	if err := s.repo.Update(ctx, empleado); err != nil {
		return nil, fmt.Errorf("empleadoService.UpdateEmpleado: %w", err)
	}
	return empleado, nil
}

// DeleteEmpleado performs the administrative baja through repository soft delete.
func (s *EmpleadoService) DeleteEmpleado(ctx context.Context, numEmpleado, empresaID int64) error {
	if err := s.repo.SoftDelete(ctx, numEmpleado, empresaID); err != nil {
		return fmt.Errorf("empleadoService.DeleteEmpleado: %w", err)
	}
	return nil
}

func validateCreateEmpleadoRequest(req CreateEmpleadoRequest) error {
	if req.EmpresaID <= 0 {
		return fmt.Errorf("%w: empresaID is required", ErrInvalidEmpleadoRequest)
	}
	if strings.TrimSpace(req.Email) == "" {
		return fmt.Errorf("%w: email is required", ErrInvalidEmpleadoRequest)
	}
	if strings.TrimSpace(req.IDEmpleado) == "" {
		return fmt.Errorf("%w: IDEmpleado is required", ErrInvalidEmpleadoRequest)
	}
	if strings.TrimSpace(req.Nombre) == "" {
		return fmt.Errorf("%w: nombre is required", ErrInvalidEmpleadoRequest)
	}
	if strings.TrimSpace(req.ApellidoPaterno) == "" {
		return fmt.Errorf("%w: apellidoPaterno is required", ErrInvalidEmpleadoRequest)
	}
	return nil
}
