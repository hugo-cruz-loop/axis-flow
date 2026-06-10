// Package events defines the event publishing interface and stream name
// constants for the AtencionSeguimiento module.
package events

import "context"

// EventPublisher publishes domain events to a named stream.
type EventPublisher interface {
	Publish(ctx context.Context, stream string, payload map[string]any) error
}

// NoopPublisher is an EventPublisher that discards all events.
// It is intended for use in unit tests that do not need to verify publishing.
type NoopPublisher struct{}

// Publish implements EventPublisher and always returns nil.
func (n *NoopPublisher) Publish(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

// Stream name constants for the AtencionSeguimiento domain.
const (
	StreamQuejaRegistrada              = "atencion:queja_registrada"
	StreamQuejaRespuestaEnviada        = "atencion:queja_respuesta_enviada"
	StreamTicketServicioCreado         = "atencion:ticket_servicio_creado"
	StreamTicketServicioActualizado    = "atencion:ticket_servicio_actualizado"
	StreamIncidenciaOperativaRegistrada = "atencion:incidencia_operativa_registrada"
)
