// runner.go — the small interface surface the admin REST handlers
// depend on for the cron runner.
//
// Why an interface?
// =================
// JobsHandler (Pause) and TriggerHandler (Trigger) need to call a
// tiny subset of *service.CronRunner:
//
//   - Pause(ctx, jobKey)     — invoked by PATCH .../pause with paused=true
//   - Resume(ctx, jobKey)    — invoked by PATCH .../pause with paused=false
//   - TriggerNow(ctx, jobKey) — invoked by POST .../trigger
//
// The concrete *service.CronRunner satisfies this interface
// structurally, so production wiring (cmd/server/scheduler_routes.go)
// continues to work without any changes — it passes a
// *service.CronRunner and the field is satisfied by Go's structural
// typing.
//
// Tests in handler_test (PR 5B-i) substitute a testify/mock-based
// fake, which lets the HTTP layer be exercised end-to-end without
// standing up a real cron runner, a real Redis lock, or a real
// database. The full runner lifecycle (lock, heartbeat, lifecycle
// wrapper) is covered separately in service/cron_runner_test.go
// (PR 5B-i task 5.3) — the handlers do not need to re-test that
// flow, they just need to know the runner method returned the
// right sentinel or nil.
package handler

import "context"

// Runner is the minimal surface the admin REST handlers need from
// the cron runner. *service.CronRunner satisfies it implicitly.
type Runner interface {
	// Pause removes the cron entry for jobKey and marks the row
	// inactive in the database.
	Pause(ctx context.Context, jobKey string) error
	// Resume re-binds the cron entry for jobKey and marks the row
	// active in the database.
	Resume(ctx context.Context, jobKey string) error
	// TriggerNow runs jobKey once asynchronously, returning nil
	// immediately so the HTTP handler can answer 202 Accepted.
	TriggerNow(ctx context.Context, jobKey string) error
}
