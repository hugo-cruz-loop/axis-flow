package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/parametrizacion"
	"axis-flow-back/internal/parametrizacion/handler"

	"github.com/go-chi/chi/v5"
)

type stubService struct {
	configureServiceCalled bool
	addInactiveCalled      bool
	updateSystemCalled     bool
}

func (s *stubService) ListEvaluacionServicio(ctx context.Context, empresaID int64) ([]*parametrizacion.EvaluacionServicio, error) {
	return []*parametrizacion.EvaluacionServicio{{ID: 101, EmpresaID: empresaID, ServicioID: 5, PeriodicidadID: 2, Activa: true, CreatedAt: fixedTime(), UpdatedAt: fixedTime()}}, nil
}
func (s *stubService) ConfigurarEvaluacionServicio(ctx context.Context, cfg parametrizacion.EvaluacionServicio) (*parametrizacion.EvaluacionServicio, error) {
	s.configureServiceCalled = true
	cfg.ID = 101
	cfg.CreatedAt = fixedTime()
	cfg.UpdatedAt = fixedTime()
	return &cfg, nil
}
func (s *stubService) ListEvaluacionPersonal(ctx context.Context, empresaID int64) ([]*parametrizacion.EvaluacionPersonal, error) {
	return []*parametrizacion.EvaluacionPersonal{{ID: 12, EmpresaID: empresaID, PeriodicidadID: 2, Activa: true, CreatedAt: fixedTime(), UpdatedAt: fixedTime()}}, nil
}
func (s *stubService) ConfigurarEvaluacionPersonal(ctx context.Context, cfg parametrizacion.EvaluacionPersonal) (*parametrizacion.EvaluacionPersonal, error) {
	cfg.ID = 12
	cfg.CreatedAt = fixedTime()
	cfg.UpdatedAt = fixedTime()
	return &cfg, nil
}
func (s *stubService) AgregarDiaInactivo(ctx context.Context, dia parametrizacion.DiaInactivo) (*parametrizacion.DiaInactivo, error) {
	s.addInactiveCalled = true
	dia.ID = 154
	dia.CreatedAt = fixedTime()
	dia.UpdatedAt = fixedTime()
	return &dia, nil
}
func (s *stubService) EliminarDiaInactivo(ctx context.Context, id, empresaID int64) error { return nil }
func (s *stubService) GetDiasInactivosEmpresa(ctx context.Context, empresaID int64, year int) (*parametrizacion.CompanyInactiveDays, error) {
	return &parametrizacion.CompanyInactiveDays{EmpresaID: empresaID, UmbralDias: 5, DiasInactivos: []*parametrizacion.DiaInactivo{{ID: 154, EmpresaID: empresaID, Fecha: date(2026, 12, 25), Descripcion: "Navidad", CreatedAt: fixedTime(), UpdatedAt: fixedTime()}}}, nil
}
func (s *stubService) ConfigurarUmbralDiasInactivos(ctx context.Context, umbral parametrizacion.DiasInactivosUmbral) (*parametrizacion.DiasInactivosUmbral, error) {
	umbral.UpdatedAt = fixedTime()
	return &umbral, nil
}

func (s *stubService) GetSistemaParametro(ctx context.Context, clave string) (*parametrizacion.SistemaParametro, error) {
	return &parametrizacion.SistemaParametro{ClaveParametro: clave, Valor: "axis_flow", Descripcion: "Websocket channel", CreatedAt: fixedTime(), UpdatedAt: fixedTime()}, nil
}
func (s *stubService) ListSistemaParametros(ctx context.Context) ([]*parametrizacion.SistemaParametro, error) {
	return []*parametrizacion.SistemaParametro{{ClaveParametro: "WEBSOCKET_CHANNEL_NAME", Valor: "axis_flow", Descripcion: "Websocket channel", CreatedAt: fixedTime(), UpdatedAt: fixedTime()}}, nil
}
func (s *stubService) UpsertSistemaParametro(ctx context.Context, p parametrizacion.SistemaParametro, actor parametrizacion.Actor) (*parametrizacion.SistemaParametro, error) {
	s.updateSystemCalled = true
	p.CreatedAt = fixedTime()
	p.UpdatedAt = fixedTime()
	return &p, nil
}

