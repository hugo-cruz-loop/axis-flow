// scheduler_routes.go — Single mount point for the Scheduler
// Service's REST surface.
//
// Architecture
// ============
// The Scheduler Service is a background worker that exposes a
// small admin REST API for manual triggers, listing, pause/resume,
// and execution history. The endpoints are split into three
// groups by auth profile:
//
//   1. PUBLIC (no JWT):
//      - GET  /health/live
//      - GET  /health/ready
//      - GET  /metrics
//
//   2. PROTECTED (M2M JWT, scopes scheduler:read OR scheduler:write):
//      - GET    /api/v1/scheduler/jobs
//      - POST   /api/v1/scheduler/jobs/{job_id}/trigger
//      - PATCH  /api/v1/scheduler/jobs/{job_id}/pause
//      - GET    /api/v1/scheduler/executions
//
// Design choice — single mount function
// ======================================
// Every scheduler endpoint is mounted from a single function
// (MountSchedulerRoutes) so the wiring lives in one place. The
// main.go is the only caller; it instantiates the SchedulerDeps
// struct and hands the function the chi router. This keeps
// main.go free of route definitions and lets the scheduler
// surface be evolved (added/removed/renamed routes, middleware
// changes) without touching the main wiring.
package main

import (
	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/handler"
	"axis-flow-back/internal/scheduler/repository"
	"axis-flow-back/internal/scheduler/service"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// SchedulerDeps — the dependency bundle the mount function needs.
// ---------------------------------------------------------------------------

// SchedulerDeps groups every collaborator the scheduler handlers
// (admin REST + health) need. The struct is the single argument
// to MountSchedulerRoutes; main.go constructs one and hands it
// over.
//
// Field naming follows the pattern <name> <type> from the
// constructor signatures in the corresponding packages so the
// call site reads naturally:
//
//	MountSchedulerRoutes(r, SchedulerDeps{
//	    DB: dbPool,
//	    Redis: redisClient,
//	    Config: schedulerCfg,
//	    ...
//	})
type SchedulerDeps struct {
	// DB is the PostgreSQL pool shared with the rest of the
	// service. Used by the admin handlers (job/execution
	// repositories) and the readiness probe.
	DB *pgxpool.Pool
	// Redis is the connection used by the lock manager and
	// the readiness probe. The same client is shared with the
	// SyncConsumer in the wireScheduler flow.
	Redis *redis.Client
	// Config is the Scheduler Service's environment-loaded
	// config. Only M2MSigningKey is consumed here; the rest
	// is used by the job wiring.
	Config scheduler.Config
	// Logger is the structured logfmt logger. Mounted on the
	// auth middleware so the access log lines land on the
	// scheduler pipeline.
	Logger *service.Logger
	// Runner is the cron runner the admin handlers (pause /
	// resume / trigger) call into. Built by wireScheduler.
	Runner *service.CronRunner
	// JobRepo is the persistence layer for the
	// scheduler.scheduler_jobs table.
	JobRepo repository.JobRepository
	// ExecutionRepo is the persistence layer for the
	// scheduler.scheduler_executions table.
	ExecutionRepo repository.ExecutionRepository
	// Metrics is the typed holder for the three Prometheus
	// collectors. Used by the readiness handler to build the
	// /metrics endpoint.
	Metrics *service.SchedulerMetrics
	// MetricsRegistry is the registerer the metrics were
	// registered against. Used by the readiness handler to
	// build the /metrics scrape handler. Typically the same
	// value passed to scheduler.service.RegisterMetrics.
	MetricsRegistry prometheus.Registerer
	// FCM is the Firebase Cloud Messaging client. Optional;
	// when nil the readiness probe reports fcm: "skipped".
	FCM service.FCMClient
	// Parametrizacion is the HTTP client used by the
	// auto-deactivation job to read per-company thresholds.
	// Not consumed by the handlers directly, but the mount
	// function keeps a reference so future handlers (e.g. a
	// per-job dry-run) can reach it through deps.
	Parametrizacion service.ParametrizacionClient
}

// ---------------------------------------------------------------------------
// MountSchedulerRoutes — single entry point.
// ---------------------------------------------------------------------------

// MountSchedulerRoutes wires every scheduler route onto the
// supplied chi router. The function does NOT start the HTTP
// server — that lifecycle is owned by main.go. It only
// instantiates the handlers and registers the routes (with
// the appropriate middleware).
//
// Route map
// ---------
//
//	GET    /health/live                          (public)
//	GET    /health/ready                         (public)
//	GET    /metrics                              (public)
//	GET    /api/v1/scheduler/jobs                (JWT, scheduler:read|write)
//	POST   /api/v1/scheduler/jobs/{job_id}/trigger  (JWT, scheduler:read|write)
//	PATCH  /api/v1/scheduler/jobs/{job_id}/pause    (JWT, scheduler:read|write)
//	GET    /api/v1/scheduler/executions         (JWT, scheduler:read|write)
func MountSchedulerRoutes(r chi.Router, deps SchedulerDeps) {
	// Install the access-log logger on the JWT middleware so
	// the auth-check log lines land on the scheduler's logfmt
	// pipeline.
	handler.SetAuthLogger(deps.Logger)

	// Health + metrics (PUBLIC).
	healthH := handler.NewHealthHandler(handler.HealthHandlerDeps{
		DB:       deps.DB,
		Redis:    deps.Redis,
		FCM:      deps.FCM,
		Logger:   deps.Logger,
		Registry: deps.MetricsRegistry,
	})
	r.Get("/health/live", healthH.Live)
	r.Get("/health/ready", healthH.Ready)
	r.Method("GET", "/metrics", healthH.MetricsHandler())

	// Admin REST surface (PROTECTED by M2M JWT). The signing
	// key is the same shared secret the M2M client uses to
	// mint outbound tokens (SCHEDULER_M2M_SIGNING_KEY); the
	// middleware accepts both "scheduler:read" and
	// "scheduler:write" so the read-only listing endpoints
	// and the write/trigger endpoints share the same auth
	// envelope. Per-endpoint scope enforcement is left to a
	// future iteration — the spec mandates
	// "scheduler:read" / "scheduler:write" as the only two
	// scopes, and accepting either at the middleware layer
	// matches the existing project convention (see
	// internal/middleware/rbac.go, which similarly accepts
	// any role from the allowlist).
	jwtMW := handler.RequireJWT([]byte(deps.Config.M2MSigningKey), "scheduler:read", "scheduler:write")

	jobsH := handler.NewJobsHandler(handler.JobsHandlerDeps{
		JobRepo: deps.JobRepo,
		Runner:  deps.Runner,
		Logger:  deps.Logger,
	})
	triggerH := handler.NewTriggerHandler(handler.TriggerHandlerDeps{
		JobRepo: deps.JobRepo,
		Runner:  deps.Runner,
		Logger:  deps.Logger,
	})
	execsH := handler.NewExecutionsHandler(handler.ExecutionsHandlerDeps{
		ExecRepo: deps.ExecutionRepo,
		JobRepo:  deps.JobRepo,
		Logger:   deps.Logger,
	})

	r.Route("/api/v1/scheduler", func(r chi.Router) {
		r.Use(jwtMW)

		r.Get("/jobs", jobsH.List)
		r.Post("/jobs/{job_id}/trigger", triggerH.Trigger)
		r.Patch("/jobs/{job_id}/pause", jobsH.Pause)
		r.Get("/executions", execsH.List)
	})
}
