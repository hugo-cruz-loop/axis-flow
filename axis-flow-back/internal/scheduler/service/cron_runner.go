// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the CronRunner — the scheduling engine that ties
// together the engine primitives defined in PR 2A (LockManager, BaseJob,
// JobHandler, WrapWithLifecycle, Metrics). It owns the robfig/cron/v3
// instance, registers jobs by job_key, and runs each scheduled tick
// through a lock-gated execution flow that:
//
//  1. Mints a fresh run_id (UUID) used as the lock token.
//  2. Computes the per-job lock TTL (min(schedule_interval * 0.9, 10m)).
//  3. Acquires the distributed lock via LockManager.Acquire — on
//     collision the run is skipped, the
//     scheduler_redis_lock_failures_total counter is incremented, and
//     the MsgJobSkippedLock lifecycle log is emitted.
//  4. Starts a heartbeat goroutine that calls LockManager.RenewHeartbeat
//     every lockTTL/3 to keep the lease alive for long-running jobs.
//  5. Constructs a BaseJob and delegates the execution-row lifecycle
//     (insert RUNNING / finalize SUCCESS|FAILED) to WrapWithLifecycle.
//  6. Emits the MsgJobStart / MsgJobSuccess / MsgJobFailure logs.
//  7. Records the scheduler_job_duration_seconds observation.
//  8. Stops the heartbeat, releases the lock, and best-effort updates
//     last_run_at / next_run_at on the job row.
//
// Wiring note
// ============
// The CronRunner separates handler registration from Start. Concrete jobs
// (Phase 3) call runner.Register(job, handler) for each of their jobs
// before Start is called. The runner does not know about specific job
// types — it only knows job_key -> handler. Start loads the active jobs
// from the database for a verification pass and then calls
// cron.Cron.Start(); it does not re-register any handler on its own.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/repository"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	// ErrNoHandler is returned by TriggerNow and Resume when no handler
	// has been registered for the requested job_key. The expected
	// recovery is to call Register(job, handler) first; this is a
	// programmer error, not a transient failure.
	ErrNoHandler = errors.New("scheduler: no handler registered for job_key")
)

// ---------------------------------------------------------------------------
// CronRunner — the scheduling engine.
// ---------------------------------------------------------------------------

// CronRunner owns the robfig/cron/v3 instance and the maps that connect
// job_key -> handler and job_key -> cron.EntryID. It is safe for
// concurrent use: handlers and entries are guarded by mu, and the
// underlying cron.Cron is internally synchronized.
//
// CronRunner is the only consumer of LockManager in the runtime path:
// every scheduled tick (or TriggerNow invocation) is serialized across
// replicas by the Redis lock acquired before the handler runs.
type CronRunner struct {
	// cron is the underlying robfig/cron/v3 scheduler. It is configured
	// with the standard 5-field parser and a slog-backed logger so cron
	// library diagnostics flow through the same logfmt pipeline as the
	// rest of the service.
	cron *cron.Cron
	// logger emits the MsgJobStart / JobSuccess / JobFailure /
	// JobSkippedLock lifecycle events defined in the spec.
	logger *Logger
	// jobRepo persists pause/resume state and the last/next run
	// timestamps for each job.
	jobRepo repository.JobRepository
	// execRepo is passed to WrapWithLifecycle so the lifecycle wrapper
	// can open and finalize the execution row.
	execRepo repository.ExecutionRepository
	// lockMgr serializes execution across replicas.
	lockMgr LockManager
	// metrics is the typed holder for the three Prometheus collectors.
	// It is nil-safe — every helper tolerates a nil receiver.
	metrics *SchedulerMetrics
	// lockTTL is the fallback lock TTL (from SCHEDULER_LOCK_TTL_SEC)
	// used when the per-job schedule does not yield a known interval
	// (e.g. an exotic cron expression with no obvious period).
	lockTTL time.Duration
	// handlers maps job_key to the registered JobHandler. Populated by
	// Register; read by Resume and TriggerNow.
	handlers map[string]JobHandler
	// entries maps job_key to the cron.EntryID returned by Schedule.
	// Updated by Register / Resume; consulted by Pause and updateLastRun.
	entries map[string]cron.EntryID
	// mu guards handlers and entries.
	mu sync.RWMutex
}

