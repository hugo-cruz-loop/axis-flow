// Package handler exposes the Scheduler Service's REST API surface as
// net/http handler functions. The package is intentionally small and
// dependency-light: each handler holds the typed collaborators it needs
// (repositories, runner, logger) and emits the response envelopes
// declared in the Scheduler Service OpenAPI contract
// (docs/services/16_Scheduler_Service_Spec/openapi/scheduler-service.yaml).
//
// This file centralizes the response-envelope helpers shared by every
// handler in the package. There are two envelope shapes:
//
//  1. Success envelope for single-resource responses: {"data": <payload>}.
//  2. Success envelope for paginated list responses:
//     {"data": [...], "pagination": {"page": N, "limit": M, "total": T,
//     "pages": ceil(T/M)}}.
//  3. Error envelope (ErrorEnvelope schema in the OpenAPI):
//     {"error": {"code": "<UPPER_SNAKE>", "message": "<human>"}}.
//
// These shapes are contractual — the OpenAPI file is the source of
// truth and the handlers MUST NOT freelance a different shape. The
// helpers in this file are the only place where the marshaling is
// performed; every handler funnels its responses through them so the
// wire format stays consistent.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/service"
)

// ---------------------------------------------------------------------------
// Envelope types — mirror the OpenAPI schemas verbatim.
// ---------------------------------------------------------------------------

// Pagination is the openapi-shaped pagination object. Mirrors the
// Pagination schema in the OpenAPI.
type Pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
	Pages int `json:"pages"`
}

// ListEnvelope is the {"data": [...], "pagination": ...} shape.
type ListEnvelope struct {
	Data       any         `json:"data"`
	Pagination *Pagination `json:"pagination"`
}

// DataEnvelope is the {"data": <single>} shape.
type DataEnvelope struct {
	Data any `json:"data"`
}

// ErrorBody is the inner {"code":..., "message":...} object.
type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// ErrorDetail is the optional structured validation error.
type ErrorDetail struct {
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ErrorEnvelope is the {"error": {...}} shape returned for non-2xx
// responses. Mirrors the ErrorEnvelope schema in the OpenAPI.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ---------------------------------------------------------------------------
// Stable error codes — match the OpenAPI ErrorEnvelope.code examples.
// ---------------------------------------------------------------------------

const (
	// ErrCodeValidation is returned for 400 responses caused by bad
	// query params or request bodies.
	ErrCodeValidation = "VALIDATION_ERROR"
	// ErrCodeNotFound is returned for 404 responses.
	ErrCodeNotFound = "NOT_FOUND"
	// ErrCodeConflict is returned for 409 responses (e.g. shutting down).
	ErrCodeConflict = "CONFLICT"
	// ErrCodeInternal is returned for 500 responses.
	ErrCodeInternal = "INTERNAL"
)

// ---------------------------------------------------------------------------
// JSON helpers.
// ---------------------------------------------------------------------------

// writeJSON serializes body as JSON with the supplied status code. It
// is the only path through which a handler writes a response. Errors
// from encoding are logged but not returned to the client — by the
// time encoding fails the response has likely already been partially
// flushed and there is no useful recovery.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("scheduler handler: encode response",
			slog.String("error", err.Error()),
		)
	}
}

// writeError writes the standard ErrorEnvelope with the supplied HTTP
// status and error code. The message is safe to expose to API clients
// (no PII, no internal stack traces). Extra details are optional.
func writeError(w http.ResponseWriter, status int, code, message string, details ...ErrorDetail) {
	writeJSON(w, status, ErrorEnvelope{Error: ErrorBody{
		Code:    code,
		Message: message,
		Details: details,
	}})
}

// writeData writes a single-resource success envelope.
func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, DataEnvelope{Data: data})
}

// writeList writes a paginated list envelope. The total/page/limit
// inputs are integer counts; the Pages field is derived as
// ceil(total/limit), or 0 when total is zero (no pages of zero
// results) — and clamped to at least 1 when total > 0 and limit > 0
// so the value is never zero on a non-empty result set.
func writeList(w http.ResponseWriter, status int, data any, page, limit, total int) {
	pages := 0
	if limit > 0 && total > 0 {
		pages = int(math.Ceil(float64(total) / float64(limit)))
	}
	writeJSON(w, status, ListEnvelope{
		Data: data,
		Pagination: &Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
			Pages: pages,
		},
	})
}

// ---------------------------------------------------------------------------
// Domain → response mappers.
// ---------------------------------------------------------------------------

