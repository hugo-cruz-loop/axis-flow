// Package events defines the event publishing interface and stream name
// constants for the Formularios module.
//
// Mirrors internal/atencionseguimiento/events/types.go (PR-1 of
// 09_AtencionSeguimiento_Service_Spec). The stream name pattern is
// `<service>:<event>` in snake_case (the 09 module uses the same shape).
// The actual Redis XADD / outbox publisher will be wired in PR-5; this
// file only declares the contract.
package events

import "context"

// ---------------------------------------------------------------------------
// Publisher port.
// ---------------------------------------------------------------------------

// EventPublisher publishes domain events to a named stream.
//
// Implementations are expected to be fire-and-forget at the call site (the
// service layer does not retry on publish errors). The PR-5 publisher will
// implement the outbox pattern with at-least-once semantics; in unit tests
// the publisher is replaced with NoopPublisher or a recording spy.
type EventPublisher interface {
	Publish(ctx context.Context, stream string, payload map[string]any) error
}

// NoopPublisher is an EventPublisher that discards all events. Intended for
// unit tests that do not need to verify publishing.
type NoopPublisher struct{}

// Publish implements EventPublisher and always returns nil.
func (n *NoopPublisher) Publish(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

// ---------------------------------------------------------------------------
// Stream name constants.
//
// Names map 1:1 to the Gherkin scenarios and the OpenAPI event_published
// refs in docs/services/10_Formularios_Service_Spec/. The DLQ names from
// the spec (e.g. "formularios-formulario-creado-dlq") are kept in the PR-5
// publisher; the service layer only cares about the stream name.
// ---------------------------------------------------------------------------

const (
	// StreamFormularioCreado is published when a new Formulario template
	// is successfully persisted (Gherkin 1: Creación de plantilla).
	StreamFormularioCreado = "formularios:formulario_creado"

	// StreamEventoIniciado is published when an employee check-in starts
	// execution of an Evento (Gherkin 2: Llenado en campo).
	StreamEventoIniciado = "formularios:evento_iniciado"

	// StreamFormularioRespondido is published when a Respuesta is
	// captured for a running EventoIniciado (Gherkin 2: Llenado en campo).
	StreamFormularioRespondido = "formularios:formulario_respondido"

	// StreamReporteGenerado is published when the PDF report is compiled
	// and uploaded to S3 (Gherkin 3: Generación asíncrona de reporte PDF).
	StreamReporteGenerado = "formularios:reporte_generado"
)
