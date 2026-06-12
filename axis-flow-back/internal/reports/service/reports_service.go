package service

import (
	"context"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/repository"
)

// ---------------------------------------------------------------------------
// ReportsService interface
// ---------------------------------------------------------------------------

// ReportsService provides business logic for the four Reports endpoints.
// It applies PII masking based on the caller's role and delegates all data
// access to repository interfaces (no SQL in this layer).
type ReportsService interface {
	// GetEvidencias returns paginated evidence records for empresaID.
	// EmpleadoNombre is masked for roles other than RoleHR and RoleAdmin.
	GetEvidencias(ctx context.Context, empresaID string, role string, clienteID *string, fecha *string, page, limit int) ([]reports.Evidencia, int, error)

	// GetAsistencias returns paginated attendance records for empresaID.
	// EmpleadoNombre is masked for roles other than RoleHR and RoleAdmin.
	GetAsistencias(ctx context.Context, empresaID string, role string, employeeID *string, status *string, dateFrom *string, dateTo *string, page, limit int) ([]reports.AsistenciaRecord, int, error)

	// GetGraficaEvidencia returns weekly evidence compliance aggregates.
	GetGraficaEvidencia(ctx context.Context, empresaID string, clienteID *string) ([]reports.GraficaEvidenciaStat, error)

	// GetCountIncidentes returns incident counts grouped by status.
	GetCountIncidentes(ctx context.Context, empresaID string) ([]reports.IncidenteCount, error)
}

// ---------------------------------------------------------------------------
// pgReportsService — implementation
// ---------------------------------------------------------------------------

type pgReportsService struct {
	evidencias  repository.EvidenciasRepo
	asistencias repository.AsistenciasRepo
	stats       repository.StatsRepo
}

// NewReportsService creates a ReportsService backed by the given repositories.
func NewReportsService(
	evidencias repository.EvidenciasRepo,
	asistencias repository.AsistenciasRepo,
	stats repository.StatsRepo,
) ReportsService {
	return &pgReportsService{
		evidencias:  evidencias,
		asistencias: asistencias,
		stats:       stats,
	}
}

// shouldMaskPII returns true when the role does NOT grant access to full
// employee names. Only HR and Admin roles see un-masked PII.
func shouldMaskPII(role string) bool {
	return role != reports.RoleHR && role != reports.RoleAdmin
}

// GetEvidencias implements ReportsService.
func (s *pgReportsService) GetEvidencias(ctx context.Context, empresaID, role string, clienteID, fecha *string, page, limit int) ([]reports.Evidencia, int, error) {
	items, total, err := s.evidencias.List(ctx, empresaID, clienteID, fecha, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if shouldMaskPII(role) {
		for i := range items {
			items[i].EmpleadoNombre = reports.MaskEmpleadoName(items[i].EmpleadoNombre)
		}
	}
	return items, total, nil
}

// GetAsistencias implements ReportsService.
func (s *pgReportsService) GetAsistencias(ctx context.Context, empresaID, role string, employeeID, status, dateFrom, dateTo *string, page, limit int) ([]reports.AsistenciaRecord, int, error) {
	items, total, err := s.asistencias.List(ctx, empresaID, employeeID, status, dateFrom, dateTo, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if shouldMaskPII(role) {
		for i := range items {
			items[i].EmpleadoNombre = reports.MaskEmpleadoName(items[i].EmpleadoNombre)
		}
	}
	return items, total, nil
}

// GetGraficaEvidencia implements ReportsService.
func (s *pgReportsService) GetGraficaEvidencia(ctx context.Context, empresaID string, clienteID *string) ([]reports.GraficaEvidenciaStat, error) {
	return s.stats.GraficaEvidencia(ctx, empresaID, clienteID)
}

// GetCountIncidentes implements ReportsService.
func (s *pgReportsService) GetCountIncidentes(ctx context.Context, empresaID string) ([]reports.IncidenteCount, error) {
	return s.stats.CountIncidentes(ctx, empresaID)
}