// jobView is the public representation of a scheduler.Job, aligned with
// the OpenAPI "Job" schema. The "job_id" field in the API surface is
// the job_key (e.g. "notificaciones_en_tiempo_real"), NOT the UUID
// primary key — see the file header in jobs_handler.go for the
// rationale.
type jobView struct {
	JobID           string     `json:"job_id"`
	Status          string     `json:"status"`
	Schedule        string     `json:"schedule"`
	LastExecutionAt *time.Time `json:"last_execution_at"`
	NextExecutionAt *time.Time `json:"next_execution_at"`
	Module          string     `json:"module"`
	Description     string     `json:"description"`
}

// toJobView maps a scheduler.Job to its public jobView.
//
// Mapping notes:
//   - job_id      ← JobKey (string, public identifier used in URLs).
//   - status      ← IsActive ? "ACTIVE" : "PAUSED".
//   - schedule    ← CronExpression verbatim, or "@every <N>s" for
//     interval-based jobs.
//   - last_execution_at ← LastRunAt (nil preserved as JSON null).
//   - next_execution_at ← NextRunAt (nil preserved as JSON null).
//   - module      ← Module.
//   - description ← Description (nil → "" so the field is always
//     present; the openapi does not mark it nullable).
func toJobView(j scheduler.Job) jobView {
	status := "PAUSED"
	if j.IsActive {
		status = "ACTIVE"
	}
	schedule := ""
	switch {
	case j.CronExpression != nil:
		schedule = *j.CronExpression
	case j.IntervalSeconds != nil:
		schedule = "@every " + itoa(*j.IntervalSeconds) + "s"
	}
	desc := ""
	if j.Description != nil {
		desc = *j.Description
	}
	return jobView{
		JobID:           j.JobKey,
		Status:          status,
		Schedule:        schedule,
		LastExecutionAt: j.LastRunAt,
		NextExecutionAt: j.NextRunAt,
		Module:          j.Module,
		Description:     desc,
	}
}

// executionView is the public representation of a scheduler.Execution,
// aligned with the OpenAPI "JobExecution" schema. The "job_id" field
// is the job_key (NOT the UUID); the handler resolves the UUID→key
// mapping in a single batched lookup. exception_message and traceback
// are populated only on FAILED executions per the openapi description
// and the task contract.
type executionView struct {
	ExecutionID      string     `json:"execution_id"`
	JobID            string     `json:"job_id"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	DurationSeconds  *float64   `json:"duration_seconds"`
	ExceptionMessage *string    `json:"exception_message,omitempty"`
	Traceback        *string    `json:"traceback,omitempty"`
}

// toExecutionView maps a scheduler.Execution to its public executionView.
// exception_message and traceback are included only when status == FAILED,
// matching the OpenAPI's nullable description. jobKey is supplied by the
// caller (the handler resolves UUID→key in a single batched lookup).
func toExecutionView(e scheduler.Execution, jobKey string) executionView {
	view := executionView{
		ExecutionID:     e.ExecutionID.String(),
		JobID:           jobKey,
		Status:          string(e.Status),
		StartedAt:       e.StartedAt,
		FinishedAt:      e.EndedAt,
		DurationSeconds: e.DurationSeconds,
	}
	if e.Status == scheduler.StatusFailed {
		view.ExceptionMessage = e.ErrorLog
		// The domain does not persist a separate stack trace; traceback
		// is always nil. This matches the openapi description: nullable.
		view.Traceback = nil
	}
	return view
}

// itoa is a tiny strconv-free integer formatter used to keep this
// helper file dependency-light. strconv.Itoa would do, but the only
// call site is the interval-schedule rendering above, so a local
// helper avoids a new import in a frequently-included file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------------------------------------------------------------------------
// Per-request log helpers — shared by all handlers in this package.
// ---------------------------------------------------------------------------

// slogString is a tiny convenience over slog.String used to keep the
// logRequest call sites short.
func slogString(key, value string) slog.Attr {
	return slog.String(key, value)
}

// slogLevelFor returns the slog.Level a request should log at based on
// the response status code: 5xx → Error, 4xx → Warn, anything else →
// Info. PR 4B may swap this for a rate-limiter-aware mapping; for now
// it is the simplest reasonable heuristic.
func slogLevelFor(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// requestIDFromContext resolves the current request's correlation id
// from the context. It defers to the scheduler service's
// TraceIDFromContext (which itself walks OTel span, the explicit
// WithTrace value, and finally returns "") so a value installed by
// the future JWT/request-id middleware in PR 4B is automatically picked
// up here.
func requestIDFromContext(ctx context.Context) string {
	return service.TraceIDFromContext(ctx)
}
