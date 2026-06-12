// jobs_handler.go — REST handlers for the scheduler admin surface:
//
//   - GET    /api/v1/scheduler/jobs            (JobsHandler.List)
//   - PATCH  /api/v1/scheduler/jobs/{id}/pause (JobsHandler.Pause)
//
// Wiring is performed in PR 4B (cmd/server/scheduler_routes.go). The
// handlers in this file are pure net/http and can be unit-tested with
// httptest.NewRecorder without a router.
//
// # job_id identifier contract
//
// The OpenAPI contract uses the string "job_id" everywhere (path
// parameters, response fields, and query parameters). The example
// given is "notificaciones_en_tiempo_real", which is the job_key
// (public string identifier) — NOT the UUID primary key stored in
// scheduler.scheduler_jobs.job_id. The mapping rule is therefore:
//
//	API job_id  ⇆  domain scheduler.Job.JobKey   (public string)
//	DB job_id   ⇆  domain scheduler.Job.JobID    (UUID, never exposed)
//
// This handler treats the URL parameter as a job_key and resolves it
// via JobRepository.GetByKey. The internal UUID is used only for
// UPDATE statements (UpdatePause).
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/repository"
	"axis-flow-back/internal/scheduler/service"

	"github.com/go-chi/chi/v5"
)

// ---------------------------------------------------------------------------
// Defaults and bounds for pagination query parameters.
// ---------------------------------------------------------------------------

const (
	defaultJobsPage  = 1
	defaultJobsLimit = 20
	maxJobsLimit     = 100
)

// ---------------------------------------------------------------------------
// JobsHandler dependencies.
// ---------------------------------------------------------------------------

// JobsHandlerDeps groups the collaborators JobsHandler needs. Tests
// can supply fakes for any of these without touching the constructor.
type JobsHandlerDeps struct {
	JobRepo repository.JobRepository
	Runner  Runner
	Logger  *service.Logger
}

// JobsHandler is the HTTP handler for the /scheduler/jobs admin
// surface. It is safe to share across goroutines — every collaborator
// it holds is itself safe for concurrent use.
type JobsHandler struct {
	jobRepo repository.JobRepository
	runner  Runner
	logger  *service.Logger
}

// NewJobsHandler builds a JobsHandler. All dependencies are required;
// passing a nil collaborator yields a handler whose calls will panic
// at the first request (intentional — wiring bugs should fail fast,
// not silently).
func NewJobsHandler(deps JobsHandlerDeps) *JobsHandler {
	return &JobsHandler{
		jobRepo: deps.JobRepo,
		runner:  deps.Runner,
		logger:  deps.Logger,
	}
}

// ---------------------------------------------------------------------------
// GET /api/v1/scheduler/jobs
// ---------------------------------------------------------------------------

// List handles GET /api/v1/scheduler/jobs. Query params:
//
//	page   — 1-indexed page number (default 1, min 1).
//	limit  — items per page (default 20, max 100).
//
// Returns 200 with the JobListResponse envelope on success. Bad query
// params produce 400 with the ErrorEnvelope; repo errors produce 500.
func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	page, limit, perr := parsePagination(r, defaultJobsPage, defaultJobsLimit, maxJobsLimit)
	if perr != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, perr.Error())
		h.logRequest(r.Context(), "jobs.list", "validation_error", perr.Error(), time.Since(start), http.StatusBadRequest)
		return
	}

	jobs, total, err := h.jobRepo.List(ctx, scheduler.JobListFilter{
		Limit:  limit,
		Offset: (page - 1) * limit,
	})
	if err != nil {
		h.logRequest(ctx, "jobs.list", "internal_error", err.Error(), time.Since(start), http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to list jobs")
		return
	}

	views := make([]jobView, 0, len(jobs))
	for _, j := range jobs {
		views = append(views, toJobView(j))
	}

	h.logRequest(ctx, "jobs.list", "ok", "", time.Since(start), http.StatusOK)
	writeList(w, http.StatusOK, views, page, limit, total)
}

// ---------------------------------------------------------------------------
// PATCH /api/v1/scheduler/jobs/{job_id}/pause
// ---------------------------------------------------------------------------

// pauseRequest is the JSON body for PATCH .../pause.
type pauseRequest struct {
	Paused *bool `json:"paused"`
}

