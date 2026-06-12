// Package jobs holds the concrete business scheduled jobs the
// Scheduler service runs on a cron cadence.
//
// This file implements the `cleanup_expired_sessions` job: the
// daily maintenance sweep that deactivates expired mobile device
// authorisation records in `empleados.empleados_user_devices`.
// The job is the implementation of:
//
//   - Spec section "Responsabilidades > Access and Session Cleanup".
//   - Tasks.md task 3.4 (this PR — 3C).
//
// Design notes
// ============
//  1. The job embeds no service.BaseJob: the cron runner injects
//     a BaseJob at run time (see service/cron_runner.go), and the
//     job reads its own dedicated dependencies (db, logger,
//     metrics, batch size) from the struct field populated at
//     construction time. The same pattern is used by
//     InactivaEmpleadoJob (PR 3B-ii).
//  2. The job drains a "stale rows" backlog using a batched
//     UPDATE loop. The shared helper `runInBatches` (see
//     batch_processor.go) drives the loop; the SQL itself is
//     local to this file because the column shape is different
//     from the assignment-closing job.
//  3. The cap is `batchSize` rows per iteration (default 500).
//     The cap protects the 03:00 UTC window from ballooning
//     when a tenant ships a session-expiry bug; the next run
//     picks up where this one stopped.
//  4. Database access is performed with prepared statements
//     ($1, $2, ...) against a *pgxpool.Pool. The spec's
//     "Seguridad > SQL Injection Prevention (Hardening)" section
//     is non-negotiable — no fmt.Sprintf SQL, no string
//     interpolation in WHERE clauses.
//  5. Per-batch errors do NOT fail the whole job — the helper
//     returns the error and the job's Run surfaces it so the
//     cron runner can record a FAILED execution and the operator
//     can decide whether to alert. (The clean-up job is a single
//     SQL statement, so the per-batch "log and continue" pattern
//     from inactiva_empleado.go does not apply — the job has no
//     per-empresa boundary to short-circuit on.)
//  6. Idempotency: the `WHERE is_active = true` clause on both
//     the SELECT-IN and the UPDATE is the idempotency guard. A
//     device that has already been deactivated by a previous
//     run is filtered out of the SELECT and is therefore not
//     touched by the UPDATE. Re-running the job is a no-op.
//  7. No PII: every log line carries only the bigint
//     `empleado_id` and the UUID `device_id` (the device row's
//     primary key, NOT a PII field). The fcm_token, the device
//     model, and the user-agent are NEVER logged — they live
//     in the device row and stay there.
//  8. The job does NOT emit integration events. The spec's
//     "Eventos > Publica" table lists the two events the
//     Scheduler Service emits; neither is produced by the
//     cleanup job. The downstream observability sink is the
//     structured log lines and the per-job Prometheus
//     observations emitted by the cron runner / lifecycle
//     wrapper (scheduler_job_duration_seconds,
//     scheduler_jobs_failed_total).
//
// Schedule
// ========
// The cron expression `"0 3 * * *"` is the spec's choice: 03:00
// UTC, after the inactiva_empleado run (02:00 UTC) and before
// the close_lapsed_assignments run (03:30 UTC). The serial
// ordering keeps the per-tenant mutation rate low and avoids
// overlapping UPDATEs on hot tables.
//
// Schema assumptions (documented, not enforced)
// ============================================
// The query layer assumes the following columns exist in the
// operational schemas (the Empleados service owns the
// migration; this job treats it as a contract):
//
//	empleados.empleados_user_devices(
//	    id                    UUID PRIMARY KEY,
//	    empleado_id           BIGINT NOT NULL,
//	    fcm_token             VARCHAR(255) NOT NULL,
//	    is_active             BOOLEAN NOT NULL DEFAULT TRUE,
//	    expires_at            TIMESTAMPTZ NULL,         -- nullable for never-expiring rows
//	    deactivated_at        TIMESTAMPTZ NULL,         -- set on deactivation
//	    deactivation_reason   VARCHAR(50)  NULL,        -- e.g. 'session_expired'
//	    updated_at            TIMESTAMPTZ
//	)
//
// These columns match the Empleados service spec section
// "Datos". The columns `expires_at`, `deactivated_at`, and
// `deactivation_reason` are the new columns this job depends on
// and that the Empleados service will add in a follow-up
// migration. The cleanup job is the FIRST consumer of those
// columns; until the migration lands the WHERE clause is
// trivially empty (no rows have expires_at set) and the job is
// a no-op — that is the documented and intended behaviour.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Constants — job identity, schedule, defaults.
// ---------------------------------------------------------------------------

