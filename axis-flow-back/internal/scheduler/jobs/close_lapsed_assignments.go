// Package jobs holds the concrete business scheduled jobs the
// Scheduler service runs on a cron cadence.
//
// This file implements the `close_lapsed_assignments` job: the
// daily maintenance sweep that closes employee assignments whose
// end date is in the past. The job is the implementation of:
//
//   - Spec section "Responsabilidades > Access and Session Cleanup".
//   - Tasks.md task 3.5 (this PR — 3C).
//
// Design notes
// ============
//  1. The job embeds no service.BaseJob: the cron runner injects
//     a BaseJob at run time (see service/cron_runner.go), and the
//     job reads its own dedicated dependencies (db, logger,
//     metrics, batch size) from the struct field populated at
//     construction time. The same pattern is used by
//     InactivaEmpleadoJob (PR 3B-ii) and
//     CleanupExpiredSessionsJob (also PR 3C).
//  2. The job drains a "stale rows" backlog using a batched
//     UPDATE loop. The shared helper `runInBatches` (see
//     batch_processor.go) drives the loop; the SQL itself is
//     local to this file because the column shape is different
//     from the session-cleanup job.
//  3. The cap is `batchSize` rows per iteration (default 500).
//     The cap protects the 03:30 UTC window from ballooning
//     when a tenant ships an assignment-scheduling bug; the
//     next run picks up where this one stopped.
//  4. Database access is performed with prepared statements
//     ($1, $2, ...) against a *pgxpool.Pool. The spec's
//     "Seguridad > SQL Injection Prevention (Hardening)" section
//     is non-negotiable — no fmt.Sprintf SQL, no string
//     interpolation in WHERE clauses.
//  5. Per-batch errors do NOT fail the whole job — the helper
//     returns the error and the job's Run surfaces it so the
//     cron runner can record a FAILED execution and the operator
//     can decide whether to alert. (The close job is a single
//     SQL statement, so the per-batch "log and continue" pattern
//     from inactiva_empleado.go does not apply — the job has no
//     per-empresa boundary to short-circuit on.)
//  6. Idempotency: the `WHERE estatus = 1` clause on both the
//     SELECT-IN and the UPDATE is the idempotency guard. An
//     assignment that has already been closed by a previous
//     run is filtered out of the SELECT and is therefore not
//     touched by the UPDATE. Re-running the job is a no-op.
//  7. No PII: every log line carries only the bigint
//     `empleado_id`, the bigint `empresa_id`, and the UUID
//     assignment `id`. Employee names, emails, and any other
//     PII are NEVER logged.
//  8. The job does NOT emit integration events. The spec's
//     "Eventos > Publica" table lists the two events the
//     Scheduler Service emits; neither is produced by the
//     close job. The downstream observability sink is the
//     structured log lines and the per-job Prometheus
//     observations emitted by the cron runner / lifecycle
//     wrapper (scheduler_job_duration_seconds,
//     scheduler_jobs_failed_total).
//
// Schedule
// ========
// The cron expression `"30 3 * * *"` is the spec's choice:
// 03:30 UTC every day, after the cleanup_expired_sessions run
// (03:00 UTC) and the inactiva_empleado run (02:00 UTC). The
// serial ordering keeps the per-tenant mutation rate low and
// avoids overlapping UPDATEs on hot tables.
//
// Schema assumptions (documented, not enforced)
// ============================================
// The query layer assumes the following columns exist in the
// operational schemas (the Asignacion service owns the
// migration; this job treats it as a contract):
//
//	asignacion.asignacion_asignacion(
//	    id                UUID PRIMARY KEY,
//	    empleado_id       BIGINT NOT NULL,
//	    empresa_id        BIGINT NOT NULL,
//	    estatus           INT    NOT NULL,         -- 1=Activo, 2=Baja, 3=Cerrada (lapsed)
//	    fecha_inicio      TIMESTAMPTZ NOT NULL,
//	    fecha_fin         TIMESTAMPTZ NULL,         -- nullable for open-ended assignments
//	    fecha_cierre      TIMESTAMPTZ NULL,         -- set on close
//	    motivo_cierre     VARCHAR(100) NULL,        -- e.g. 'lapsed'
//	    updated_at        TIMESTAMPTZ
//	)
//
// The columns `estatus`, `fecha_fin`, `fecha_cierre`, and
// `motivo_cierre` are documented in the Asignacion service
// spec; the `empleado_id` and `empresa_id` columns are part of
// the standard tenant-aware indexing shape. The close job is
// the FIRST consumer of the "lapsed" status (3 = Cerrada) and
// the "lapsed" motivo_cierre value; the Asignacion service
// owns the canonical enum mapping. The documented
// `1=Activo / 2=Baja / 3=Cerrada` mapping matches the
// existing in-package assumption in
// `notificaciones_en_tiempo_real.go` (which also treats
// `estatus = 1` as "active" for the upcoming-shifts query).
//
// If a future migration renames any of these columns, the
// job's documented assumptions and the in-package join in
// `notificaciones_en_tiempo_real.go` will need to be updated
// together — the two queries are joined on the same `estatus =
// 1` predicate.
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

