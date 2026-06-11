// Package parametrizacion defines domain contracts for the Parametrizacion module.
package parametrizacion

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

const (
	EventEvaluacionServicioConfigurada = "EvaluacionServicioConfigurada"
	EventDiaInactivoConfigurado        = "DiaInactivoConfigurado"
	EventSystemSettingUpdated          = "SystemSettingUpdated"
	CacheTTL                           = 24 * time.Hour
	SystemSettingCacheTTL              = 7 * 24 * time.Hour
)

var systemKeyPattern = regexp.MustCompile(`^[A-Z0-9_]+$`)

type SistemaParametro struct {
	ClaveParametro, Valor, Descripcion string
	CreatedAt, UpdatedAt               time.Time
}

func (p SistemaParametro) Validate() error {
	if !systemKeyPattern.MatchString(p.ClaveParametro) {
		return fmt.Errorf("%w: clave_parametro must match ^[A-Z0-9_]+$", ErrInvalidInput)
	}
	if p.Valor == "" {
		return fmt.Errorf("%w: valor is required", ErrInvalidInput)
	}
	return nil
}

type EvaluacionServicio struct {
	ID, EmpresaID, ServicioID, PeriodicidadID int64
	Activa                                    bool
	CreatedAt, UpdatedAt                      time.Time
}

func (e EvaluacionServicio) Validate() error {
	if e.EmpresaID <= 0 || e.ServicioID <= 0 || e.PeriodicidadID <= 0 {
		return fmt.Errorf("%w: empresa_id, servicio_id and periodicidad_id are required", ErrInvalidInput)
	}
	return nil
}

type EvaluacionPersonal struct {
	ID, EmpresaID, PeriodicidadID int64
	Activa                        bool
	CreatedAt, UpdatedAt          time.Time
}

func (e EvaluacionPersonal) Validate() error {
	if e.EmpresaID <= 0 || e.PeriodicidadID <= 0 {
		return fmt.Errorf("%w: empresa_id and periodicidad_id are required", ErrInvalidInput)
	}
	return nil
}

type DiaInactivo struct {
	ID, EmpresaID        int64
	Fecha                time.Time
	Descripcion          string
	CreatedAt, UpdatedAt time.Time
}

func (d DiaInactivo) Validate() error {
	if d.EmpresaID <= 0 {
		return fmt.Errorf("%w: empresa_id is required", ErrInvalidInput)
	}
	if d.Fecha.IsZero() {
		return fmt.Errorf("%w: fecha is required", ErrInvalidInput)
	}
	if d.Descripcion == "" {
		return fmt.Errorf("%w: descripcion is required", ErrInvalidInput)
	}
	return nil
}

type DiasInactivosUmbral struct {
	EmpresaID            int64
	UmbralDias           int
	CreatedAt, UpdatedAt time.Time
}

func (u DiasInactivosUmbral) Validate() error {
	if u.EmpresaID <= 0 {
		return fmt.Errorf("%w: empresa_id is required", ErrInvalidInput)
	}
	if u.UmbralDias < 0 {
		return fmt.Errorf("%w: umbral_dias must be greater than or equal to zero", ErrInvalidInput)
	}
	return nil
}

// CompanyInactiveDays is the cached read model returned by GET /dias-inactivos/empresa/{empresa_id}.
type CompanyInactiveDays struct {
	EmpresaID     int64          `json:"empresa_id"`
	UmbralDias    int            `json:"umbral_dias"`
	DiasInactivos []*DiaInactivo `json:"dias_inactivos"`
}

// Actor carries audit metadata from the authenticated request.
type Actor struct {
	UserID    uuid.UUID
	IPAddress string
	TraceID   string
}

type DomainEvent struct {
	Name       string
	OccurredAt time.Time
	Payload    any
}

type ParametrizacionService interface {
	UpsertSistemaParametro(context.Context, SistemaParametro, Actor) (*SistemaParametro, error)
	GetSistemaParametro(context.Context, string) (*SistemaParametro, error)
	ListSistemaParametros(context.Context) ([]*SistemaParametro, error)
	ListEvaluacionServicio(context.Context, int64) ([]*EvaluacionServicio, error)
	ConfigurarEvaluacionServicio(context.Context, EvaluacionServicio) (*EvaluacionServicio, error)
	ListEvaluacionPersonal(context.Context, int64) ([]*EvaluacionPersonal, error)
	ConfigurarEvaluacionPersonal(context.Context, EvaluacionPersonal) (*EvaluacionPersonal, error)
	AgregarDiaInactivo(context.Context, DiaInactivo) (*DiaInactivo, error)
	EliminarDiaInactivo(ctx context.Context, id, empresaID int64) error
	GetDiasInactivosEmpresa(ctx context.Context, empresaID int64, year int) (*CompanyInactiveDays, error)
	ConfigurarUmbralDiasInactivos(context.Context, DiasInactivosUmbral) (*DiasInactivosUmbral, error)
}

type SistemaRepository interface {
	Upsert(context.Context, *SistemaParametro) error
	GetByClave(context.Context, string) (*SistemaParametro, error)
	List(context.Context) ([]*SistemaParametro, error)
}

type EvaluacionServicioRepository interface {
	Upsert(context.Context, *EvaluacionServicio) error
	GetByEmpresaServicio(ctx context.Context, empresaID, servicioID int64) (*EvaluacionServicio, error)
	ListByEmpresa(ctx context.Context, empresaID int64) ([]*EvaluacionServicio, error)
}

type EvaluacionPersonalRepository interface {
	Upsert(context.Context, *EvaluacionPersonal) error
	GetByEmpresa(ctx context.Context, empresaID int64) (*EvaluacionPersonal, error)
}

type DiasInactivosRepository interface {
	AddDia(context.Context, *DiaInactivo) error
	DeleteDia(ctx context.Context, id, empresaID int64) error
	ListByEmpresaYear(ctx context.Context, empresaID int64, year int) ([]*DiaInactivo, error)
	UpsertUmbral(context.Context, *DiasInactivosUmbral) error
	GetUmbral(ctx context.Context, empresaID int64) (*DiasInactivosUmbral, error)
}

type CachePort interface {
	Get(ctx context.Context, key string, dest any) (bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type EventPublisher interface {
	Publish(context.Context, DomainEvent) error
}

func SistemaCacheKey(clave string) string {
	return fmt.Sprintf("parametrizacion:sistema:%s", clave)
}

func EvaluacionServicioCacheKey(empresaID, servicioID int64) string {
	return fmt.Sprintf("parametrizacion:evaluacion_servicio:empresa:%d:servicio:%d", empresaID, servicioID)
}

func EvaluacionPersonalCacheKey(empresaID int64) string {
	return fmt.Sprintf("parametrizacion:evaluacion_personal:empresa:%d", empresaID)
}

func DiasInactivosCacheKey(empresaID int64, year int) string {
	return fmt.Sprintf("parametrizacion:dias_inactivos:empresa:%d:year:%d", empresaID, year)
}

var (
	ErrNotFound       = errors.New("parametrizacion: not found")
	ErrForbidden      = errors.New("parametrizacion: forbidden")
	ErrConflict       = errors.New("parametrizacion: conflict")
	ErrInvalidInput   = errors.New("parametrizacion: invalid input")
	ErrTenantMismatch = errors.New("parametrizacion: tenant mismatch")
)
