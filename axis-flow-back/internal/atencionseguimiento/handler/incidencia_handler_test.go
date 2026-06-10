package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/handler"
	"axis-flow-back/internal/middleware"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Inline mock for IncidenciaService
// ---------------------------------------------------------------------------

type mockIncidenciaService struct {
	createIncidenciaFn       func(ctx context.Context, inc *atencionseguimiento.IncidenciaSupervisor, supervisorID, empresaID uuid.UUID) (*atencionseguimiento.IncidenciaSupervisor, error)
	getIncidenciasByEmpresaFn func(ctx context.Context, empresaID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error)
}

func (m *mockIncidenciaService) CreateIncidencia(ctx context.Context, inc *atencionseguimiento.IncidenciaSupervisor, supervisorID, empresaID uuid.UUID) (*atencionseguimiento.IncidenciaSupervisor, error) {
	return m.createIncidenciaFn(ctx, inc, supervisorID, empresaID)
}
func (m *mockIncidenciaService) GetIncidenciasByEmpresa(ctx context.Context, empresaID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error) {
	return m.getIncidenciasByEmpresaFn(ctx, empresaID, page, pageSize)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func injectIncidenciaCtx(r *http.Request, tenantID, userID string, role string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, tenantID)
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, role)
	return r.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// POST /incidencia with empresa mismatch → 403
func TestCreateIncidencia_EmpresaMismatch_Returns403(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	differentEmpresa := uuid.New()

	svc := &mockIncidenciaService{
		createIncidenciaFn: func(_ context.Context, _ *atencionseguimiento.IncidenciaSupervisor, _, _ uuid.UUID) (*atencionseguimiento.IncidenciaSupervisor, error) {
			return nil, atencionseguimiento.ErrForbidden
		},
	}
	h := handler.NewIncidenciaHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"empresa_id":        differentEmpresa, // mismatch with JWT tenantID
		"empleado_id":       int64(42),
		"tipo_incidencia_id": uuid.New(),
		"descripcion":       "test incident",
		"fecha_incidencia":  "2026-01-15T00:00:00Z",
	})
	r := httptest.NewRequest(http.MethodPost, "/incidencia", bytes.NewReader(body))
	r = injectIncidenciaCtx(r, tenantID.String(), userID.String(), "Supervisor")
	w := httptest.NewRecorder()

	h.CreateIncidencia(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d — body: %s", w.Code, w.Body.String())
	}
}

// POST /incidencia with valid payload → 201
func TestCreateIncidencia_Valid_Returns201(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	result := &atencionseguimiento.IncidenciaSupervisor{ID: uuid.New(), EmpresaID: tenantID}

	svc := &mockIncidenciaService{
		createIncidenciaFn: func(_ context.Context, _ *atencionseguimiento.IncidenciaSupervisor, _, _ uuid.UUID) (*atencionseguimiento.IncidenciaSupervisor, error) {
			return result, nil
		},
	}
	h := handler.NewIncidenciaHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"empresa_id":        tenantID,
		"empleado_id":       int64(42),
		"tipo_incidencia_id": uuid.New(),
		"descripcion":       "valid incident",
		"fecha_incidencia":  "2026-01-15T00:00:00Z",
	})
	r := httptest.NewRequest(http.MethodPost, "/incidencia", bytes.NewReader(body))
	r = injectIncidenciaCtx(r, tenantID.String(), userID.String(), "Supervisor")
	w := httptest.NewRecorder()

	h.CreateIncidencia(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}
}
