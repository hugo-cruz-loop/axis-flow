// Package events defines the integration boundary between the Scheduler
// service and the Notificaciones service. The Scheduler emits business
// events (PreShiftAlertaProgramada, EmpleadoInactivadoAutomaticamente) to
// describe what just happened; downstream services (Notificaciones, HR
// Dashboard) consume those events to materialise the user-facing effect.
//
// Boundary contract
// =================
// The Scheduler NEVER imports the Notificaciones service package
// directly. The wiring layer in main.go (Phase 4) is the only place
// that knows about the concrete publisher implementation. Every
// concrete job that needs to emit an event receives an EventPublisher
// through its constructor and falls back to NoopEventPublisher when
// the wiring has not yet landed. This file lives under
// `internal/scheduler/events` so the boundary is self-documenting and
// future publishers (HTTP client, Redis Streams producer, outbox row)
// can be added without touching job code.
package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// PreShiftAlertEvent — the payload described in the Scheduler Service
// spec section "Eventos > Publica > PreShiftAlertaProgramada".
// ---------------------------------------------------------------------------

// PreShiftAlertEvent is the JSON payload exchanged between the Scheduler
// service and the Notificaciones service when a pre-shift push
// notification has been (or is about to be) dispatched. Field names
// match the spec verbatim:
//
//	alerta_id     UUID
//	empleado_id   bigint
//	fcm_token     string
//	tipo_evento   string  ("entrada" | "comida" | "salida")
//	timestamp     RFC3339 timestamp
//
// FCMToken in the Scheduler-side copy is the redacted prefix (first
// 8 characters + ellipsis) — the actual token never leaves the FCM
// call site, only the prefix is forwarded to the Notificaciones
// service so the row it writes into notificaciones.notificacionenviada
// can correlate the delivery with the originating alert. When a real
// publisher is wired in Phase 4 it will resolve the full token from
// the device row before writing the audit row.
type PreShiftAlertEvent struct {
	// AlertID is a freshly-minted UUID v4 that uniquely identifies
	// the alert attempt. It becomes the primary key of the row the
	// Notificaciones service writes into notificaciones.notificacionenviada.
	AlertID uuid.UUID
	// EmpleadoID is the business identifier of the target employee.
	// It is the FK the Notificaciones service uses to fan the alert
	// out to the WebSocket group.
	EmpleadoID int64
	// FCMToken is the redacted FCM token prefix (first 8 chars +
	// ellipsis). The full token is never serialised outside the FCM
	// call site.
	FCMToken string
	// TipoEvento is the human-readable shift event class:
	// "entrada", "comida" or "salida".
	TipoEvento string
	// Timestamp is the wall-clock instant the alert was emitted.
	Timestamp time.Time
}

// ---------------------------------------------------------------------------
// EventPublisher — the integration boundary.
// ---------------------------------------------------------------------------

// EventPublisher is the narrow contract the Scheduler jobs use to
// emit integration events. Each method MUST be safe for concurrent
// invocation — the inactiva_empleado job (PR 3B-ii) and the
// notificaciones job (this PR) may both call PublishPreShiftAlert
// from different goroutines when the cron runner schedules them on
// overlapping ticks.
//
// Implementations are expected to be best-effort from the Scheduler
// side: a publisher error is logged inside the publisher and
// surfaced to the caller so the job can decide whether to retry,
// fail the run, or move on. The job treats the alert dispatch as the
// primary success criterion (the Notificaciones write is a
// correlation log, not the source of truth for the user-facing
// notification).
type EventPublisher interface {
	// PublishPreShiftAlert emits a PreShiftAlertaProgramada event
	// for the given alert. Returns a non-nil error only for
	// unrecoverable transport failures; the implementation is
	// expected to retry transient errors itself.
	PublishPreShiftAlert(ctx context.Context, evt PreShiftAlertEvent) error
}

// ---------------------------------------------------------------------------
// NoopEventPublisher — default implementation used when the real
// publisher has not been wired in (Phase 4 main.go).
// ---------------------------------------------------------------------------

// NoopEventPublisher is the default EventPublisher used until the
// real publisher (HTTP client to the Notificaciones service, or a
// Redis Streams producer) is wired in main.go. It logs the alert at
// debug level — production observability for the alert will come
// from the existing scheduler_job_duration_seconds / per-shift
// summary log, not from this sink — and returns nil.
//
// Tests and wiring code that have not yet been updated can rely on
// the noop to keep the system observable: the alert has already been
// dispatched to FCM by the time PublishPreShiftAlert is called, so
// "drop the event" is the safe default while the Notificaciones
// service integration is being built.
type NoopEventPublisher struct {
	// Logger is the structured logger used for the debug line.
	// nil is allowed; the noop falls back to slog.Default().
	Logger *slog.Logger
}

// NewNoopEventPublisher returns a NoopEventPublisher that uses the
// supplied *slog.Logger. A nil logger is replaced with slog.Default()
// so the noop is always safe to call.
func NewNoopEventPublisher(logger *slog.Logger) *NoopEventPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &NoopEventPublisher{Logger: logger}
}

// PublishPreShiftAlert logs the alert at debug level and returns nil.
func (n *NoopEventPublisher) PublishPreShiftAlert(_ context.Context, evt PreShiftAlertEvent) error {
	if n == nil || n.Logger == nil {
		return nil
	}
	prefix := redactTokenPrefix(evt.FCMToken)
	n.Logger.Debug("noop event publisher: pre-shift alert event",
		slog.String("alerta_id", evt.AlertID.String()),
		slog.Int64("empleado_id", evt.EmpleadoID),
		slog.String("fcm_token_prefix", prefix),
		slog.String("tipo_evento", evt.TipoEvento),
		slog.Time("timestamp", evt.Timestamp),
	)
	return nil
}

// redactTokenPrefix returns the first 8 characters of a token
// followed by an ellipsis, mirroring the FCM client's redaction
// policy. Tokens shorter than 8 characters are returned in full.
func redactTokenPrefix(t string) string {
	if len(t) <= 8 {
		return t
	}
	return t[:8] + "…"
}
