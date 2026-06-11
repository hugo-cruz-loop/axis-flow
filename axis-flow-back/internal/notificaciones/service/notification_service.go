package service

import (
	"context"
	"encoding/json"
	"time"

	"axis-flow-back/internal/notificaciones"
	"axis-flow-back/internal/notificaciones/events"
	"axis-flow-back/internal/notificaciones/repository"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// NotificationService — interface.
// ---------------------------------------------------------------------------

// NotificationService defines use-case operations for in-app and push notifications.
type NotificationService interface {
	// CreateAlert persists a new atencion notification and returns the record.
	CreateAlert(ctx context.Context, n notificaciones.NotificacionAtencion) (notificaciones.NotificacionAtencion, error)

	// ListAlertsByUser returns paginated atencion notifications for a user.
	ListAlertsByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionAtencion, int, error)

	// CountUnread returns the number of unread atencion notifications for a user.
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)

	// MarkAsRead marks an atencion notification as read.
	// IDOR check: requesterID must match the notification's UserID; returns ErrForbidden if not.
	MarkAsRead(ctx context.Context, requesterID, notifID uuid.UUID) (notificaciones.NotificacionAtencion, error)

	// RegisterPush persists a push notification record and returns it.
	RegisterPush(ctx context.Context, n notificaciones.NotificacionEnviada) (notificaciones.NotificacionEnviada, error)

	// ListPushByUser returns paginated push notification records for a user.
	ListPushByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionEnviada, int, error)
}

// ---------------------------------------------------------------------------
// pgNotificationService — concrete implementation.
// ---------------------------------------------------------------------------

type pgNotificationService struct {
	alerts    repository.AlertRepository
	pushes    repository.PushRepository
	publisher events.EventPublisher
}

// NewNotificationService constructs a NotificationService backed by the given repositories
// and an EventPublisher for fire-and-forget domain events.
func NewNotificationService(alerts repository.AlertRepository, pushes repository.PushRepository, publisher events.EventPublisher) NotificationService {
	return &pgNotificationService{alerts: alerts, pushes: pushes, publisher: publisher}
}

// publish marshals payload and fires the event. Errors are intentionally ignored
// (fire-and-forget semantics — the primary operation must not fail if publishing fails).
func (s *pgNotificationService) publish(ctx context.Context, stream string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = s.publisher.Publish(ctx, stream, data)
}

func (s *pgNotificationService) CreateAlert(ctx context.Context, n notificaciones.NotificacionAtencion) (notificaciones.NotificacionAtencion, error) {
	result, err := s.alerts.InsertAlert(ctx, n)
	if err != nil {
		return notificaciones.NotificacionAtencion{}, err
	}
	s.publish(ctx, events.StreamNotificacionEnviada, map[string]any{
		"notification_id": result.ID,
		"user_id":         result.UserID,
	})
	return result, nil
}

func (s *pgNotificationService) ListAlertsByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionAtencion, int, error) {
	return s.alerts.ListByUser(ctx, userID, page, pageSize)
}

func (s *pgNotificationService) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.alerts.CountUnread(ctx, userID)
}

// MarkAsRead fetches the alert, enforces IDOR, then updates estatus to Read.
// Returns ErrForbidden when requesterID does not match the alert owner.
// Returns ErrNotFound when the alert does not exist.
func (s *pgNotificationService) MarkAsRead(ctx context.Context, requesterID, notifID uuid.UUID) (notificaciones.NotificacionAtencion, error) {
	alert, err := s.alerts.GetByID(ctx, notifID)
	if err != nil {
		return notificaciones.NotificacionAtencion{}, err
	}
	if alert.UserID != requesterID {
		return notificaciones.NotificacionAtencion{}, notificaciones.ErrForbidden
	}
	updated, err := s.alerts.UpdateEstatus(ctx, notifID, int(notificaciones.EstatusRead))
	if err != nil {
		return notificaciones.NotificacionAtencion{}, err
	}
	s.publish(ctx, events.StreamNotificacionLeida, map[string]any{
		"notification_id": updated.ID,
		"user_id":         updated.UserID,
		"read_at":         time.Now().UTC(),
	})
	return updated, nil
}

func (s *pgNotificationService) RegisterPush(ctx context.Context, n notificaciones.NotificacionEnviada) (notificaciones.NotificacionEnviada, error) {
	result, err := s.pushes.InsertPush(ctx, nil, n)
	if err != nil {
		return notificaciones.NotificacionEnviada{}, err
	}
	s.publish(ctx, events.StreamDispositivoRegistrado, map[string]any{
		"user_id":     result.UserID,
		"device_type": result.DeviceType,
	})
	return result, nil
}

func (s *pgNotificationService) ListPushByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionEnviada, int, error) {
	return s.pushes.ListPushByUser(ctx, userID, page, pageSize)
}
