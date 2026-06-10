// Package clientes defines domain types and sentinel errors for the Clientes module.
package clientes

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status constants for clientes_cliente.estatus.
// 1 = Activo, 2 = Incompleto (default).
const (
	ClienteStatusActivo     = 1
	ClienteStatusIncompleto = 2
)

// Sentinel errors.
var (
	ErrClienteNotFound      = errors.New("cliente not found")
	ErrClienteHasDependents = errors.New("cliente has dependents")
	ErrFacturaExists        = errors.New("factura already exists for this cliente")
	ErrPresupuestoExists    = errors.New("presupuesto already exists for this cliente")
	ErrCalendarioExists     = errors.New("calendario already exists for this cliente")
	ErrActivationBlocked    = errors.New("activation blocked by quality gate")
	ErrLocalidadNotFound    = errors.New("localidad not found")
	ErrTenantMismatch       = errors.New("tenant mismatch")
)

// Cliente is the aggregate root for a client record.
type Cliente struct {
	ID                  uuid.UUID
	EmpresaID           int64
	RepresentanteID     uuid.UUID
	NombreComercial     string
	RazonSocial         string
	FechaInicioContrato *time.Time
	Estatus             int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Factura holds fiscal/billing data for a cliente (1:1).
// RFC is stored as AES-256 ciphertext in the DB; this struct holds the decrypted plaintext.
type Factura struct {
	ID              uuid.UUID
	ClienteID       uuid.UUID
	RFC             string // decrypted in-memory, never persisted raw
	RazonSocial     string
	DomicilioFiscal string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Presupuesto holds budget information for a cliente (1:1).
type Presupuesto struct {
	ID                uuid.UUID
	ClienteID         uuid.UUID
	PersonalRequerido *int
	MaterialEstimado  *string
	CostoMensual      *float64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CalendarioLaboral defines working days and non-working days for a cliente (1:1, JSONB).
type CalendarioLaboral struct {
	ClienteID     uuid.UUID
	SemanaLaboral map[string]any // e.g. {"monday":true,...}
	DiasInhabiles map[string]any // e.g. [] stored as map or use []any — JSON flexible
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Localidad represents a branch/site for a cliente (1:N).
type Localidad struct {
	ID              uuid.UUID
	ClienteID       uuid.UUID
	Nombre          string
	Direccion       string
	SupervisorID    uuid.UUID // FK to hr.employees omitted until hr module exists
	TipoLocalidadID int64
	Latitud         *float64
	Longitud        *float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ServiciosLocalidad links a localidad to a catalog service (M:N).
type ServiciosLocalidad struct {
	ID           uuid.UUID
	LocalidadID  uuid.UUID
	ServicioID   int64
	StatusActivo bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Horario defines entry/exit times for a localidad.
type Horario struct {
	ID               uuid.UUID
	LocalidadID      uuid.UUID
	HoraEntrada      string // "HH:MM" — time.Time not needed for time-only fields
	HoraSalida       string
	HoraComidaInicio *string
	HoraComidaFin    *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Herramienta represents a tool/equipment assigned to a localidad.
type Herramienta struct {
	ID               uuid.UUID
	LocalidadID      uuid.UUID
	Nombre           string
	Cantidad         int
	Especificaciones *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Actividad defines a recurring activity for a localidad.
type Actividad struct {
	ID          uuid.UUID
	LocalidadID uuid.UUID
	Descripcion string
	Frecuencia  string
	Orden       int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// EvaluacionServicio records a service evaluation for a cliente.
type EvaluacionServicio struct {
	ID          uuid.UUID
	ClienteID   uuid.UUID
	Puntuacion  int // 1–5
	Comentarios *string
	DeUsuarioID uuid.UUID
	Fecha       time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// QualityGateStatus summarises which sections are complete for a cliente activation check.
type QualityGateStatus struct {
	ClienteID     uuid.UUID
	FacturaOK     bool
	PresupuestoOK bool
	CalendarioOK  bool
	CanActivate   bool
}
