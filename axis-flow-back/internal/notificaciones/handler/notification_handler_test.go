package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/notificaciones"
	"axis-flow-back/internal/notificaciones/events"
	"axis-flow-back/internal/notificaciones/handler"
	"axis-flow-back/internal/notificaciones/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockNotificationService struct {
	listPushByUserFn func(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionEnviada, int, error)
	countUnreadFn    func(ctx context.Context, userID uuid.UUID) (int, error)
	markAsReadFn     func(ctx context.Context, requesterID, notifID uuid.UUID) (notificaciones.NotificacionAtencion, error)
	createAlertFn    func(ctx context.Context, n notificaciones.NotificacionAtencion) (notificaciones.NotificacionAtencion, error)
	listAlertsFn     func(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionAtencion, int, error)
	registerPushFn   func(ctx context.Context, n notificaciones.NotificacionEnviada) (notificaciones.NotificacionEnviada, error)
}

func (m *mockNotificationService) ListPushByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionEnviada, int, error) {
	if m.listPushByUserFn != nil {
		return m.listPushByUserFn(ctx, userID, page, pageSize)
	}
	return nil, 0, nil
}

func (m *mockNotificationService) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	if m.countUnreadFn != nil {
		return m.countUnreadFn(ctx, userID)
	}
	return 0, nil
}

func (m *mockNotificationService) MarkAsRead(ctx context.Context, requesterID, notifID uuid.UUID) (notificaciones.NotificacionAtencion, error) {
	if m.markAsReadFn != nil {
		return m.markAsReadFn(ctx, requesterID, notifID)
	}
	return notificaciones.NotificacionAtencion{}, nil
}

func (m *mockNotificationService) CreateAlert(ctx context.Context, n notificaciones.NotificacionAtencion) (notificaciones.NotificacionAtencion, error) {
	if m.createAlertFn != nil {
		return m.createAlertFn(ctx, n)
	}
	return n, nil
}

func (m *mockNotificationService) ListAlertsByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionAtencion, int, error) {
	if m.listAlertsFn != nil {
		return m.listAlertsFn(ctx, userID, page, pageSize)
	}
	return nil, 0, nil
}

func (m *mockNotificationService) RegisterPush(ctx context.Context, n notificaciones.NotificacionEnviada) (notificaciones.NotificacionEnviada, error) {
	if m.registerPushFn != nil {
		return m.registerPushFn(ctx, n)
	}
	return n, nil
}

var _ service.NotificationService = (*mockNotificationService)(nil)

type mockEmailService struct {
	triggerRecoveryFn func(ctx context.Context, email string) error
	triggerWelcomeFn  func(ctx context.Context, userID uuid.UUID, email, name string) error
}

func (m *mockEmailService) TriggerRecovery(ctx context.Context, email string) error {
	if m.triggerRecoveryFn != nil {
		return m.triggerRecoveryFn(ctx, email)
	}
	return nil
}

func (m *mockEmailService) TriggerWelcome(ctx context.Context, userID uuid.UUID, email, name string) error {
	if m.triggerWelcomeFn != nil {
		return m.triggerWelcomeFn(ctx, userID, email, name)
	}
	return nil
}

var _ service.EmailService = (*mockEmailService)(nil)

// withUserCtx injects a userID string into the request context (simulates JWTAuth middleware).
func withUserCtx(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ContextKeyUserID, userID.String())
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, "EMPLEADO")
	return r.WithContext(ctx)
}

// withAdminCtx injects a userID + ADMIN role into the request context.
func withAdminCtx(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ContextKeyUserID, userID.String())
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, "ADMIN")
	return r.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// Helper — build a NotificationHandler with no-op deps
// ---------------------------------------------------------------------------

func newTestHandler(svc service.NotificationService, email service.EmailService, pub events.EventPublisher) *handler.NotificationHandler {
	if pub == nil {
		pub = events.NoopPublisher{}
	}
	return handler.NewNotificationHandler(svc, email, pub)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPostRecuperarPassword_HappyPath(t *testing.T) {
	h := newTestHandler(&mockNotificationService{}, &mockEmailService{}, nil)

	body := `{"email":"user@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notificaciones/recuperar-password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.RecuperarPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}
}

func TestPostRecuperarPassword_MissingEmail(t *testing.T) {
	h := newTestHandler(&mockNotificationService{}, &mockEmailService{}, nil)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notificaciones/recuperar-password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.RecuperarPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d — body: %s", rr.Code, rr.Body.String())
	}
}

func TestGetUnreadCount_HappyPath(t *testing.T) {
	userID := uuid.New()

	svc := &mockNotificationService{
		countUnreadFn: func(_ context.Context, uid uuid.UUID) (int, error) {
			if uid != userID {
				return 0, fmt.Errorf("wrong user")
			}
			return 7, nil
		},
	}
	h := newTestHandler(svc, &mockEmailService{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notificaciones/usuario/"+userID.String()+"/no-leidas/count", nil)
	req = withUserCtx(req, userID)
	// Simulate chi path param
	req = requestWithPathParam(req, "userId", userID.String())

	rr := httptest.NewRecorder()
	h.CountUnread(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", resp["data"])
	}
	if data["unreadCount"] != float64(7) {
		t.Errorf("expected unreadCount=7, got %v", data["unreadCount"])
	}
}

func TestPatchEstatus_HappyPath(t *testing.T) {
	userID := uuid.New()
	notifID := uuid.New()

	svc := &mockNotificationService{
		markAsReadFn: func(_ context.Context, rID, nID uuid.UUID) (notificaciones.NotificacionAtencion, error) {
			return notificaciones.NotificacionAtencion{
				ID:        nID,
				UserID:    rID,
				Estatus:   notificaciones.EstatusRead,
				UpdatedAt: time.Now(),
			}, nil
		},
	}
	h := newTestHandler(svc, &mockEmailService{}, nil)

	body := `{"estatus":2}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/notificaciones/"+notifID.String()+"/estatus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserCtx(req, userID)
	req = requestWithPathParam(req, "id", notifID.String())

	rr := httptest.NewRecorder()
	h.UpdateEstatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
}

func TestPatchEstatus_InvalidEstatus(t *testing.T) {
	userID := uuid.New()
	notifID := uuid.New()

	h := newTestHandler(&mockNotificationService{}, &mockEmailService{}, nil)

	body := `{"estatus":99}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/notificaciones/"+notifID.String()+"/estatus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserCtx(req, userID)
	req = requestWithPathParam(req, "id", notifID.String())

	rr := httptest.NewRecorder()
	h.UpdateEstatus(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d — body: %s", rr.Code, rr.Body.String())
	}
}

func TestPatchEstatus_IDOR(t *testing.T) {
	requesterID := uuid.New()
	notifID := uuid.New()

	svc := &mockNotificationService{
		markAsReadFn: func(_ context.Context, _, _ uuid.UUID) (notificaciones.NotificacionAtencion, error) {
			return notificaciones.NotificacionAtencion{}, notificaciones.ErrForbidden
		},
	}
	h := newTestHandler(svc, &mockEmailService{}, nil)

	body := `{"estatus":2}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/notificaciones/"+notifID.String()+"/estatus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserCtx(req, requesterID)
	req = requestWithPathParam(req, "id", notifID.String())

	rr := httptest.NewRecorder()
	h.UpdateEstatus(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d — body: %s", rr.Code, rr.Body.String())
	}
}
