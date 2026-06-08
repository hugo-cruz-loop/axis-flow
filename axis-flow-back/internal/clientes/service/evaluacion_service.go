package service

import (
	"context"
	"fmt"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
)

// EvaluacionRepository is the subset of repository.EvaluacionRepository used by the service layer.
type EvaluacionRepository interface {
	Create(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error
	ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error)
}

// EvaluacionServicer defines business operations for EvaluacionServicio.
type EvaluacionServicer interface {
	CreateEvaluacion(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error
	ListEvaluaciones(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error)
}

// EvaluacionService implements EvaluacionServicer.
type EvaluacionService struct {
	repo EvaluacionRepository
}

// NewEvaluacionService constructs an EvaluacionService.
func NewEvaluacionService(repo EvaluacionRepository) *EvaluacionService {
	return &EvaluacionService{repo: repo}
}

func (s *EvaluacionService) CreateEvaluacion(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error {
	if err := s.repo.Create(ctx, e, empresaID); err != nil {
		return fmt.Errorf("evaluacionService.CreateEvaluacion: %w", err)
	}
	return nil
}

func (s *EvaluacionService) ListEvaluaciones(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error) {
	return s.repo.ListByCliente(ctx, clienteID, empresaID)
}
