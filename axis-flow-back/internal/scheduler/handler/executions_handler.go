// executions_handler.go — REST handler for the historical executions
// listing endpoint.
//
//   - GET /api/v1/scheduler/executions (ExecutionsHandler.List)
//
// job_id filter resolution
// ========================
// The OpenAPI defines the `job_id` query parameter as a string with
// example "notificaciones_en_tiempo_real". In the domain model that
// string is a JobKey; the executions table stores the JobID (UUID).
// The handler therefore performs ONE jobRepo.GetByKey call when the
// filter is present, then threads the resolved UUID into the
// ExecutionListFilter. A non-existent key produces 400 VALIDATION_ERROR
// (the filter names a job that does not exist) — not 404, because
// 404 in this API is reserved for the resource being listed.
//
// status filter
// =============
// The handler validates the status query param against the
// scheduler.ExecutionStatus enum (RUNNING, SUCCESS, FAILED). Anything
// else yields 400.
//
// job_id resolution in the response
// =================================
// The OpenAPI's "JobExecution.job_id" example is the job_key, not the
// UUID. To honour that without an N+1 lookup the handler does a
// single JobRepository.ListActive() call, builds a
// map[uuid.UUID]string of UUID→key, and uses it to render the
// response. Jobs that were hard-deleted (rare, soft-delete is the
// project policy) will fall back to the UUID string and the
// response will still be valid JSON; this is documented here and
// tolerated by the contract.
//
// exception_message
// =================
// The openapi marks this field nullable; the spec contract in the PR
// 4A task description is explicit that the field is only present on
// FAILED executions. The toExecutionView helper enforces that.
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

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Defaults and bounds for pagination query parameters.
// ---------------------------------------------------------------------------

const (
	defaultExecPage  = 1
	defaultExecLimit = 20
	maxExecLimit     = 100
)

// ---------------------------------------------------------------------------
// ExecutionsHandler dependencies.
// ---------------------------------------------------------------------------

// ExecutionsHandlerDeps groups the collaborators ExecutionsHandler needs.
type ExecutionsHandlerDeps struct {
	ExecRepo repository.ExecutionRepository
	JobRepo  repository.JobRepository
	Logger   *service.Logger
}

// ExecutionsHandler is the HTTP handler for GET /scheduler/executions.
type ExecutionsHandler struct {
	execRepo repository.ExecutionRepository
	jobRepo  repository.JobRepository
	logger   *service.Logger
}

// NewExecutionsHandler builds an ExecutionsHandler.
func NewExecutionsHandler(deps ExecutionsHandlerDeps) *ExecutionsHandler {
	return &ExecutionsHandler{
		execRepo: deps.ExecRepo,
		jobRepo:  deps.JobRepo,
		logger:   deps.Logger,
	}
}

// ---------------------------------------------------------------------------
// GET /api/v1/scheduler/executions
// ---------------------------------------------------------------------------

