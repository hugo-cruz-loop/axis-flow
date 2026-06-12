// Package jobs holds the concrete business scheduled jobs the
// Scheduler service runs on a cron cadence. The package is the
// first slice of Phase 3 of the Scheduler Service implementation;
// subsequent slices (PR 3B-ii: inactiva_empleado, PR 3C: cleanup +
// close) will add more files alongside this one.
//
// Design notes
// ============
//  1. Every concrete job embeds *service.BaseJob so the cron runner
//     can wire the lock, execution-row, logger, and lock manager
//     through a single constructor. The job's own Run method is
//     focused exclusively on the per-shift business logic; lock
//     acquisition, execution-row lifecycle, and metrics are owned
//     by the runner + service layer (see service/cron_runner.go and
//     service/job.go).
//  2. Database access is performed with prepared statements
//     ($1, $2, ...) against a *pgxpool.Pool. No fmt.Sprintf SQL —
//     the spec's "Seguridad > SQL Injection Prevention (Hardening)"
//     section is non-negotiable.
//  3. The EventPublisher is the integration boundary with the
//     Notificaciones service. This file does NOT import the
//     Notificaciones service package directly. The noop default
//     is replaced by the real publisher in main.go (Phase 4) via
//     SetEventPublisher.
//  4. Per-shift logs are emitted at debug level only — production
//     runs this job every 5 minutes, info-level per-shift lines
//     would flood the log pipeline. The end-of-run summary line
//     is the only info-level record the job emits.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/events"
	"axis-flow-back/internal/scheduler/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Constants — job identity, schedule, defaults.
// ---------------------------------------------------------------------------

// NotificacionesJobKey is the canonical job_key the seed file
// (Phase 6, task 6.1) inserts and that the wiring layer in main.go
// looks up via JobRepository.GetByKey. The value matches the
// spec's GET /api/v1/scheduler/jobs response.
const NotificacionesJobKey = "notificaciones_en_tiempo_real"

// notificacionesCronExpr is the schedule from the spec: every 5
// minutes. Stored as a *string so it round-trips through the
// scheduler.Job domain type without copy.
var notificacionesCronExpr = "*/5 * * * *"

// notificacionesDescription is the human-readable description from
// the spec's GET /jobs response.
var notificacionesDescription = "Alertas Push 15 min antes de entrada/comida/salida."

// Default values. Exposed as package-level vars so tests can tweak
// them in-place before constructing the job.
var (
	// DefaultShiftLookahead is the 15-minute window the spec calls
	// "ventana estricta de 15 minutos".
	DefaultShiftLookahead = 15 * time.Minute
	// DefaultPollGrace is the dedup window — once we have
	// notified a (empleado, shift) pair we do not notify again
	// inside the next 30 seconds. The cron tick is 5 minutes so
	// the pair can be hit twice only across a manual trigger
	// during the same window (e.g. operator hits the trigger
	// button) or across overlapping shifts that share the same
	// shift_id (which is filtered out by the WHERE clause).
	DefaultPollGrace = 30 * time.Second
	// DedupSweepInterval is how often the in-memory dedup cache
	// is swept for expired entries. Conservative: every minute
	// is enough to keep the map bounded at the observed 5-50
	// entries per tick.
	DedupSweepInterval = 1 * time.Minute
)

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	// ErrNoUpcomingShifts is a non-fatal sentinel: it signals
	// that the 15-minute window contained no shifts. The job
	// converts it to a successful no-op so the cron runner does
	// not record a FAILED execution when the only "problem"
	// is that nobody is starting a shift in the next 15
	// minutes.
	ErrNoUpcomingShifts = errors.New("scheduler: no upcoming shifts in window")
	// ErrDeviceDeactivated is the internal sentinel the FCM
	// path returns when a device has been marked is_active =
	// false after an ErrFCMPermanentFailure from FCM. It is not
	// returned to the cron runner; it is used as a branching
	// hint inside the per-device loop.
	ErrDeviceDeactivated = errors.New("scheduler: device deactivated after FCM permanent failure")
)

