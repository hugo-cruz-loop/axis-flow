package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/events"
	"axis-flow-back/internal/reports/handler"
	"axis-flow-back/internal/reports/service"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockReportsService struct {
	evidencias    []reports.Evidencia
	evidenciasTot int
	evidenciasErr error

	asistencias    []reports.AsistenciaRecord
	asistenciasTot int
	asistenciasErr error

	graficaStats []reports.GraficaEvidenciaStat
	graficaErr   error

	incidentes    []reports.IncidenteCount
	incidentesErr error
}

func (m *mockReportsService) GetEvidencias(_ context.Context, _, _ string, _, _ *string, _, _ int) ([]reports.Evidencia, int, error) {
	return m.evidencias, m.evidenciasTot, m.evidenciasErr
}
func (m *mockReportsService) GetAsistencias(_ context.Context, _, _ string, _, _, _, _ *string, _, _ int) ([]reports.AsistenciaRecord, int, error) {
	return m.asistencias, m.asistenciasTot, m.asistenciasErr
}
func (m *mockReportsService) GetGraficaEvidencia(_ context.Context, _ string, _ *string) ([]reports.GraficaEvidenciaStat, error) {
	return m.graficaStats, m.graficaErr
}
func (m *mockReportsService) GetCountIncidentes(_ context.Context, _ string) ([]reports.IncidenteCount, error) {
	return m.incidentes, m.incidentesErr
}

var _ service.ReportsService = (*mockReportsService)(nil)

type mockGeocodingService struct {
	result reports.GeocodingResult
	err    error
}

func (m *mockGeocodingService) Reverse(_ context.Context, _, _ float64) (reports.GeocodingResult, error) {
	return m.result, m.err
}

var _ service.GeocodingService = (*mockGeocodingService)(nil)

// ---------------------------------------------------------------------------
// Helper: build a request with JWT context values
// ---------------------------------------------------------------------------

func withJWT(r *http.Request, empresaID, role string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, empresaID)
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, role)
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, "user-1")
	return r.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// GET /evidencias happy path → 200 + data + meta
// ---------------------------------------------------------------------------

