// health_handler.go — Liveness and readiness probes for the Scheduler
// Service.
//
//   - GET /health/live   (HealthHandler.Live)   — 200 if the process
//     is up. No upstream calls. Matches the spec's "Liveness probe
//     indicating if the service process is running" entry in the
//     Endpoint Directory.
//
//   - GET /health/ready  (HealthHandler.Ready)  — 200 when every
//     downstream dependency (PostgreSQL, Redis, FCM) answers its
//     lightweight probe within 2s. Any failed check flips the
//     response to 503 with the per-check status surfaced. Matches
//     the spec's "Readiness probe verifying connections to
//     PostgreSQL, Redis, and SQS" entry — the FCM probe is an
//     addition driven by the security contract (a misconfigured
//     Firebase project must not pass the readiness probe silently).
//
//   - GET /metrics       (HealthHandler.MetricsHandler) — Prometheus
//     scrape endpoint, exposed on the chi router. The metrics
//     registry is the one passed to scheduler.service.RegisterMetrics
//     (NOT the default Prometheus registry) so the scheduler
//     observations are scoped to the scheduler service.
//
// Auth
// ===
// Both endpoints are PUBLIC (per the spec's Endpoint Directory —
// Auth column reads "None"). The metrics endpoint is also public so
// the Prometheus scraper does not need to mint a JWT.
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"axis-flow-back/internal/scheduler/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Sentinel error.
// ---------------------------------------------------------------------------

// ErrHealthCheckFailed is returned by the inner probe helper when at
// least one of the downstream checks (PostgreSQL, Redis, FCM) failed.
// The HTTP response itself is a structured JSON envelope (see
// healthResponse below) and the status code is 503 rather than 500 —
// the operationally useful "service is not ready to accept traffic"
// signal.
var ErrHealthCheckFailed = errors.New("scheduler: at least one health check failed")

// ---------------------------------------------------------------------------
// Probe timeouts and constants.
// ---------------------------------------------------------------------------

// healthCheckTimeout is the per-check upper bound. The readiness
// probe is called frequently (every few seconds in a typical
// Kubernetes deployment) so the cap must be small enough that a
// single hung dependency does not turn the probe into a load
// generator. 2s matches the spec's "Redis Ping" and "Postgres Ping"
// recommendations and is well above the typical sub-millisecond
// healthy round-trip.
const healthCheckTimeout = 2 * time.Second

// ---------------------------------------------------------------------------
// HealthHandler — HTTP handlers for the readiness / liveness probes.
// ---------------------------------------------------------------------------

// HealthHandlerDeps groups the collaborators HealthHandler needs. A
// nil DB or Redis is a wiring bug and is reported on every probe
// (the response is "down"); a nil FCM is the supported "no
// credentials configured" state and is reported as "skipped" rather
// than "down".
type HealthHandlerDeps struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
	// FCM may be nil. When nil, the readiness probe reports
	// `fcm: "skipped"` instead of attempting the probe — the
	// scheduler is allowed to run without push dispatch when
	// the operator has not yet provisioned Firebase credentials.
	FCM    service.FCMClient
	Logger *service.Logger
	// Registry is the Prometheus registerer the scheduler's
	// metrics were registered against. It is used to build the
	// /metrics handler. A nil registry falls back to the
	// Prometheus default registry.
	Registry prometheus.Registerer
}

// HealthHandler is the HTTP handler for the /health/live and
// /health/ready endpoints. It is safe to share across goroutines —
// every collaborator it holds is itself safe for concurrent use.
type HealthHandler struct {
	db       *pgxpool.Pool
	redis    *redis.Client
	fcm      service.FCMClient
	logger   *service.Logger
	registry prometheus.Registerer
}

// NewHealthHandler builds a HealthHandler. See HealthHandlerDeps for
// the nil-collaborator contract.
func NewHealthHandler(deps HealthHandlerDeps) *HealthHandler {
	return &HealthHandler{
		db:       deps.DB,
		redis:    deps.Redis,
		fcm:      deps.FCM,
		logger:   deps.Logger,
		registry: deps.Registry,
	}
}

// ---------------------------------------------------------------------------
// Response envelope (JSON).
// ---------------------------------------------------------------------------

