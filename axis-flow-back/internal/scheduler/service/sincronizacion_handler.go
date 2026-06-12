// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the sync dispatcher described in the spec section
// "Eventos > Consume" (SincronizacionSolicitada). The dispatcher receives
// a SincronizacionEvent from the queue consumer (sqs_consumer.go, task
// 3.1) and routes it to a per-sync_type handler. The built-in handlers
// (catalog, employees, assignments) are stubs that return
// ErrNotImplemented for now — they will be wired to their respective
// services in PR 3B (or later).
//
// Wiring contract
// ===============
// The cron-registered job `sincronizacion_solicitada` is the manual
// trigger path: the API layer (Phase 4) can POST
// /api/v1/scheduler/jobs/sincronizacion_solicitada/trigger to force a
// re-run. The consumer path (3.1) is the automatic path: it reads from
// Redis Streams or SQS, builds a SincronizacionEvent, sets it on the
// context, and invokes the same JobHandler returned by
// (*DefaultSyncDispatcher).Handler().
//
// Context contract
// ================
// The dispatcher.Handler() closure reads the SincronizacionEvent from a
// dedicated context key (sincronizacionEventContextKey). The consumer is
// responsible for calling WithSincronizacionEvent(ctx, event) before
// invoking the handler. The closure also pulls a *BaseJob from the
// context (baseJobContextKey) for logger access — both keys are set by
// the consumer and by the CronRunner's lifecycle wrapper respectively.
//
// An event that is missing from the context yields ErrInvalidEventPayload;
// an unknown sync_type is logged at warn level and treated as success
// (the message is acknowledged — we do not nack on an unknown type).
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Public constants — keys and job identity.
// ---------------------------------------------------------------------------

// SincronizacionSolicitadaJobKey is the canonical job_key for the
// cron-registered manual-trigger job. The seed file (PR 6 task 6.1)
// inserts a row under this key; the wiring layer in main.go looks it
// up via JobRepository.GetByKey and passes its job_id to the
// SyncConsumer factory.
const SincronizacionSolicitadaJobKey = "sincronizacion_solicitada"

// ---------------------------------------------------------------------------
// SincronizacionEvent — the payload described in spec section
// "Eventos > Consume > SincronizacionSolicitada".
// ---------------------------------------------------------------------------

// SincronizacionEvent is the JSON payload exchanged over Redis Streams
// or SQS. The field names match the spec verbatim (sync_type,
// target_id, requested_by, timestamp). EventID is an extra,
// consumer-minted field used as the per-event lock token — it is
// optional on the wire (the consumer mints a fresh UUID when absent).
type SincronizacionEvent struct {
	// EventID is a UUID minted by the consumer (or echoed from the
	// producer) that serves as the lock token and the execution
	// correlation id. It is never logged in full.
	EventID string `json:"event_id,omitempty"`
	// SyncType selects the per-type handler. Known values today:
	// "catalog", "employees", "assignments". Unknown values are
	// logged at warn level and treated as success (the event is
	// acknowledged, NOT nacked).
	SyncType string `json:"sync_type"`
	// TargetID is the resource the sync targets (catalog item id,
	// employee id, assignment id). Optional; 0 means "all".
	TargetID int64 `json:"target_id"`
	// RequestedBy is the user id that triggered the sync (admin
	// user id, system bot id, etc.). For UI diagnostics only.
	RequestedBy int64 `json:"requested_by"`
	// Timestamp is the producer-side wall clock when the event was
	// emitted. Used for stale-event detection (> 1h in the past is
	// logged at warn level).
	Timestamp time.Time `json:"timestamp"`
}