// List handles GET /api/v1/scheduler/executions. Query params:
//
//	job_id    — optional job_key string filter.
//	status    — optional, must be RUNNING | SUCCESS | FAILED.
//	from      — optional RFC3339 timestamp; lower bound on started_at.
//	to        — optional RFC3339 timestamp; upper bound on started_at.
//	page      — 1-indexed page number (default 1, min 1).
//	limit     — items per page (default 20, max 100).
//
// On success returns 200 with the ExecutionListResponse envelope.
// Bad query params produce 400 with the ErrorEnvelope; repo errors
// produce 500.
func (h *ExecutionsHandler) List(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	page, limit, perr := parsePagination(r, defaultExecPage, defaultExecLimit, maxExecLimit)
	if perr != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, perr.Error())
		h.logExec(ctx, "executions.list", "validation_error", perr.Error(), time.Since(start), http.StatusBadRequest)
		return
	}

	filter, ferr := h.buildFilter(ctx, r, page, limit)
	if ferr != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, ferr.Error())
		h.logExec(ctx, "executions.list", "validation_error", ferr.Error(), time.Since(start), http.StatusBadRequest)
		return
	}

	execs, total, err := h.execRepo.List(ctx, filter)
	if err != nil {
		h.logExec(ctx, "executions.list", "internal_error", err.Error(), time.Since(start), http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to list executions")
		return
	}

	// Resolve UUID→key for the response rendering. See file header.
	keyByID, kerr := h.buildJobKeyMap(ctx)
	if kerr != nil {
		h.logExec(ctx, "executions.list", "internal_error", kerr.Error(), time.Since(start), http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to resolve job keys")
		return
	}

	views := make([]executionView, 0, len(execs))
	for _, e := range execs {
		key, ok := keyByID[e.JobID]
		if !ok {
			// Fall back to the UUID string when a job is missing from
			// the active-jobs snapshot. The response is still valid;
			// the contract tolerates a UUID here.
			key = e.JobID.String()
		}
		views = append(views, toExecutionView(e, key))
	}

	h.logExec(ctx, "executions.list", "ok", "", time.Since(start), http.StatusOK)
	writeList(w, http.StatusOK, views, page, limit, total)
}

// ---------------------------------------------------------------------------
// Filter construction.
// ---------------------------------------------------------------------------

// buildFilter parses and validates the optional query parameters and
// returns an ExecutionListFilter. The filter is built in terms of
// the domain types — the handler is the only place that maps the
// string-shaped wire format to the typed domain.
//
// page and limit are supplied by the caller (parsed once in List) so
// we never run parsePagination twice for the same request.
func (h *ExecutionsHandler) buildFilter(ctx context.Context, r *http.Request, page, limit int) (scheduler.ExecutionListFilter, error) {
	q := r.URL.Query()

	filter := scheduler.ExecutionListFilter{
		Limit:  limit,
		Offset: (page - 1) * limit,
	}

	if raw := q.Get("job_id"); raw != "" {
		job, err := h.jobRepo.GetByKey(ctx, raw)
		if err != nil {
			if errors.Is(err, scheduler.ErrNotFound) {
				return scheduler.ExecutionListFilter{}, &badParamError{Field: "job_id", Reason: "unknown job_key"}
			}
			return scheduler.ExecutionListFilter{}, err
		}
		jobID := job.JobID
		filter.JobID = &jobID
	}

	if raw := q.Get("status"); raw != "" {
		st := scheduler.ExecutionStatus(raw)
		if !st.Valid() {
			return scheduler.ExecutionListFilter{}, &badParamError{Field: "status", Reason: "must be RUNNING, SUCCESS, or FAILED"}
		}
		filter.Status = &st
	}

	if raw := q.Get("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return scheduler.ExecutionListFilter{}, &badParamError{Field: "from", Reason: "must be an RFC3339 timestamp"}
		}
		filter.StartDate = &t
	}

	if raw := q.Get("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return scheduler.ExecutionListFilter{}, &badParamError{Field: "to", Reason: "must be an RFC3339 timestamp"}
		}
		filter.EndDate = &t
	}

	return filter, nil
}

// buildJobKeyMap returns a map[uuid.UUID]string of every active job's
// UUID → job_key. Used to render the executions response without
// performing an N+1 lookup. Best-effort: a hard-deleted job simply
// won't appear in the map and the response will fall back to the
// UUID string (see file header).
func (h *ExecutionsHandler) buildJobKeyMap(ctx context.Context) (map[uuid.UUID]string, error) {
	jobs, err := h.jobRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(jobs))
	for _, j := range jobs {
		out[j.JobID] = j.JobKey
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Per-request log emitter.
// ---------------------------------------------------------------------------

// logExec emits the per-request log line for the executions handler.
func (h *ExecutionsHandler) logExec(ctx context.Context, op, outcome, detail string, dur time.Duration, status int) {
	if h.logger == nil {
		return
	}
	h.logger.Inner().LogAttrs(ctx, slogLevelFor(status),
		op,
		slog.String("status", strconv.Itoa(status)),
		slog.String("duration_ms", strconv.FormatInt(dur.Milliseconds(), 10)),
		slog.String("outcome", outcome),
		slog.String("request_id", requestIDFromContext(ctx)),
		slog.String("detail", detail),
	)
}
