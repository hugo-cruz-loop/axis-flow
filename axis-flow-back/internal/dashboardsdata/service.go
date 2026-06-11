package dashboardsdata

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"axis-flow-back/internal/middleware"

	"github.com/google/uuid"
)

// Sentinel errors for dashboardsdata domain.
var (
	ErrUnauthorized    = errors.New("dashboardsdata: unauthorized")
	ErrForbidden       = errors.New("dashboardsdata: forbidden access (IDOR validation failed)")
	ErrCompanyNotFound = errors.New("dashboardsdata: company not found")
	ErrClientNotFound  = errors.New("dashboardsdata: client not found")
)

// Service orchestrates operational aggregations, cache lookups, and security policies.
type Service struct {
	repo  *PgxRepository
	cache *CacheClient
}

// NewService constructs a dashboardsdata Service.
func NewService(repo *PgxRepository, cache *CacheClient) *Service {
	return &Service{repo: repo, cache: cache}
}

// ValidateCompanyAccess checks that the authenticated user belongs to the requested company ID.
func (s *Service) ValidateCompanyAccess(ctx context.Context, companyID int64) error {
	role, _ := middleware.RoleFromContext(ctx)
	// CheckOn admins bypass IDOR verification
	if role == "ADMIN_CHECK_ON" {
		return nil
	}

	tenantIDStr, ok := ctx.Value(middleware.ContextKeyTenantID).(string)
	if !ok || tenantIDStr == "" {
		return ErrUnauthorized
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return ErrUnauthorized
	}

	companyTenantID, err := s.repo.GetCompanyTenantID(ctx, companyID)
	if err != nil {
		return ErrCompanyNotFound
	}

	if companyTenantID != tenantID {
		return ErrForbidden
	}
	return nil
}

// ValidateClientAccess checks that the requestor owns or belongs to the target client.
func (s *Service) ValidateClientAccess(ctx context.Context, clientID uuid.UUID) error {
	role, _ := middleware.RoleFromContext(ctx)
	// CheckOn admins bypass IDOR verification
	if role == "ADMIN_CHECK_ON" {
		return nil
	}

	tenantIDStr, ok := ctx.Value(middleware.ContextKeyTenantID).(string)
	if !ok || tenantIDStr == "" {
		return ErrUnauthorized
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return ErrUnauthorized
	}

	// 1. Direct match: client user accessing their own dashboard
	if tenantID == clientID {
		return nil
	}

	// 2. Company admin accessing one of their clients
	var companyID int64
	if err := s.repo.GetCompanyIDByTenantID(ctx, tenantID, &companyID); err == nil {
		var clientCompanyID int64
		if err := s.repo.GetClientCompanyID(ctx, clientID, &clientCompanyID); err == nil {
			if clientCompanyID == companyID {
				return nil
			}
		}
	}

	return ErrForbidden
}

// GetEvaluacionesClientes returns client evaluations grouped by client, checking cache first.
func (s *Service) GetEvaluacionesClientes(ctx context.Context, companyID int64, clientIDFilter *uuid.UUID) ([]ClientEvaluation, error) {
	if err := s.ValidateCompanyAccess(ctx, companyID); err != nil {
		return nil, err
	}

	metric := "evaluaciones-clientes"
	if clientIDFilter != nil {
		metric += ":" + clientIDFilter.String()
	}

	cacheKey := s.cache.FormatKey("admin", strconv.FormatInt(companyID, 10), metric)
	var cached []ClientEvaluation
	if hit, _ := s.cache.Get(ctx, cacheKey, &cached); hit {
		return cached, nil
	}

	dbData, err := s.repo.GetEvaluacionesClientes(ctx, companyID, clientIDFilter)
	if err != nil {
		return nil, fmt.Errorf("service.GetEvaluacionesClientes: %w", err)
	}

	_ = s.cache.Set(ctx, cacheKey, dbData)
	return dbData, nil
}