// ---------------------------------------------------------------------------
// Constructor.
// ---------------------------------------------------------------------------

// NewCronRunner constructs a CronRunner. The cron.Cron instance is built
// with:
//
//   - cron.WithParser(...): a 5-field parser (Minute | Hour | Dom |
//     Month | Dow | Descriptor). The spec mandates 5-field cron; the
//     default parser in robfig/cron/v3 is already 5-field, but we
//     install our own explicitly so the intent is clear in the source.
//   - cron.WithLogger(cronLoggerAdapter{...}): forwards cron library
//     diagnostics to the same *slog.Logger that emits the lifecycle
//     events so they share the configured logfmt handler.
//
// lockTTL is the fallback Redis lock TTL — used as the per-tick TTL
// whenever the job schedule does not yield a known interval.
func NewCronRunner(
	logger *Logger,
	jobRepo repository.JobRepository,
	execRepo repository.ExecutionRepository,
	lockMgr LockManager,
	metrics *SchedulerMetrics,
	lockTTL time.Duration,
) *CronRunner {
	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	cr := cron.New(
		cron.WithParser(parser),
		cron.WithLogger(cronLoggerAdapter{log: logger.Inner()}),
	)
	return &CronRunner{
		cron:     cr,
		logger:   logger,
		jobRepo:  jobRepo,
		execRepo: execRepo,
		lockMgr:  lockMgr,
		metrics:  metrics,
		lockTTL:  lockTTL,
		handlers: make(map[string]JobHandler),
		entries:  make(map[string]cron.EntryID),
	}
}

// ---------------------------------------------------------------------------
// Register — bind a job_key to a handler.
// ---------------------------------------------------------------------------

// Register validates the job schedule, parses it into a robfig
// Schedule, and binds the supplied handler to a fresh cron entry.
//
//   - If job.CronExpression is set, it is parsed via cron.ParseStandard
//     (5-field, descriptor-aware).
//   - Otherwise job.IntervalSeconds is used to build a constant-delay
//     schedule via cron.Every.
//
// The handler is stored in the handlers map and a closure wrapping the
// lock-gated execution flow is registered with cron. The closure is a
// cron.FuncJob with no arguments — job metadata and handler are
// captured in the closure so the cron library can call them as func().
//
// Register returns scheduler.ErrInvalidInput on validation or parse
// failures, and scheduler.ErrConflict when a handler is already
// registered for the same job_key.
func (c *CronRunner) Register(job scheduler.Job, handler JobHandler) error {
	if handler == nil {
		return fmt.Errorf("cron_runner.Register: %w: handler is nil", scheduler.ErrInvalidInput)
	}
	if job.JobKey == "" {
		return fmt.Errorf("cron_runner.Register: %w: job_key is required", scheduler.ErrInvalidInput)
	}
	if err := job.ValidateSchedule(); err != nil {
		return fmt.Errorf("cron_runner.Register: %w", err)
	}

	sched, err := c.parseSchedule(job)
	if err != nil {
		return fmt.Errorf("cron_runner.Register: %w", err)
	}

	// Capture the metadata in locals so the closure is bound to the
	// values seen at registration time, not the caller's variables.
	j := job
	h := handler

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.handlers[job.JobKey]; exists {
		return fmt.Errorf("cron_runner.Register: %w: handler already registered for job_key=%s",
			scheduler.ErrConflict, job.JobKey)
	}

	entryID := c.cron.Schedule(sched, cron.FuncJob(func() {
		c.runLockGated(j, h)
	}))
	c.handlers[job.JobKey] = handler
	c.entries[job.JobKey] = entryID
	return nil
}

// parseSchedule turns a scheduler.Job's cron/interval fields into a
// robfig/cron/v3 Schedule. It is a small helper factored out of
// Register because Resume reuses it.
func (c *CronRunner) parseSchedule(job scheduler.Job) (cron.Schedule, error) {
	switch {
	case job.CronExpression != nil && *job.CronExpression != "":
		parsed, err := cron.ParseStandard(*job.CronExpression)
		if err != nil {
			return nil, fmt.Errorf("parse cron expression %q: %v", *job.CronExpression, err)
		}
		return parsed, nil
	case job.IntervalSeconds != nil:
		if *job.IntervalSeconds <= 0 {
			return nil, fmt.Errorf("interval_seconds must be > 0 (got %d)", *job.IntervalSeconds)
		}
		return cron.Every(time.Duration(*job.IntervalSeconds) * time.Second), nil
	default:
		return nil, fmt.Errorf("job has no cron_expression and no interval_seconds")
	}
}

