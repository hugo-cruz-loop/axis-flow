package service

import (
	"context"
	"time"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/events"
	"axis-flow-back/internal/atencionseguimiento/repository"

	"github.com/google/uuid"
)

// TicketService defines use-case operations for client service tickets.
type TicketService interface {
	// CreateTicket creates a new ticket and its initial message. Returns both.
	CreateTicket(ctx context.Context, t *atencionseguimiento.TicketServicio, clienteID, empresaID uuid.UUID) (*atencionseguimiento.TicketServicio, *atencionseguimiento.RespuestaServicio, error)

	// GetTicketsByCliente lists tickets for a client scoped to an empresa. Enforces IDOR for Cliente role.
	GetTicketsByCliente(ctx context.Context, clienteID, empresaID uuid.UUID, requesterClienteID uuid.UUID, requesterRole string, page, pageSize int) ([]*atencionseguimiento.TicketServicio, int, error)

	// UpdateTicketEstatus transitions a ticket to a new estatus. Publishes event with
	// both the previous and new status.
	UpdateTicketEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int, updatedByID uuid.UUID) (*atencionseguimiento.TicketServicio, error)

	// GetMensajesServicio lists messages for a ticket thread.
	GetMensajesServicio(ctx context.Context, ticketID uuid.UUID, requesterClienteID uuid.UUID, requesterRole string, page, pageSize int) ([]*atencionseguimiento.RespuestaServicio, int, error)

	// CreateMensajeServicio appends a message to a ticket thread.
	CreateMensajeServicio(ctx context.Context, ticketID uuid.UUID, msg *atencionseguimiento.RespuestaServicio, requesterClienteID uuid.UUID, requesterRole string) (*atencionseguimiento.RespuestaServicio, error)

	// GetTicketStats returns ticket counts per estatus for an empresa.
	GetTicketStats(ctx context.Context, empresaID uuid.UUID) (*atencionseguimiento.TicketStats, error)

	// CloseTicketsByCliente closes all open tickets for an inactive client.
	CloseTicketsByCliente(ctx context.Context, clienteID uuid.UUID) error
}

// ticketService is the concrete implementation.
type ticketService struct {
	repo repository.TicketRepository
	pub  events.EventPublisher
}

// NewTicketService constructs a TicketService.
func NewTicketService(repo repository.TicketRepository, pub events.EventPublisher) TicketService {
	return &ticketService{repo: repo, pub: pub}
}

func (s *ticketService) CreateTicket(ctx context.Context, t *atencionseguimiento.TicketServicio, clienteID, empresaID uuid.UUID) (*atencionseguimiento.TicketServicio, *atencionseguimiento.RespuestaServicio, error) {
	t.ID = uuid.New()
	t.ClienteID = clienteID
	t.EmpresaID = empresaID
	t.Estatus = atencionseguimiento.EstatusPendiente
	t.UltimaResp = atencionseguimiento.RolEmpleadoCliente
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	if err := s.repo.CreateTicket(ctx, t); err != nil {
		return nil, nil, err
	}

	msg := &atencionseguimiento.RespuestaServicio{
		ID:           uuid.New(),
		TicketID:     t.ID,
		RemitenteID:  clienteID,
		RolRespuesta: atencionseguimiento.RolEmpleadoCliente,
		Mensaje:      t.Descripcion,
		Leido:        false,
		CreatedAt:    now,
	}
	if err := s.repo.CreateRespuestaServicio(ctx, msg); err != nil {
		return nil, nil, err
	}

	_ = s.pub.Publish(ctx, events.StreamTicketServicioCreado, map[string]any{
		"ticket_id":  t.ID,
		"empresa_id": empresaID,
		"cliente_id": clienteID,
	})

	return t, msg, nil
}

func (s *ticketService) GetTicketsByCliente(ctx context.Context, clienteID, empresaID uuid.UUID, requesterClienteID uuid.UUID, requesterRole string, page, pageSize int) ([]*atencionseguimiento.TicketServicio, int, error) {
	if requesterRole == "Cliente" && clienteID != requesterClienteID {
		return nil, 0, atencionseguimiento.ErrForbidden
	}
	return s.repo.GetTicketsByCliente(ctx, clienteID, empresaID, page, pageSize)
}

func (s *ticketService) UpdateTicketEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int, updatedByID uuid.UUID) (*atencionseguimiento.TicketServicio, error) {
	if newEstatus < 1 || newEstatus > 3 {
		return nil, atencionseguimiento.ErrInvalidInput
	}

	t, err := s.repo.GetTicket(ctx, id)
	if err != nil {
		return nil, err
	}

	// Tenant check.
	if t.EmpresaID != empresaID {
		return nil, atencionseguimiento.ErrForbidden
	}

	prevEstatus := t.Estatus

	updated, err := s.repo.UpdateTicketEstatus(ctx, id, empresaID, newEstatus)
	if err != nil {
		return nil, err
	}

	_ = s.pub.Publish(ctx, events.StreamTicketServicioActualizado, map[string]any{
		"ticket_id":        id,
		"empresa_id":       empresaID,
		"estatus_anterior": prevEstatus,
		"estatus_nuevo":    newEstatus,
		"updated_by":       updatedByID,
	})

	return updated, nil
}

func (s *ticketService) GetMensajesServicio(ctx context.Context, ticketID uuid.UUID, requesterClienteID uuid.UUID, requesterRole string, page, pageSize int) ([]*atencionseguimiento.RespuestaServicio, int, error) {
	if requesterRole == "Cliente" {
		t, err := s.repo.GetTicket(ctx, ticketID)
		if err != nil {
			return nil, 0, err
		}
		if t.ClienteID != requesterClienteID {
			return nil, 0, atencionseguimiento.ErrForbidden
		}
	}
	return s.repo.GetRespuestasServicio(ctx, ticketID, page, pageSize)
}

func (s *ticketService) CreateMensajeServicio(ctx context.Context, ticketID uuid.UUID, msg *atencionseguimiento.RespuestaServicio, requesterClienteID uuid.UUID, requesterRole string) (*atencionseguimiento.RespuestaServicio, error) {
	if requesterRole == "Cliente" {
		t, err := s.repo.GetTicket(ctx, ticketID)
		if err != nil {
			return nil, err
		}
		if t.ClienteID != requesterClienteID {
			return nil, atencionseguimiento.ErrForbidden
		}
	}

	msg.ID = uuid.New()
	msg.TicketID = ticketID
	msg.Leido = false
	msg.CreatedAt = time.Now()

	if err := s.repo.CreateRespuestaServicio(ctx, msg); err != nil {
		return nil, err
	}

	_ = s.pub.Publish(ctx, events.StreamQuejaRespuestaEnviada, map[string]any{
		"ticket_id":    ticketID,
		"mensaje_id":   msg.ID,
		"remitente_id": msg.RemitenteID,
	})

	return msg, nil
}

func (s *ticketService) GetTicketStats(ctx context.Context, empresaID uuid.UUID) (*atencionseguimiento.TicketStats, error) {
	return s.repo.GetTicketStats(ctx, empresaID)
}

func (s *ticketService) CloseTicketsByCliente(ctx context.Context, clienteID uuid.UUID) error {
	return s.repo.CloseTicketsByCliente(ctx, clienteID)
}