// GetEmpleadosCount returns the active employee headcount, checking cache first.
func (s *Service) GetEmpleadosCount(ctx context.Context, companyID int64) (int, error) {
	if err := s.ValidateCompanyAccess(ctx, companyID); err != nil {
		return 0, err
	}

	cacheKey := s.cache.FormatKey("admin", strconv.FormatInt(companyID, 10), "empleados-count")
	var count int
	if hit, _ := s.cache.Get(ctx, cacheKey, &count); hit {
		return count, nil
	}

	dbCount, err := s.repo.GetEmpleadosCount(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("service.GetEmpleadosCount: %w", err)
	}

	_ = s.cache.Set(ctx, cacheKey, dbCount)
	return dbCount, nil
}

// GetActividadesCount returns the completed activities headcount within a period, checking cache first.
func (s *Service) GetActividadesCount(ctx context.Context, companyID int64, period string) (int, error) {
	if err := s.ValidateCompanyAccess(ctx, companyID); err != nil {
		return 0, err
	}

	cacheKey := s.cache.FormatKey("admin", strconv.FormatInt(companyID, 10), "actividades-count:"+period)
	var count int
	if hit, _ := s.cache.Get(ctx, cacheKey, &count); hit {
		return count, nil
	}

	var since time.Time
	now := time.Now().UTC()
	switch period {
	case "day":
		since = now.Add(-24 * time.Hour)
	case "week":
		since = now.Add(-7 * 24 * time.Hour)
	case "month":
		since = now.Add(-30 * 24 * time.Hour)
	case "year":
		since = now.Add(-365 * 24 * time.Hour)
	default:
		since = now.Add(-30 * 24 * time.Hour) // default to month
	}

	dbCount, err := s.repo.GetActividadesCount(ctx, companyID, since)
	if err != nil {
		return 0, fmt.Errorf("service.GetActividadesCount: %w", err)
	}

	_ = s.cache.Set(ctx, cacheKey, dbCount)
	return dbCount, nil
}

// GetAusenciasCount returns historical total absences count, checking cache first.
func (s *Service) GetAusenciasCount(ctx context.Context, companyID int64, year *int) (int, error) {
	if err := s.ValidateCompanyAccess(ctx, companyID); err != nil {
		return 0, err
	}

	metric := "ausencias-count"
	if year != nil {
		metric += ":" + strconv.Itoa(*year)
	}

	cacheKey := s.cache.FormatKey("admin", strconv.FormatInt(companyID, 10), metric)
	var count int
	if hit, _ := s.cache.Get(ctx, cacheKey, &count); hit {
		return count, nil
	}

	dbCount, err := s.repo.GetAusenciasCount(ctx, companyID, year)
	if err != nil {
		return 0, fmt.Errorf("service.GetAusenciasCount: %w", err)
	}

	_ = s.cache.Set(ctx, cacheKey, dbCount)
	return dbCount, nil
}

// GetServiciosLocalidad returns localidad branches and their active services, checking cache first.
func (s *Service) GetServiciosLocalidad(ctx context.Context, clientID uuid.UUID) ([]LocalidadServicio, error) {
	if err := s.ValidateClientAccess(ctx, clientID); err != nil {
		return nil, err
	}

	cacheKey := s.cache.FormatKey("cliente", clientID.String(), "servicios-localidad")
	var cached []LocalidadServicio
	if hit, _ := s.cache.Get(ctx, cacheKey, &cached); hit {
		return cached, nil
	}

	dbData, err := s.repo.GetServiciosLocalidad(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("service.GetServiciosLocalidad: %w", err)
	}

	_ = s.cache.Set(ctx, cacheKey, dbData)
	return dbData, nil
}

// GetAtencionSeguimientoStatus returns ticket status breakdown, checking cache first.
func (s *Service) GetAtencionSeguimientoStatus(ctx context.Context, clientID uuid.UUID) (TicketBreakdown, int, error) {
	if err := s.ValidateClientAccess(ctx, clientID); err != nil {
		return TicketBreakdown{}, 0, err
	}

	cacheKey := s.cache.FormatKey("cliente", clientID.String(), "atencion-seguimiento")
	type responseWrapper struct {
		Tickets TicketBreakdown `json:"tickets"`
		Total   int             `json:"total"`
	}
	var cached responseWrapper
	if hit, _ := s.cache.Get(ctx, cacheKey, &cached); hit {
		return cached.Tickets, cached.Total, nil
	}

	tickets, total, err := s.repo.GetAtencionSeguimientoStatus(ctx, clientID)
	if err != nil {
		return TicketBreakdown{}, 0, fmt.Errorf("service.GetAtencionSeguimientoStatus: %w", err)
	}

	_ = s.cache.Set(ctx, cacheKey, responseWrapper{Tickets: tickets, Total: total})
	return tickets, total, nil
}

