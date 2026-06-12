// scheduler_wiring.go — wireScheduler function: the integration
// boundary that builds the entire Scheduler Service runtime and
// hands the results to main.go.
//
// What wireScheduler owns
// =======================
// wireScheduler is the only place in the codebase that knows the
// construction order of the scheduler runtime. It is the
// integration boundary for the Scheduler Service: it loads the
// scheduler config, instantiates every collaborator (FCM client,
// M2M client, Parametrizacion client, lock manager, metrics,
// cron runner, sync dispatcher, sync consumer, and the five
// concrete jobs), registers the jobs with the cron runner,
// starts the consumer and the runner, mounts the scheduler
// routes on the supplied chi router, and returns a typed
// handle the caller can use to stop the goroutines during
// graceful shutdown.
//
// Why a separate file
// ====================
// The function is ~150 lines. Keeping it in its own file
// (rather than appended to main.go) keeps main.go focused on
// HTTP lifecycle (server start/stop, OTel, signal handling) and
// lets a reviewer read the scheduler-specific wiring in one
// place without scrolling through 300+ lines of HTTP plumbing.
//
// Sync consumer wiring — design choice
// =====================================
// The sincronizacion_solicitada job is registered with the
// CronRunner using the consumer's Handler() (NOT a separate
// goroutine). The same handler is invoked from the consumer's
// per-message flow:
//
//   1. The cron path: CronRunner schedules the job and invokes
//      the registered handler on each tick. The admin trigger
//      endpoint (POST /api/v1/scheduler/jobs/{id}/trigger) can
//      also fire the same handler via runner.TriggerNow. The
//      handler reads the SincronizacionEvent from the context;
//      when the cron path triggers the job directly (no event
//      on the context) the handler returns ErrInvalidEventPayload
//      and the runner records a FAILED execution. The cron
//      path exists so the admin trigger surface can force a
//      manual re-run.
//
//   2. The consumer path: The consumer's Run loop is started
//      in its own goroutine, reads from the configured backend
//      (Redis Streams by default; SQS when SQSQueueURL is set),
//      and invokes the same handler for every message. The
//      consumer sets the SincronizacionEvent on the context
//      before invoking the handler.
//
// Both paths share the same per-event lock and execution-row
// lifecycle (see service/sqs_consumer.go for the full
// contract). The choice to register the handler with the cron
// runner is deliberate: the admin trigger surface needs the
// runner's pause/resume and trigger semantics; the consumer
// path does not.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/jobs"
	"axis-flow-back/internal/scheduler/repository"
	"axis-flow-back/internal/scheduler/service"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// SchedulerHandles — return value of wireScheduler.
// ---------------------------------------------------------------------------

// SchedulerHandles is the typed bundle the caller receives from
// wireScheduler. main.go defers the Stop calls on the fields
// during graceful shutdown.
type SchedulerHandles struct {
	// Runner is the cron engine. Stop drains in-flight jobs
	// bounded by the supplied context.
	Runner *service.CronRunner
	// Consumer is the SQS / Redis Streams consumer that feeds
	// the sincronizacion_solicitada handler. The goroutine
	// running consumer.Run is cancelled by the context the
	// caller passed to wireScheduler — there is no explicit
	// Stop method on SyncConsumer.
	Consumer service.SyncConsumer
	// Dispatcher is the sync dispatcher used by the
	// sincronizacion_solicitada job. Held here for symmetry
	// with the consumer (the dispatcher is also referenced by
	// the cron-registered handler closure).
	Dispatcher service.SyncDispatcher
}

// Stop drains the cron runner. The consumer goroutine is
// cancelled by the parent context the caller passed to
// wireScheduler, so it needs no explicit shutdown here.
func (h *SchedulerHandles) Stop(ctx context.Context) error {
	if h == nil || h.Runner == nil {
		return nil
	}
	return h.Runner.Stop(ctx)
}

// ---------------------------------------------------------------------------
// wireScheduler — single integration boundary.
// ---------------------------------------------------------------------------

