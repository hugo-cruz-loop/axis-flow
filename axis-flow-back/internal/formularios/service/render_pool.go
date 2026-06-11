// Package service — render_pool.go: the bounded worker pool
// (Celery replacement) for the PDF service.
//
// PR-6 (6.3) implementation. The spec calls for
// "Celery max(1, num_cores-1)" concurrent render workers. The
// Go equivalent is a buffered channel semaphore: the channel's
// capacity is the worker count; every Execute acquires a slot
// (blocks when the pool is full) and releases it on completion.
//
// The pool is intentionally minimal: it does not own a goroutine
// pool (no fixed workers); it relies on the caller to invoke
// Execute from a goroutine. This matches the
// "blocking channel" idiom and avoids the complexity of a
// dedicated dispatcher goroutine.
package service

import (
	"context"
	"errors"
	"runtime"
)

// ---------------------------------------------------------------------------
// RenderPool
// ---------------------------------------------------------------------------

// RenderPool is a bounded worker pool that uses a buffered
// channel as a semaphore. Each Execute blocks until a slot is
// available, then runs fn, then releases the slot.
type RenderPool struct {
	sem chan struct{}
}

// NewRenderPool constructs a RenderPool with the given max
// concurrency. A non-positive maxConc is floored at 1 (a 0-size
// pool would deadlock every Execute — see the
// TestRenderPool_DefaultSizeIsAtLeastOne test).
func NewRenderPool(maxConc int) *RenderPool {
	if maxConc < 1 {
		maxConc = 1
	}
	return &RenderPool{sem: make(chan struct{}, maxConc)}
}

// NewRenderPoolDefault constructs a RenderPool with the
// "max(1, NumCPU-1)" default the spec calls for. PR-6 6.3
// translates "Celery max(1, num_cores-1)" to Go's
// "max(1, runtime.NumCPU()-1)".
func NewRenderPoolDefault() *RenderPool {
	n := runtime.NumCPU() - 1
	if n < 1 {
		n = 1
	}
	return NewRenderPool(n)
}

// Execute runs fn with the semaphore held, blocking when the
// pool is full. Honors ctx cancellation: if the context is
// cancelled while waiting for a slot, Execute returns
// ctx.Err() WITHOUT running fn. If fn returns an error, Execute
// returns that error (the slot is still released).
func (p *RenderPool) Execute(ctx context.Context, fn func() error) error {
	if fn == nil {
		return errors.New("renderpool: nil fn")
	}

	// Acquire a slot, honoring ctx cancellation. The select
	// blocks until ONE of: the pool is not full (we send a slot
	// marker), the context is cancelled, OR the wait context
	// times out (not currently exposed in the public API but
	// the select's third case is a defensive default).
	select {
	case p.sem <- struct{}{}:
		// got a slot — release it when we're done.
		defer func() { <-p.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	return fn()
}

// Size returns the current max concurrency. Useful for metrics
// labels.
func (p *RenderPool) Size() int { return cap(p.sem) }
