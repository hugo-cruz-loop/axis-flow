package main

import (
	"context"

	"axis-flow-back/internal/atencionseguimiento"
	atencionhandler "axis-flow-back/internal/atencionseguimiento/handler"
	"axis-flow-back/internal/atencionseguimiento/events"

	"github.com/google/uuid"
)

// ── stubQuejaSvc ──────────────────────────────────────────────────────────────

type stubQuejaSvc struct{}

func (s *stubQuejaSvc) CreateQueja(_ context.Context, q *atencionseguimiento.SolicitudQueja, _ int64, _ uuid.UUID) (*atencionseguimiento.SolicitudQueja, *atencionseguimiento.RespuestaQueja, error) {
	return q, &atencionseguimiento.RespuestaQueja{}, nil
}
func (s *stubQuejaSvc) GetQueja(_ context.Context, _ uuid.UUID, _ int64, _ string) (*atencionseguimiento.SolicitudQueja, error) {
	return &atencionseguimiento.SolicitudQueja{}, nil
}
func (s *stubQuejaSvc) GetQuejasByEmpresa(_ context.Context, _ uuid.UUID, _ *int, _, _ int) ([]*atencionseguimiento.SolicitudQueja, int, error) {
	return nil, 0, nil
}
func (s *stubQuejaSvc) CreateMensaje(_ context.Context, _ uuid.UUID, msg *atencionseguimiento.RespuestaQueja, _ int64, _ string) (*atencionseguimiento.RespuestaQueja, error) {
	return msg, nil
}
func (s *stubQuejaSvc) GetMensajes(_ context.Context, _ uuid.UUID, _ int64, _ string, _, _ int) ([]*atencionseguimiento.RespuestaQueja, int, error) {
	return nil, 0, nil
}
func (s *stubQuejaSvc) SuspendQuejasByEmpleado(_ context.Context, _ int64) error { return nil }

// ── stubTicketSvc ─────────────────────────────────────────────────────────────

type stubTicketSvc struct{}

func (s *stubTicketSvc) CreateTicket(_ context.Context, t *atencionseguimiento.TicketServicio, _, _ uuid.UUID) (*atencionseguimiento.TicketServicio, *atencionseguimiento.RespuestaServicio, error) {
	return t, &atencionseguimiento.RespuestaServicio{}, nil
}
func (s *stubTicketSvc) GetTicketsByCliente(_ context.Context, _, _, _ uuid.UUID, _ string, _, _ int) ([]*atencionseguimiento.TicketServicio, int, error) {
	return nil, 0, nil
}
func (s *stubTicketSvc) UpdateTicketEstatus(_ context.Context, _, _ uuid.UUID, _ int, _ uuid.UUID) (*atencionseguimiento.TicketServicio, error) {
	return &atencionseguimiento.TicketServicio{}, nil
}
func (s *stubTicketSvc) GetMensajesServicio(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _, _ int) ([]*atencionseguimiento.RespuestaServicio, int, error) {
	return nil, 0, nil
}
func (s *stubTicketSvc) CreateMensajeServicio(_ context.Context, _ uuid.UUID, msg *atencionseguimiento.RespuestaServicio, _ uuid.UUID, _ string) (*atencionseguimiento.RespuestaServicio, error) {
	return msg, nil
}
func (s *stubTicketSvc) GetTicketStats(_ context.Context, _ uuid.UUID) (*atencionseguimiento.TicketStats, error) {
	return &atencionseguimiento.TicketStats{}, nil
}
func (s *stubTicketSvc) CloseTicketsByCliente(_ context.Context, _ uuid.UUID) error { return nil }

// ── stubIncidenciaSvc ─────────────────────────────────────────────────────────

type stubIncidenciaSvc struct{}

func (s *stubIncidenciaSvc) CreateIncidencia(_ context.Context, inc *atencionseguimiento.IncidenciaSupervisor, _, _ uuid.UUID) (*atencionseguimiento.IncidenciaSupervisor, error) {
	return inc, nil
}
func (s *stubIncidenciaSvc) GetIncidenciasByEmpresa(_ context.Context, _ uuid.UUID, _, _ int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error) {
	return nil, 0, nil
}

// ── factory ───────────────────────────────────────────────────────────────────

func newAtencionModuleForTest() *atencionModule {
	quejaSvc := &stubQuejaSvc{}
	ticketSvc := &stubTicketSvc{}

	return &atencionModule{
		quejaH:                  atencionhandler.NewQuejaHandler(quejaSvc),
		ticketH:                 atencionhandler.NewTicketHandler(ticketSvc),
		incidenciaH:             atencionhandler.NewIncidenciaHandler(&stubIncidenciaSvc{}),
		EmpleadoDeBajaConsumer:  events.NewEmpleadoDeBajaConsumer(nil, quejaSvc),
		ClienteInactivoConsumer: events.NewClienteInactivoConsumer(nil, ticketSvc),
	}
}