// healthResponse is the JSON body returned by Ready. The shape is
// documented in the spec section "Observabilidad > Health Checks":
//
//	{
//	  "status": "ok" | "degraded",
//	  "checks": {
//	    "postgres": "ok" | "down",
//	    "redis":    "ok" | "down",
//	    "fcm":      "ok" | "skipped" | "down"
//	  },
//	  "timestamp": "<rfc3339>"
//	}
//
// The "degraded" status means at least one of the checks failed;
// the per-check status field tells the operator which one(s).
type healthResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
	Timestamp string            `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// GET /health/live — liveness probe.
// ---------------------------------------------------------------------------

// Live handles GET /health/live. It returns 200 with
// `{"status":"ok"}` and makes no upstream calls. The liveness probe
// is intended to answer the single question "is the process up?";
// all upstream checks belong on the readiness probe.
func (h *HealthHandler) Live(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// ---------------------------------------------------------------------------
// GET /health/ready — readiness probe.
// ---------------------------------------------------------------------------

// Ready handles GET /health/ready. It runs three checks — Postgres,
// Redis, and FCM — each bounded by a 2s timeout. The response is
// 200 when every check passes, 503 when any check fails. The body
// is the structured envelope documented on healthResponse.
//
// The handler is safe for concurrent invocation; the underlying
// pool/redis clients are themselves concurrency-safe.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout*2)
	defer cancel()

	checks := map[string]string{
		"postgres": h.checkPostgres(ctx),
		"redis":    h.checkRedis(ctx),
		"fcm":      h.checkFCM(ctx),
	}

	status := "ok"
	httpStatus := http.StatusOK
	for _, v := range checks {
		if v == "down" {
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
			break
		}
	}

	resp := healthResponse{
		Status:    status,
		Checks:    checks,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if h.logger != nil {
		h.logger.Inner().LogAttrs(ctx, slogLevelFor(httpStatus),
			"scheduler health probe",
			slogString("status", status),
			slog.String("checks.postgres", checks["postgres"]),
			slog.String("checks.redis", checks["redis"]),
			slog.String("checks.fcm", checks["fcm"]),
			slog.String("request_id", requestIDFromContext(ctx)),
		)
	}

	writeJSON(w, httpStatus, resp)
}

// checkPostgres pings the database with a 2s timeout. Returns
// "ok" on success, "down" otherwise. The error itself is not
// surfaced to the response (it could leak the DSN or other
// internals); it is logged at debug level when a logger is
// available.
func (h *HealthHandler) checkPostgres(parent context.Context) string {
	if h.db == nil {
		return "down"
	}
	ctx, cancel := context.WithTimeout(parent, healthCheckTimeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		h.logProbeErr(parent, "scheduler readiness: postgres ping failed", err)
		return "down"
	}
	return "ok"
}

// checkRedis pings Redis with a 2s timeout. The go-redis client
// exposes the Ping error via .Err() so we match the established
// pattern from main.go.
func (h *HealthHandler) checkRedis(parent context.Context) string {
	if h.redis == nil {
		return "down"
	}
	ctx, cancel := context.WithTimeout(parent, healthCheckTimeout)
	defer cancel()
	if err := h.redis.Ping(ctx).Err(); err != nil {
		h.logProbeErr(parent, "scheduler readiness: redis ping failed", err)
		return "down"
	}
	return "ok"
}

// checkFCM invokes the lightweight Ping on the FCM client. When
// the FCM client is nil (the operator has not provisioned
// Firebase credentials) the check reports "skipped" rather than
// "down" — the scheduler is allowed to run without push
// dispatch and a missing FCM client is a known-supported
// configuration, not a degraded state.
func (h *HealthHandler) checkFCM(parent context.Context) string {
	if h.fcm == nil {
		return "skipped"
	}
	ctx, cancel := context.WithTimeout(parent, healthCheckTimeout)
	defer cancel()
	if err := h.fcm.Ping(ctx); err != nil {
		h.logProbeErr(parent, "scheduler readiness: fcm ping failed", err)
		return "down"
	}
	return "ok"
}

// logProbeErr emits a debug-level structured log line for a
// failed probe. The message is intentionally not "error" or
// "warn" because readiness probes are noisy by design — a flaky
// dependency will spam the log at info/warn level. A future
// observability pass may rate-limit the per-probe log; for now
// debug is the right level.
func (h *HealthHandler) logProbeErr(ctx context.Context, msg string, err error) {
	if h.logger == nil {
		return
	}
	h.logger.Inner().DebugContext(ctx, msg,
		slog.String("error", err.Error()),
	)
}

// ---------------------------------------------------------------------------
// GET /metrics — Prometheus scrape endpoint.
// ---------------------------------------------------------------------------

// MetricsHandler returns the http.Handler for the /metrics endpoint.
// It uses the registry passed to NewHealthHandler (typically the
// one passed to scheduler.service.RegisterMetrics). When the
// registry is nil the handler falls back to the Prometheus default
// registry so a misconfigured wiring still produces a working
// /metrics endpoint — even if it returns zero observations.
func (h *HealthHandler) MetricsHandler() http.Handler {
	if h.registry == nil {
		return promhttp.Handler()
	}
	gatherer, ok := h.registry.(prometheus.Gatherer)
	if !ok {
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})
}
