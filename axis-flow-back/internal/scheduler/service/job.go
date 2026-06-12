// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file defines the Job contract and the BaseJob dependency holder
// that every concrete scheduled job receives. The CronRunner (added in
// PR 2B) will call Job.Run on schedule, and concrete job implementations
// wrap their work in WrapWithLifecycle so the execution row is opened
// before the handler runs and finalized after it returns.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Sentinel errors. Use errors.Is to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrAlreadyRunning is returned by WrapWithLifecycle when the
	// underlying Insert on scheduler.scheduler_executions collides with
	// a unique constraint. It is a defensive sentinel — the current
	// schema does not enforce such a constraint, but the wrapper is
	// ready for the day a partial unique index on (job_id, status='RUNNING')
	// is added.
	ErrAlreadyRunning = errors.New("scheduler: job already running")
)

// ---------------------------------------------------------------------------
// Job — public contract implemented by every concrete scheduled job.
// ---------------------------------------------------------------------------

// Job is the unit of work the scheduler engine schedules and runs. A
// concrete Job is registered with the CronRunner (added in PR 2B) by
// its class name and is responsible for:
type Job interface {
	// Metadata returns the scheduler.Job record that describes this
	// concrete job (job_key, cron/interval schedule, module, etc.). The
	// scheduler engine reads Metadata once at registration time to
	// build the cron/interval schedule; the concrete job also uses it
	// at runtime to log its identity.
	Metadata() scheduler.Job
	// Run executes the job. The implementation must be idempotent
	// because the engine may invoke it again after a restart even if a
	// previous run appeared to fail. Implementations should use the
	// BaseJob's LockManager to ensure single-replica execution and
	// their Logger to emit the structured lifecycle events.
	Run(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// BaseJob — common dependencies shared by every concrete job.
// ---------------------------------------------------------------------------

// BaseJob bundles the cross-cutting dependencies every scheduled job
// needs: a structured logger, the execution repository for opening and
// finalizing execution rows, the lock manager for single-replica
// execution, and the job's own JobID so it can self-identify in logs
// and execution rows. Concrete jobs embed BaseJob to inherit these
// dependencies without each having to re-plumb them.
type BaseJob struct {
	// JobID is the scheduler.scheduler_jobs.job_id of this job. It is
	// the foreign key written into scheduler.scheduler_executions and
	// is also the value used as the job_id label on every Prometheus
	// observation.
	JobID uuid.UUID
	// Logger emits the structured lifecycle events (start, success,
	// failure, lock collision).
	Logger *Logger
	// ExecutionRepo persists the RUNNING/SUCCESS/FAILED row that
	// corresponds to each invocation of this job.
	ExecutionRepo repository.ExecutionRepository
	// LockManager serializes runs of this job across replicas. Long-
	// running jobs should call RenewHeartbeat periodically inside Run.
	LockManager LockManager
}

// NewBaseJob constructs a BaseJob with the provided dependencies. The
// caller (typically the concrete job's constructor) supplies a JobID
// derived from the scheduler.Job metadata.
func NewBaseJob(jobID uuid.UUID, logger *Logger, execRepo repository.ExecutionRepository, lockMgr LockManager) *BaseJob {
	return &BaseJob{
		JobID:         jobID,
		Logger:        logger,
		ExecutionRepo: execRepo,
		LockManager:   lockMgr,
	}
}

// ---------------------------------------------------------------------------
// JobHandler — the function signature CronRunner invokes.
// ---------------------------------------------------------------------------

// JobHandler is the unit of work the cron runner dispatches. Each concrete
// scheduled job exposes its Run as a JobHandler-compatible function so
// WrapWithLifecycle can drive the execution-row lifecycle around it.
//
// JobHandlers must be safe to invoke from a single goroutine. The cron
// runner does not run a given job concurrently with itself, but the
// function may be called from different goroutines for different jobs.
type JobHandler func(ctx context.Context, base *BaseJob) error

// ---------------------------------------------------------------------------
// WrapWithLifecycle — open/close execution row around a JobHandler.
// ---------------------------------------------------------------------------

// WrapWithLifecycle returns a function with the JobHandler signature that:
//  1. Mints a fresh execution_id (UUID v4).
//  2. Inserts a RUNNING row into scheduler.scheduler_executions.
//  3. Invokes the underlying handler.
//  4. On success, finalizes the row as StatusSuccess with a nil error.
//  5. On failure, finalizes the row as StatusFailed with the error and
//     increments the scheduler_jobs_failed_total counter with the
//     error's class name.
//  6. Returns the handler's error unchanged so the caller (the cron
//     runner) can log or react to it.
//
// The lifecycle wrapper never panics on a finalize failure: a Finalize
// error is logged through the base Logger and otherwise swallowed so a
// transient database hiccup at the end of a run does not mask the
// original handler error.
func WrapWithLifecycle(base *BaseJob, handler JobHandler) JobHandler {
	return func(ctx context.Context, b *BaseJob) error {
		if b == nil {
			b = base
		}
		if b.Logger == nil {
			return fmt.Errorf("WrapWithLifecycle: %w: base.Logger is nil", scheduler.ErrInvalidInput)
		}
		if b.ExecutionRepo == nil {
			return fmt.Errorf("WrapWithLifecycle: %w: base.ExecutionRepo is nil", scheduler.ErrInvalidInput)
		}
		if handler == nil {
			return fmt.Errorf("WrapWithLifecycle: %w: handler is nil", scheduler.ErrInvalidInput)
		}

		execID := uuid.New()
		jobID := b.JobID
		row := scheduler.Execution{
			JobID:     jobID,
			Status:    scheduler.StatusRunning,
			StartedAt: time.Now().UTC(),
		}
		inserted, err := b.ExecutionRepo.Insert(ctx, row)
		if err != nil {
			if isExecutionUniqueViolation(err) {
				return fmt.Errorf("WrapWithLifecycle: %w: job_id=%s", ErrAlreadyRunning, jobID)
			}
			return fmt.Errorf("WrapWithLifecycle: insert execution: %w", err)
		}
		// Use the server-assigned execution_id for finalize.
		execID = inserted.ExecutionID

		handlerErr := handler(ctx, b)
		if handlerErr != nil {
			errMsg := handlerErr.Error()
			if _, ferr := b.ExecutionRepo.Finalize(ctx, execID, scheduler.StatusFailed, &errMsg); ferr != nil {
				// Best-effort: log and continue so the original
				// handler error is still returned to the runner.
				b.Logger.Inner().ErrorContext(ctx, "failed to finalize execution as FAILED",
					"execution_id", execID.String(),
					"job_id", jobID.String(),
					"finalize_error", ferr.Error(),
				)
			}
			if Metrics != nil {
				Metrics.IncJobFailed(jobID.String(), errorClass(handlerErr))
			}
			return handlerErr
		}

		if _, ferr := b.ExecutionRepo.Finalize(ctx, execID, scheduler.StatusSuccess, nil); ferr != nil {
			b.Logger.Inner().ErrorContext(ctx, "failed to finalize execution as SUCCESS",
				"execution_id", execID.String(),
				"job_id", jobID.String(),
				"finalize_error", ferr.Error(),
			)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Internal helpers.
// ---------------------------------------------------------------------------

// isExecutionUniqueViolation reports whether the error coming back from
// scheduler.scheduler_executions.Insert is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505). It is exported to the package only.
func isExecutionUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// errorClass returns a stable, low-cardinality label suitable for the
// scheduler_jobs_failed_total counter. We strip pointer prefixes and
// keep only the type name so the cardinality of the error_class label
// stays bounded.
func errorClass(err error) string {
	if err == nil {
		return "unknown"
	}
	return fmt.Sprintf("%T", err)
}
