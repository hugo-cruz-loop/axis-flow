// Package atencionseguimiento defines domain types, constants, repository
// interfaces, and sentinel errors for the AtencionSeguimiento module.
package atencionseguimiento

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Estatus constants — shared across solicitudes_queja and tickets_servicio.
// ---------------------------------------------------------------------------

const (
	EstatusPendiente  = 1 // awaiting first response
	EstatusEnProceso  = 2 // active conversation in progress
	EstatusFinalizado = 3 // closed / resolved
)

// ---------------------------------------------------------------------------
// UltimaResp — tracks which role last replied in a conversation thread.
// ---------------------------------------------------------------------------

const (
	RolEmpleadoCliente = 1 // employee (queja) or client (ticket) last replied
	RolRHGestor        = 2 // HR / gestor last replied
	RolSupervisor      = 3 // supervisor last replied (queja only)
)

// ---------------------------------------------------------------------------
// Redis cache key patterns.
// ---------------------------------------------------------------------------

const (
	// KeyClienteUnreadTickets is the Redis key pattern for unread ticket count
	// per client: fmt.Sprintf(KeyClienteUnreadTickets, clienteID).
	KeyClienteUnreadTickets = "atencion:cliente:%s:unread_tickets"

	// KeyClienteActiveTickets is the Redis key pattern for active tickets list
	// per client.
	KeyClienteActiveTickets = "atencion:cliente:%s:active_tickets"

	// KeyEmpleadoUnreadComplaints is the Redis key pattern for unread complaint
	// messages per employee.
	KeyEmpleadoUnreadComplaints = "atencion:empleado:%s:unread_complaints"

	// KeyEmpresaUnreadComplaints is the Redis key pattern for total unread
	// complaint messages across an empresa.
	KeyEmpresaUnreadComplaints = "atencion:empresa:%s:unread_complaints"

	// KeyEmpresaUnreadTickets is the Redis key pattern for total unread ticket
	// messages across an empresa.
	KeyEmpresaUnreadTickets = "atencion:empresa:%s:unread_tickets"
)

// ---------------------------------------------------------------------------
// Domain structs.
// ---------------------------------------------------------------------------

// SolicitudQueja represents an employee labor complaint.
type SolicitudQueja struct {
	ID            uuid.UUID
	EmpresaID     uuid.UUID
	EmpleadoID    int64
	TipoQuejaID   uuid.UUID
	Titulo        string
	Descripcion   string
	Estatus       int
	FechaVigencia *time.Time
	UltimaResp    int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// RespuestaQueja represents a single chat message in a queja thread.
type RespuestaQueja struct {
	ID                uuid.UUID
	SolicitudID       uuid.UUID
	RemitenteID       uuid.UUID
	RolRespuesta      int
	Mensaje           string
	ArchivoAdjuntoURL string
	Leido             bool
	CreatedAt         time.Time
}

// TicketServicio represents a client service ticket.
type TicketServicio struct {
	ID          uuid.UUID
	EmpresaID   uuid.UUID
	ClienteID   uuid.UUID
	LocalidadID uuid.UUID
	Asunto      string
	Descripcion string
	Estatus     int
	UltimaResp  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RespuestaServicio represents a single chat message in a ticket thread.
type RespuestaServicio struct {
	ID                uuid.UUID
	TicketID          uuid.UUID
	RemitenteID       uuid.UUID
	RolRespuesta      int
	Mensaje           string
	ArchivoAdjuntoURL string
	Leido             bool
	CreatedAt         time.Time
}

// IncidenciaSupervisor represents a supervisor-filed incident report about an
// employee.
type IncidenciaSupervisor struct {
	ID               uuid.UUID
	EmpresaID        uuid.UUID
	SupervisorID     uuid.UUID
	EmpleadoID       int64
	LocalidadID      uuid.UUID
	TipoIncidenciaID uuid.UUID
	Descripcion      string
	SancionSugerida  string
	EvidenciaURL     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// TicketStats aggregates ticket counts per estatus for an empresa.
type TicketStats struct {
	EmpresaID  uuid.UUID
	Pendiente  int
	EnProceso  int
	Finalizado int
}

// ---------------------------------------------------------------------------
// Repository interfaces (ports).
// ---------------------------------------------------------------------------

// SolicitudQuejaRepository defines persistence operations for queja entities.
type SolicitudQuejaRepository interface {
	Create(ctx context.Context, s *SolicitudQueja) error
	GetByID(ctx context.Context, id uuid.UUID, empleadoID int64) (*SolicitudQueja, error)
	ListByEmpresa(ctx context.Context, empresaID uuid.UUID, estatus *int, page, pageSize int) ([]*SolicitudQueja, int, error)
	UpdateEstatus(ctx context.Context, id uuid.UUID, estatus int) error

	CreateRespuesta(ctx context.Context, r *RespuestaQueja) error
	ListRespuestas(ctx context.Context, solicitudID uuid.UUID) ([]*RespuestaQueja, error)
}

// TicketServicioRepository defines persistence operations for ticket entities.
type TicketServicioRepository interface {
	Create(ctx context.Context, t *TicketServicio) error
	GetByID(ctx context.Context, id, clienteID uuid.UUID) (*TicketServicio, error)
	ListByCliente(ctx context.Context, clienteID uuid.UUID, estatus *int, page, pageSize int) ([]*TicketServicio, int, error)
	UpdateEstatus(ctx context.Context, id uuid.UUID, estatus, ultimaResp int) error
	GetStatsByEmpresa(ctx context.Context, empresaID uuid.UUID) (*TicketStats, error)

	CreateRespuesta(ctx context.Context, r *RespuestaServicio) error
	ListRespuestas(ctx context.Context, ticketID uuid.UUID) ([]*RespuestaServicio, error)
}

// IncidenciaSupervisorRepository defines persistence operations for incidencia
// entities.
type IncidenciaSupervisorRepository interface {
	Create(ctx context.Context, i *IncidenciaSupervisor) error
	ListByEmpresa(ctx context.Context, empresaID uuid.UUID, page, pageSize int) ([]*IncidenciaSupervisor, int, error)
}

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	ErrNotFound         = errors.New("atencionseguimiento: not found")
	ErrForbidden        = errors.New("atencionseguimiento: forbidden")
	ErrConflict         = errors.New("atencionseguimiento: conflict")
	ErrInvalidInput     = errors.New("atencionseguimiento: invalid input")
	ErrSLAConfigMissing = errors.New("atencionseguimiento: SLA config missing")
)
