// Shared test helpers for the formularios service package.
//
// PR-3 (Services) — the recordingPublisher captures (stream, payload) tuples
// for every Publish call so the service tests can assert on the event the
// Gherkin scenario requires. The 09 module uses a single-payload spy; we
// need a slice spy to support multiple publish calls in a single test
// (CreateFormulario publishes 1, GenerateReporte publishes 1, etc.).
package service_test

import (
	"context"
)

// recordedEvent captures a single Publish call.
type recordedEvent struct {
	stream  string
	payload map[string]any
}

// recordingPublisher is an events.EventPublisher spy that records every
// event in insertion order. If err is non-nil, Publish returns it so the
// postWriteHook swallow path can be exercised in tests.
type recordingPublisher struct {
	events []recordedEvent
	err    error
}

func (p *recordingPublisher) Publish(_ context.Context, stream string, payload map[string]any) error {
	p.events = append(p.events, recordedEvent{stream: stream, payload: payload})
	return p.err
}
