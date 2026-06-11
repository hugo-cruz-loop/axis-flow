// Package formularios defines domain types, constants, repository port
// interfaces, and sentinel errors for the Formularios module.
//
// Mirrors the structure of internal/atencionseguimiento/domain.go (PR-1 of
// 09_AtencionSeguimiento_Service_Spec). Repository implementations live in
// internal/formularios/repository/ and land in PR-2.
package formularios

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Status constants — eventos_evento.status and eventos_evento_iniciado.status.
// ---------------------------------------------------------------------------

const (
	// EventoStatus values mirror the CHECK constraint on
	// formularios.eventos_evento.status.
	EventoStatusPendiente  = "pendiente"  // event created, not yet started
	EventoStatusIniciado   = "iniciado"   // check-in happened, execution in progress
	EventoStatusCompletado = "completado" // all forms submitted
	EventoStatusCancelado  = "cancelado"  // canceled (e.g. by EventoCancelado consumer)
)

// EventoIniciadoStatus values mirror the CHECK constraint on
// formularios.eventos_evento_iniciado.status.
const (
	IniciadoStatus = "iniciado"
	CompletadoStatus = "completado"
	CanceladoStatus  = "cancelado"
)

// ---------------------------------------------------------------------------
// Question type constants — formularios_pregunta.tipo_pregunta.
//
//   1 = Texto    (ShortText)
//   2 = Checkbox
//   3 = Rating
//   5 = Matriz   (Matrix — rows × columns of values)
//   8 = Foto     (Camera evidence)
//   11 = Firma   (Signature canvas)
//
// The CHECK constraint on the column enforces this closed set.
// ---------------------------------------------------------------------------

const (
	TipoPreguntaTexto    = 1
	TipoPreguntaCheckbox = 2
	TipoPreguntaRating   = 3
	TipoPreguntaMatriz   = 5
	TipoPreguntaFoto     = 8
	TipoPreguntaFirma    = 11
)

// ---------------------------------------------------------------------------
// Redis cache key patterns.
// ---------------------------------------------------------------------------

const (
	// KeyFormularioByEmpresa is the Redis key pattern for the cached
	// list of form templates belonging to an empresa.
	KeyFormularioByEmpresa = "formularios:empresa:%s:formularios"

	// KeyEventoByEmpCte is the Redis key pattern for the cached list of
	// pending events visible to an employee for a given client.
	KeyEventoByEmpCte = "formularios:empresa:%s:cliente:%s:eventos"

	// KeyRespuestasByIniciado is the Redis key pattern for the cached
	// list of respuestas belonging to a given check-in.
	KeyRespuestasByIniciado = "formularios:iniciado:%s:respuestas"

	// KeyReportePending is the Redis set pattern for reports that have
	// been requested but whose PDF has not yet been generated.
	KeyReportePending = "formularios:reportes:pending"

	// KeyReporteS3Lock is the Redis lock key used to deduplicate
	// concurrent PDF generation jobs for the same check-in.
	KeyReporteS3Lock = "formularios:reporte:lock:%s"
)

// ---------------------------------------------------------------------------
// Domain structs.
// ---------------------------------------------------------------------------

