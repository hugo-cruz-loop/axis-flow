package service_test

import (
	"context"
	"sync"
	"testing"

	"axis-flow-back/internal/notificaciones"
	"axis-flow-back/internal/notificaciones/repository"
	"axis-flow-back/internal/notificaciones/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Mock EventPublisher
// ---------------------------------------------------------------------------

type mockPublisher struct {
	mu      sync.Mutex
	records []publishRecord
}

type publishRecord struct {
	stream  string
	payload []byte
}

func (m *mockPublisher) Publish(_ context.Context, stream string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, publishRecord{stream: stream, payload: payload})
	return nil
}

func (m *mockPublisher) published(stream string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.records {
		if r.stream == stream {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newNotifSvcWithPublisher() (service.NotificationService, *mockPublisher) {
	alertRepo := repository.NewInMemAlertRepository()
	pushRepo := repository.NewInMemPushRepository()
	pub := &mockPublisher{}
	return service.NewNotificationService(alertRepo, pushRepo, pub), pub
}

func newNotifSvc() service.NotificationService {
	svc, _ := newNotifSvcWithPublisher()
	return svc
}

// ---------------------------------------------------------------------------
// CreateAlert
// ---------------------------------------------------------------------------

func TestNotificationService_CreateAlert_HappyPath(t *testing.T) {
	svc := newNotifSvc()
	userID := uuid.New()

	input := notificaciones.NotificacionAtencion{
		UserID:  userID,
		Mensaje: "Tu ticket fue actualizado",
		Estatus: notificaciones.EstatusUnread,
	}

	got, err := svc.CreateAlert(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateAlert: unexpected error: %v", err)
	}
	if got.ID == uuid.Nil {
		t.Error("CreateAlert: expected non-nil ID after insert")
	}
	if got.UserID != userID {
		t.Errorf("CreateAlert: UserID mismatch: got %v want %v", got.UserID, userID)
	}
	if got.Mensaje != input.Mensaje {
		t.Errorf("CreateAlert: Mensaje mismatch: got %q want %q", got.Mensaje, input.Mensaje)
	}
}

// ---------------------------------------------------------------------------
// MarkAsRead
// ---------------------------------------------------------------------------

func TestNotificationService_MarkAsRead_HappyPath(t *testing.T) {
	svc := newNotifSvc()
	userID := uuid.New()

	created, err := svc.CreateAlert(context.Background(), notificaciones.NotificacionAtencion{
		UserID:  userID,
		Mensaje: "msg",
		Estatus: notificaciones.EstatusUnread,
	})
	if err != nil {
		t.Fatalf("setup CreateAlert: %v", err)
	}

	updated, err := svc.MarkAsRead(context.Background(), userID, created.ID)
	if err != nil {
		t.Fatalf("MarkAsRead: unexpected error: %v", err)
	}
	if updated.Estatus != notificaciones.EstatusRead {
		t.Errorf("MarkAsRead: Estatus want %v got %v", notificaciones.EstatusRead, updated.Estatus)
	}
}

func TestNotificationService_MarkAsRead_IDOR(t *testing.T) {
	svc := newNotifSvc()
	ownerID := uuid.New()
	attackerID := uuid.New()

	created, err := svc.CreateAlert(context.Background(), notificaciones.NotificacionAtencion{
		UserID:  ownerID,
		Mensaje: "private",
		Estatus: notificaciones.EstatusUnread,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = svc.MarkAsRead(context.Background(), attackerID, created.ID)
	if err == nil {
		t.Fatal("MarkAsRead: expected ErrForbidden, got nil")
	}
	if err != notificaciones.ErrForbidden {
		t.Errorf("MarkAsRead: expected ErrForbidden, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CountUnread
// ---------------------------------------------------------------------------

func TestNotificationService_CountUnread(t *testing.T) {
	svc := newNotifSvc()
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		if _, err := svc.CreateAlert(context.Background(), notificaciones.NotificacionAtencion{
			UserID:  userID,
			Mensaje: "n",
			Estatus: notificaciones.EstatusUnread,
		}); err != nil {
			t.Fatalf("setup CreateAlert: %v", err)
		}
	}

	count, err := svc.CountUnread(context.Background(), userID)
	if err != nil {
		t.Fatalf("CountUnread: %v", err)
	}
	if count != 3 {
		t.Errorf("CountUnread: want 3 got %d", count)
	}
}

// ---------------------------------------------------------------------------
// ListAlertsByUser
// ---------------------------------------------------------------------------

func TestNotificationService_ListAlertsByUser_Pagination(t *testing.T) {
	svc := newNotifSvc()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		if _, err := svc.CreateAlert(context.Background(), notificaciones.NotificacionAtencion{
			UserID:  userID,
			Mensaje: "msg",
			Estatus: notificaciones.EstatusUnread,
		}); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	page1, total, err := svc.ListAlertsByUser(context.Background(), userID, 1, 3)
	if err != nil {
		t.Fatalf("ListAlertsByUser: %v", err)
	}
	if total != 5 {
		t.Errorf("total: want 5 got %d", total)
	}
	if len(page1) != 3 {
		t.Errorf("page1 len: want 3 got %d", len(page1))
	}

	page2, _, err := svc.ListAlertsByUser(context.Background(), userID, 2, 3)
	if err != nil {
		t.Fatalf("ListAlertsByUser page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 len: want 2 got %d", len(page2))
	}
}

// ---------------------------------------------------------------------------
// RegisterPush
// ---------------------------------------------------------------------------

func TestNotificationService_RegisterPush_HappyPath(t *testing.T) {
	svc := newNotifSvc()
	userID := uuid.New()

	input := notificaciones.NotificacionEnviada{
		UserID:     userID,
		Token:      "fcm-token-abc",
		DeviceType: notificaciones.DeviceTypeAndroid,
	}
	got, err := svc.RegisterPush(context.Background(), input)
	if err != nil {
		t.Fatalf("RegisterPush: %v", err)
	}
	if got.ID == uuid.Nil {
		t.Error("RegisterPush: expected non-nil ID")
	}
}

// ---------------------------------------------------------------------------
// ListPushByUser
// ---------------------------------------------------------------------------

func TestNotificationService_ListPushByUser_HappyPath(t *testing.T) {
	svc := newNotifSvc()
	userID := uuid.New()

	for i := 0; i < 4; i++ {
		if _, err := svc.RegisterPush(context.Background(), notificaciones.NotificacionEnviada{
			UserID:     userID,
			Token:      "tok",
			DeviceType: notificaciones.DeviceTypeWeb,
		}); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	list, total, err := svc.ListPushByUser(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("ListPushByUser: %v", err)
	}
	if total != 4 {
		t.Errorf("total: want 4 got %d", total)
	}
	if len(list) != 4 {
		t.Errorf("len: want 4 got %d", len(list))
	}
}

// ---------------------------------------------------------------------------
// EventPublisher integration
// ---------------------------------------------------------------------------

func TestNotificationService_CreateAlert_PublishesEvent(t *testing.T) {
	svc, pub := newNotifSvcWithPublisher()
	userID := uuid.New()

	_, err := svc.CreateAlert(context.Background(), notificaciones.NotificacionAtencion{
		UserID:  userID,
		Mensaje: "event test",
		Estatus: notificaciones.EstatusUnread,
	})
	if err != nil {
		t.Fatalf("CreateAlert: %v", err)
	}
	if !pub.published("notificaciones:notificacion_enviada") {
		t.Error("CreateAlert: expected StreamNotificacionEnviada to be published")
	}
}

func TestNotificationService_MarkAsRead_PublishesEvent(t *testing.T) {
	svc, pub := newNotifSvcWithPublisher()
	userID := uuid.New()

	created, err := svc.CreateAlert(context.Background(), notificaciones.NotificacionAtencion{
		UserID:  userID,
		Mensaje: "msg",
		Estatus: notificaciones.EstatusUnread,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = svc.MarkAsRead(context.Background(), userID, created.ID)
	if err != nil {
		t.Fatalf("MarkAsRead: %v", err)
	}
	if !pub.published("notificaciones:notificacion_leida") {
		t.Error("MarkAsRead: expected StreamNotificacionLeida to be published")
	}
}

func TestNotificationService_RegisterPush_PublishesEvent(t *testing.T) {
	svc, pub := newNotifSvcWithPublisher()
	userID := uuid.New()

	_, err := svc.RegisterPush(context.Background(), notificaciones.NotificacionEnviada{
		UserID:     userID,
		Token:      "fcm-tok",
		DeviceType: notificaciones.DeviceTypeAndroid,
	})
	if err != nil {
		t.Fatalf("RegisterPush: %v", err)
	}
	if !pub.published("notificaciones:dispositivo_registrado") {
		t.Error("RegisterPush: expected StreamDispositivoRegistrado to be published")
	}
}