// CloseLapsedAssignmentsJobKey is the canonical job_key the
// seed file (Phase 6, task 6.1) inserts and that the wiring
// layer in main.go looks up via JobRepository.GetByKey. Matches
// the job_key documented in the spec's maintenance-section
// summary.
const CloseLapsedAssignmentsJobKey = "close_lapsed_assignments"

// closeLapsedAssignmentsCronExpr is the schedule from the
// prompt: 03:30 UTC every day, after the
// cleanup_expired_sessions run (03:00 UTC) and the
// inactiva_empleado run (02:00 UTC). Stored as a *string so
// it round-trips through the scheduler.Job domain type
// without copy.
var closeLapsedAssignmentsCronExpr = "30 3 * * *"

// closeLapsedAssignmentsDescription is the human-readable
// description for the scheduler_jobs.description column.
// Matches the spec's "Responsabilidades > Access and Session
// Cleanup" summary ("... and mark lapsed employee assignments
// as closed").
var closeLapsedAssignmentsDescription = "Cierre automático de asignaciones de empleados cuya fecha_fin ya venció (asignacion.asignacion_asignacion)."

// estatusAsignacionActivo is the documented `estatus` value
// for an active assignment. Matches the `estatus = 1` predicate
// used by the in-package upcoming-shifts query in
// `notificaciones_en_tiempo_real.go` (PR 3B-i). The value is
// treated as an INT to match the existing documented schema
// for `asignacion.asignacion_asignacion.estatus`.
const estatusAsignacionActivo = 1

// estatusAsignacionCerrada is the documented `estatus` value
// written to a lapsed assignment. The value `3` is the
// documented "Cerrada (lapsed)" slot in the schema assumption
// block at the top of this file; the Asignacion service owns
// the canonical enum mapping and the seed migration that
// establishes the `3` slot. The close job is the FIRST
// consumer of this status value; the choice of `3` (rather
// than reusing the `2=Baja` value) is intentional — "Baja"
// implies an operator-initiated termination, while
// "Cerrada (lapsed)" is a system-initiated close whose
// semantic must be distinguishable in downstream reports.
const estatusAsignacionCerrada = 3

// motivoCierreLapsed is the canonical value written to
// `asignacion_asignacion.motivo_cierre`. The column is
// free-form on the schema so future jobs (e.g. an
// operator-initiated close sweep) can flow through without a
// schema change.
const motivoCierreLapsed = "lapsed"

// Default values. Exposed as package-level vars so tests can
// tweak them in-place before constructing the job.
var (
	// DefaultCloseBatchSize is the default cap on the per-
	// iteration UPDATE row count. The cap protects the
	// 03:30 UTC window from ballooning when a tenant
	// ships an assignment-scheduling bug; the next run
	// picks up where this one stopped. 500 matches the
	// spec's recommendation for daily maintenance jobs and
	// is the same default used by
	// cleanup_expired_sessions.go.
	DefaultCloseBatchSize = 500
)

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	// ErrCloseFailed is the sentinel wrapped around any
	// underlying DB error that prevents the close loop from
	// making progress (context cancellation, connection
	// refused, schema mismatch). The cron runner's
	// WrapWithLifecycle converts it into a FAILED execution
	// row and increments scheduler_jobs_failed_total with
	// the error's class label.
	ErrCloseFailed = errors.New("scheduler: close_lapsed_assignments failed")
)

