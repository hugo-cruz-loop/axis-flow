// Package empleados defines domain types and sentinel errors for the Empleados module.
package empleados

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status constants for empleados_empleado.status.
const (
	EmpleadoStatusActivo     = 1 // active employee
	EmpleadoStatusIncompleto = 2 // onboarding incomplete (default)
	EmpleadoStatusBaja       = 4 // terminated
)

// EstatusObservacion constants for asistencia biometric validation result.
const (
	ObservacionPendiente = 1 // pending biometric review
	ObservacionValidada  = 2 // face match passed
	ObservacionRechazada = 3 // face match failed
)

// EstatusRango constants for geographic range check on clock-in.
const (
	RangoEnRango    = 1 // within allowed range
	RangoFueraRango = 2 // outside allowed range
)

// TipoRegistro constants for attendance record type.
const (
	TipoEntradaLaboral = "ENTRADA_LABORAL"
	TipoSalidaLaboral  = "SALIDA_LABORAL"
	TipoEntradaComida  = "ENTRADA_COMIDA"
	TipoSalidaComida   = "SALIDA_COMIDA"
)

// Sentinel errors for the empleados domain.
var (
	ErrEmpleadoNotFound      = errors.New("empleado not found")
	ErrEmpleadoAlreadyExists = errors.New("empleado already exists")
	ErrDuplicateCURP         = errors.New("CURP already registered")
	ErrDuplicateNSS          = errors.New("NSS already registered")
	ErrDuplicateIdEmpleado   = errors.New("id_empleado already registered for this empresa")
	ErrTenantMismatch        = errors.New("tenant mismatch")
	ErrAsistenciaImmutable   = errors.New("attendance records are immutable")
	ErrDeviceNotFound        = errors.New("device not found")
	ErrInvalidMIMEType       = errors.New("invalid file type: only PDF, PNG, JPG allowed")
	ErrFileTooLarge          = errors.New("file exceeds size limit")
)

// Empleado is the aggregate root — maps to empleados.empleados_empleado.
type Empleado struct {
	NumEmpleado     int64     `json:"num_empleado"`
	IDEmpleado      string    `json:"id_empleado"`
	UsuarioID       uuid.UUID `json:"usuario_id"`
	EmpresaID       int64     `json:"empresa_id"`
	Nombre          string    `json:"nombre"`
	ApellidoPaterno string    `json:"apellido_paterno"`
	ApellidoMaterno string    `json:"apellido_materno,omitempty"`
	Status          int       `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Ubicacion holds address and government-ID data for an employee (1:1).
type Ubicacion struct {
	EmpleadoID     int64     `json:"empleado_id"`
	CURP           string    `json:"curp"`
	NSS            string    `json:"nss"`
	Calle          string    `json:"calle"`
	NumeroExterior string    `json:"numero_exterior"`
	NumeroInterior string    `json:"numero_interior,omitempty"`
	Colonia        string    `json:"colonia"`
	CodigoPostal   string    `json:"codigo_postal"`
	CiudadID       int64     `json:"ciudad_id"`
	EstadoID       int64     `json:"estado_id"`
	PaisID         int64     `json:"pais_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Adicionales holds emergency contacts and beneficiaries for an employee (1:1).
type Adicionales struct {
	EmpleadoID                   int64            `json:"empleado_id"`
	ContactoEmergenciaNombre     string           `json:"contacto_emergencia_nombre"`
	ContactoEmergenciaTelefono   string           `json:"contacto_emergencia_telefono"`
	ContactoEmergenciaParentesco string           `json:"contacto_emergencia_parentesco"`
	Beneficiarios                []map[string]any `json:"beneficiarios"`
	CreatedAt                    time.Time        `json:"created_at"`
	UpdatedAt                    time.Time        `json:"updated_at"`
}

// Documentos holds document URLs for an employee's digital dossier (1:1).
type Documentos struct {
	EmpleadoID              int64     `json:"empleado_id"`
	ActaURL                 *string   `json:"acta_url,omitempty"`
	INEURL                  *string   `json:"ine_url,omitempty"`
	ComprobanteDomicilioURL *string   `json:"comprobante_domicilio_url,omitempty"`
	CURPPdfURL              *string   `json:"curp_pdf_url,omitempty"`
	NSSPdfURL               *string   `json:"nss_pdf_url,omitempty"`
	ContratoURL             *string   `json:"contrato_url,omitempty"`
	EstatusValidacion       int       `json:"estatus_validacion"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// Asistencia records a single clock-in/out event — IMMUTABLE after creation.
type Asistencia struct {
	ID                        uuid.UUID `json:"id"`
	EmpleadoID                int64     `json:"empleado_id"`
	Geolocalizacion           string    `json:"geolocalizacion"`
	EstatusRango              int       `json:"estatus_rango"`
	FotoEntradaURL            string    `json:"foto_entrada_url"`
	EstatusObservacionEntrada int       `json:"estatus_observacion_entrada"`
	SimilitudFacial           *float64  `json:"similitud_facial,omitempty"`
	TipoRegistro              string    `json:"tipo_registro"`
	Fecha                     time.Time `json:"fecha"`
	HoraEntrada               string    `json:"hora_entrada"` // "HH:MM" string — time-only
	HoraSalida                *string   `json:"hora_salida,omitempty"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// AsistenciaComida records the lunch break start/end for an attendance record (1:1).
type AsistenciaComida struct {
	AsistenciaID uuid.UUID `json:"asistencia_id"`
	HoraSalida   string    `json:"hora_salida"`
	HoraEntrada  *string   `json:"hora_entrada,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Inasistencia records an absence or incidence for an employee.
type Inasistencia struct {
	ID              uuid.UUID  `json:"id"`
	EmpleadoID      int64      `json:"empleado_id"`
	TipoIncidencia  string     `json:"tipo_incidencia"`
	FechaInicio     time.Time  `json:"fecha_inicio"`
	FechaFin        time.Time  `json:"fecha_fin"`
	JustificanteURL *string    `json:"justificante_url,omitempty"`
	Aprobado        bool       `json:"aprobado"`
	Observaciones   *string    `json:"observaciones,omitempty"`
	ResolvedBy      *uuid.UUID `json:"resolved_by,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Fotologin stores a biometric base photo used for face recognition.
type Fotologin struct {
	ID          uuid.UUID `json:"id"`
	EmpleadoID  int64     `json:"empleado_id"`
	FotoBaseURL string    `json:"foto_base_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserDevice represents an authorized mobile device for an employee.
// FCMToken MUST NEVER be logged.
type UserDevice struct {
	ID          uuid.UUID `json:"id"`
	EmpleadoID  int64     `json:"empleado_id"`
	DeviceUUID  string    `json:"device_uuid"`
	DeviceModel *string   `json:"device_model,omitempty"`
	OSVersion   *string   `json:"os_version,omitempty"`
	FCMToken    string    `json:"-"` // NEVER LOG — excluded from JSON marshaling
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
