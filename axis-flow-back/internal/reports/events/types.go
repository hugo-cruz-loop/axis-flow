// Package events provides the event publisher interface, a no-op implementation,
// stream name constants, and the EmpresaDeBaja consumer for the Reports module.
package events

import "context"

// EventPublisher publishes events to a named stream.
type EventPublisher interface {
	Publish(ctx context.Context, stream string, payload any) error
}

// NoopPublisher is a no-op EventPublisher used in tests and when Redis is absent.
type NoopPublisher struct{}

// Publish discards the event and returns nil.
func (n *NoopPublisher) Publish(_ context.Context, _ string, _ any) error { return nil }

// Stream name constants — ALWAYS use these; never write bare strings.
const (
	// StreamGeocodingCacheEvicted is published when the geocoding cache is
	// purged (e.g. after an EmpresaDeBaja event wipes all entries for a tenant).
	StreamGeocodingCacheEvicted = "reports:geocoding_cache_evicted"

	// StreamEmpresaDeBaja is consumed (not published) by this module.
	// The empresas module publishes to this stream when a company is deactivated.
	StreamEmpresaDeBaja = "empresas:empresa_de_baja"
)