// GetBolsaTrabajoVacantesActivas returns active vacancies summary, checking cache first.
func (s *Service) GetBolsaTrabajoVacantesActivas(ctx context.Context, companyID int64) ([]Vacante, error) {
	if err := s.ValidateCompanyAccess(ctx, companyID); err != nil {
		return nil, err
	}

	cacheKey := s.cache.FormatKey("rh", strconv.FormatInt(companyID, 10), "vacantes-activas")
	var cached []Vacante
	if hit, _ := s.cache.Get(ctx, cacheKey, &cached); hit {
		return cached, nil
	}

	companyTenantID, err := s.repo.GetCompanyTenantID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("service.GetBolsaTrabajoVacantesActivas resolve tenant: %w", err)
	}

	dbData, err := s.repo.GetBolsaTrabajoVacantesActivas(ctx, companyTenantID)
	if err != nil {
		return nil, fmt.Errorf("service.GetBolsaTrabajoVacantesActivas: %w", err)
	}

	_ = s.cache.Set(ctx, cacheKey, dbData)
	return dbData, nil
}

// GetEmpleadosAbsentismo calculates and returns the absenteeism statistics, checking cache first.
func (s *Service) GetEmpleadosAbsentismo(ctx context.Context, companyID int64) (RHEmpleadosAbsentismoData, error) {
	if err := s.ValidateCompanyAccess(ctx, companyID); err != nil {
		return RHEmpleadosAbsentismoData{}, err
	}

	cacheKey := s.cache.FormatKey("rh", strconv.FormatInt(companyID, 10), "absentismo")
	var cached RHEmpleadosAbsentismoData
	if hit, _ := s.cache.Get(ctx, cacheKey, &cached); hit {
		return cached, nil
	}

	// Calculate last 30 days
	since := time.Now().UTC().AddDate(0, 0, -30)

	// Fetch count of active employees
	totalEmployees, err := s.repo.GetEmpleadosCount(ctx, companyID)
	if err != nil {
		return RHEmpleadosAbsentismoData{}, fmt.Errorf("service.GetEmpleadosAbsentismo headcount: %w", err)
	}

	// Fetch absences and affected employees in last 30 days
	query := `
		SELECT 
			COUNT(i.id)::int AS total_inasistencias,
			COUNT(DISTINCT i.empleado_id)::int AS empleados_afectados
		FROM empleados.empleados_inasistencia i
		JOIN empleados.empleados_empleado e ON i.empleado_id = e.num_empleado
		WHERE e.empresa_id = $1 AND e.status = 1 AND i.fecha_inicio >= $2`

	var (
		totalInasistencias int
		empleadosAfectados int
	)
	err = s.repo.db.QueryRow(ctx, query, companyID, since).Scan(&totalInasistencias, &empleadosAfectados)
	if err != nil {
		return RHEmpleadosAbsentismoData{}, fmt.Errorf("service.GetEmpleadosAbsentismo query: %w", err)
	}

	diasLaborables := 22 // Standard business month days
	tasaAbsentismo := 0.0
	if totalEmployees > 0 {
		tasaAbsentismo = (float64(totalInasistencias) / float64(totalEmployees*diasLaborables)) * 100
		if tasaAbsentismo > 100.0 {
			tasaAbsentismo = 100.0
		}
	}

	res := RHEmpleadosAbsentismoData{
		EmpresaID:             companyID,
		TasaAbsentismo:        tasaAbsentismo,
		DiasLaborablesTotales: diasLaborables,
		TotalInasistencias:    totalInasistencias,
		EmpleadosAfectados:    empleadosAfectados,
	}

	_ = s.cache.Set(ctx, cacheKey, res)
	return res, nil
}
