// Package events defines the event publishing contract for the bolsatrabajo module.
package events

import "context"

// EventPublisher is the port for publishing domain events to an external stream.
// Implementations are provided by the infrastructure layer (e.g. NATS, SQS).
type EventPublisher interface {
	// Publish emits a named event to the given stream with an arbitrary payload.
	Publish(ctx context.Context, stream string, payload map[string]any) error
}

// NoopPublisher is a no-op EventPublisher used in tests and development.
// Every Publish call returns nil without performing any I/O.
type NoopPublisher struct{}

// Publish always returns nil.
func (NoopPublisher) Publish(_ context.Context, _ string, _ map[string]any) error { return nil }

// Compile-time check.
var _ EventPublisher = NoopPublisher{}