// ---------------------------------------------------------------------------
// Start / Stop.
// ---------------------------------------------------------------------------

// Start runs a verification pass over the active jobs in the database
// (warning on any job_key that has no registered handler) and then
// starts the underlying cron scheduler. Start is idempotent — calling
// it more than once is a no-op, matching cron.Cron.Start's contract.
//
// Handlers are NOT re-registered here; they must be installed via
// Register before Start is called. The verification pass is purely
// diagnostic.
func (c *CronRunner) Start(ctx context.Context) error {
	jobs, err := c.jobRepo.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("cron_runner.Start: list active jobs: %w", err)
	}
	c.mu.RLock()
	for _, j := range jobs {
		if _, ok := c.handlers[j.JobKey]; !ok {
			if c.logger != nil {
				c.logger.Inner().WarnContext(ctx, "no handler registered for active job",
					"job_key", j.JobKey,
					"job_id", j.JobID.String(),
				)
			}
		}
	}
	c.mu.RUnlock()
	c.cron.Start()
	return nil
}

// Stop signals the underlying cron scheduler to stop accepting new
// runs and then waits for the in-flight jobs to drain, bounded by ctx.
// If ctx is cancelled before the drain completes, Stop returns ctx.Err
// — the in-flight jobs are still allowed to finish on their own by
// cron, but the caller no longer waits.
//
// The runner's maps (handlers / entries) are left intact so a later
// Start can resume scheduling the same set of jobs.
func (c *CronRunner) Stop(ctx context.Context) error {
	waitCtx := c.cron.Stop()
	if waitCtx == nil {
		return nil
	}
	select {
	case <-waitCtx.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ---------------------------------------------------------------------------
// Pause / Resume / TriggerNow — administrative operations.
// ---------------------------------------------------------------------------

// Pause removes the cron entry for jobKey (if any) and persists
// is_active = false on the job row. The handler is kept in the
// handlers map so a subsequent Resume can re-register the same
// closure without forcing the caller to re-supply it.
//
// Pause returns scheduler.ErrNotFound when the jobKey is unknown in
// the database — Pause is meaningful only for jobs that exist as rows
// in scheduler.scheduler_jobs.
func (c *CronRunner) Pause(ctx context.Context, jobKey string) error {
	if jobKey == "" {
		return fmt.Errorf("cron_runner.Pause: %w: job_key is required", scheduler.ErrInvalidInput)
	}
	// Look up the job first so the operation is well-defined even when
	// the runner was started before the row existed.
	j, err := c.jobRepo.GetByKey(ctx, jobKey)
	if err != nil {
		return fmt.Errorf("cron_runner.Pause: get job: %w", err)
	}

	c.mu.Lock()
	if entryID, ok := c.entries[jobKey]; ok {
		c.cron.Remove(entryID)
		delete(c.entries, jobKey)
	}
	c.mu.Unlock()

	if _, err := c.jobRepo.UpdatePause(ctx, j.JobID, true); err != nil {
		return fmt.Errorf("cron_runner.Pause: update pause: %w", err)
	}
	return nil
}

// Resume re-registers the cron entry for jobKey and persists
// is_active = true. The handler must already be present in the
// handlers map — Resume does not invent a handler for a job that was
// never registered. This is the contract documented in the file
// header's wiring note: callers (Phase 3) own the registration
// lifecycle, Resume only re-binds an existing registration to a
// fresh cron entry.
//
// Resume returns ErrNoHandler if the handler is missing, the same
// parse/sentinel errors as Register when the schedule is invalid, or
// scheduler.ErrNotFound when the jobKey is unknown in the database.
func (c *CronRunner) Resume(ctx context.Context, jobKey string) error {
	if jobKey == "" {
		return fmt.Errorf("cron_runner.Resume: %w: job_key is required", scheduler.ErrInvalidInput)
	}
	j, err := c.jobRepo.GetByKey(ctx, jobKey)
	if err != nil {
		return fmt.Errorf("cron_runner.Resume: get job: %w", err)
	}
	if err := j.ValidateSchedule(); err != nil {
		return fmt.Errorf("cron_runner.Resume: %w", err)
	}
	sched, err := c.parseSchedule(j)
	if err != nil {
		return fmt.Errorf("cron_runner.Resume: %w", err)
	}

	c.mu.RLock()
	handler, ok := c.handlers[jobKey]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("cron_runner.Resume: %w: job_key=%s", ErrNoHandler, jobKey)
	}

	// Capture the freshly-loaded metadata.
	jj := j
	hh := handler

	c.mu.Lock()
	entryID := c.cron.Schedule(sched, cron.FuncJob(func() {
		c.runLockGated(jj, hh)
	}))
	c.entries[jobKey] = entryID
	c.mu.Unlock()

	if _, err := c.jobRepo.UpdatePause(ctx, j.JobID, false); err != nil {
		return fmt.Errorf("cron_runner.Resume: update pause: %w", err)
	}
	return nil
}

// TriggerNow runs jobKey once, asynchronously, in a fresh goroutine.
// The HTTP layer (Phase 4) calls TriggerNow from
// POST /api/v1/scheduler/jobs/{id}/trigger and returns 202 Accepted
// immediately, so the goroutine uses context.Background() with its
// own timeout instead of the request context — the request lifetime
// must not bound the scheduled work.
//
// The goroutine fetches the job row from the database so the run
// reflects the latest persisted metadata (description, module,
// updated schedule, etc.) rather than the snapshot captured at
// Register time.
func (c *CronRunner) TriggerNow(ctx context.Context, jobKey string) error {
	if jobKey == "" {
		return fmt.Errorf("cron_runner.TriggerNow: %w: job_key is required", scheduler.ErrInvalidInput)
	}
	c.mu.RLock()
	handler, ok := c.handlers[jobKey]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("cron_runner.TriggerNow: %w: job_key=%s", ErrNoHandler, jobKey)
	}

	// The handler closure is captured at registration time, so we only
	// need to look up the metadata here.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*c.lockTTL)
		defer cancel()
		j, err := c.jobRepo.GetByKey(bgCtx, jobKey)
		if err != nil {
			if c.logger != nil {
				c.logger.Inner().ErrorContext(bgCtx, "TriggerNow: failed to load job",
					"job_key", jobKey,
					"error", err.Error(),
				)
			}
			return
		}
		c.runLockGated(j, handler)
	}()
	return nil
}