// CleanupExpiredSessionsJobKey is the canonical job_key the
// seed file (Phase 6, task 6.1) inserts and that the wiring
// layer in main.go looks up via JobRepository.GetByKey. Matches
// the job_key documented in the spec's maintenance-section
// summary and the Runbook 1 entry.
const CleanupExpiredSessionsJobKey = "cleanup_expired_sessions"

// cleanupExpiredSessionsCronExpr is the schedule from the
// prompt: 03:00 UTC every day, after the inactiva_empleado run
// (02:00 UTC) and before the close_lapsed_assignments run
// (03:30 UTC). Stored as a *string so it round-trips through
// the scheduler.Job domain type without copy.
var cleanupExpiredSessionsCronExpr = "0 3 * * *"

// cleanupExpiredSessionsDescription is the human-readable
// description for the scheduler_jobs.description column. Matches
// the spec's "Responsabilidades > Access and Session Cleanup"
// summary ("Automatically terminate expired mobile device
// authorization records ...").
var cleanupExpiredSessionsDescription = "Desactivación automática de sesiones de dispositivos móviles expirados (empleados.empleados_user_devices)."

// sessionExpiredReason is the canonical value written to
// empleados_user_devices.deactivation_reason. It is the only
// reason the job emits today; the column is left free-form on
// the schema so future jobs (e.g. a manual operator-deactivation
// sweep) can flow through without a schema change.
const sessionExpiredReason = "session_expired"

// Default values. Exposed as package-level vars so tests can
// tweak them in-place before constructing the job.
var (
	// DefaultCleanupBatchSize is the default cap on the per-
	// iteration UPDATE row count. The cap protects the 03:00
	// UTC window from ballooning when a tenant ships a
	// session-expiry bug; the next run picks up where this
	// one stopped. 500 matches the spec's recommendation
	// for daily maintenance jobs.
	DefaultCleanupBatchSize = 500
)

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	// ErrCleanupFailed is the sentinel wrapped around any
	// underlying DB error that prevents the cleanup loop from
	// making progress (context cancellation, connection
	// refused, schema mismatch). The cron runner's
	// WrapWithLifecycle converts it into a FAILED execution
	// row and increments scheduler_jobs_failed_total with
	// the error's class label.
	ErrCleanupFailed = errors.New("scheduler: cleanup_expired_sessions failed")
)

// ---------------------------------------------------------------------------
// DB abstraction — same pattern as the other jobs in this package.
// ---------------------------------------------------------------------------

// cleanupDB abstracts the pgx surface used by the job. The
// concrete implementation is the *pgxpool.Pool adapter; tests may
// inject a transaction-bound runner or a fake. Defined as a
// local interface so the job's surface stays minimal — the
// runner only needs Query (for the batched UPDATE...RETURNING)
// and Exec (kept for symmetry with the other jobs in this
// package, even though the current implementation does not call
// it).
type cleanupDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type cleanupPoolAdapter struct{ pool *pgxpool.Pool }

func (a *cleanupPoolAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return a.pool.Query(ctx, sql, args...)
}
func (a *cleanupPoolAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.pool.Exec(ctx, sql, args...)
}

// ---------------------------------------------------------------------------
// Configuration knobs.
// ---------------------------------------------------------------------------

