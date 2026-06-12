// trigger_handler.go — REST handler for the manual job trigger endpoint.
//
//   - POST /api/v1/scheduler/jobs/{job_id}/trigger (TriggerHandler.Trigger)
//
// 202 Accepted semantics
// ======================
// Per the OpenAPI contract (TriggerResponse), the endpoint returns
// 202 Accepted with a small acknowledgement body. The actual job
// execution is fired on a fresh goroutine inside
// CronRunner.TriggerNow — the HTTP request lifetime must NOT bound
// the scheduled work. Callers poll GET /api/v1/scheduler/executions
// to observe the resulting execution row.
//
// 404 Not Found
// =============
// The handler resolves the URL job_id (a job_key) via the repository
// before calling the runner. A missing job yields 404; a known job
// whose handler is not registered yields 404 too — the trigger surface
// has no useful work to do for a job_key the runner has never seen.
//
// 409 Conflict (shutting down)
// ============================
// The CronRunner does not currently expose a "shutting down" sentinel;
// the trigger goroutine runs on a context derived from
// context.Background. To keep this handler honest about the runtime
// contract, we surface 409 with code CONFLICT and message "scheduler
// is shutting down" when context.Done() fires between receiving the
// request and queuing the goroutine. In practice the shutdown path is
// managed at the http.Server level (graceful drain in PR 4B) and this
// branch is rarely hit — but the contract is documented here so the
// OpenAPI and the handler agree.
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/repository"
	"axis-flow-back/internal/scheduler/service"

	"github.com/go-chi/chi/v5"
)

// ---------------------------------------------------------------------------
// TriggerHandler dependencies.
// ---------------------------------------------------------------------------

// TriggerHandlerDeps groups the collaborators TriggerHandler needs.
type TriggerHandlerDeps struct {
	JobRepo repository.JobRepository
	Runner  Runner
	Logger  *service.Logger
}

// TriggerHandler is the HTTP handler for POST /scheduler/jobs/{id}/trigger.
type TriggerHandler struct {
	jobRepo repository.JobRepository
	runner  Runner
	logger  *service.Logger
}

// NewTriggerHandler builds a TriggerHandler. See JobsHandlerDeps for
// the nil-collaborator contract.
func NewTriggerHandler(deps TriggerHandlerDeps) *TriggerHandler {
	return &TriggerHandler{
		jobRepo: deps.JobRepo,
		runner:  deps.Runner,
		logger:  deps.Logger,
	}
}

// ---------------------------------------------------------------------------
// POST /api/v1/scheduler/jobs/{job_id}/trigger
// ---------------------------------------------------------------------------

// triggerView is the public representation of a successful trigger
// acknowledgement, aligned with the OpenAPI TriggerResponse schema
// (the {"data": {...}} envelope is added by the writeData helper).
type triggerView struct {
	JobID       string    `json:"job_id"`
	TriggeredAt time.Time `json:"triggered_at"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
}

// Trigger handles POST /api/v1/scheduler/jobs/{job_id}/trigger. The
// handler:
//
//  1. Extracts the job_id (job_key) from the URL.
//  2. Verifies the job exists in the database (404 otherwise).
//  3. Checks the request context for an early cancellation and emits
//     409 if the server is shutting down.
//  4. Calls CronRunner.TriggerNow — this returns nil immediately and
//     runs the actual job in a detached goroutine.
//  5. Returns 202 Accepted with a TriggerResponse-shaped body.
//
// The TriggerResponse status field is "TRIGGERED" on success (the
// OpenAPI example value); the message field is fixed copy that tells
// the caller the run is async.
func (h *TriggerHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	jobKey := chi.URLParam(r, "job_id")
	if jobKey == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "job_id is required")
		h.logTrigger(ctx, jobKey, "validation_error", "missing job_id", time.Since(start), http.StatusBadRequest)
		return
	}

	// Verify the job exists. We use GetByKey (not GetByID) because the
	// URL parameter is the public job_key string, not a UUID.
	if _, err := h.jobRepo.GetByKey(ctx, jobKey); err != nil {
		if errors.Is(err, scheduler.ErrNotFound) {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "job not found")
			h.logTrigger(ctx, jobKey, "not_found", jobKey, time.Since(start), http.StatusNotFound)
			return
		}
		h.logTrigger(ctx, jobKey, "internal_error", err.Error(), time.Since(start), http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to load job")
		return
	}

	// Early-cancellation check: if the server is in the middle of a
	// graceful shutdown, the request context is already cancelled and
	// we should refuse to accept more work. There is no
	// ErrShuttingDown sentinel in the runner today, so this branch
	// documents the intended 409 semantics — PR 4B will surface the
	// same shape from the lifecycle plumbing.
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			writeError(w, http.StatusConflict, ErrCodeConflict, "scheduler is shutting down")
			h.logTrigger(ctx, jobKey, "shutting_down", jobKey, time.Since(start), http.StatusConflict)
			return
		}
	}

	if err := h.runner.TriggerNow(ctx, jobKey); err != nil {
		if errors.Is(err, service.ErrNoHandler) || errors.Is(err, scheduler.ErrNotFound) {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "job not found")
			h.logTrigger(ctx, jobKey, "not_found", jobKey, time.Since(start), http.StatusNotFound)
			return
		}
		h.logTrigger(ctx, jobKey, "internal_error", err.Error(), time.Since(start), http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to trigger job")
		return
	}

	h.logTrigger(ctx, jobKey, "ok", jobKey, time.Since(start), http.StatusAccepted)
	writeData(w, http.StatusAccepted, triggerView{
		JobID:       jobKey,
		TriggeredAt: time.Now().UTC(),
		Status:      "TRIGGERED",
		Message:     "Job triggered successfully in the background.",
	})
}

// logTrigger emits the per-request log line for the trigger handler.
func (h *TriggerHandler) logTrigger(ctx context.Context, jobKey, outcome, detail string, dur time.Duration, status int) {
	if h.logger == nil {
		return
	}
	h.logger.Inner().LogAttrs(ctx, slogLevelFor(status),
		"jobs.trigger",
		slog.String("status", strconv.Itoa(status)),
		slog.String("duration_ms", strconv.FormatInt(dur.Milliseconds(), 10)),
		slog.String("outcome", outcome),
		slog.String("request_id", requestIDFromContext(ctx)),
		slog.String("job_id", jobKey),
		slog.String("detail", detail),
	)
}
