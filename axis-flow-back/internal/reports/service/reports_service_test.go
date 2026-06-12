package service_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/service"
)

// ---------------------------------------------------------------------------
// Mock repos for reports service
// ---------------------------------------------------------------------------

type mockEvidenciasRepo struct {
	data []reports.Evidencia
}

func (m *mockEvidenciasRepo) List(_ context.Context, empresaID string, _ *string, _ *string, _, _ int) ([]reports.Evidencia, int, error) {
	return m.data, len(m.data), nil
}

type mockAsistenciasRepo struct {
	data []reports.AsistenciaRecord
}

func (m *mockAsistenciasRepo) List(_ context.Context, empresaID string, _ *string, _ *string, _ *string, _ *string, _, _ int) ([]reports.AsistenciaRecord, int, error) {
	return m.data, len(m.data), nil
}

type mockStatsRepo struct {
	graficaData []reports.GraficaEvidenciaStat
	incidentData []reports.IncidenteCount
}

func (m *mockStatsRepo) GraficaEvidencia(_ context.Context, _ string, _ *string) ([]reports.GraficaEvidenciaStat, error) {
	return m.graficaData, nil
}

func (m *mockStatsRepo) CountIncidentes(_ context.Context, _ string) ([]reports.IncidenteCount, error) {
	return m.incidentData, nil
}

// ---------------------------------------------------------------------------
// Tests: GetEvidencias PII masking
// ---------------------------------------------------------------------------

func evidenciasFixture() []reports.Evidencia {
	return []reports.Evidencia{
		{EmpleadoNombre: "Juan Pérez"},
		{EmpleadoNombre: "María López"},
	}
}

func TestReportsService_GetEvidencias_RoleHR_NotMasked(t *testing.T) {
	svc := service.NewReportsService(
		&mockEvidenciasRepo{data: evidenciasFixture()},
		&mockAsistenciasRepo{},
		&mockStatsRepo{},
	)
	evs, _, err := svc.GetEvidencias(context.Background(), "emp1", reports.RoleHR, nil, nil, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if evs[0].EmpleadoNombre != "Juan Pérez" {
		t.Errorf("HR role: expected full name, got %q", evs[0].EmpleadoNombre)
	}
}

func TestReportsService_GetEvidencias_RoleAdmin_NotMasked(t *testing.T) {
	svc := service.NewReportsService(
		&mockEvidenciasRepo{data: evidenciasFixture()},
		&mockAsistenciasRepo{},
		&mockStatsRepo{},
	)
	evs, _, err := svc.GetEvidencias(context.Background(), "emp1", reports.RoleAdmin, nil, nil, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if evs[0].EmpleadoNombre != "Juan Pérez" {
		t.Errorf("Admin role: expected full name, got %q", evs[0].EmpleadoNombre)
	}
}

func TestReportsService_GetEvidencias_RoleSupervisor_Masked(t *testing.T) {
	svc := service.NewReportsService(
		&mockEvidenciasRepo{data: evidenciasFixture()},
		&mockAsistenciasRepo{},
		&mockStatsRepo{},
	)
	evs, _, err := svc.GetEvidencias(context.Background(), "emp1", reports.RoleSupervisor, nil, nil, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := reports.MaskEmpleadoName("Juan Pérez") // "J. Pérez"
	if evs[0].EmpleadoNombre != want {
		t.Errorf("Supervisor role: expected %q, got %q", want, evs[0].EmpleadoNombre)
	}
}

func TestReportsService_GetEvidencias_RoleClient_Masked(t *testing.T) {
	svc := service.NewReportsService(
		&mockEvidenciasRepo{data: evidenciasFixture()},
		&mockAsistenciasRepo{},
		&mockStatsRepo{},
	)
	evs, _, err := svc.GetEvidencias(context.Background(), "emp1", reports.RoleClient, nil, nil, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := reports.MaskEmpleadoName("Juan Pérez")
	if evs[0].EmpleadoNombre != want {
		t.Errorf("Client role: expected %q, got %q", want, evs[0].EmpleadoNombre)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetAsistencias PII masking
// ---------------------------------------------------------------------------

func asistenciasFixture() []reports.AsistenciaRecord {
	return []reports.AsistenciaRecord{
		{EmpleadoNombre: "Pedro García"},
	}
}

func TestReportsService_GetAsistencias_RoleSupervisor_Masked(t *testing.T) {
	svc := service.NewReportsService(
		&mockEvidenciasRepo{},
		&mockAsistenciasRepo{data: asistenciasFixture()},
		&mockStatsRepo{},
	)
	asis, _, err := svc.GetAsistencias(context.Background(), "emp1", reports.RoleSupervisor, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := reports.MaskEmpleadoName("Pedro García")
	if asis[0].EmpleadoNombre != want {
		t.Errorf("Supervisor role: expected %q, got %q", want, asis[0].EmpleadoNombre)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetGraficaEvidencia and GetCountIncidentes
// ---------------------------------------------------------------------------

func TestReportsService_GetGraficaEvidencia_DelegatesToStatsRepo(t *testing.T) {
	fixture := []reports.GraficaEvidenciaStat{
		{Week: "2024-W01", Total: 10, Compliant: 8},
	}
	svc := service.NewReportsService(
		&mockEvidenciasRepo{},
		&mockAsistenciasRepo{},
		&mockStatsRepo{graficaData: fixture},
	)
	stats, err := svc.GetGraficaEvidencia(context.Background(), "emp1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Week != "2024-W01" {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestReportsService_GetCountIncidentes_DelegatesToStatsRepo(t *testing.T) {
	fixture := []reports.IncidenteCount{
		{Status: "tardanza", Count: 5},
	}
	svc := service.NewReportsService(
		&mockEvidenciasRepo{},
		&mockAsistenciasRepo{},
		&mockStatsRepo{incidentData: fixture},
	)
	counts, err := svc.GetCountIncidentes(context.Background(), "emp1")
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 1 || counts[0].Status != "tardanza" {
		t.Errorf("unexpected counts: %+v", counts)
	}
}
