// Package events defines event publisher interfaces and stream name constants
// for the Notificaciones module.
package events

import "context"

// Stream name constants — use these exclusively; never use bare string literals
// in publisher call sites.
const (
	StreamNotificacionEnviada      = "notificaciones:notificacion_enviada"
	StreamNotificacionLeida        = "notificaciones:notificacion_leida"
	StreamDispositivoRegistrado    = "notificaciones:dispositivo_registrado"
)

// EventPublisher abstracts event publishing so the service layer can be tested
// without a real Redis instance.
type EventPublisher interface {
	Publish(ctx context.Context, stream string, payload []byte) error
}

// NoopPublisher discards every published event. Use in tests or when event
// publishing is intentionally disabled.
type NoopPublisher struct{}

// Publish implements EventPublisher by discarding the event.
func (NoopPublisher) Publish(_ context.Context, _ string, _ []byte) error {
	return nil
}