// wireScheduler loads the scheduler config, constructs every
// collaborator the scheduler runtime needs, registers the five
// concrete jobs with the cron runner, starts the sync consumer
// in a background goroutine, starts the cron runner, mounts the
// scheduler REST surface on the supplied chi router, and
// returns the typed handles.
//
// The router argument is the existing main chi router. The
// function does NOT start the HTTP server (lifecycle of main)
// — it only mounts routes on the router.
//
// Returns an error when any of the construction steps fails;
// main.go treats the error as fatal.
func wireScheduler(ctx context.Context, r chi.Router, dbPool *pgxpool.Pool, redisClient *redis.Client) (*SchedulerHandles, error) {
	if r == nil {
		return nil, fmt.Errorf("wireScheduler: router is required")
	}
	if dbPool == nil {
		return nil, fmt.Errorf("wireScheduler: dbPool is required")
	}
	if redisClient == nil {
		return nil, fmt.Errorf("wireScheduler: redisClient is required")
	}

	// 1. Load the scheduler config. The env-var loading lives
	// in the scheduler package so this service's main config
	// (which does not know about scheduler-specific env vars)
	// does not need to be extended.
	cfg, err := scheduler.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("wireScheduler: load config: %w", err)
	}

	// 2. Structured logger. The scheduler service uses its
	// own logfmt pipeline (the same shape used by the
	// notificaciones / empleados services) so the lifecycle
	// events are aggregated on the same dashboard.
	schedLogger := service.NewLogger(cfg.LogFormat, slog.LevelInfo)

	// 3. FCM client. Optional — when FIREBASE_CREDENTIALS_JSON
	// is empty (the spec mandates the env var, but tests and
	// local dev environments may not have it) the readiness
	// probe reports fcm: "skipped" and the notificaciones job
	// is the only consumer that would fail. We construct the
	// client only when the JSON is present.
	var fcm service.FCMClient
	if cfg.FirebaseCredentialsJSON != "" {
		fcmClient, ferr := service.NewFCMClient(ctx, []byte(cfg.FirebaseCredentialsJSON))
		if ferr != nil {
			return nil, fmt.Errorf("wireScheduler: fcm client: %w", ferr)
		}
		fcm = fcmClient
		schedLogger.Inner().Info("scheduler: fcm client initialised",
			slog.Bool("fcm_configured", true),
		)
	} else {
		schedLogger.Inner().Warn("scheduler: FIREBASE_CREDENTIALS_JSON is empty; push notifications disabled",
			slog.Bool("fcm_configured", false),
		)
	}

	// 4. M2M client. The auto-renewing wrapper is shared with
	// the Parametrizacion client (5) so the bearer-token
	// logic is in a single place.
	m2mClient := service.NewM2MClient(cfg)

	// 5. Parametrizacion client. Used by the inactiva_empleado
	// job to read per-company thresholds. The client uses the
	// M2M wrapper for outbound auth.
	parametrizacionClient := service.NewParametrizacionClient(
		cfg.ParametrizacionServiceURL, m2mClient, schedLogger,
	)

	// 6. Repositories. The scheduler uses two repos: jobs
	// (the canonical jobs table) and executions (the per-run
	// history). Both are constructed with the shared pool.
	jobRepo := repository.NewPgJobRepository(dbPool)
	execRepo := repository.NewPgExecutionRepository(dbPool)

	// 7. Lock manager. The Redis client is the same instance
	// the rest of the service uses; the lock manager shares
	// the connection pool with the cache / rate limiter.
	lockMgr := service.NewRedisLockManager(redisClient)

	// 8. Metrics. A dedicated registry is built so the
	// /metrics endpoint exposes only the scheduler's three
	// collectors and does not collide with the main service's
	// metrics. The Prometheus default registry is intentionally
	// avoided so a future test (or a different main service)
	// can swap the registry without breaking the scheduler
	// observations.
	metricsRegistry := prometheus.NewRegistry()
	metrics, merr := service.RegisterMetrics(metricsRegistry)
	if merr != nil {
		return nil, fmt.Errorf("wireScheduler: register metrics: %w", merr)
	}
	// InitMetrics sets the package-level Metrics singleton so
	// the per-job helper methods can reach it without a
	// struct dependency.
	service.InitMetrics(metricsRegistry)

	// 9. Cron runner. The runner is the heart of the
	// scheduler; it owns the robfig/cron/v3 instance and the
	// per-job lock lifecycle. Construction needs every
	// collaborator; the lock TTL comes from the config.
	runner := service.NewCronRunner(
		schedLogger,
		jobRepo,
		execRepo,
		lockMgr,
		metrics,
		cfg.SchedulerLockTTL(),
	)

	// 10. Build the four concrete cron jobs (the fifth
	// "sincronizacion_solicitada" job is wired separately
	// below because it has a special handler — the consumer's
	// closure). The constructor signatures vary by job; we
	// build them inline so the wiring is readable. The two
	// business jobs (notificaciones + inactiva_empleado)
	// receive typed repository / factory adapters (PR 5B-ii)
	// instead of the raw pgxpool.Pool.
	notifJob, njerr := jobs.NewNotificacionesEnTiempoRealJob(jobs.NotificacionesJobDeps{
		Shifts: jobs.NewPgShiftRepository(dbPool),
		FCM:    fcm,
		Logger: schedLogger,
	})
	if njerr != nil {
		return nil, fmt.Errorf("wireScheduler: build notificaciones job: %w", njerr)
	}
	employeeRepo := jobs.NewPgEmployeeRepository(dbPool)
	inactivaJob, ijerr := jobs.NewInactivaEmpleadoJob(jobs.InactivaEmpleadoJobDeps{
		Emps:           employeeRepo,
		TxFactory:      jobs.NewPgEmployeeTxFactory(dbPool, employeeRepo),
		Parametrizacion: parametrizacionClient,
		Outbox:         jobs.NewNoopOutboxWriter(),
		Logger:         schedLogger,
		Metrics:        metrics,
	})
	if ijerr != nil {
		return nil, fmt.Errorf("wireScheduler: build inactiva_empleado job: %w", ijerr)
	}
	cleanupJob, cjerr := jobs.NewCleanupExpiredSessionsJob(jobs.CleanupExpiredSessionsJobDeps{
		DB:      dbPool,
		Logger:  schedLogger,
		Metrics: metrics,
	})
	if cjerr != nil {
		return nil, fmt.Errorf("wireScheduler: build cleanup_expired_sessions job: %w", cjerr)
	}
	closeJob, cljerr := jobs.NewCloseLapsedAssignmentsJob(jobs.CloseLapsedAssignmentsJobDeps{
		DB:      dbPool,
		Logger:  schedLogger,
		Metrics: metrics,
	})
	if cljerr != nil {
		return nil, fmt.Errorf("wireScheduler: build close_lapsed_assignments job: %w", cljerr)
	}

	// 11. Sync dispatcher. The dispatcher is the
	// sincronizacion_solicitada job's handler; the same
	// handler is registered with the cron runner AND invoked
	// by the consumer.
	dispatcher := service.NewDefaultSyncDispatcher()

	// 12. Look up the sincronizacion_solicitada job row to
	// get its UUID primary key — the consumer needs the
	// job_id to write the FK on the scheduler_executions
	// row. When the row is missing (the seed file is PR 6
	// and has not landed yet) we log a warning and continue
	// with a zero UUID; the consumer's per-message path will
	// log an error on the first message and the cron path
	// will warn at Start time.
	syncJob, sjer := jobRepo.GetByKey(ctx, service.SincronizacionSolicitadaJobKey)
	if sjer != nil {
		schedLogger.Inner().Warn("scheduler: sincronizacion_solicitada row not yet seeded; the consumer path is disabled",
			slog.String("error", sjer.Error()),
		)
		syncJob = scheduler.Job{JobKey: service.SincronizacionSolicitadaJobKey}
	}

	// 13. Sync consumer. The consumer's Run loop is the
	// long-running background path; it is started in a
	// goroutine below. The Handler() closure is registered
	// with the cron runner so the admin trigger endpoint
	// (POST .../trigger) can also invoke it.
	consumer, cerr := service.NewSyncConsumer(
		cfg, lockMgr, execRepo, syncJob.JobID,
		dispatcher.Handler(), schedLogger,
	)
	if cerr != nil {
		return nil, fmt.Errorf("wireScheduler: build sync consumer: %w", cerr)
	}

	// 14. Register the five cron jobs. Each Register call
	// validates the schedule and binds the handler to a
	// fresh cron entry; failure here is a wiring bug
	// (invalid cron expression) and is fatal at startup.
	// The JobID on the metadata is left as uuid.Nil by the
	// concrete jobs; we re-resolve the row from the database
	// for each registration so the execution-row FK is
	// correct. A missing row is logged and the registration
	// proceeds with the metadata value (the runner's Start
	// will log another warning; the job will not run until
	// the row is seeded).
	type jobRegistration struct {
		name    string
		meta    scheduler.Job
		handler service.JobHandler
	}
	registrations := []jobRegistration{
		{name: "notificaciones_en_tiempo_real", meta: notifJob.Metadata(), handler: jobHandlerForRun(notifJob.Run)},
		{name: "inactiva_empleado", meta: inactivaJob.Metadata(), handler: jobHandlerForRun(inactivaJob.Run)},
		{name: "cleanup_expired_sessions", meta: cleanupJob.Metadata(), handler: jobHandlerForRun(cleanupJob.Run)},
		{name: "close_lapsed_assignments", meta: closeJob.Metadata(), handler: jobHandlerForRun(closeJob.Run)},
		{name: service.SincronizacionSolicitadaJobKey, meta: syncMetaJob(syncJob), handler: consumer.Handler()},
	}
	for _, reg := range registrations {
		row, lerr := jobRepo.GetByKey(ctx, reg.meta.JobKey)
		if lerr != nil {
			schedLogger.Inner().Warn("scheduler: job row missing in database; runner will not schedule it until it is seeded",
				slog.String("job_key", reg.meta.JobKey),
				slog.String("error", errStringOrUnknown(lerr)),
			)
			if rerr := runner.Register(reg.meta, reg.handler); rerr != nil {
				return nil, fmt.Errorf("wireScheduler: register %s: %w", reg.name, rerr)
			}
			continue
		}
		hydrated := reg.meta
		hydrated.JobID = row.JobID
		if rerr := runner.Register(hydrated, reg.handler); rerr != nil {
			return nil, fmt.Errorf("wireScheduler: register %s: %w", reg.name, rerr)
		}
	}

	// 15. Start the consumer in a goroutine. The consumer's
	// Run loop blocks until ctx is cancelled; the goroutine
	// does not return an error to the caller because the
	// consumer's per-message errors are logged internally and
	// the only way Run returns is on context cancellation or
	// a fatal transport error (which the consumer logs at
	// error level).
	consumerCtx, consumerCancel := context.WithCancel(ctx)
	go func() {
		defer consumerCancel()
		if rerr := consumer.Run(consumerCtx); rerr != nil {
			schedLogger.Inner().Error("scheduler: sync consumer exited with error",
				slog.String("error", rerr.Error()),
			)
		}
	}()

	// 16. Start the cron runner. Start is non-blocking: it
	// kicks off the underlying cron.Cron goroutine and
	// returns.
	if serr := runner.Start(ctx); serr != nil {
		consumerCancel()
		return nil, fmt.Errorf("wireScheduler: start runner: %w", serr)
	}

	// 17. Mount the REST surface. The mount function is the
	// single source of truth for the route table; the
	// scheduler routes live on the main router so health
	// probes and the Prometheus scraper reach the
	// scheduler service through the same public listener as
	// the rest of the platform.
	MountSchedulerRoutes(r, SchedulerDeps{
		DB:              dbPool,
		Redis:           redisClient,
		Config:          cfg,
		Logger:          schedLogger,
		Runner:          runner,
		JobRepo:         jobRepo,
		ExecutionRepo:   execRepo,
		Metrics:         metrics,
		MetricsRegistry: metricsRegistry,
		FCM:             fcm,
		Parametrizacion: parametrizacionClient,
	})

	schedLogger.Inner().Info("scheduler: runtime started",
		slog.Int("registered_jobs", len(registrations)),
		slog.Bool("fcm_configured", fcm != nil),
	)

	_ = time.Second // keep the time import in case future tweaks need it

	return &SchedulerHandles{
		Runner:     runner,
		Consumer:   consumer,
		Dispatcher: dispatcher,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

// jobHandlerForRun adapts a `func(ctx) error` (the signature on
// the concrete job's Run method) to the JobHandler signature
// the cron runner expects. The BaseJob is supplied by the
// runner and is ignored here — concrete jobs read their
// dependencies from the struct fields populated at construction
// time (see jobs/notificaciones_en_tiempo_real.go for the
// pattern).
func jobHandlerForRun(run func(ctx context.Context) error) service.JobHandler {
	return func(ctx context.Context, _ *service.BaseJob) error {
		return run(ctx)
	}
}

// syncMetaJob builds the scheduler.Job metadata for the
// sincronizacion_solicitada job. The Job row already exists
// (looked up earlier); we copy the schedule fields so the
// runner can build the cron entry. When the row is missing
// (the seed has not landed yet) the function returns a Job
// with nil schedule fields — the runner.Register call
// validates the schedule and the loop above catches the
// error with a warning.
func syncMetaJob(row scheduler.Job) scheduler.Job {
	return scheduler.Job{
		JobKey:          service.SincronizacionSolicitadaJobKey,
		JobID:           row.JobID,
		CronExpression:  row.CronExpression,
		IntervalSeconds: row.IntervalSeconds,
		JobClass:        "service.SyncDispatcher.Handler",
		Module:          "sync",
		IsActive:        true,
	}
}

// errStringOrUnknown returns the error's message, or "unknown"
// when the error is nil. Used by log lines that may carry a
// nil error after a soft-fail branch.
func errStringOrUnknown(err error) string {
	if err == nil {
		return "unknown"
	}
	return err.Error()
}