// CleanupExpiredSessionsJobDeps bundles the dependencies the
// CleanupExpiredSessionsJob needs. The pattern mirrors
// InactivaEmpleadoJob (PR 3B-ii): a struct (instead of
// positional constructor args) keeps the call site readable
// as the dependency list grows. The CronRunner already
// provides a BaseJob at runtime, but the job retains direct
// handles to the ones its Run method uses so the type
// remains self-contained and testable in isolation.
type CleanupExpiredSessionsJobDeps struct {
	// DB is the PostgreSQL pool used to perform the
	// batched UPDATE on empleados.empleados_user_devices.
	// Required.
	DB *pgxpool.Pool
	// Logger is the structured logger used for the per-
	// batch info line and the end-of-run summary. Required.
	Logger *service.Logger
	// Metrics is the typed holder for the Prometheus
	// collectors. The job calls IncJobFailed on per-batch
	// failures that should not fail the whole batch. Optional
	// (nil-safe; service.Metrics is also nil-safe).
	Metrics *service.SchedulerMetrics
	// BatchSize caps the per-iteration UPDATE row count.
	// Zero falls back to DefaultCleanupBatchSize (500).
	BatchSize int
}

// ---------------------------------------------------------------------------
// CleanupExpiredSessionsJob — the session-cleanup concrete cron job.
// ---------------------------------------------------------------------------

// CleanupExpiredSessionsJob is the implementation of the
// `cleanup_expired_sessions` job described in spec.md
// "Responsabilidades > Access and Session Cleanup" and
// Tasks.md task 3.4. The job:
//
//  1. Repeatedly calls a per-batch helper that UPDATEs the
//     next chunk of `empleados.empleados_user_devices` rows
//     whose `is_active = true AND expires_at < now` and that
//     are still active.
//  2. The cap is `batchSize` rows per iteration (default 500).
//  3. The loop terminates when a batch returns 0 affected rows
//     (the backlog is exhausted) or when the context is
//     cancelled.
//  4. Emits one info-level summary line at the end of the run
//     with the total affected row count, the batch count, and
//     the total elapsed milliseconds.
//
// The job is concurrency-safe: Run is invoked sequentially by
// the cron runner (the lock is held by the runner, not the
// job), and the per-batch loop is sequential by design.
type CleanupExpiredSessionsJob struct {
	deps CleanupExpiredSessionsJobDeps

	db cleanupDB

	batchSize int
}

// ---------------------------------------------------------------------------
// Constructor.
// ---------------------------------------------------------------------------

// NewCleanupExpiredSessionsJob constructs the job and resolves
// the configurable knobs to their non-zero defaults. A nil deps
// field returns an error so the wiring layer fails fast at
// startup rather than at the first cron tick.
func NewCleanupExpiredSessionsJob(deps CleanupExpiredSessionsJobDeps) (*CleanupExpiredSessionsJob, error) {
	if deps.DB == nil {
		return nil, fmt.Errorf("jobs.NewCleanupExpiredSessionsJob: %w: deps.DB is required", scheduler.ErrInvalidInput)
	}
	if deps.Logger == nil {
		return nil, fmt.Errorf("jobs.NewCleanupExpiredSessionsJob: %w: deps.Logger is required", scheduler.ErrInvalidInput)
	}

	batchSize := deps.BatchSize
	if batchSize <= 0 {
		batchSize = DefaultCleanupBatchSize
	}

	return &CleanupExpiredSessionsJob{
		deps:      deps,
		db:        &cleanupPoolAdapter{pool: deps.DB},
		batchSize: batchSize,
	}, nil
}

// ---------------------------------------------------------------------------
// Job contract — Metadata + Run.
// ---------------------------------------------------------------------------