func TestRegisterRoutesMatchesOpenAPIParametrizacionPaths(t *testing.T) {
	svc := &stubService{}
	r := chi.NewRouter()
	handler.NewHTTPHandler(svc).RegisterRoutes(r)

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"list service evaluations", http.MethodGet, "/api/v1/parametrizacion/evaluacion-servicio?empresa_id=12", ``, http.StatusOK},
		{"create service evaluation", http.MethodPost, "/api/v1/parametrizacion/evaluacion-servicio", `{"empresa_id":12,"servicio_id":5,"periodicidad_id":2,"activa":true}`, http.StatusCreated},
		{"list personal evaluations", http.MethodGet, "/api/v1/parametrizacion/evaluacion-personal/filtrar?empresa_id=12", ``, http.StatusOK},
		{"create personal evaluation", http.MethodPost, "/api/v1/parametrizacion/evaluacion-personal", `{"empresa_id":12,"periodicidad_id":2,"activa":true}`, http.StatusCreated},
		{"add inactive day", http.MethodPost, "/api/v1/parametrizacion/dias-inactivos", `{"empresa_id":12,"fecha":"2026-12-25","descripcion":"Navidad"}`, http.StatusCreated},
		{"delete inactive day", http.MethodDelete, "/api/v1/parametrizacion/dias-inactivos/154?empresa_id=12", ``, http.StatusOK},
		{"get inactive days", http.MethodGet, "/api/v1/parametrizacion/dias-inactivos/empresa/12?year=2026", ``, http.StatusOK},
		{"set threshold", http.MethodPost, "/api/v1/parametrizacion/dias-inactivos/umbral", `{"empresa_id":12,"umbral_dias":5}`, http.StatusOK},
		{"list system settings", http.MethodGet, "/api/v1/parametrizacion/sistema", ``, http.StatusOK},
		{"patch system setting", http.MethodPatch, "/api/v1/parametrizacion/sistema/WEBSOCKET_CHANNEL_NAME", `{"valor":"axis_flow_websocket_production"}`, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = withAuth(req, 12, "ADMIN_CHECK_ON")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("%s %s got status %d, want %d; body=%s", tc.method, tc.path, rec.Code, tc.wantStatus, rec.Body.String())
			}
			var envelope map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("response is not JSON: %v; body=%s", err, rec.Body.String())
			}
			if envelope["success"] != true {
				t.Fatalf("success envelope mismatch: %#v", envelope)
			}
			if _, ok := envelope["data"]; !ok {
				t.Fatalf("response must include data envelope: %#v", envelope)
			}
		})
	}
}

func TestTenantMismatchReturns403AndDoesNotMutate(t *testing.T) {
	svc := &stubService{}
	r := chi.NewRouter()
	handler.NewHTTPHandler(svc).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parametrizacion/dias-inactivos", bytes.NewBufferString(`{"empresa_id":15,"fecha":"2026-12-25","descripcion":"Navidad"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, 12, "ADMIN_CHECK_ON")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if svc.addInactiveCalled {
		t.Fatal("service must not be called when JWT tenant differs from request empresa_id")
	}
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if envelope["success"] != false {
		t.Fatalf("expected success=false error envelope, got %#v", envelope)
	}
}

func TestNonAdminReadAndWriteAuthorization(t *testing.T) {
	svc := &stubService{}
	r := chi.NewRouter()
	handler.NewHTTPHandler(svc).RegisterRoutes(r)

	readReq := httptest.NewRequest(http.MethodGet, "/api/v1/parametrizacion/evaluacion-servicio?empresa_id=12", nil)
	readReq = withAuth(readReq, 12, "CLIENT")
	readRec := httptest.NewRecorder()
	r.ServeHTTP(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("CLIENT read got %d, want 200", readRec.Code)
	}

	writeReq := httptest.NewRequest(http.MethodPost, "/api/v1/parametrizacion/evaluacion-servicio", bytes.NewBufferString(`{"empresa_id":12,"servicio_id":5,"periodicidad_id":2}`))
	writeReq = withAuth(writeReq, 12, "CLIENT")
	writeRec := httptest.NewRecorder()
	r.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusForbidden {
		t.Fatalf("CLIENT write got %d, want 403", writeRec.Code)
	}
	if svc.configureServiceCalled {
		t.Fatal("service must not be called when role lacks write permission")
	}
}

func withAuth(r *http.Request, tenantID int64, role string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, "12")
	if tenantID != 12 {
		ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, "15")
	}
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, role)
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, "00000000-0000-0000-0000-000000000012")
	return r.WithContext(ctx)
}

func fixedTime() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }
func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
