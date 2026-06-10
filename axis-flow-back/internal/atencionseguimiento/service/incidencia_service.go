package service

import (
	"context"
	"time"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/events"
	"axis-flow-back/internal/atencionseguimiento/repository"

	"github.com/google/uuid"
)

// IncidenciaService defines use-case operations for supervisor incident reports.
type IncidenciaService interface {
	// CreateIncidencia validates the tenant, persists the record, and publishes an event.
	CreateIncidencia(ctx context.Context, inc *atencionseguimiento.IncidenciaSupervisor, supervisorID, empresaID uuid.UUID) (*atencionseguimiento.IncidenciaSupervisor, error)

	// GetIncidenciasByEmpresa lists incidents for an empresa with pagination.
	GetIncidenciasByEmpresa(ctx context.Context, empresaID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error)
}

// incidenciaService is the concrete implementation.
type incidenciaService struct {
	repo repository.IncidenciaRepository
	pub  events.EventPublisher
}

// NewIncidenciaService constructs an IncidenciaService.
func NewIncidenciaService(repo repository.IncidenciaRepository, pub events.EventPublisher) IncidenciaService {
	return &incidenciaService{repo: repo, pub: pub}
}

func (s *incidenciaService) CreateIncidencia(ctx context.Context, inc *atencionseguimiento.IncidenciaSupervisor, supervisorID, empresaID uuid.UUID) (*atencionseguimiento.IncidenciaSupervisor, error) {
	// Tenant check: body EmpresaID must match JWT empresaID.
	if inc.EmpresaID != empresaID {
		return nil, atencionseguimiento.ErrForbidden
	}

	inc.ID = uuid.New()
	inc.SupervisorID = supervisorID
	now := time.Now()
	inc.CreatedAt = now
	inc.UpdatedAt = now

	if err := s.repo.Create(ctx, inc); err != nil {
		return nil, err
	}

	_ = s.pub.Publish(ctx, events.StreamIncidenciaOperativaRegistrada, map[string]any{
		"incidencia_id": inc.ID,
		"empresa_id":    empresaID,
		"supervisor_id": supervisorID,
		"empleado_id":   inc.EmpleadoID,
	})

	return inc, nil
}

func (s *incidenciaService) GetIncidenciasByEmpresa(ctx context.Context, empresaID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error) {
	return s.repo.ListByEmpresa(ctx, empresaID, page, pageSize)
}