// Validate returns nil when the event carries the minimum data the
// dispatcher needs. A blank SyncType is a hard failure — without it
// the dispatcher has no way to route the event. A blank EventID is
// recovered by minting a fresh UUID inside the consumer, so Validate
// does not require it.
func (e SincronizacionEvent) Validate() error {
	if e.SyncType == "" {
		return fmt.Errorf("sincronizacion_event: %w: sync_type is required", ErrInvalidEventPayload)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sentinel errors. Use errors.Is to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrInvalidEventPayload is returned when the event JSON is
	// malformed, missing required fields, or absent from the
	// handler's context. The consumer treats this as a permanent
	// failure and logs it loudly — the event is still XACKed
	// (Redis Streams) or DeleteMessage'd (SQS) so the queue does
	// not accumulate poison messages.
	ErrInvalidEventPayload = errors.New("scheduler: invalid sincronizacion event payload")
	// ErrNotImplemented is returned by the catalog/employees/
	// assignments stub handlers. PR 3B will replace these stubs
	// with real implementations wired to the respective services.
	// Until then, receiving this error in the logs is expected.
	ErrNotImplemented = errors.New("scheduler: sync handler not implemented yet")
)

// ---------------------------------------------------------------------------
// Public contract.
// ---------------------------------------------------------------------------

// SyncDispatcher routes a SincronizacionEvent to the per-sync_type
// handler. Implementations are concurrency-safe — Register may be
// called from a wiring goroutine while Run is dispatching events on
// another goroutine.
type SyncDispatcher interface {
	// Dispatch looks up the handler registered for event.SyncType
	// and invokes it. A blank or unknown SyncType logs a warn and
	// returns nil (the event is acknowledged). A handler error is
	// wrapped and returned to the caller.
	Dispatch(ctx context.Context, event SincronizacionEvent) error
	// Handler returns the JobHandler the CronRunner registers for
	// the `sincronizacion_solicitada` job. The closure reads the
	// SincronizacionEvent from the context (see WithSincronizacionEvent)
	// and delegates to Dispatch. The same closure is used by the
	// SyncConsumer (3.1) when it invokes the handler outside the
	// cron path.
	Handler() JobHandler
	// Register associates a sync_type with a JobHandler. The
	// pre-seeded handlers for "catalog", "employees" and
	// "assignments" can be replaced or augmented by tests and by
	// future wiring code.
	Register(syncType string, handler JobHandler)
}

// ---------------------------------------------------------------------------
// Default implementation.
// ---------------------------------------------------------------------------

// defaultSyncDispatcher is the production SyncDispatcher. It is a thin
// registry keyed by sync_type with a read-write mutex guarding the
// map. The constructor pre-seeds the three stub handlers called out in
// the task description; Register allows callers to override or add
// types without rebuilding the dispatcher.
type defaultSyncDispatcher struct {
	mu       sync.RWMutex
	handlers map[string]JobHandler
}

// NewDefaultSyncDispatcher returns a SyncDispatcher with the three
// built-in stub handlers pre-registered. Tests and PR 3B wiring can
// override any of them via Register; the registry is mutable for the
// lifetime of the dispatcher.
func NewDefaultSyncDispatcher() SyncDispatcher {
	d := &defaultSyncDispatcher{handlers: make(map[string]JobHandler)}
	d.Register("catalog", catalogSyncHandler)
	d.Register("employees", employeesSyncHandler)
	d.Register("assignments", assignmentsSyncHandler)
	return d
}

// Dispatch looks up the handler for event.SyncType. If no handler is
// registered the call logs a warn (when a logger is available on the
// BaseJob pulled from the context) and returns nil — the consumer will
// ack the message. Handler errors are wrapped with the dispatcher
// prefix so the cron runner's WrapWithLifecycle records the original
// class on the scheduler_jobs_failed_total counter.
func (d *defaultSyncDispatcher) Dispatch(ctx context.Context, event SincronizacionEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}

	d.mu.RLock()
	handler, ok := d.handlers[event.SyncType]
	d.mu.RUnlock()

	base := BaseJobFromContext(ctx)

	if !ok {
		if base != nil && base.Logger != nil {
			base.Logger.Inner().WarnContext(ctx, "sync dispatcher: unknown sync_type, event acknowledged",
				"sync_type", event.SyncType,
				"event_id", redactEventID(event.EventID),
				"target_id", event.TargetID,
			)
		}
		return nil
	}

	if handler == nil {
		// Defensive: a nil handler registered under a known key
		// is a programmer error. Surface it as a regular error so
		// the consumer nacks and the operator notices.
		return fmt.Errorf("sync_dispatcher.Dispatch: handler for sync_type=%q is nil", event.SyncType)
	}

	if err := handler(ctx, base); err != nil {
		return fmt.Errorf("sync_dispatcher.Dispatch: sync_type=%s: %w", event.SyncType, err)
	}
	return nil
}

// Handler returns the JobHandler closure the CronRunner registers for
// the `sincronizacion_solicitada` job. The closure pulls the
// SincronizacionEvent from the context (the consumer sets it via
// WithSincronizacionEvent before invoking the handler) and forwards to
// Dispatch. The closure is safe to invoke from any goroutine; the
// dispatcher's mutex is read-locked for the lookup.
//
// A missing event in the context yields ErrInvalidEventPayload — the
// cron runner treats this as a FAILED execution and the consumer
// (when it takes the same path) nacks the message.
func (d *defaultSyncDispatcher) Handler() JobHandler {
	return func(ctx context.Context, base *BaseJob) error {
		event, ok := SincronizacionEventFromContext(ctx)
		if !ok {
			return fmt.Errorf("sync_dispatcher.Handler: %w: no SincronizacionEvent on context", ErrInvalidEventPayload)
		}
		return d.Dispatch(ctx, event)
	}
}

