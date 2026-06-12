// Package jobs holds the concrete business scheduled jobs the
// Scheduler service runs on a cron cadence.
//
// This file defines the tiny shared helper `runInBatches` used by
// the maintenance jobs (PR 3C) to drain a "work backlog" table in
// capped batches. The helper is intentionally minimal — the two
// jobs that consume it (`cleanup_expired_sessions.go` and
// `close_lapsed_assignments.go`) are the only call sites today,
// and the helper is shaped to fit them exactly.
//
// Why a helper
// ============
// Both maintenance jobs are "drain stale rows" jobs:
//
//   - cleanup_expired_sessions: every device row whose expires_at
//     is in the past and that is still active must be flipped to
//     is_active = false.
//   - close_lapsed_assignments: every assignment row whose
//     fecha_fin is in the past and that is still active must be
//     flipped to a closed status.
//
// The natural implementation is a batched UPDATE loop:
//
//	for {
//	    rows, err := deactivateOneBatch(ctx, batchSize)
//	    if err != nil { ... }
//	    if rows == 0 { break }
//	    total += rows
//	}
//
// The same shape appears in both files. Extracting it into a
// 20-line helper keeps the job files focused on the SQL and the
// per-batch logging, and it gives us a single place to wire the
// context-cancellation short-circuit and the per-iteration
// observability hook (if we ever want one).
//
// Non-goals
// =========
// The helper does NOT own:
//
//   - The SQL. Each job passes its own `processBatch` closure that
//     performs the UPDATE and returns the affected-row count.
//   - The metrics. Per-batch metrics (when added) are emitted
//     from inside `processBatch` so the call site owns the
//     labelling.
//   - The locks. The CronRunner holds the per-job distributed
//     lock for the entire Run; the helper is single-replica by
//     construction.
//
// Generic type parameters are intentionally NOT used: the only
// value that flows through the loop is the int64 affected-row
// count. Adding generics here would cost more in cognitive load
// than it saves in lines of code.
package jobs

import "context"

// runInBatches repeatedly invokes `processBatch` until it returns
// zero affected rows or the supplied context is cancelled. The
// return value is the cumulative affected-row count across every
// successful iteration of the loop.
//
// Contract
// ========
//   - `batchSize` is unused by the helper itself. The caller
//     passes it through to `processBatch` (typically as a
//     LIMIT $N inside a prepared statement) so the per-iteration
//     row count is bounded. The parameter is part of the helper
//     signature for two reasons: (1) it documents the call
//     pattern, and (2) it lets future iterations add a
//     configurable loop bound (max iterations) without changing
//     the call sites again.
//   - `processBatch` MUST be idempotent and MUST return the
//     number of rows it affected. A return value of 0 terminates
//     the loop — the helper assumes the work is exhausted when
//     one batch comes back empty.
//   - `processBatch` MUST honour ctx cancellation: when ctx is
//     done, the helper returns whatever total it had accumulated
//     and the underlying ctx.Err() wrapped error.
//   - A non-nil error from `processBatch` terminates the loop
//     immediately and the error is returned wrapped with %w so
//     the caller's logging chain stays intact.
//
// Why a closure instead of a channel or a generic batch struct
// ------------------------------------------------------------
// The two call sites each have one or two context-specific
// parameters to thread through to the per-batch SQL (the cutoff
// timestamp, the batch size). A closure captures them naturally
// without a dedicated struct. The cost is one heap-allocated
// closure per Run, which is negligible compared to the database
// round-trips.
func runInBatches(
	ctx context.Context,
	batchSize int,
	processBatch func(ctx context.Context) (int64, error),
) (int64, error) {
	_ = batchSize // documented in the contract; the closure consumes it.
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		affected, err := processBatch(ctx)
		if err != nil {
			return total, err
		}
		total += affected
		if affected == 0 {
			return total, nil
		}
	}
}
