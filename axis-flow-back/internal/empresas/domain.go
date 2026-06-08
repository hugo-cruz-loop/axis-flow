package empresas

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// EmpresaStatus represents the lifecycle status of an empresa.
type EmpresaStatus string

const (
	EmpresaStatusPendingPayment EmpresaStatus = "PENDING_PAYMENT"
	EmpresaStatusActive         EmpresaStatus = "ACTIVE"
	EmpresaStatusInactive       EmpresaStatus = "INACTIVE"
	EmpresaStatusSuspended      EmpresaStatus = "SUSPENDED"
)

// PagoStatus represents the payment status.
type PagoStatus string

const (
	PagoStatusPending  PagoStatus = "PENDING"
	PagoStatusPaid     PagoStatus = "PAID"
	PagoStatusFailed   PagoStatus = "FAILED"
	PagoStatusRefunded PagoStatus = "REFUNDED"
)

type Empresa struct {
	ID              int64
	Nombre          string
	Direccion       string
	Telefono        string
	RepresentanteID uuid.UUID
	PlanID          int64
	Status          EmpresaStatus
	Vigencia        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DatosFiscales struct {
	ID           int64
	EmpresaID    int64
	RFC          string
	RazonSocial  string
	LogoURL      string
	IMSSPatronal string
	REPSE        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Pago struct {
	ID              int64
	EmpresaID       int64
	TokenPago       string
	StripeSessionID string
	EstatusPago     PagoStatus
	Monto           float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Apoderado struct {
	ID        int64
	EmpresaID int64
	Nombre    string
	CURP      string
	RFC       string
	Email     string
	Telefono  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Servicio struct {
	ID           int64
	EmpresaID    int64
	Nombre       string
	Descripcion  string
	Precio       float64
	StatusActivo bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Sentinel errors
var (
	ErrEmpresaNotFound        = errors.New("empresas: empresa not found")
	ErrDuplicateRFC           = errors.New("empresas: RFC already registered")
	ErrDuplicateTokenPago     = errors.New("empresas: payment token already used")
	ErrEmpresaHasDependents   = errors.New("empresas: empresa has active dependents")
	ErrInvalidStatus          = errors.New("empresas: invalid status transition")
	ErrInvalidStripeSignature = errors.New("empresas: invalid stripe webhook signature")
)