// ---------------------------------------------------------------------------
// runLockGated — the per-tick execution flow.
// ---------------------------------------------------------------------------

// runLockGated is the closure executed by the cron library for every
// tick of a registered job. It is also called by TriggerNow. The flow
// is described in the package-level comment at the top of this file.
//
// The lock token is a freshly-minted run_id (UUID) — distinct from
// the execution_id that WrapWithLifecycle mints internally. Keeping
// the lock token independent from the database row PK means the
// heartbeat goroutine can keep the lease alive even if the insert
// into scheduler.scheduler_executions fails: the lock is still
// released cleanly when the run returns.
func (c *CronRunner) runLockGated(job scheduler.Job, handler JobHandler) {
	runID := uuid.New()

	// Bound the run duration by 2x the configured lock TTL so a runaway
	// job never outlives its lease by a wide margin. Each tick gets a
	// fresh context — we do not chain tick contexts together.
	ctx, cancel := context.WithTimeout(context.Background(), 2*c.lockTTL)
	defer cancel()

	lockTTL := c.computeLockTTL(job)

	// 1. Acquire the lock.
	acquired, err := c.lockMgr.Acquire(ctx, job.JobKey, runID.String(), lockTTL)
	if err != nil {
		// Transport-level failure — log loudly and bail. We do NOT
		// increment IncRedisLockFailure because that counter is
		// reserved for "another replica owns the lock" collisions per
		// the spec's label semantics.
		if c.logger != nil {
			c.logger.Inner().ErrorContext(ctx, "failed to acquire distributed lock",
				"job_key", job.JobKey,
				"execution_id", runID.String(),
				"error", err.Error(),
			)
		}
		return
	}
	if !acquired {
		if c.metrics != nil {
			c.metrics.IncRedisLockFailure(job.JobKey)
		}
		if c.logger != nil {
			c.logger.JobSkippedLock(ctx, JobSkippedLockEvent{
				JobID:   job.JobKey,
				TraceID: TraceIDFromContext(ctx),
			})
		}
		return
	}

	// 2. Heartbeat goroutine. The stop/done channels coordinate
	// shutdown so the lock release in the deferred block happens AFTER
	// the heartbeat has exited.
	heartbeatStop := make(chan struct{})
	heartbeatDone := make(chan struct{})
	heartbeatInterval := lockTTL / 3
	if heartbeatInterval <= 0 {
		heartbeatInterval = time.Second
	}
	go c.heartbeatLoop(ctx, job.JobKey, runID.String(), lockTTL, heartbeatInterval, heartbeatStop, heartbeatDone)

	defer func() {
		// Stop the heartbeat first so it does not try to renew a lock
		// we are about to release.
		close(heartbeatStop)
		<-heartbeatDone

		// Release the lock with a fresh, bounded context — the per-tick
		// context may have been cancelled by the handler.
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		if rerr := c.lockMgr.Release(releaseCtx, job.JobKey, runID.String()); rerr != nil {
			if c.logger != nil {
				c.logger.Inner().ErrorContext(releaseCtx, "failed to release distributed lock",
					"job_key", job.JobKey,
					"execution_id", runID.String(),
					"error", rerr.Error(),
				)
			}
		}

		// Update last_run_at / next_run_at best-effort. The same
		// bounded context is used so a hung DB does not delay
		// shutdown indefinitely.
		updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer updateCancel()
		c.updateLastRun(updateCtx, job)
	}()

	// 3. Build the BaseJob and wrap the handler with the lifecycle
	// (insert RUNNING row / finalize SUCCESS|FAILED, IncJobFailed on
	// error). WrapWithLifecycle owns the execution_id — the runner
	// does not see it.
	base := NewBaseJob(job.JobID, c.logger, c.execRepo, c.lockMgr)
	wrapped := WrapWithLifecycle(base, handler)

	start := time.Now()
	if c.logger != nil {
		c.logger.JobStart(ctx, JobStartEvent{
			JobID:   job.JobKey,
			TraceID: TraceIDFromContext(ctx),
		})
	}

	runErr := wrapped(ctx, base)

	duration := time.Since(start)
	if c.metrics != nil {
		c.metrics.ObserveJobDuration(job.JobKey, duration)
	}

	switch {
	case runErr != nil:
		if c.logger != nil {
			c.logger.JobFailure(ctx, JobFailureEvent{
				JobID:          job.JobKey,
				TraceID:        TraceIDFromContext(ctx),
				DurationMillis: MillisSince(start),
				Err:            runErr,
			})
		}
		// IncJobFailed is incremented by WrapWithLifecycle to keep the
		// "failed run" metric consistent with the FAILED execution
		// row it writes.
	default:
		if c.logger != nil {
			c.logger.JobSuccess(ctx, JobSuccessEvent{
				JobID:            job.JobKey,
				TraceID:          TraceIDFromContext(ctx),
				DurationMillis:   MillisSince(start),
				ProcessedRecords: 0,
			})
		}
	}
}