// ---------------------------------------------------------------------------
// DB abstraction — replaced by ShiftRepository (PR 5B-ii). The
// `jobDB` interface and the `pgxPoolAdapter` lived here from
// PR 3B-i; they are removed because the job now talks to
// PostgreSQL exclusively through the typed ShiftRepository
// contract. The pgx implementation is in
// `shift_repository_pg.go` and the wiring in
// `cmd/server/scheduler_wiring.go` constructs the repo and
// passes it to the constructor.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Configuration knobs.
// ---------------------------------------------------------------------------

// NotificacionesJobDeps bundles the dependencies the
// NotificacionesEnTiempoRealJob needs. A struct (instead of
// positional constructor args) keeps the call site readable as the
// dependency list grows. The CronRunner already provides a
// *BaseJob at runtime, but the job retains direct handles to the
// ones its Run method uses (shifts, fcm, eventPublisher) so the type
// remains self-contained and testable in isolation.
type NotificacionesJobDeps struct {
	// Shifts is the repository the job uses to read the
	// upcoming-shifts window and to deactivate devices that
	// return ErrFCMPermanentFailure. Required.
	Shifts ShiftRepository
	// FCM is the FCM client used to dispatch push notifications.
	// Required.
	FCM service.FCMClient
	// EventPublisher emits PreShiftAlertaProgramada events to
	// the Notificaciones service. Optional; a noop default is
	// substituted when nil. The wiring layer in main.go (Phase 4)
	// replaces the noop via SetEventPublisher.
	EventPublisher events.EventPublisher
	// Logger is the structured logger used for the end-of-run
	// summary line and debug per-shift traces. Required.
	Logger *service.Logger
	// ShiftLookahead is the size of the upcoming-shifts window.
	// Zero falls back to DefaultShiftLookahead (15m).
	ShiftLookahead time.Duration
	// PollGrace is the dedup window. Zero falls back to
	// DefaultPollGrace (30s).
	PollGrace time.Duration
}

// ---------------------------------------------------------------------------
// NotificacionesEnTiempoRealJob — the first concrete cron job.
// ---------------------------------------------------------------------------

// NotificacionesEnTiempoRealJob is the implementation of the
// `notificaciones_en_tiempo_real` job described in
// spec.md "Responsabilidades > Real-Time Push Alerts" and Gherkin
// scenario 1 ("Envío exitoso de notificación push previa al
// turno (Happy Path)"). The job:
//
//  1. Queries shifts starting in the next 15 minutes
//     (configurable via ShiftLookahead).
//  2. For each (empleado, shift, device) tuple:
//     - Skips tuples recently notified (pollGrace dedup).
//     - Builds a NotificationPayload and calls FCM.
//     - On ErrFCMPermanentFailure, deactivates the device row.
//     - Emits a PreShiftAlertaProgramada event through the
//     EventPublisher.
//  3. Emits one info-level summary line with the inspection and
//     dispatch counters.
//
// The job is concurrency-safe: Run is invoked sequentially by the
// cron runner, but the inner per-device loop reads from a
// mutex-guarded dedup cache that survives across runs.
type NotificacionesEnTiempoRealJob struct {
	// deps holds the static dependencies. The struct is
	// intentionally small — the runner constructs the
	// *service.BaseJob, the job reads from this struct to do
	// its work.
	deps NotificacionesJobDeps

	// shifts is the data-access surface. Built from deps.Shifts
	// at construction time.
	shifts ShiftRepository

	// publisher is the event publisher. Defaults to a noop; can
	// be replaced at any time via SetEventPublisher. Guarded by
	// publisherMu so the wiring layer can swap it without
	// blocking Run.
	publisher   events.EventPublisher
	publisherMu sync.RWMutex

	// dedup is the in-memory (empleado_id, shift_id) -> last
	// notified timestamp cache. Guarded by dedupMu. Swept
	// opportunistically on every Run invocation.
	dedup   map[dedupKey]time.Time
	dedupMu sync.Mutex
	// lastSweep tracks the last time dedup was swept so we do
	// not sweep on every Run (which would burn CPU during a
	// burst of manual triggers).
	lastSweep time.Time

	// lookahead and pollGrace are the resolved (non-zero) knobs
	// used at runtime.
	lookahead time.Duration
	pollGrace time.Duration
}

