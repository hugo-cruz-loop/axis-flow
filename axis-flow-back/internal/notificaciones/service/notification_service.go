package service

import (
	"context"

	"axis-flow-back/internal/notificaciones"
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
	alerts repository.AlertRepository
	pushes repository.PushRepository
}

// NewNotificationService constructs a NotificationService backed by the given repositories.
func NewNotificationService(alerts repository.AlertRepository, pushes repository.PushRepository) NotificationService {
	return &pgNotificationService{alerts: alerts, pushes: pushes}
}

func (s *pgNotificationService) CreateAlert(ctx context.Context, n notificaciones.NotificacionAtencion) (notificaciones.NotificacionAtencion, error) {
	return s.alerts.InsertAlert(ctx, n)
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
	return s.alerts.UpdateEstatus(ctx, notifID, int(notificaciones.EstatusRead))
}

func (s *pgNotificationService) RegisterPush(ctx context.Context, n notificaciones.NotificacionEnviada) (notificaciones.NotificacionEnviada, error) {
	return s.pushes.InsertPush(ctx, nil, n)
}

func (s *pgNotificationService) ListPushByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionEnviada, int, error) {
	return s.pushes.ListPushByUser(ctx, userID, page, pageSize)
}