// Formulario is a form template header.
type Formulario struct {
	ID          uuid.UUID
	EmpresaID   uuid.UUID
	Nombre      string
	Descripcion string
	Activo      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Pregunta is a question that belongs to a Formulario.
type Pregunta struct {
	ID                   uuid.UUID
	FormularioID         uuid.UUID
	Orden                int
	TipoPregunta         int
	TextoPregunta        string
	Obligatoria          bool
	RespuestaPredefinida  json.RawMessage
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Evento is an operational event that bundles one or more form templates
// to be executed in the field for a given client.
type Evento struct {
	ID              uuid.UUID
	EmpresaID       uuid.UUID
	ClienteID       uuid.UUID
	LocalidadID     uuid.UUID
	Nombre          string
	Descripcion     string
	FechaProgramada time.Time
	Status          string
	CreatedAt       time.Time
}

// EventoFormulario is the M:N association row between an Evento and a
// Formulario that the event should execute.
type EventoFormulario struct {
	ID           uuid.UUID
	EventoID     uuid.UUID
	FormularioID uuid.UUID
	CreatedAt    time.Time
}

// EventoIniciado is the employee check-in that begins execution of the
// forms linked to an Evento.
type EventoIniciado struct {
	ID                       uuid.UUID
	EventoID                 uuid.UUID
	EmpleadoID               int64
	GeolocalizacionInicioLat *float64
	GeolocalizacionInicioLon *float64
	CheckInTime              time.Time
	Status                   string
	CreatedAt                time.Time
}

// Respuesta is a single answer captured in the field for a Pregunta as
// part of a running EventoIniciado.
type Respuesta struct {
	ID                          uuid.UUID
	EventoIniciadoID            uuid.UUID
	PreguntaID                  uuid.UUID
	RespuestaTexto              string
	RespuestaLista              json.RawMessage
	Evidencia1                  string
	Evidencia2                  string
	Evidencia3                  string
	DocumentoURL                string
	GeolocalizacionRespuestaLat *float64
	GeolocalizacionRespuestaLon *float64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// ---------------------------------------------------------------------------
// Repository port interfaces (implementations land in PR-2).
// ---------------------------------------------------------------------------

// FormularioRepository defines persistence operations for Formulario
// template headers.
type FormularioRepository interface {
	Create(ctx context.Context, f *Formulario) error
	GetByID(ctx context.Context, id, empresaID uuid.UUID) (*Formulario, error)
	ListByEmpresa(ctx context.Context, empresaID uuid.UUID, activo *bool, page, pageSize int) ([]*Formulario, int, error)
}

// PreguntaRepository defines persistence operations for Pregunta rows
// attached to a Formulario.
type PreguntaRepository interface {
	Create(ctx context.Context, p *Pregunta) error
	ListByFormulario(ctx context.Context, formularioID uuid.UUID) ([]*Pregunta, error)
}

// EventoRepository defines persistence operations for Evento headers and
// their check-in (EventoIniciado) lifecycle.
type EventoRepository interface {
	Create(ctx context.Context, e *Evento, formularioIDs []uuid.UUID) error
	GetByID(ctx context.Context, id, empresaID uuid.UUID) (*Evento, error)
	ListByEmpCte(ctx context.Context, empresaID, clienteID uuid.UUID, status *string, page, pageSize int) ([]*Evento, int, error)

	CreateIniciado(ctx context.Context, i *EventoIniciado) error

	// UpdateStatus flips an evento's status scoped to empresaID. PR-5
	// (5.2a) — the CancelEvento service method uses this to mark an
	// evento as 'cancelado' when the EventoCancelado consumer
	// receives a cross-domain event. Returns formularios.ErrNotFound
	// for unknown IDs or foreign tenants.
	UpdateStatus(ctx context.Context, id, empresaID uuid.UUID, status string) error

	// GetEmpresaIDByID looks up an evento's tenant by primary key
	// alone (NO empresaID filter). PR-5 (5.2b) — the cross-domain
	// EventoCancelado consumer does not have the tenant in the
	// message, so the service uses this method to resolve the
	// tenant scope and then call the tenant-scoped CancelEvento.
	// Returns formularios.ErrNotFound for unknown IDs.
	GetEmpresaIDByID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

// RespuestaRepository defines persistence operations for field answers
// captured during a running check-in.
type RespuestaRepository interface {
	Create(ctx context.Context, r *Respuesta) error
	ListByIniciado(ctx context.Context, iniciadoID uuid.UUID) ([]*Respuesta, error)
}

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	ErrNotFound     = errors.New("formularios: not found")
	ErrForbidden    = errors.New("formularios: forbidden")
	ErrConflict     = errors.New("formularios: conflict")
	ErrInvalidInput = errors.New("formularios: invalid input")
	// ErrInternal is the surfaced error class for any unexpected
	// infrastructure failure (AWS S3 error, Gotenberg HTTP error,
	// Redis timeout, etc.). PR-6 (6.2): the S3 transport wraps
	// every AWS error as ErrInternal with a generic message; the
	// underlying vendor detail is logged via slog at the service
	// layer (no PII / no bucket / no key in the surfaced error).
	ErrInternal = errors.New("formularios: internal")
)
