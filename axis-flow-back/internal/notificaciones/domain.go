// Package notificaciones defines domain types, constants, repository interfaces,
// and sentinel errors for the Notificaciones module.
package notificaciones

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// DeviceType — push notification target device category.
// ---------------------------------------------------------------------------

// DeviceType represents the category of device receiving a push notification.
type DeviceType string

const (
	DeviceTypeWeb     DeviceType = "WEB"
	DeviceTypeAndroid DeviceType = "ANDROID"
	DeviceTypeIOS     DeviceType = "IOS"
)

// ---------------------------------------------------------------------------
// EstatusNotif — read status for atencion notifications.
// ---------------------------------------------------------------------------

// EstatusNotif represents the read/unread state of an atencion notification.
type EstatusNotif int

const (
	EstatusUnread EstatusNotif = 1
	EstatusRead   EstatusNotif = 2
)

// ---------------------------------------------------------------------------
// Domain structs.
// ---------------------------------------------------------------------------

// NotificacionEnviada records a push notification sent to a specific device.
type NotificacionEnviada struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Token      string
	NotiID     *string
	DeviceType DeviceType
	CreatedAt  time.Time
}

// NotificacionAtencion is an in-app notification linked to a service ticket
// or complaint.
type NotificacionAtencion struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TicketID  *int
	QuejaID   *int
	Mensaje   string
	Estatus   EstatusNotif
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	ErrNotFound     = errors.New("notificaciones: not found")
	ErrForbidden    = errors.New("notificaciones: forbidden")
	ErrInvalidInput = errors.New("notificaciones: invalid input")
)
