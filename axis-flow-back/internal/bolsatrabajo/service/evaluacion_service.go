package service

import (
	"context"
	"fmt"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/events"
	"axis-flow-back/internal/bolsatrabajo/repository"

	"github.com/google/uuid"
)

// EvaluacionService defines business operations for post-interview evaluations.
type EvaluacionService interface {
	Create(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
	GetByPostulacion(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
}

// evaluacionService is the production implementation of EvaluacionService.
type evaluacionService struct {
	repo repository.EvaluacionRepository
	pub  events.EventPublisher
}

// NewEvaluacionService constructs an evaluacionService.
func NewEvaluacionService(repo repository.EvaluacionRepository, pub events.EventPublisher) EvaluacionService {
	return &evaluacionService{repo: repo, pub: pub}
}

// Compile-time interface check.
var _ EvaluacionService = (*evaluacionService)(nil)

func validateScore(name string, v int) error {
	if v < 1 || v > 5 {
		return fmt.Errorf("evaluacion: %s score %d is out of range (must be 1..5): %w", name, v, bolsatrabajo.ErrNotFound)
	}
	return nil
}

func (s *evaluacionService) Create(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	if err := validateScore("Puntualidad", e.Puntualidad); err != nil {
		return nil, err
	}
	if err := validateScore("Cortesia", e.Cortesia); err != nil {
		return nil, err
	}
	if err := validateScore("SoftSkills", e.SoftSkills); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, e, empresaID); err != nil {
		return nil, err
	}

	_ = s.pub.Publish(ctx, "PostulacionEvaluada", map[string]any{
		"evaluacion_id":  e.ID,
		"postulacion_id": e.PostulacionID,
		"empresa_id":     empresaID,
	})

	return e, nil
}

func (s *evaluacionService) GetByPostulacion(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	return s.repo.GetByPostulacion(ctx, postulacionID, empresaID)
}