// heartbeatLoop renews the Redis lock every `interval` until stop is
// closed or ctx is cancelled. It exits cleanly on either signal: a
// closed stop means the run is finishing and the caller will release
// the lock; a cancelled ctx means the run is aborting and the lock
// will be released by the deferred block in runLockGated.
func (c *CronRunner) heartbeatLoop(
	ctx context.Context,
	jobKey, runID string,
	lockTTL, interval time.Duration,
	stop <-chan struct{},
	done chan<- struct{},
) {
	defer close(done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			renewCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			renewed, rerr := c.lockMgr.RenewHeartbeat(renewCtx, jobKey, runID, lockTTL)
			cancel()
			if rerr != nil || !renewed {
				if c.logger != nil {
					c.logger.Inner().ErrorContext(renewCtx, "failed to renew heartbeat",
						"job_key", jobKey,
						"execution_id", runID,
						"renewed", renewed,
						"error", errString(rerr),
					)
				}
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Internal helpers.
// ---------------------------------------------------------------------------

// computeLockTTL returns the per-job lock TTL: 0.9 * schedule_interval,
// capped at 10 minutes and floored at 1 second. If the schedule is
// cron-based and the next-fire interval cannot be computed, the
// configured fallback (c.lockTTL from SCHEDULER_LOCK_TTL_SEC) is used.
func (c *CronRunner) computeLockTTL(job scheduler.Job) time.Duration {
	const (
		maxLockTTL = 10 * time.Minute
		minLockTTL = time.Second
		factor     = 0.9
	)
	switch {
	case job.IntervalSeconds != nil && *job.IntervalSeconds > 0:
		ttl := time.Duration(float64(*job.IntervalSeconds)*factor) * time.Second
		return clampLockTTL(ttl, minLockTTL, maxLockTTL, c.lockTTL)
	case job.CronExpression != nil && *job.CronExpression != "":
		parsed, err := cron.ParseStandard(*job.CronExpression)
		if err != nil {
			return c.lockTTL
		}
		next := parsed.Next(time.Now())
		if next.IsZero() {
			return c.lockTTL
		}
		interval := time.Until(next)
		if interval <= 0 {
			return c.lockTTL
		}
		ttl := time.Duration(float64(interval) * factor)
		return clampLockTTL(ttl, minLockTTL, maxLockTTL, c.lockTTL)
	default:
		return c.lockTTL
	}
}

// clampLockTTL applies the documented bounds (>= 1s, <= 10m) and
// falls back to the supplied default when the computed TTL is not
// positive. Kept as a small helper so computeLockTTL stays readable.
func clampLockTTL(ttl, min, max, fallback time.Duration) time.Duration {
	if ttl <= 0 {
		return fallback
	}
	if ttl < min {
		return min
	}
	if ttl > max {
		return max
	}
	return ttl
}

// updateLastRun records the last_run_at timestamp and, if the job
// currently has a live cron entry, the next_run_at timestamp. The
// call is best-effort: any error is logged and swallowed so a
// transient database hiccup at the end of a run does not mask the
// original handler error.
func (c *CronRunner) updateLastRun(ctx context.Context, job scheduler.Job) {
	now := time.Now().UTC()
	var nextPtr *time.Time
	c.mu.RLock()
	entryID, ok := c.entries[job.JobKey]
	c.mu.RUnlock()
	if ok {
		entry := c.cron.Entry(entryID)
		if entry.Valid() && !entry.Next.IsZero() {
			next := entry.Next.UTC()
			nextPtr = &next
		}
	}
	if _, err := c.jobRepo.UpdateLastRun(ctx, job.JobID, &now, nextPtr); err != nil {
		if c.logger != nil {
			c.logger.Inner().ErrorContext(ctx, "failed to update last_run_at",
				"job_key", job.JobKey,
				"job_id", job.JobID.String(),
				"error", err.Error(),
			)
		}
	}
}

// errString is a tiny nil-safe error stringifier used by the heartbeat
// loop. Defined here to avoid a one-line helper in another file.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ---------------------------------------------------------------------------
// cronLoggerAdapter — bridge cron.Logger to *slog.Logger.
// ---------------------------------------------------------------------------

// cronLoggerAdapter adapts a *slog.Logger to robfig/cron/v3's Logger
// interface. The cron library emits Printf-style key/value messages
// (msg, key1, val1, key2, val2, ...); slog.Info / slog.Error accept
// the same shape as variadic any arguments, so we can forward the
// slice directly.
type cronLoggerAdapter struct {
	log *slog.Logger
}

// Info implements cron.Logger.Info. It is a no-op when the embedded
// logger is nil (defensive — NewCronRunner never sets it to nil, but
// the tests may).
func (a cronLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	if a.log == nil {
		return
	}
	a.log.Info(msg, keysAndValues...)
}

// Error implements cron.Logger.Error. The err argument is inserted as
// the "error" key so the structured-log pipeline tags it the same way
// every other error log in the service is tagged.
func (a cronLoggerAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	if a.log == nil {
		return
	}
	args := make([]any, 0, len(keysAndValues)+2)
	args = append(args, slog.String("error", errString(err)))
	args = append(args, keysAndValues...)
	a.log.Error(msg, args...)
}