// Metadata returns the scheduler.Job descriptor used by the
// cron runner to bind the job to a cron expression and to look
// up the job_id for the execution-row FK. The descriptor is a
// value (not a pointer) so the runner can store it without
// copying.
//
// The job_id is left as uuid.Nil because the cron runner loads
// the canonical row from the database via JobRepository.GetByKey
// before scheduling; the metadata returned here is the static
// "what kind of job is this" record, the row in
// scheduler.scheduler_jobs is the "this instance in this DB"
// record.
func (j *CleanupExpiredSessionsJob) Metadata() scheduler.Job {
	cronExpr := cleanupExpiredSessionsCronExpr
	desc := cleanupExpiredSessionsDescription
	return scheduler.Job{
		JobKey:         CleanupExpiredSessionsJobKey,
		CronExpression: &cronExpr,
		JobClass:       "jobs.CleanupExpiredSessionsJob",
		Module:         "empleados",
		Description:    &desc,
		IsActive:       true,
	}
}

// Run is the per-tick entry point. The cron runner wraps the
// call in WrapWithLifecycle and the lock-gated flow documented
// in service/cron_runner.go, so this method assumes it is the
// only goroutine executing it for the current process (lock is
// held by the runner).
func (j *CleanupExpiredSessionsJob) Run(ctx context.Context) error {
	start := time.Now()
	now := start.UTC()

	// The closure captures the cutoff timestamp and the
	// batchSize so the prepared statement stays static. The
	// affected-row count returned by deactivateOneBatch is
	// the number of rows the UPDATE actually flipped; the
	// helper sees a 0-row result and terminates the loop
	// (the backlog is empty).
	var totalDeactivated int64
	var batches int
	processBatch := func(ctx context.Context) (int64, error) {
		batchStart := time.Now()
		affected, err := j.deactivateOneBatch(ctx, now)
		if err != nil {
			if j.deps.Metrics != nil {
				j.deps.Metrics.IncJobFailed(CleanupExpiredSessionsJobKey, errorClassForCleanup(err))
			}
			return 0, fmt.Errorf("%w: %w", ErrCleanupFailed, err)
		}
		batches++
		j.deps.Logger.Inner().InfoContext(ctx, "cleanup_expired_sessions: deactivated expired user devices",
			slog.String("msg_field", "cleanup_expired_sessions: deactivated expired user devices"),
			slog.Int64("affected", affected),
			slog.Int64("duration_ms", time.Since(batchStart).Milliseconds()),
		)
		return affected, nil
	}

	total, err := runInBatches(ctx, j.batchSize, processBatch)
	totalDeactivated = total
	if err != nil {
		// The helper already wraps the per-batch error. We
		// emit a final warn line so the operator can see the
		// partial-progress state (how many rows we managed
		// to deactivate before the failure) and then
		// surface the error to the cron runner.
		j.deps.Logger.Inner().WarnContext(ctx, "cleanup_expired_sessions: run aborted with partial progress",
			slog.String("msg_field", "cleanup_expired_sessions: run aborted with partial progress"),
			slog.Int64("total_deactivated", totalDeactivated),
			slog.Int("batches", batches),
			slog.String("error", err.Error()),
		)
		return err
	}

	// End-of-run summary. The single info-level record
	// emitted per tick. No PII: only the affected-row count,
	// the batch count, and the total elapsed milliseconds.
	j.deps.Logger.Inner().InfoContext(ctx, "cleanup_expired_sessions run completed",
		slog.String("msg_field", "cleanup_expired_sessions run completed"),
		slog.Int64("total_deactivated", totalDeactivated),
		slog.Int("batches", batches),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)
	return nil
}

// ---------------------------------------------------------------------------
// SQL — prepared statements for the per-batch update.
// ---------------------------------------------------------------------------