// dedupKey is the (empleado, shift) tuple used by the dedup
// cache. We use a single int64 (the shift's PK is an UUID but
// we treat it as a string for human-readable logs) and a
// separate string for the shift id to avoid packing two string
// keys together.
type dedupKey struct {
	EmpleadoID int64
	ShiftID    string
}

// ---------------------------------------------------------------------------
// Constructor.
// ---------------------------------------------------------------------------

// NewNotificacionesEnTiempoRealJob constructs the job and
// resolves the configurable knobs to their non-zero defaults. A
// nil deps field returns an error so the wiring layer fails fast
// at startup rather than at the first cron tick.
func NewNotificacionesEnTiempoRealJob(deps NotificacionesJobDeps) (*NotificacionesEnTiempoRealJob, error) {
	if deps.Shifts == nil {
		return nil, fmt.Errorf("jobs.NewNotificacionesEnTiempoRealJob: %w: deps.Shifts is required", scheduler.ErrInvalidInput)
	}
	if deps.FCM == nil {
		return nil, fmt.Errorf("jobs.NewNotificacionesEnTiempoRealJob: %w: deps.FCM is required", scheduler.ErrInvalidInput)
	}
	if deps.Logger == nil {
		return nil, fmt.Errorf("jobs.NewNotificacionesEnTiempoRealJob: %w: deps.Logger is required", scheduler.ErrInvalidInput)
	}

	lookahead := deps.ShiftLookahead
	if lookahead <= 0 {
		lookahead = DefaultShiftLookahead
	}
	pollGrace := deps.PollGrace
	if pollGrace <= 0 {
		pollGrace = DefaultPollGrace
	}

	pub := deps.EventPublisher
	if pub == nil {
		pub = events.NewNoopEventPublisher(deps.Logger.Inner())
	}

	return &NotificacionesEnTiempoRealJob{
		deps:      deps,
		shifts:    deps.Shifts,
		publisher: pub,
		dedup:     make(map[dedupKey]time.Time),
		lookahead: lookahead,
		pollGrace: pollGrace,
	}, nil
}

// SetEventPublisher swaps the publisher used by subsequent Run
// invocations. The wiring layer in main.go (Phase 4) calls this
// once at startup with the real Notificaciones-service-backed
// implementation; tests can use it to inject a spy. The swap is
// safe to perform concurrently with an in-flight Run — the
// in-flight run keeps using the publisher it loaded at the top of
// the method.
func (j *NotificacionesEnTiempoRealJob) SetEventPublisher(pub events.EventPublisher) {
	if j == nil {
		return
	}
	if pub == nil {
		pub = events.NewNoopEventPublisher(j.deps.Logger.Inner())
	}
	j.publisherMu.Lock()
	j.publisher = pub
	j.publisherMu.Unlock()
}

// currentPublisher returns the publisher snapshot for use within a
// single Run invocation. Reads the field under the read-lock once.
func (j *NotificacionesEnTiempoRealJob) currentPublisher() events.EventPublisher {
	j.publisherMu.RLock()
	defer j.publisherMu.RUnlock()
	return j.publisher
}

// ---------------------------------------------------------------------------
// Job contract — Metadata + Run.
// ---------------------------------------------------------------------------

// Metadata returns the scheduler.Job descriptor used by the cron
// runner to bind the job to a cron expression and to look up the
// job_id for the execution-row FK. The descriptor is a value (not
// a pointer) so the runner can store it without copying.
//
// The job_id is left as uuid.Nil because the cron runner loads the
// canonical row from the database via JobRepository.GetByKey
// before scheduling; the metadata returned here is the static
// "what kind of job is this" record, the row in
// scheduler.scheduler_jobs is the "this instance in this DB"
// record.
func (j *NotificacionesEnTiempoRealJob) Metadata() scheduler.Job {
	cronExpr := notificacionesCronExpr
	desc := notificacionesDescription
	return scheduler.Job{
		JobKey:         NotificacionesJobKey,
		CronExpression: &cronExpr,
		JobClass:       "jobs.NotificacionesEnTiempoRealJob",
		Module:         "notificaciones",
		Description:    &desc,
		IsActive:       true,
	}
}

