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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Inline mock for QuejaService
// ---------------------------------------------------------------------------

type mockQuejaService struct {
	createQuejaFn          func(ctx context.Context, q *atencionseguimiento.SolicitudQueja, empleadoID int64, empresaID uuid.UUID) (*atencionseguimiento.SolicitudQueja, *atencionseguimiento.RespuestaQueja, error)
	getQuejaFn             func(ctx context.Context, id uuid.UUID, requesterEmpleadoID int64, requesterRole string) (*atencionseguimiento.SolicitudQueja, error)
	getQuejasByEmpresaFn   func(ctx context.Context, empresaID uuid.UUID, estatus *int, page, pageSize int) ([]*atencionseguimiento.SolicitudQueja, int, error)
	createMensajeFn        func(ctx context.Context, solicitudID uuid.UUID, msg *atencionseguimiento.RespuestaQueja, requesterEmpleadoID int64, requesterRole string) (*atencionseguimiento.RespuestaQueja, error)
	getMensajesFn          func(ctx context.Context, solicitudID uuid.UUID, requesterEmpleadoID int64, requesterRole string, page, pageSize int) ([]*atencionseguimiento.RespuestaQueja, int, error)
	suspendQuejasFn        func(ctx context.Context, empleadoID int64) error
}

func (m *mockQuejaService) CreateQueja(ctx context.Context, q *atencionseguimiento.SolicitudQueja, empleadoID int64, empresaID uuid.UUID) (*atencionseguimiento.SolicitudQueja, *atencionseguimiento.RespuestaQueja, error) {
	return m.createQuejaFn(ctx, q, empleadoID, empresaID)
}
func (m *mockQuejaService) GetQueja(ctx context.Context, id uuid.UUID, requesterEmpleadoID int64, requesterRole string) (*atencionseguimiento.SolicitudQueja, error) {
	return m.getQuejaFn(ctx, id, requesterEmpleadoID, requesterRole)
}
func (m *mockQuejaService) GetQuejasByEmpresa(ctx context.Context, empresaID uuid.UUID, estatus *int, page, pageSize int) ([]*atencionseguimiento.SolicitudQueja, int, error) {
	return m.getQuejasByEmpresaFn(ctx, empresaID, estatus, page, pageSize)
}
func (m *mockQuejaService) CreateMensaje(ctx context.Context, solicitudID uuid.UUID, msg *atencionseguimiento.RespuestaQueja, requesterEmpleadoID int64, requesterRole string) (*atencionseguimiento.RespuestaQueja, error) {
	return m.createMensajeFn(ctx, solicitudID, msg, requesterEmpleadoID, requesterRole)
}
func (m *mockQuejaService) GetMensajes(ctx context.Context, solicitudID uuid.UUID, requesterEmpleadoID int64, requesterRole string, page, pageSize int) ([]*atencionseguimiento.RespuestaQueja, int, error) {
	return m.getMensajesFn(ctx, solicitudID, requesterEmpleadoID, requesterRole, page, pageSize)
}
func (m *mockQuejaService) SuspendQuejasByEmpleado(ctx context.Context, empleadoID int64) error {
	return m.suspendQuejasFn(ctx, empleadoID)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func injectQuejaCtx(r *http.Request, tenantID, userID string, empleadoID int64, role string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, tenantID)
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextKeyEmpleadoID, empleadoID)
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, role)
	return r.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// POST /queja without JWT → 401
func TestCreateQueja_NoJWT_Returns401(t *testing.T) {
	svc := &mockQuejaService{}
	h := handler.NewQuejaHandler(svc)

	body, _ := json.Marshal(map[string]any{"tipo_queja_id": uuid.New(), "contenido": "test"})
	r := httptest.NewRequest(http.MethodPost, "/queja", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateQueja(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// POST /queja with valid JWT → 201
func TestCreateQueja_ValidJWT_Returns201(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	queja := &atencionseguimiento.SolicitudQueja{ID: uuid.New(), EmpresaID: tenantID}
	msg := &atencionseguimiento.RespuestaQueja{ID: uuid.New()}

	svc := &mockQuejaService{
		createQuejaFn: func(_ context.Context, q *atencionseguimiento.SolicitudQueja, _ int64, _ uuid.UUID) (*atencionseguimiento.SolicitudQueja, *atencionseguimiento.RespuestaQueja, error) {
			return queja, msg, nil
		},
	}
	h := handler.NewQuejaHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"tipo_queja_id": uuid.New(),
		"contenido":     "mi queja",
	})
	r := httptest.NewRequest(http.MethodPost, "/queja", bytes.NewReader(body))
	r = injectQuejaCtx(r, tenantID.String(), userID.String(), 42, "Empleado")
	w := httptest.NewRecorder()

	h.CreateQueja(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}
}

// GET /queja/{id} with wrong role (Cliente) → 403
func TestGetQueja_WrongRole_Returns403(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	svc := &mockQuejaService{
		getQuejaFn: func(_ context.Context, _ uuid.UUID, _ int64, _ string) (*atencionseguimiento.SolicitudQueja, error) {
			return nil, atencionseguimiento.ErrForbidden
		},
	}
	h := handler.NewQuejaHandler(svc)

	router := chi.NewRouter()
	router.Get("/queja/{id}", h.GetQueja)

	r := httptest.NewRequest(http.MethodGet, "/queja/"+uuid.New().String(), nil)
	r = injectQuejaCtx(r, tenantID.String(), userID.String(), 0, "Cliente")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// GET /queja/{id} by Empleado accessing another employee's queja → 403 (IDOR)
func TestGetQueja_IDOR_Returns403(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	svc := &mockQuejaService{
		getQuejaFn: func(_ context.Context, _ uuid.UUID, _ int64, _ string) (*atencionseguimiento.SolicitudQueja, error) {
			return nil, atencionseguimiento.ErrForbidden
		},
	}
	h := handler.NewQuejaHandler(svc)

	router := chi.NewRouter()
	router.Get("/queja/{id}", h.GetQueja)

	r := httptest.NewRequest(http.MethodGet, "/queja/"+uuid.New().String(), nil)
	// empleadoID 99 tries to access queja owned by empleadoID 42
	r = injectQuejaCtx(r, tenantID.String(), userID.String(), 99, "Empleado")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