// ---------------------------------------------------------------------------
// DB abstraction — same pattern as the other jobs in this package.
// ---------------------------------------------------------------------------

// closeDB abstracts the pgx surface used by the job. The
// concrete implementation is the *pgxpool.Pool adapter; tests may
// inject a transaction-bound runner or a fake. Defined as a
// local interface so the job's surface stays minimal — the
// runner only needs Query (for the batched UPDATE...RETURNING)
// and Exec (kept for symmetry with the other jobs in this
// package, even though the current implementation does not call
// it).
type closeDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type closePoolAdapter struct{ pool *pgxpool.Pool }

func (a *closePoolAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return a.pool.Query(ctx, sql, args...)
}
func (a *closePoolAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.pool.Exec(ctx, sql, args...)
}

// ---------------------------------------------------------------------------
// Configuration knobs.
// ---------------------------------------------------------------------------

// CloseLapsedAssignmentsJobDeps bundles the dependencies the
// CloseLapsedAssignmentsJob needs. The pattern mirrors
// InactivaEmpleadoJob (PR 3B-ii) and
// CleanupExpiredSessionsJob (also PR 3C): a struct (instead of
// positional constructor args) keeps the call site readable
// as the dependency list grows. The CronRunner already
// provides a BaseJob at runtime, but the job retains direct
// handles to the ones its Run method uses so the type
// remains self-contained and testable in isolation.
type CloseLapsedAssignmentsJobDeps struct {
	// DB is the PostgreSQL pool used to perform the
	// batched UPDATE on asignacion.asignacion_asignacion.
	// Required.
	DB *pgxpool.Pool
	// Logger is the structured logger used for the per-
	// batch info line and the end-of-run summary. Required.
	Logger *service.Logger
	// Metrics is the typed holder for the Prometheus
	// collectors. The job calls IncJobFailed on per-batch
	// failures that should not fail the whole batch.
	// Optional (nil-safe; service.Metrics is also nil-safe).
	Metrics *service.SchedulerMetrics
	// BatchSize caps the per-iteration UPDATE row count.
	// Zero falls back to DefaultCloseBatchSize (500).
	BatchSize int
}

// ---------------------------------------------------------------------------
// CloseLapsedAssignmentsJob — the assignment-closing concrete cron job.
// ---------------------------------------------------------------------------

// CloseLapsedAssignmentsJob is the implementation of the
// `close_lapsed_assignments` job described in spec.md
// "Responsabilidades > Access and Session Cleanup" and
// Tasks.md task 3.5. The job:
//
//  1. Repeatedly calls a per-batch helper that UPDATEs the
//     next chunk of `asignacion.asignacion_asignacion` rows
//     whose `estatus = 1 AND fecha_fin < now` and that are
//     still active.
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
type CloseLapsedAssignmentsJob struct {
	deps CloseLapsedAssignmentsJobDeps

	db closeDB

	batchSize int
}

// ---------------------------------------------------------------------------
// Constructor.
// ---------------------------------------------------------------------------

