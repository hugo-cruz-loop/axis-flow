// Package service — hooks.go: fire-and-forget post-write helper.
//
// postWriteHook is the single chokepoint for the publish + cache-invalidate
// pair every mutating service method performs. The helper swallows both
// the publish error and the cache error (the spec's outbox pattern in PR-5
// is the source of truth for delivery; cache invalidation is best-effort).
//
// PR-3 task 3.5 refactor consolidates the previously-inlined call sites
// into this single helper and adds the no-PII audit.
//
// No-PII audit (PR-3 task 3.5):
//
//   - The helper itself takes a payload map[string]any. Call sites are
//     responsible for ensuring the payload does NOT contain user-supplied
//     strings (nombre, descripcion, texto_pregunta, evidencia URLs, file
//     paths, geo coordinates).
//   - Current call sites use only IDs (UUIDs, int64 empleadoID) and
//     time.Time stamps in the payload. No PII fields are included.
//   - On a future field-schema change, the audit MUST be re-run: search
//     for `Stream*` constants in this package and check that the
//     corresponding payload map does not include user-supplied strings.
//
// Companion rule: error returns across the 4 services must use generic
// sentinel errors (formularios.ErrInvalidInput / ErrForbidden /
// ErrNotFound / ErrConflict) or generic fmt.Errorf("pdf: <step> failed")
// without wrapping the underlying error. The PR-3 test
// `*NoPIIInErrorMessages` (one per service) asserts the error message
// does not contain the offending user input.
package service

import (
	"context"

	"axis-flow-back/internal/formularios/events"
)

// postWriteHook fires a publish on stream and an optional cache
// invalidation. Both errors are intentionally swallowed: the spec uses an
// outbox pattern (PR-5) for at-least-once delivery, and cache invalidation
// is best-effort. If a stricter failure mode is needed in the future, swap
// this helper for one that returns the error and update the call sites.
//
//   - ctx       : request context (used for cancellation/tracing).
//   - pub       : event publisher (must not be nil — pass events.NoopPublisher{} in tests).
//   - cache     : cache invalidation closure; may be nil (no-op).
//   - stream    : stream name constant (e.g. events.StreamFormularioCreado).
//   - payload   : event payload (UUIDs and short strings only; no PII).
func postWriteHook(
	ctx context.Context,
	pub events.EventPublisher,
	cache func(context.Context) error,
	stream string,
	payload map[string]any,
) {
	_ = pub.Publish(ctx, stream, payload)
	if cache != nil {
		_ = cache(ctx)
	}
}
