// Package jobs holds the concrete business scheduled jobs the
// Scheduler service runs on a cron cadence.
//
// This file defines the OutboxWriter contract used by jobs that
// need to publish integration events transactionally with their
// primary state changes. The auto-deactivation job
// (`inactiva_empleado.go`, PR 3B-ii) is the first consumer; future
// jobs (cleanup, close, etc.) that need the same at-least-once
// delivery semantics can adopt the same interface.
//
// The outbox pattern
// ==================
// The Scheduler Service spec (section "Eventos > Publica")
// documents the delivery mode for the events emitted by these
// jobs as "At-least-once via Outbox". The outbox is a single
// database row written inside the SAME transaction that performs
// the business state change. A separate outbox poller (not part
// of this file — wired in a later PR) drains the outbox and
// forwards the payload to Redis Streams / the Notificaciones
// service. This split gives us:
//
//   - Atomicity: if the business change fails, the outbox row is
//     not written. If the outbox write fails, the business
//     change is rolled back. The downstream consumer only ever
//     sees events whose business state has already been
//     committed.
//   - At-least-once delivery: the poller is allowed to crash and
//     resume; the only constraint is that it must not delete the
//     outbox row until the downstream publish has been acked.
//
// Transactional contract
// =======================
// The interface accepts a Tx (the minimal Commit/Rollback
// abstraction defined in employee_repository.go) so the writer
// participates in the caller's transaction. The decision to
// thread the tx through the interface (rather than hold a
// pool inside the writer) is documented here because it
// deviates from a typical "ctx-only" pattern: we need the
// outbox row to be in the same atomicity envelope as the
// empleado status update and the identity revoke. A writer
// that owns its own transaction would force those three
// operations into separate transactions and re-introduce the
// dual-write problem the outbox pattern is designed to solve.
//
// The interface takes the `Tx` contract instead of the
// concrete `pgx.Tx` so the inactiva_empleado job can drive
// the envelope through the EmployeeRepository abstraction
// (PR 5B-ii). The production pgx-backed implementation
// receives the underlying pgx.Tx through the Tx wrapper; the
// noop implementation ignores the argument.
//
// The noop default (NewNoopOutboxWriter) returns nil without
// touching the transaction. The real implementation will land in
// a follow-up PR (PR 4 — REST + wiring) once the
// scheduler.outbox table is provisioned; the noop keeps the
// system observable (and the build green) in the meantime.
package jobs

import (
	"context"
)

// ---------------------------------------------------------------------------
// OutboxWriter — public contract.
// ---------------------------------------------------------------------------

// OutboxWriter writes an integration event into the scheduler
// outbox inside the caller's transaction. The writer is the only
// place the job talks to the outbox; the rest of the job stays
// focused on the business state change.
//
// Implementations MUST execute the insert on the supplied tx so
// the row is part of the same atomic envelope as the surrounding
// updates. A noop implementation may simply return nil; the
// default wiring uses the noop until the real table is in place
// (PR 4).
type OutboxWriter interface {
	// Insert appends a new outbox row to the caller's
	// transaction. The (eventType, aggregateID, payload) tuple
	// is the contract the outbox poller consumes; payload MUST
	// already be JSON-encoded (the caller owns the serialisation
	// shape so the poller can match by event name without a
	// second deserialisation pass).
	//
	// eventType is the canonical event name from the spec
	// (e.g. "EmpleadoInactivadoAutomaticamente"); aggregateID is
	// the empleado id the event is about; payload is the
	// already-JSON-encoded event body.
	//
	// Implementations should treat any driver-level error
	// (serialisation, constraint violation, context
	// cancellation) as a transactional failure and return it
	// unwrapped so the caller's %w wrapping makes the chain
	// traceable in the scheduler_jobs_failed_total error_class
	// label.
	Insert(ctx context.Context, tx Tx, eventType string, aggregateID int64, payload []byte) error
}

// ---------------------------------------------------------------------------
// noopOutboxWriter — default implementation used until the real
// outbox infra is wired in (PR 4).
// ---------------------------------------------------------------------------

// noopOutboxWriter satisfies OutboxWriter by returning nil. It is
// the default dependency used by NewInactivaEmpleadoJob when the
// caller has not yet wired the real outbox infra. The
// (eventType, aggregateID, payload, tx) arguments are accepted but
// ignored: the noop deliberately does NOT log, because the job
// already emits a per-empresa summary line and a per-empleado
// debug line. Adding another log sink here would double-count the
// event.
type noopOutboxWriter struct{}

// NewNoopOutboxWriter returns a no-op OutboxWriter. Useful as a
// default in tests and in the early wiring layer (main.go) before
// the real scheduler.outbox table is provisioned.
func NewNoopOutboxWriter() OutboxWriter {
	return &noopOutboxWriter{}
}

// Insert is a no-op that always returns nil. See the type-level
// doc comment for the rationale.
func (n *noopOutboxWriter) Insert(_ context.Context, _ Tx, _ string, _ int64, _ []byte) error {
	return nil
}