// NewCloseLapsedAssignmentsJob constructs the job and resolves
// the configurable knobs to their non-zero defaults. A nil
// deps field returns an error so the wiring layer fails fast
// at startup rather than at the first cron tick.
func NewCloseLapsedAssignmentsJob(deps CloseLapsedAssignmentsJobDeps) (*CloseLapsedAssignmentsJob, error) {
	if deps.DB == nil {
		return nil, fmt.Errorf("jobs.NewCloseLapsedAssignmentsJob: %w: deps.DB is required", scheduler.ErrInvalidInput)
	}
	if deps.Logger == nil {
		return nil, fmt.Errorf("jobs.NewCloseLapsedAssignmentsJob: %w: deps.Logger is required", scheduler.ErrInvalidInput)
	}

	batchSize := deps.BatchSize
	if batchSize <= 0 {
		batchSize = DefaultCloseBatchSize
	}

	return &CloseLapsedAssignmentsJob{
		deps:      deps,
		db:        &closePoolAdapter{pool: deps.DB},
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
func (j *CloseLapsedAssignmentsJob) Metadata() scheduler.Job {
	cronExpr := closeLapsedAssignmentsCronExpr
	desc := closeLapsedAssignmentsDescription
	return scheduler.Job{
		JobKey:         CloseLapsedAssignmentsJobKey,
		CronExpression: &cronExpr,
		JobClass:       "jobs.CloseLapsedAssignmentsJob",
		Module:         "asignacion",
		Description:    &desc,
		IsActive:       true,
	}
}

// Run is the per-tick entry point. The cron runner wraps the
// call in WrapWithLifecycle and the lock-gated flow documented
// in service/cron_runner.go, so this method assumes it is the
// only goroutine executing it for the current process (lock is
// held by the runner).
func (j *CloseLapsedAssignmentsJob) Run(ctx context.Context) error {
	start := time.Now()
	now := start.UTC()

	// The closure captures the cutoff timestamp and the
	// batchSize so the prepared statement stays static. The
	// affected-row count returned by closeOneBatch is the
	// number of rows the UPDATE actually flipped; the helper
	// sees a 0-row result and terminates the loop (the
	// backlog is empty).
	var totalClosed int64
	var batches int
	processBatch := func(ctx context.Context) (int64, error) {
		batchStart := time.Now()
		affected, err := j.closeOneBatch(ctx, now)
		if err != nil {
			if j.deps.Metrics != nil {
				j.deps.Metrics.IncJobFailed(CloseLapsedAssignmentsJobKey, errorClassForClose(err))
			}
			return 0, fmt.Errorf("%w: %w", ErrCloseFailed, err)
		}
		batches++
		j.deps.Logger.Inner().InfoContext(ctx, "close_lapsed_assignments: closed lapsed assignments",
			slog.String("msg_field", "close_lapsed_assignments: closed lapsed assignments"),
			slog.Int64("affected", affected),
			slog.Int64("duration_ms", time.Since(batchStart).Milliseconds()),
		)
		return affected, nil
	}

	total, err := runInBatches(ctx, j.batchSize, processBatch)
	totalClosed = total
	if err != nil {
		// The helper already wraps the per-batch error. We
		// emit a final warn line so the operator can see the
		// partial-progress state (how many rows we managed
		// to close before the failure) and then surface
		// the error to the cron runner.
		j.deps.Logger.Inner().WarnContext(ctx, "close_lapsed_assignments: run aborted with partial progress",
			slog.String("msg_field", "close_lapsed_assignments: run aborted with partial progress"),
			slog.Int64("total_closed", totalClosed),
			slog.Int("batches", batches),
			slog.String("error", err.Error()),
		)
		return err
	}

	// End-of-run summary. The single info-level record
	// emitted per tick. No PII: only the affected-row
	// count, the batch count, and the total elapsed
	// milliseconds.
	j.deps.Logger.Inner().InfoContext(ctx, "close_lapsed_assignments run completed",
		slog.String("msg_field", "close_lapsed_assignments run completed"),
		slog.Int64("total_closed", totalClosed),
		slog.Int("batches", batches),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)
	return nil
}

// ---------------------------------------------------------------------------
// SQL — prepared statements for the per-batch update.
// ---------------------------------------------------------------------------

// closeLapsedAssignmentsSQL flips a capped batch of
// `asignacion.asignacion_asignacion` rows from active to
// closed, stamping the cierre timestamp and the motivo_cierre
// reason. The shape is the canonical "drain the backlog"
// pattern: a SELECT-IN subquery capped by LIMIT $2 picks the
// next batch of candidate rows, and the outer UPDATE applies
// the state change to exactly that batch.
//
// Why this shape
// --------------
//   - The WHERE clause in the SELECT-IN matches the WHERE
//     clause in the outer UPDATE byte-for-byte, so the rows
//     the UPDATE touches are exactly the rows the SELECT
//     picked. The two predicates are:
//     estatus = 1 AND fecha_fin IS NOT NULL AND fecha_fin < $1
//     The `IS NOT NULL` guard is intentional: an assignment
//     row whose `fecha_fin` is NULL is "open-ended" (no
//     scheduled end date) and must not be touched by the
//     close job.
//   - The LIMIT $2 cap protects the 03:30 UTC window. The
//     cap is the same value across iterations until the
//     backlog is empty; the next iteration picks up where
//     the previous one left off because the WHERE clause
//     filters out the already-closed rows.
//   - The RETURNING clause emits the
//     (id, empleado_id, empresa_id) triple so the per-batch
//     info log can log the affected employees and tenants
//     without a second round-trip. The PII surface is the
//     bare minimum: the assignment id (UUID), the empleado
//     id (bigint), and the empresa id (bigint). No names,
//     no emails, no assignment metadata.
//   - The outer UPDATE is idempotent: the inner SELECT
//     filters on `estatus = 1`, so a row that was closed by
//     a concurrent run between the SELECT and the UPDATE
//     is silently skipped by the UPDATE's own
//     `estatus = 1` guard. The race is benign because the
//     per-job distributed lock is held by the cron runner;
//     the guard is defence-in-depth.
//
// The query is intentionally written with positional
// parameters ($1 = cutoff, $2 = batch size, $3 = new estatus,
// $4 = motivo_cierre) — no string interpolation, per the
// SQL-injection hardening policy.
const closeLapsedAssignmentsSQL = `
UPDATE asignacion.asignacion_asignacion
SET estatus = $3,
    fecha_cierre = $1,
    motivo_cierre = $4,
    updated_at = $1
WHERE id IN (
    SELECT id
    FROM asignacion.asignacion_asignacion
    WHERE estatus = 1
      AND fecha_fin IS NOT NULL
      AND fecha_fin < $1
    LIMIT $2
)
  AND estatus = 1
RETURNING id, empleado_id, empresa_id
`

// closeOneBatch runs the prepared statement and returns the
// affected-row count. The cutoff is the UTC `now` captured at
// the top of Run; the batchSize is the constructor-resolved
// cap. The per-batch info log is emitted by the Run caller so
// the per-batch error and per-batch success log lines share
// the same call site.
//
// The error is wrapped with %w so callers can errors.As on the
// underlying pgx error class. The sentinel ErrCloseFailed is
// added one level up by the Run caller.
func (j *CloseLapsedAssignmentsJob) closeOneBatch(ctx context.Context, now time.Time) (int64, error) {
	rows, err := j.db.Query(ctx, closeLapsedAssignmentsSQL, now, j.batchSize, estatusAsignacionCerrada, motivoCierreLapsed)
	if err != nil {
		return 0, fmt.Errorf("closeOneBatch query: %w", err)
	}
	defer rows.Close()

	// Drain the RETURNING rows. The close has already been
	// applied at the database level; the row scan is only
	// there to confirm the row count and to log the
	// affected (id, empleado_id, empresa_id) triple — per
	// the spec's no-PII rule, only bigint identifiers and
	// the assignment's UUID are surfaced.
	var count int64
	for rows.Next() {
		var (
			assignmentID uuid.UUID
			empleadoID   int64
			empresaID    int64
		)
		if scanErr := rows.Scan(&assignmentID, &empleadoID, &empresaID); scanErr != nil {
			return 0, fmt.Errorf("closeOneBatch scan: %w", scanErr)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("closeOneBatch rows: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Internal helpers — error class label for the metric.
// ---------------------------------------------------------------------------

// errorClassForClose returns a stable, low-cardinality label
// suitable for the scheduler_jobs_failed_total counter. The
// label is the Go type name of the deepest unwrappable error
// so the metric stays bounded even when many distinct error
// values flow through the job.
func errorClassForClose(err error) string {
	if err == nil {
		return "unknown"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return "pg_" + pgErr.Code
	}
	return "close_lapsed_assignments_error"
}
