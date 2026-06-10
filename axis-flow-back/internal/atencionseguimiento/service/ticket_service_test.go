package service_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Inline mock — TicketRepository (matches repository.TicketRepository interface)
// ---------------------------------------------------------------------------

type mockTicketRepo struct {
	ticket    *atencionseguimiento.TicketServicio
	createErr error
	getErr    error

	lastEstatusUpdate int
	lastUpdatedID     uuid.UUID

	createdRespuesta *atencionseguimiento.RespuestaServicio
	closed           []uuid.UUID
}

func (m *mockTicketRepo) CreateTicket(_ context.Context, t *atencionseguimiento.TicketServicio) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.ticket = t
	return nil
}

func (m *mockTicketRepo) GetTicket(_ context.Context, id uuid.UUID) (*atencionseguimiento.TicketServicio, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.ticket != nil {
		return m.ticket, nil
	}
	return &atencionseguimiento.TicketServicio{ID: id}, nil
}

func (m *mockTicketRepo) GetTicketsByCliente(_ context.Context, _, _ uuid.UUID, _, _ int) ([]*atencionseguimiento.TicketServicio, int, error) {
	return nil, 0, nil
}

func (m *mockTicketRepo) UpdateTicketEstatus(_ context.Context, id uuid.UUID, _ uuid.UUID, estatus int) (*atencionseguimiento.TicketServicio, error) {
	m.lastUpdatedID = id
	m.lastEstatusUpdate = estatus
	if m.ticket != nil {
		m.ticket.Estatus = estatus
		return m.ticket, nil
	}
	return &atencionseguimiento.TicketServicio{ID: id, Estatus: estatus}, nil
}

func (m *mockTicketRepo) GetTicketStats(_ context.Context, _ uuid.UUID) (*atencionseguimiento.TicketStats, error) {
	return &atencionseguimiento.TicketStats{}, nil
}

func (m *mockTicketRepo) CreateRespuestaServicio(_ context.Context, r *atencionseguimiento.RespuestaServicio) error {
	m.createdRespuesta = r
	return nil
}

func (m *mockTicketRepo) GetRespuestasServicio(_ context.Context, _ uuid.UUID, _, _ int) ([]*atencionseguimiento.RespuestaServicio, int, error) {
	return nil, 0, nil
}

func (m *mockTicketRepo) CloseTicketsByCliente(_ context.Context, clienteID uuid.UUID) error {
	m.closed = append(m.closed, clienteID)
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTicketService_CreateTicket_PublishesAndReturnsBoth(t *testing.T) {
	repo := &mockTicketRepo{}
	pub := &spyPublisher{}
	svc := service.NewTicketService(repo, pub)

	clienteID := uuid.New()
	empresaID := uuid.New()
	ticket := &atencionseguimiento.TicketServicio{
		ClienteID:   clienteID,
		EmpresaID:   empresaID,
		Asunto:      "soporte",
		Descripcion: "desc",
	}

	created, msg, err := svc.CreateTicket(context.Background(), ticket, clienteID, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created == nil {
		t.Fatal("expected ticket, got nil")
	}
	if msg == nil {
		t.Fatal("expected mensaje_inicial, got nil")
	}
	if !pub.called {
		t.Fatal("expected event to be published")
	}
	if pub.stream != "atencion:ticket_servicio_creado" {
		t.Fatalf("wrong stream: %s", pub.stream)
	}
}

func TestTicketService_UpdateTicketEstatus_PublishesWithOldAndNew(t *testing.T) {
	ticketID := uuid.New()
	empresaID := uuid.New()
	repo := &mockTicketRepo{
		ticket: &atencionseguimiento.TicketServicio{
			ID:        ticketID,
			EmpresaID: empresaID,
			Estatus:   atencionseguimiento.EstatusPendiente,
		},
	}
	pub := &spyPublisher{}
	svc := service.NewTicketService(repo, pub)

	_, err := svc.UpdateTicketEstatus(context.Background(), ticketID, empresaID, atencionseguimiento.EstatusEnProceso, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pub.called {
		t.Fatal("expected event to be published")
	}
	if pub.stream != "atencion:ticket_servicio_actualizado" {
		t.Fatalf("wrong stream: %s", pub.stream)
	}
	if pub.payload["estatus_anterior"] != atencionseguimiento.EstatusPendiente {
		t.Fatalf("expected estatus_anterior=%d, got %v", atencionseguimiento.EstatusPendiente, pub.payload["estatus_anterior"])
	}
	if pub.payload["estatus_nuevo"] != atencionseguimiento.EstatusEnProceso {
		t.Fatalf("expected estatus_nuevo=%d, got %v", atencionseguimiento.EstatusEnProceso, pub.payload["estatus_nuevo"])
	}
}

func TestTicketService_UpdateTicketEstatus_InvalidEstatus(t *testing.T) {
	ticketID := uuid.New()
	empresaID := uuid.New()
	repo := &mockTicketRepo{
		ticket: &atencionseguimiento.TicketServicio{
			ID:        ticketID,
			EmpresaID: empresaID,
			Estatus:   atencionseguimiento.EstatusPendiente,
		},
	}
	pub := &spyPublisher{}
	svc := service.NewTicketService(repo, pub)

	_, err := svc.UpdateTicketEstatus(context.Background(), ticketID, empresaID, 99, uuid.New())
	if err == nil {
		t.Fatal("expected ErrInvalidInput, got nil")
	}
	if err != atencionseguimiento.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTicketService_GetTicketsByCliente_ClienteForbiddenOnMismatch(t *testing.T) {
	repo := &mockTicketRepo{}
	pub := &spyPublisher{}
	svc := service.NewTicketService(repo, pub)

	ownerID := uuid.New()
	requesterID := uuid.New() // different from ownerID

	_, _, err := svc.GetTicketsByCliente(context.Background(), ownerID, uuid.New(), requesterID, "Cliente", 1, 10)
	if err == nil {
		t.Fatal("expected ErrForbidden, got nil")
	}
	if err != atencionseguimiento.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}