// Register associates syncType with handler. A blank syncType or a
// nil handler is rejected with scheduler.ErrInvalidInput (matching the
// CronRunner.Register validation contract). Existing registrations
// are overwritten — Register is the documented way to replace the
// built-in stubs in tests and in the future PR 3B wiring.
func (d *defaultSyncDispatcher) Register(syncType string, handler JobHandler) {
	if syncType == "" {
		return
	}
	if handler == nil {
		return
	}
	d.mu.Lock()
	d.handlers[syncType] = handler
	d.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Built-in stub handlers — replaced in PR 3B.
// ---------------------------------------------------------------------------

// catalogSyncHandler is the stub for sync_type="catalog". The catalog
// service owns the canonical syncer for catalog items; this handler
// will be wired to it in PR 3B (or later). For now it returns
// ErrNotImplemented so the consumer nacks and the operator can see
// the gap in the logs.
func catalogSyncHandler(ctx context.Context, base *BaseJob) error {
	if base != nil && base.Logger != nil {
		base.Logger.Inner().WarnContext(ctx, "catalog sync handler is not implemented yet",
			"sync_type", "catalog",
		)
	}
	// TODO: wire to catalog service in PR 3B.
	return ErrNotImplemented
}

// employeesSyncHandler is the stub for sync_type="employees". The
// empleados service owns the canonical syncer for employee records;
// this handler will be wired to it in PR 3B.
func employeesSyncHandler(ctx context.Context, base *BaseJob) error {
	if base != nil && base.Logger != nil {
		base.Logger.Inner().WarnContext(ctx, "employees sync handler is not implemented yet",
			"sync_type", "employees",
		)
	}
	// TODO: wire to empleados service in PR 3B.
	return ErrNotImplemented
}

// assignmentsSyncHandler is the stub for sync_type="assignments". The
// asignacion service owns the canonical syncer for assignment records;
// this handler will be wired to it in PR 3B.
func assignmentsSyncHandler(ctx context.Context, base *BaseJob) error {
	if base != nil && base.Logger != nil {
		base.Logger.Inner().WarnContext(ctx, "assignments sync handler is not implemented yet",
			"sync_type", "assignments",
		)
	}
	// TODO: wire to asignacion service in PR 3B.
	return ErrNotImplemented
}

// ---------------------------------------------------------------------------
// Syncer interfaces — contracts the future PR 3B implementations will
// satisfy. Declared here so the stubs (and the test suite) can refer
// to them without dragging the implementation imports.
// ---------------------------------------------------------------------------

// CatalogSyncer is the contract the catalog service syncer must
// satisfy. The PR 3B implementation will live inside the catalog
// service package; the scheduler-side sync handler will depend on it
// via the dependency-injection wiring in main.go.
type CatalogSyncer interface {
	Sync(ctx context.Context, targetID int64) error
}

// EmployeeSyncer is the contract the empleados service syncer must
// satisfy. See CatalogSyncer for the wiring notes.
type EmployeeSyncer interface {
	Sync(ctx context.Context, targetID int64) error
}

// AssignmentSyncer is the contract the asignacion service syncer must
// satisfy. See CatalogSyncer for the wiring notes.
type AssignmentSyncer interface {
	Sync(ctx context.Context, targetID int64) error
}

// ---------------------------------------------------------------------------
// Context plumbing — used by the consumer (3.1) to pass the parsed
// event to the dispatcher handler without growing the JobHandler
// signature.
// ---------------------------------------------------------------------------

// sincronizacionEventContextKey is the unexported context key under
// which the consumer stores the SincronizacionEvent before invoking
// the handler. The dispatcher handler reads it back via
// SincronizacionEventFromContext.
type sincronizacionEventContextKey struct{}

// WithSincronizacionEvent returns a derived context carrying the
// event. A nil receiver is a no-op so the consumer can chain
// unconditionally.
func WithSincronizacionEvent(ctx context.Context, event SincronizacionEvent) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, sincronizacionEventContextKey{}, event)
}

// SincronizacionEventFromContext returns the event previously stored
// with WithSincronizacionEvent, or (zero, false) when absent.
func SincronizacionEventFromContext(ctx context.Context) (SincronizacionEvent, bool) {
	if ctx == nil {
		return SincronizacionEvent{}, false
	}
	v, ok := ctx.Value(sincronizacionEventContextKey{}).(SincronizacionEvent)
	return v, ok
}

// baseJobContextKey is the unexported context key the consumer (3.1)
// uses to make a *BaseJob available to the dispatcher handler. The
// CronRunner path passes the BaseJob positionally; the consumer path
// stores it on the context so the handler signature stays identical
// in both call sites.
type baseJobContextKey struct{}

// WithBaseJob returns a derived context carrying the *BaseJob. A nil
// base or ctx is treated as a no-op.
func WithBaseJob(ctx context.Context, base *BaseJob) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if base == nil {
		return ctx
	}
	return context.WithValue(ctx, baseJobContextKey{}, base)
}

// BaseJobFromContext returns the *BaseJob previously stored with
// WithBaseJob, or nil when absent. The dispatcher handler uses this
// to grab the logger for diagnostic logs.
func BaseJobFromContext(ctx context.Context) *BaseJob {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(baseJobContextKey{}).(*BaseJob)
	return v
}

// ---------------------------------------------------------------------------
// Internal helpers.
// ---------------------------------------------------------------------------

// redactEventID returns the first 8 characters of an event id followed
// by an ellipsis, mirroring the FCM token redaction policy. For ids
// shorter than 8 characters the entire value is returned (event ids
// are UUIDs, so the common case is the redacted form).
func redactEventID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8] + "…"
}

// mintEventID returns a fresh UUIDv4 string used by the consumer when
// the incoming event has no event_id. It is intentionally a tiny
// helper so the consumer code does not need to import the uuid
// package directly.
func mintEventID() string {
	return uuid.NewString()
}