func TestGetEvidencias_HappyPath(t *testing.T) {
	svc := &mockReportsService{
		evidencias:    []reports.Evidencia{{AsignacionID: "ev-1", EmpleadoNombre: "Juan Pérez"}},
		evidenciasTot: 1,
	}
	h := handler.NewReportsHandler(svc, &mockGeocodingService{}, &events.NoopPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/evidencias", nil)
	req = withJWT(req, "emp-1", reports.RoleHR)
	w := httptest.NewRecorder()

	h.GetEvidencias(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["success"] != true {
		t.Error("expected success:true")
	}
	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatal("expected meta object")
	}
	if meta["total_records"].(float64) != 1 {
		t.Errorf("unexpected total_records: %v", meta["total_records"])
	}
}

// ---------------------------------------------------------------------------
// GET /evidencias with malformed cliente_id → 400
// ---------------------------------------------------------------------------

func TestGetEvidencias_MalformedClienteID(t *testing.T) {
	svc := &mockReportsService{}
	h := handler.NewReportsHandler(svc, &mockGeocodingService{}, &events.NoopPublisher{})

	// Use net/url to safely encode the malicious value.
	u, _ := http.NewRequest(http.MethodGet, "/api/v1/reports/evidencias", nil)
	q := u.URL.Query()
	q.Set("cliente_id", "'; DROP TABLE evidencias; --")
	u.URL.RawQuery = q.Encode()

	req := httptest.NewRequest(http.MethodGet, u.URL.String(), nil)
	req = withJWT(req, "emp-1", reports.RoleSupervisor)
	w := httptest.NewRecorder()

	h.GetEvidencias(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	errObj, _ := resp["error"].(map[string]any)
	if errObj["code"] != "INVALID_PARAM" {
		t.Errorf("expected INVALID_PARAM, got %v", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// GET /asistencias happy path → 200
// ---------------------------------------------------------------------------

func TestGetAsistencias_HappyPath(t *testing.T) {
	svc := &mockReportsService{
		asistencias:    []reports.AsistenciaRecord{{ID: "a-1", EmpleadoNombre: "Jane Doe"}},
		asistenciasTot: 1,
	}
	h := handler.NewReportsHandler(svc, &mockGeocodingService{}, &events.NoopPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/asistencias", nil)
	req = withJWT(req, "emp-1", reports.RoleAdmin)
	w := httptest.NewRecorder()

	h.GetAsistencias(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GET /graficaevidencia happy path → 200
// ---------------------------------------------------------------------------

func TestGetGraficaEvidencia_HappyPath(t *testing.T) {
	svc := &mockReportsService{
		graficaStats: []reports.GraficaEvidenciaStat{{Week: "2024-W01", Total: 10, Compliant: 8}},
	}
	h := handler.NewReportsHandler(svc, &mockGeocodingService{}, &events.NoopPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/graficaevidencia", nil)
	req = withJWT(req, "emp-1", reports.RoleAdmin)
	w := httptest.NewRecorder()

	h.GetGraficaEvidencia(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["success"] != true {
		t.Error("expected success:true")
	}
}

// ---------------------------------------------------------------------------
// GET /countincidentes happy path → 200
// ---------------------------------------------------------------------------

func TestGetCountIncidentes_HappyPath(t *testing.T) {
	svc := &mockReportsService{
		incidentes: []reports.IncidenteCount{{Status: "abierto", Count: 5}},
	}
	h := handler.NewReportsHandler(svc, &mockGeocodingService{}, &events.NoopPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/countincidentes", nil)
	req = withJWT(req, "emp-1", reports.RoleAdmin)
	w := httptest.NewRecorder()

	h.GetCountIncidentes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /geocoding/reverse happy path → 200 + cached field
// ---------------------------------------------------------------------------

func TestReverseGeocode_HappyPath(t *testing.T) {
	geo := &mockGeocodingService{
		result: reports.GeocodingResult{
			Latitud:   19.4326,
			Longitud:  -99.1332,
			Direccion: "Av. Reforma 1",
			Cached:    true,
		},
	}
	h := handler.NewReportsHandler(&mockReportsService{}, geo, &events.NoopPublisher{})

	body := `{"latitud":19.4326,"longitud":-99.1332}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/geocoding/reverse", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withJWT(req, "emp-1", reports.RoleAdmin)
	w := httptest.NewRecorder()

	h.ReverseGeocode(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data["cached"] != true {
		t.Errorf("expected cached:true, got %v", data["cached"])
	}
	if data["direccion"] != "Av. Reforma 1" {
		t.Errorf("unexpected direccion: %v", data["direccion"])
	}
}

// ---------------------------------------------------------------------------
// POST /geocoding/reverse invalid lat (>90) → 400
// ---------------------------------------------------------------------------

func TestReverseGeocode_InvalidLat(t *testing.T) {
	h := handler.NewReportsHandler(&mockReportsService{}, &mockGeocodingService{}, &events.NoopPublisher{})

	body := `{"latitud":91.0,"longitud":-99.1332}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/geocoding/reverse", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withJWT(req, "emp-1", reports.RoleAdmin)
	w := httptest.NewRecorder()

	h.ReverseGeocode(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /geocoding/reverse rate-limited → 429
// ---------------------------------------------------------------------------

func TestReverseGeocode_RateLimited(t *testing.T) {
	geo := &mockGeocodingService{err: reports.ErrRateLimitExceeded}
	h := handler.NewReportsHandler(&mockReportsService{}, geo, &events.NoopPublisher{})

	body := `{"latitud":19.4326,"longitud":-99.1332}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/geocoding/reverse", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withJWT(req, "emp-1", reports.RoleAdmin)
	w := httptest.NewRecorder()

	h.ReverseGeocode(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d; body: %s", w.Code, w.Body.String())
	}
}