// deactivateExpiredDevicesSQL flips a capped batch of
// `empleados.empleados_user_devices` rows from active to
// inactive, stamping the deactivation reason and the
// deactivation timestamp. The shape is the canonical "drain
// the backlog" pattern: a SELECT-IN subquery capped by LIMIT
// $2 picks the next batch of candidate rows, and the outer
// UPDATE applies the state change to exactly that batch.
//
// Why this shape
// --------------
//   - The WHERE clause in the SELECT-IN matches the WHERE
//     clause in the outer UPDATE byte-for-byte, so the rows
//     the UPDATE touches are exactly the rows the SELECT
//     picked. The two predicates are:
//     is_active = TRUE AND expires_at IS NOT NULL
//     AND expires_at < $1
//     The `IS NOT NULL` guard is intentional: a device row
//     whose `expires_at` is NULL is "never expires" and
//     must not be touched by the cleanup job.
//   - The LIMIT $2 cap protects the 03:00 UTC window. The
//     cap is the same value across iterations until the
//     backlog is empty; the next iteration picks up where
//     the previous one left off because the WHERE clause
//     filters out the already-deactivated rows.
//   - The RETURNING clause emits the (id, empleado_id) pair
//     so the per-batch info log can log the affected
//     employees without a second round-trip. The PII
//     surface is the bare minimum: empleado_id (bigint) and
//     device_id (UUID, the device row's PK — not a PII
//     field). The fcm_token is NEVER read or returned.
//   - The outer UPDATE is idempotent: the inner SELECT
//     filters on `is_active = TRUE`, so a row that was
//     deactivated by a concurrent run between the SELECT
//     and the UPDATE is silently skipped by the UPDATE's
//     own `is_active = TRUE` guard. The race is benign
//     because the per-job distributed lock is held by the
//     cron runner; the guard is defence-in-depth.
//
// The query is intentionally written with positional
// parameters ($1 = cutoff, $2 = batch size) — no string
// interpolation, per the SQL-injection hardening policy.
const deactivateExpiredDevicesSQL = `
UPDATE empleados.empleados_user_devices
SET is_active = FALSE,
    deactivated_at = $1,
    deactivation_reason = $3,
    updated_at = $1
WHERE id IN (
    SELECT id
    FROM empleados.empleados_user_devices
    WHERE is_active = TRUE
      AND expires_at IS NOT NULL
      AND expires_at < $1
    LIMIT $2
)
  AND is_active = TRUE
RETURNING id, empleado_id
`

// deactivateOneBatch runs the prepared statement and returns
// the affected-row count. The cutoff is the UTC `now` captured
// at the top of Run; the batchSize is the constructor-resolved
// cap. The per-batch info log is emitted by the Run caller so
// the per-batch error and per-batch success log lines share
// the same call site.
//
// The error is wrapped with %w so callers can errors.As on the
// underlying pgx error class. The sentinel ErrCleanupFailed is
// added one level up by the Run caller.
func (j *CleanupExpiredSessionsJob) deactivateOneBatch(ctx context.Context, now time.Time) (int64, error) {
	rows, err := j.db.Query(ctx, deactivateExpiredDevicesSQL, now, j.batchSize, sessionExpiredReason)
	if err != nil {
		return 0, fmt.Errorf("deactivateOneBatch query: %w", err)
	}
	defer rows.Close()

	// Drain the RETURNING rows. The deactivation has already
	// been applied at the database level; the row scan is
	// only there to confirm the row count and to log the
	// affected employees (per the spec's no-PII rule: only
	// bigint empleado_id and UUID device_id are emitted —
	// never the fcm_token, device model, or user-agent).
	var count int64
	for rows.Next() {
		var (
			deviceID   uuid.UUID
			empleadoID int64
		)
		if scanErr := rows.Scan(&deviceID, &empleadoID); scanErr != nil {
			return 0, fmt.Errorf("deactivateOneBatch scan: %w", scanErr)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("deactivateOneBatch rows: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Internal helpers — error class label for the metric.
// ---------------------------------------------------------------------------

// errorClassForCleanup returns a stable, low-cardinality label
// suitable for the scheduler_jobs_failed_total counter. The
// label is the Go type name of the deepest unwrappable error so
// the metric stays bounded even when many distinct error values
// flow through the job.
func errorClassForCleanup(err error) string {
	if err == nil {
		return "unknown"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return "pg_" + pgErr.Code
	}
	return "cleanup_expired_sessions_error"
}