// Pause handles PATCH /api/v1/scheduler/jobs/{job_id}/pause. The
// request body is `{"paused": true|false}`. The handler delegates the
// actual pause/resume to the CronRunner, then returns the freshly
// updated job row from the database so the caller sees the new state.
//
// Status codes:
//   - 200 OK     — paused/resumed; body is the JobResponse envelope.
//   - 400        — body missing, malformed, or "paused" not a bool.
//   - 404        — job_key unknown.
//   - 500        — internal error.
func (h *JobsHandler) Pause(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	jobKey := chi.URLParam(r, "job_id")
	if jobKey == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "job_id is required")
		h.logRequest(ctx, "jobs.pause", "validation_error", "missing job_id", time.Since(start), http.StatusBadRequest)
		return
	}

	// We need the JobID (UUID) for the Pause/Resume calls and the
	// post-update GetByID fetch. Resolve it once.
	job, err := h.jobRepo.GetByKey(ctx, jobKey)
	if err != nil {
		if errors.Is(err, scheduler.ErrNotFound) {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "job not found")
			h.logRequest(ctx, "jobs.pause", "not_found", jobKey, time.Since(start), http.StatusNotFound)
			return
		}
		h.logRequest(ctx, "jobs.pause", "internal_error", err.Error(), time.Since(start), http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to load job")
		return
	}

	var req pauseRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if derr := dec.Decode(&req); derr != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+derr.Error(),
			ErrorDetail{Field: "body", Reason: "must be JSON {\"paused\": <bool>}"})
		h.logRequest(ctx, "jobs.pause", "validation_error", derr.Error(), time.Since(start), http.StatusBadRequest)
		return
	}
	if req.Paused == nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "paused is required",
			ErrorDetail{Field: "paused", Reason: "must be a boolean value"})
		h.logRequest(ctx, "jobs.pause", "validation_error", "paused is nil", time.Since(start), http.StatusBadRequest)
		return
	}

	if *req.Paused {
		if rerr := h.runner.Pause(ctx, jobKey); rerr != nil {
			if errors.Is(rerr, service.ErrNoHandler) || errors.Is(rerr, scheduler.ErrNotFound) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "job not found")
				h.logRequest(ctx, "jobs.pause", "not_found", jobKey, time.Since(start), http.StatusNotFound)
				return
			}
			h.logRequest(ctx, "jobs.pause", "internal_error", rerr.Error(), time.Since(start), http.StatusInternalServerError)
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to pause job")
			return
		}
	} else {
		if rerr := h.runner.Resume(ctx, jobKey); rerr != nil {
			if errors.Is(rerr, service.ErrNoHandler) || errors.Is(rerr, scheduler.ErrNotFound) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "job not found")
				h.logRequest(ctx, "jobs.pause", "not_found", jobKey, time.Since(start), http.StatusNotFound)
				return
			}
			h.logRequest(ctx, "jobs.pause", "internal_error", rerr.Error(), time.Since(start), http.StatusInternalServerError)
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to resume job")
			return
		}
	}

	// Re-fetch to return the post-update state.
	updated, err := h.jobRepo.GetByID(ctx, job.JobID)
	if err != nil {
		// Even on a refetch failure the underlying state change has
		// already been applied, so log and return a generic 500.
		h.logRequest(ctx, "jobs.pause", "internal_error", err.Error(), time.Since(start), http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to load updated job")
		return
	}

	h.logRequest(ctx, "jobs.pause", "ok", jobKey, time.Since(start), http.StatusOK)
	writeData(w, http.StatusOK, toJobView(updated))
}

// ---------------------------------------------------------------------------
// Shared helpers.
// ---------------------------------------------------------------------------

// parsePagination reads the page/limit query parameters and applies
// defaults and bounds. It returns validation errors as Go errors so
// the caller can decide the response shape.
func parsePagination(r *http.Request, defaultPage, defaultLimit, maxLimit int) (int, int, error) {
	q := r.URL.Query()
	page := defaultPage
	if raw := q.Get("page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			return 0, 0, &badParamError{Field: "page", Reason: "must be an integer >= 1"}
		}
		page = v
	}
	limit := defaultLimit
	if raw := q.Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			return 0, 0, &badParamError{Field: "limit", Reason: "must be an integer >= 1"}
		}
		if v > maxLimit {
			return 0, 0, &badParamError{Field: "limit", Reason: "must be <= " + strconv.Itoa(maxLimit)}
		}
		limit = v
	}
	return page, limit, nil
}

// badParamError is the local error type produced by parsePagination.
type badParamError struct {
	Field  string
	Reason string
}

func (e *badParamError) Error() string {
	return e.Field + ": " + e.Reason
}

// logRequest emits the per-request access log line. Format mirrors the
// spec's logfmt style:
//
//	msg="<op>" status=<code> duration_ms=<n> outcome=<ok|...> request_id=<id> job_id=<id>
//
// Logger may be nil (e.g. in unit tests with no logger wired) — in
// that case the call is a no-op.
func (h *JobsHandler) logRequest(ctx context.Context, op, outcome, detail string, dur time.Duration, status int) {
	if h.logger == nil {
		return
	}
	h.logger.Inner().LogAttrs(ctx, slogLevelFor(status),
		op,
		slogString("status", strconv.Itoa(status)),
		slogString("duration_ms", strconv.FormatInt(dur.Milliseconds(), 10)),
		slogString("outcome", outcome),
		slogString("request_id", requestIDFromContext(ctx)),
		slogString("job_id", detail),
	)
}
