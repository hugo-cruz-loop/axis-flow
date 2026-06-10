// Package service implements use-case logic for the AtencionSeguimiento module.
package service

import (
	"context"
	"time"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/events"
	"axis-flow-back/internal/atencionseguimiento/repository"

	"github.com/google/uuid"
)

// QuejaService defines use-case operations for labor complaints (quejas).
type QuejaService interface {
	// CreateQueja creates a new queja and its initial message. Returns both.
	CreateQueja(ctx context.Context, q *atencionseguimiento.SolicitudQueja, empleadoID int64, empresaID uuid.UUID) (*atencionseguimiento.SolicitudQueja, *atencionseguimiento.RespuestaQueja, error)

	// GetQueja retrieves a queja. Enforces IDOR for Empleado role.
	GetQueja(ctx context.Context, id uuid.UUID, requesterEmpleadoID int64, requesterRole string) (*atencionseguimiento.SolicitudQueja, error)

	// GetQuejasByEmpresa lists quejas for an empresa with optional estatus filter.
	GetQuejasByEmpresa(ctx context.Context, empresaID uuid.UUID, estatus *int, page, pageSize int) ([]*atencionseguimiento.SolicitudQueja, int, error)

	// CreateMensaje appends a reply to a queja thread. Enforces IDOR.
	CreateMensaje(ctx context.Context, solicitudID uuid.UUID, msg *atencionseguimiento.RespuestaQueja, requesterEmpleadoID int64, requesterRole string) (*atencionseguimiento.RespuestaQueja, error)

	// GetMensajes lists replies for a queja thread. Enforces IDOR.
	GetMensajes(ctx context.Context, solicitudID uuid.UUID, requesterEmpleadoID int64, requesterRole string, page, pageSize int) ([]*atencionseguimiento.RespuestaQueja, int, error)

	// SuspendQuejasByEmpleado suspends all open quejas for a departing employee.
	SuspendQuejasByEmpleado(ctx context.Context, empleadoID int64) error
}

// quejaService is the concrete implementation.
type quejaService struct {
	repo repository.QuejaRepository
	pub  events.EventPublisher
}

// NewQuejaService constructs a QuejaService.
func NewQuejaService(repo repository.QuejaRepository, pub events.EventPublisher) QuejaService {
	return &quejaService{repo: repo, pub: pub}
}

func (s *quejaService) CreateQueja(ctx context.Context, q *atencionseguimiento.SolicitudQueja, empleadoID int64, empresaID uuid.UUID) (*atencionseguimiento.SolicitudQueja, *atencionseguimiento.RespuestaQueja, error) {
	q.ID = uuid.New()
	q.EmpleadoID = empleadoID
	q.EmpresaID = empresaID
	q.Estatus = atencionseguimiento.EstatusPendiente
	q.UltimaResp = atencionseguimiento.RolEmpleadoCliente
	now := time.Now()
	q.CreatedAt = now
	q.UpdatedAt = now

	if err := s.repo.Create(ctx, q); err != nil {
		return nil, nil, err
	}

	// Create initial RespuestaQueja as rol=1 (employee/client).
	msg := &atencionseguimiento.RespuestaQueja{
		ID:           uuid.New(),
		SolicitudID:  q.ID,
		RemitenteID:  uuid.New(), // placeholder — caller should set RemitenteID before calling
		RolRespuesta: atencionseguimiento.RolEmpleadoCliente,
		Mensaje:      q.Descripcion,
		CreatedAt:    now,
	}
	if err := s.repo.CreateRespuesta(ctx, msg); err != nil {
		return nil, nil, err
	}

	_ = s.pub.Publish(ctx, events.StreamQuejaRegistrada, map[string]any{
		"queja_id":    q.ID,
		"empresa_id":  empresaID,
		"empleado_id": empleadoID,
	})

	return q, msg, nil
}

func (s *quejaService) GetQueja(ctx context.Context, id uuid.UUID, requesterEmpleadoID int64, requesterRole string) (*atencionseguimiento.SolicitudQueja, error) {
	// For Empleado we pass requesterEmpleadoID so the repo enforces IDOR.
	// For RH/Admin we pass 0 (no IDOR check in repo).
	var checkID int64
	if requesterRole == "Empleado" {
		checkID = requesterEmpleadoID
	}

	q, err := s.repo.GetByID(ctx, id, checkID)
	if err != nil {
		return nil, err
	}

	// Additional service-layer IDOR: if Empleado, verify the returned queja
	// belongs to the requester.
	if requesterRole == "Empleado" && q.EmpleadoID != requesterEmpleadoID {
		return nil, atencionseguimiento.ErrForbidden
	}

	return q, nil
}

func (s *quejaService) GetQuejasByEmpresa(ctx context.Context, empresaID uuid.UUID, estatus *int, page, pageSize int) ([]*atencionseguimiento.SolicitudQueja, int, error) {
	return s.repo.ListByEmpresa(ctx, empresaID, estatus, page, pageSize)
}

func (s *quejaService) CreateMensaje(ctx context.Context, solicitudID uuid.UUID, msg *atencionseguimiento.RespuestaQueja, requesterEmpleadoID int64, requesterRole string) (*atencionseguimiento.RespuestaQueja, error) {
	if requesterRole == "Empleado" {
		// Verify ownership: fetch queja with no IDOR override (pass 0 to skip
		// repo-level IDOR, then check at service level).
		q, err := s.repo.GetByID(ctx, solicitudID, 0)
		if err != nil {
			return nil, err
		}
		if q.EmpleadoID != requesterEmpleadoID {
			return nil, atencionseguimiento.ErrForbidden
		}
	}

	msg.ID = uuid.New()
	msg.SolicitudID = solicitudID
	msg.CreatedAt = time.Now()

	if err := s.repo.CreateRespuesta(ctx, msg); err != nil {
		return nil, err
	}

	_ = s.pub.Publish(ctx, events.StreamQuejaRespuestaEnviada, map[string]any{
		"queja_id":    solicitudID,
		"mensaje_id":  msg.ID,
		"remitente_id": msg.RemitenteID,
	})

	return msg, nil
}

func (s *quejaService) GetMensajes(ctx context.Context, solicitudID uuid.UUID, requesterEmpleadoID int64, requesterRole string, page, pageSize int) ([]*atencionseguimiento.RespuestaQueja, int, error) {
	if requesterRole == "Empleado" {
		q, err := s.repo.GetByID(ctx, solicitudID, 0)
		if err != nil {
			return nil, 0, err
		}
		if q.EmpleadoID != requesterEmpleadoID {
			return nil, 0, atencionseguimiento.ErrForbidden
		}
	}
	return s.repo.GetRespuestas(ctx, solicitudID, page, pageSize)
}

func (s *quejaService) SuspendQuejasByEmpleado(ctx context.Context, empleadoID int64) error {
	return s.repo.SuspendQuejasByEmpleado(ctx, empleadoID)
}
