// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the logfmt structured logger required by the
// Observabilidad > Structured Logging section of the spec. It emits
// lifecycle events ("Starting scheduled job execution", "Scheduled job
// execution completed successfully") and supports trace_id propagation
// across the request scope.
package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"axis-flow-back/internal/logging"
	"axis-flow-back/internal/middleware"

	"go.opentelemetry.io/otel/trace"
)

// ---------------------------------------------------------------------------
// Lifecycle log message constants — match the spec verbatim.
// ---------------------------------------------------------------------------

const (
	// MsgJobStart is emitted at the beginning of every scheduled execution.
	MsgJobStart = "Starting scheduled job execution"
	// MsgJobSuccess is emitted when a scheduled execution completes without
	// error and carries duration_ms and processed_records attributes.
	MsgJobSuccess = "Scheduled job execution completed successfully"
	// MsgJobFailure is emitted when a scheduled execution terminates with an
	// error and carries duration_ms and the error class.
	MsgJobFailure = "Scheduled job execution failed"
	// MsgJobSkippedLock is emitted when a lock collision prevents execution.
	MsgJobSkippedLock = "Job execution skipped due to lock collision"
)

// ---------------------------------------------------------------------------
// LifecycleEvent — typed arguments for the structured log emitters.
// ---------------------------------------------------------------------------

// JobStartEvent is the input for the MsgJobStart log.
type JobStartEvent struct {
	JobID   string
	TraceID string
}

// JobSuccessEvent is the input for the MsgJobSuccess log.
type JobSuccessEvent struct {
	JobID            string
	TraceID          string
	DurationMillis   int64
	ProcessedRecords int
}

// JobFailureEvent is the input for the MsgJobFailure log.
type JobFailureEvent struct {
	JobID          string
	TraceID        string
	DurationMillis int64
	ErrorClass     string
	Err            error
}

// JobSkippedLockEvent is the input for the MsgJobSkippedLock log.
type JobSkippedLockEvent struct {
	JobID   string
	TraceID string
}

// ---------------------------------------------------------------------------
// Logger — thin wrapper around *slog.Logger that emits the lifecycle events
// described in the Scheduler Service specification.
// ---------------------------------------------------------------------------

// Logger emits the structured logfmt lifecycle events required by the
// observability contract. It is safe to share across goroutines.
type Logger struct {
	log *slog.Logger
}

// NewLogger creates a Logger that writes to stdout in the requested format.
// Supported formats: "logfmt" (default), "json". The level argument maps
// directly to slog.Level* constants via the package-level ParseSlogLevel
// helper in the middleware package.
func NewLogger(format string, level slog.Level) *Logger {
	return NewLoggerWithWriter(os.Stdout, format, level)
}

// NewLoggerWithWriter creates a Logger that writes to the provided writer.
// It is primarily useful for tests and file-backed log targets.
func NewLoggerWithWriter(w io.Writer, format string, level slog.Level) *Logger {
	return &Logger{log: logging.New(w, format, level)}
}

// With returns a Logger that has the given attributes attached to every
// emitted record. Use this to add fixed identity attributes such as
// replica_id or job_id.
func (l *Logger) With(attrs ...slog.Attr) *Logger {
	asAny := make([]any, len(attrs))
	for i, a := range attrs {
		asAny[i] = a
	}
	return &Logger{log: l.log.With(asAny...)}
}

// Inner returns the underlying *slog.Logger for callers that need to emit
// arbitrary structured records outside the lifecycle contract.
func (l *Logger) Inner() *slog.Logger { return l.log }

// ---------------------------------------------------------------------------
// Lifecycle emitters.
// ---------------------------------------------------------------------------

// JobStart emits the "Starting scheduled job execution" log.
func (l *Logger) JobStart(ctx context.Context, ev JobStartEvent) {
	traceID := ev.TraceID
	if traceID == "" {
		traceID = TraceIDFromContext(ctx)
	}
	l.log.LogAttrs(ctx, slog.LevelInfo, MsgJobStart,
		slog.String("job_id", ev.JobID),
		slog.String("trace_id", traceID),
	)
}

// JobSuccess emits the "Scheduled job execution completed successfully" log.
func (l *Logger) JobSuccess(ctx context.Context, ev JobSuccessEvent) {
	traceID := ev.TraceID
	if traceID == "" {
		traceID = TraceIDFromContext(ctx)
	}
	l.log.LogAttrs(ctx, slog.LevelInfo, MsgJobSuccess,
		slog.String("job_id", ev.JobID),
		slog.Int64("duration_ms", ev.DurationMillis),
		slog.Int("processed_records", ev.ProcessedRecords),
		slog.String("trace_id", traceID),
	)
}

// JobFailure emits the "Scheduled job execution failed" log. The err is
// recorded as an attribute but its message is not duplicated into the
// record message.
func (l *Logger) JobFailure(ctx context.Context, ev JobFailureEvent) {
	traceID := ev.TraceID
	if traceID == "" {
		traceID = TraceIDFromContext(ctx)
	}
	errClass := ev.ErrorClass
	if errClass == "" && ev.Err != nil {
		errClass = fmt.Sprintf("%T", ev.Err)
	}
	attrs := []slog.Attr{
		slog.String("job_id", ev.JobID),
		slog.Int64("duration_ms", ev.DurationMillis),
		slog.String("error_class", errClass),
		slog.String("trace_id", traceID),
	}
	if ev.Err != nil {
		attrs = append(attrs, slog.String("error", ev.Err.Error()))
	}
	l.log.LogAttrs(ctx, slog.LevelError, MsgJobFailure, attrs...)
}

// JobSkippedLock emits the "Job execution skipped due to lock collision" log.
func (l *Logger) JobSkippedLock(ctx context.Context, ev JobSkippedLockEvent) {
	traceID := ev.TraceID
	if traceID == "" {
		traceID = TraceIDFromContext(ctx)
	}
	l.log.LogAttrs(ctx, slog.LevelWarn, MsgJobSkippedLock,
		slog.String("job_id", ev.JobID),
		slog.String("trace_id", traceID),
	)
}

// ---------------------------------------------------------------------------
// Trace helpers.
// ---------------------------------------------------------------------------

// traceContextKey is the context key under which a derived trace_id is
// stored when WithTrace is called.
type traceContextKey struct{}

// WithTrace returns a derived context that carries the given trace_id
// alongside any OTel span already present. The stored value is consumed
// by TraceIDFromContext and by the lifecycle emitters when no explicit
// trace_id is provided.
func WithTrace(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, traceContextKey{}, traceID)
}

// TraceIDFromContext resolves the trace_id using the following precedence:
//  1. The explicit value stored via WithTrace (used by the cron engine when
//     it needs to backfill a trace id outside the HTTP middleware path).
//  2. The active OTel span trace id (production HTTP path).
//  3. An empty string when neither is available.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceContextKey{}).(string); ok && v != "" {
		return v
	}
	if id := middleware.TraceIDFromContext(ctx); id != "" {
		return id
	}
	if spanCtx := trace.SpanFromContext(ctx).SpanContext(); spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	return ""
}

// MillisSince returns the elapsed milliseconds between start and the call
// time. It is a small helper used by job implementations to populate the
// duration_ms attribute on success and failure logs.
func MillisSince(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