// Run is the per-tick entry point. The cron runner wraps the call
// in WrapWithLifecycle and the lock-gated flow documented in
// service/cron_runner.go, so this method assumes it is the only
// goroutine executing it for the current process (lock is held by
// the runner). The inner loop is sequential by design — see the
// Concurrency note in the file header.
func (j *NotificacionesEnTiempoRealJob) Run(ctx context.Context) error {
	start := time.Now()
	now := time.Now().UTC()
	windowEnd := now.Add(j.lookahead)

	// Opportunistic dedup sweep. Cheap, mutex-guarded, and
	// avoids the map growing unboundedly across long uptimes.
	j.maybeSweepDedup(now)

	shifts, err := j.shifts.UpcomingShiftsInWindow(ctx, now, windowEnd)
	if err != nil {
		return fmt.Errorf("notificaciones.Run: query upcoming shifts: %w", err)
	}

	// Capture the publisher once at the top of the run so a
	// concurrent SetEventPublisher call from the wiring layer
	// does not race with the in-flight loop.
	pub := j.currentPublisher()

	var (
		shiftsInspected int
		notifSent       int
		devicesDeact    int
	)
	for _, shift := range shifts {
		shiftsInspected++

		// Dedup check. Skip if we already notified this
		// (empleado, shift) tuple within the poll grace window.
		shiftIDStr := shift.ShiftID.String()
		if j.isRecentlyNotified(shift.EmpleadoID, shiftIDStr, now) {
			j.deps.Logger.Inner().DebugContext(ctx, "notificaciones: skipping recently-notified pair",
				slog.Int64("empleado_id", shift.EmpleadoID),
				slog.String("shift_id", shiftIDStr),
				slog.String("tipo_evento", shift.TipoEvento),
			)
			continue
		}

		// Fetch the active device list for the empleado. A
		// query error is logged and the shift is skipped —
		// the next tick will retry.
		devices, derr := j.shifts.ActiveDevicesForEmpleado(ctx, shift.EmpleadoID)
		if derr != nil {
			j.deps.Logger.Inner().WarnContext(ctx, "notificaciones: device query failed; skipping shift",
				slog.Int64("empleado_id", shift.EmpleadoID),
				slog.String("shift_id", shiftIDStr),
				slog.String("error", derr.Error()),
			)
			continue
		}
		if len(devices) == 0 {
			// No active devices for this empleado. Log at
			// debug level only — the per-shift summary
			// line at the end of the run is the info-level
			// record.
			j.deps.Logger.Inner().DebugContext(ctx, "notificaciones: no active devices for shift",
				slog.Int64("empleado_id", shift.EmpleadoID),
				slog.String("shift_id", shiftIDStr),
			)
			continue
		}

		for _, device := range devices {
			// Build the payload. The data map is delivered
			// verbatim to the mobile app for routing (it
			// carries the identifier triple the app uses
			// to deep-link into the pre-shift screen).
			payload := buildPayload(shift, device)

			// FCM dispatch. We treat ErrFCMPermanentFailure
			// as a normal lifecycle event: the device is
			// gone, we mark it inactive and move on. Any
			// other error is logged at warn level and we
			// do NOT mark the dedup key — a transient
			// failure on this tick may succeed on the next
			// 5-minute tick.
			_, sendErr := j.deps.FCM.Send(ctx, device.FCMToken, payload)
			switch {
			case sendErr == nil:
				notifSent++
			case errors.Is(sendErr, service.ErrFCMPermanentFailure):
				if derr := j.deactivateDevice(ctx, device.ID); derr != nil {
					j.deps.Logger.Inner().WarnContext(ctx, "notificaciones: failed to deactivate device after permanent FCM failure",
						slog.String("device_id_prefix", redactDeviceIDPrefix(device.ID.String())),
						slog.String("fcm_error", sendErr.Error()),
						slog.String("deactivate_error", derr.Error()),
					)
				} else {
					devicesDeact++
					j.deps.Logger.Inner().InfoContext(ctx, "notificaciones: device deactivated after permanent FCM failure",
						slog.String("device_id_prefix", redactDeviceIDPrefix(device.ID.String())),
						slog.Int64("empleado_id", shift.EmpleadoID),
					)
				}
			default:
				// Transient failure: do NOT mark the
				// dedup key. The next 5-minute tick will
				// retry.
				j.deps.Logger.Inner().WarnContext(ctx, "notificaciones: FCM send failed (transient), will retry next tick",
					slog.Int64("empleado_id", shift.EmpleadoID),
					slog.String("shift_id", shiftIDStr),
					slog.String("fcm_token_prefix", redactToken(device.FCMToken)),
					slog.String("error", sendErr.Error()),
				)
				continue
			}

			// Mark the (empleado, shift) pair as notified
			// BEFORE the publisher call. Even if the
			// publisher fails, the user-facing push has
			// already been delivered, and retrying the
			// publisher on the next tick would produce a
			// duplicate alert in the Notificaciones
			// service.
			j.markNotified(shift.EmpleadoID, shiftIDStr, now)

			// Best-effort event publication. The
			// Notificaciones service is a downstream
			// correlation log; a failed publish is logged
			// but does not fail the run.
			evt := events.PreShiftAlertEvent{
				AlertID:    uuid.New(),
				EmpleadoID: shift.EmpleadoID,
				FCMToken:   redactToken(device.FCMToken),
				TipoEvento: shift.TipoEvento,
				Timestamp:  now,
			}
			if perr := pub.PublishPreShiftAlert(ctx, evt); perr != nil {
				j.deps.Logger.Inner().WarnContext(ctx, "notificaciones: pre-shift event publish failed",
					slog.String("alerta_id_prefix", redactUUIDPrefix(evt.AlertID)),
					slog.Int64("empleado_id", shift.EmpleadoID),
					slog.String("error", perr.Error()),
				)
			}
		}
	}

	// End-of-run summary line. The single info-level record
	// produced per tick. No PII; the per-shift lines above run
	// at debug level and are off in production.
	j.deps.Logger.Inner().InfoContext(ctx, "pre-shift notification run completed",
		slog.Int("shifts_inspected", shiftsInspected),
		slog.Int("notifications_sent", notifSent),
		slog.Int("devices_deactivated", devicesDeact),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	if shiftsInspected == 0 {
		// Non-fatal: the window is empty. Return the sentinel
		// so the runner can opt to log it differently (the
		// runner currently treats every error as a FAILED
		// execution, so we return nil and rely on the summary
		// log for observability). The sentinel is kept exported
		// for tests and for future Phase-5 instrumentation
		// that may want to count empty-window runs.
		_ = ErrNoUpcomingShifts
	}
	return nil
}

// ---------------------------------------------------------------------------
// Payload + helpers — moved out of the SQL block (the SQL now
// lives in shift_repository_pg.go).
// ---------------------------------------------------------------------------

// buildPayload turns an (UpcomingShift, Device) pair into the
// FCM payload the mobile app consumes. Title and Body match
// the Gherkin scenario 1 expected message ("Recuerda que tu
// inicio de turno es pronto"). The Data map is consumed
// verbatim by the mobile app to deep-link into the pre-shift
// screen; shift_id / tipo_evento / empleado_id are the
// identifier triple it routes on.
func buildPayload(shift UpcomingShift, device Device) service.NotificationPayload {
	return service.NotificationPayload{
		Title: "Recordatorio de turno",
		Body:  "Recuerda que tu inicio de turno es pronto",
		Data: map[string]string{
			"shift_id":    shift.ShiftID.String(),
			"tipo_evento": shift.TipoEvento,
			"empleado_id": fmt.Sprintf("%d", shift.EmpleadoID),
		},
	}
}

// ---------------------------------------------------------------------------
// Device deactivation — on permanent FCM failure (unregistered
// token, sender ID mismatch, etc.) we mark the device row inactive
// so the next tick's ActiveDevicesForEmpleado filter drops it.
// ---------------------------------------------------------------------------

// deactivateDevice flips is_active to FALSE for the given device
// id. The repository's WHERE clause preserves the previous value
// so the UPDATE is a no-op (and the row is left untouched) when
// the device has already been deactivated. The error returned is
// wrapped with %w so callers can errors.Is on the underlying pgx
// error class.
func (j *NotificacionesEnTiempoRealJob) deactivateDevice(ctx context.Context, deviceID uuid.UUID) error {
	if err := j.shifts.DeactivateDevice(ctx, deviceID, time.Now().UTC()); err != nil {
		return fmt.Errorf("notificaciones.deactivateDevice: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Dedup cache — in-memory (empleado, shift) -> last notified time.
// ---------------------------------------------------------------------------

// isRecentlyNotified reports whether (empleado, shift) was
// notified within the poll grace window. The map is guarded by
// dedupMu; the check is O(1).
func (j *NotificacionesEnTiempoRealJob) isRecentlyNotified(empleadoID int64, shiftID string, now time.Time) bool {
	j.dedupMu.Lock()
	defer j.dedupMu.Unlock()
	last, ok := j.dedup[dedupKey{EmpleadoID: empleadoID, ShiftID: shiftID}]
	if !ok {
		return false
	}
	return now.Sub(last) < j.pollGrace
}

// markNotified records the (empleado, shift) pair as just
// notified. Subsequent calls within pollGrace are no-ops from
// the isRecentlyNotified point of view (which is the desired
// behaviour for retries on transient FCM failures that
// eventually succeed).
func (j *NotificacionesEnTiempoRealJob) markNotified(empleadoID int64, shiftID string, now time.Time) {
	j.dedupMu.Lock()
	defer j.dedupMu.Unlock()
	j.dedup[dedupKey{EmpleadoID: empleadoID, ShiftID: shiftID}] = now
}

// maybeSweepDedup removes entries older than pollGrace from the
// dedup map. Cheap when the map is small (a few dozen entries)
// and bounded by the lock duration, so it is safe to call from
// Run without a separate goroutine. We sweep at most once per
// DedupSweepInterval to amortise the cost across ticks.
func (j *NotificacionesEnTiempoRealJob) maybeSweepDedup(now time.Time) {
	j.dedupMu.Lock()
	defer j.dedupMu.Unlock()
	if !j.lastSweep.IsZero() && now.Sub(j.lastSweep) < DedupSweepInterval {
		return
	}
	for k, t := range j.dedup {
		if now.Sub(t) >= j.pollGrace {
			delete(j.dedup, k)
		}
	}
	j.lastSweep = now
}

// ---------------------------------------------------------------------------
// Internal helpers — redaction policy.
// ---------------------------------------------------------------------------

// redactToken returns the first 8 characters of a token followed
// by an ellipsis, mirroring the FCM client's redaction policy.
// For tokens shorter than 8 characters the entire value is
// returned (FCM tokens are normally > 100 chars).
func redactToken(t string) string {
	if len(t) <= 8 {
		return t
	}
	return t[:8] + "…"
}

// redactDeviceIDPrefix returns the first 8 characters of a device
// UUID followed by an ellipsis. The full UUID is safe to log in
// principle (it is not a secret) but the redaction keeps log
// lines short and consistent with the FCM token redaction
// policy.
func redactDeviceIDPrefix(id string) string {
	return redactToken(id)
}

// redactUUIDPrefix returns the first 8 characters of a UUID
// followed by an ellipsis.
func redactUUIDPrefix(id uuid.UUID) string {
	return redactToken(id.String())
}
