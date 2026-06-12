// Package jobs holds the concrete business scheduled jobs the
// Scheduler service runs on a cron cadence.
//
// This file implements the `inactiva_empleado` job: the daily
// auto-deactivation sweep that deactivates chronic absentees per
// the per-company threshold configured in the Parametrizacion
// Service. The job is the implementation of:
//
//   - Spec section "Responsabilidades > Asynchronous Rule
//     Evaluation".
//   - Spec section "Gherkin scenario 2 — Desactivación
//     automática de empleado por ausencias consecutivas".
//   - Tasks.md task 3.3 (this PR — 3B-ii).
//
// Design notes
// ============
//  1. The job embeds no service.BaseJob: the cron runner injects
//     a BaseJob at run time (see service/cron_runner.go), and the
//     job reads its own dedicated dependencies (db,
//     parametrizacion client, outbox writer, event publisher)
//     from the struct field populated at construction time. The
//     same pattern is used by NotificacionesEnTiempoRealJob.
//  2. Database access is performed with prepared statements
//     ($1, $2, ...) against a *pgxpool.Pool. The spec's
//     "Seguridad > SQL Injection Prevention (Hardening)" section
//     is non-negotiable — no fmt.Sprintf SQL, no string
//     interpolation in WHERE clauses.
//  3. The empleado status update, identity revoke, and outbox
//     insert are wrapped in a single transaction per candidato.
//     The pattern is the canonical transactional outbox: the
//     downstream poller only sees events whose business state
//     has already been committed.
//  4. Per-empresa errors do NOT fail the whole job — the job
//     treats each empresa's batch independently and surfaces
//     per-empresa summary lines. Only a context cancellation
//     or a DB connectivity failure bubbles up; everything else
//     is logged and counted on the scheduler_jobs_failed_total
//     counter (with the supplied job_id label).
//  5. The job does NOT publish to the EventPublisher directly
//     — the outbox pattern means the outbox poller (a separate
//     service / process — not part of this file) reads the
//     outbox row and publishes to Redis/whatever downstream.
//     The publisher.PublishEmpleadoInactivado method on the
//     interface exists for future direct-publishing paths that
//     may bypass the outbox; for now the noop default is used
//     for symmetry. The per-empresa summary log is the
//     authoritative observability sink for this PR.
//  6. Idempotency: the `WHERE estatus != 4` clause on the
//     empleado update means re-running the job on the same day
//     (e.g. after a manual trigger that hit a partial failure)
//     is safe — already-inactive employees are skipped without
//     triggering an identity revoke or outbox write.
//  7. No PII: every log line carries only bigint identifiers
//     (empleado_id, empresa_id) and structured counters. No
//     employee names, emails, or any other PII is ever logged
//     or serialised into the outbox payload — the payload is
//     the EmpleadoInactivadoEvent struct in events/publisher.go
//     and only carries the identifiers plus the structured
//     motivo / timestamp fields.
//
// Schema assumptions (documented, not enforced)
// ============================================
// The query layer assumes the following columns exist in the
// operational schemas (the empleados and users services own the
// migrations; this job treats them as a contract):
//
//	empleados.empleados_empleado(
//	    id           BIGINT PRIMARY KEY,
//	    empresa_id   BIGINT NOT NULL,
//	    estatus      INT    NOT NULL,            -- 1=Activo, 4=Inactivo
//	    created_at   TIMESTAMPTZ,
//	    updated_at   TIMESTAMPTZ
//	)
//
//	empleados.empleados_inasistencia(
//	    id            BIGINT PRIMARY KEY,
//	    empleado_id   BIGINT NOT NULL,
//	    fecha         DATE   NOT NULL,
//	    justificada   BOOLEAN NOT NULL DEFAULT FALSE
//	)
//
//	users.identity_users(
//	    id            UUID PRIMARY KEY,
//	    empleado_id   BIGINT,                    -- FK to empleados
//	    status        VARCHAR(30) NOT NULL,      -- INACTIVE, ACTIVE, ...
//	    updated_at    TIMESTAMPTZ
//	)
//
// These assumptions match the spec section "Datos" for the
// Empleados and Auth services. The `empleado_id` column on
// `users.identity_users` is a documented assumption — V1 of the
// users migration in this branch only ships the identity table
// itself, the empleado link will arrive in a follow-up
// migration owned by the Auth service. The job's per-empresa
// loop guards on a LEFT JOIN to identity_users so the deactivation
// still proceeds when the link is not yet present.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/events"
	"axis-flow-back/internal/scheduler/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Constants — job identity, schedule, defaults.
// ---------------------------------------------------------------------------

// InactivaEmpleadoJobKey is the canonical job_key the seed file
// (Phase 6, task 6.1) inserts and that the wiring layer in main.go
// looks up via JobRepository.GetByKey. Matches the spec's
// Gherkin scenario 2 ("se ejecuta el job diario "inactiva_empleado"
// a las "02:00:00"") and the Runbook 1 entry.
const InactivaEmpleadoJobKey = "inactiva_empleado"

// inactivaEmpleadoCronExpr is the schedule from the spec: every
// day at 02:00 local. Stored as a *string so it round-trips
// through the scheduler.Job domain type without copy.
var inactivaEmpleadoCronExpr = "0 2 * * *"

// inactivaEmpleadoDescription is the human-readable description
// for the scheduler_jobs.description column. Matches the spec's
// "Asynchronous Rule Evaluation" responsibility.
var inactivaEmpleadoDescription = "Desactivación automática de empleados por inasistencias consecutivas (umbral parametrizado por empresa)."

// motivoInasistenciasConsecutivas is the canonical motivo value
// persisted on the outbox payload and the identity revoke. It is
// the only motivo the job emits today; the field is left
// free-form on the event type so future rule changes (e.g.
// "faltas_justificadas_rechazadas") can flow through without
// changing the wire contract.
const motivoInasistenciasConsecutivas = "inasistencias_consecutivas"

// estatusInactivo is the target value written to
// empleados_empleado.estatus. Matches the Gherkin scenario 2
// expectation ("estatus del empleado "Juan" a "Inactivo" (4)").
// The same constant is duplicated in
// `employee_repository_pg.go` (where the SQL actually uses it)
// because the two files were authored independently and the
// duplication makes the SQL file self-contained.
const estatusInactivo = 4

// Default values. Exposed as package-level vars so tests can
// tweak them in-place before constructing the job.
var (
	// DefaultMaxEmpresasPerRun is the defensive bound on the
	// distinct empresa_id list the job processes per run. The
	// bound protects the 02:00 daily window from ballooning
	// when the empleados service grows the tenant population.
	// 1000 is well above the production tenant count observed
	// at the time of writing; the warn log fires when the bound
	// is hit so the operator can raise it if needed.
	DefaultMaxEmpresasPerRun = 1000
)

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	// ErrAllEmpresasSkipped is a non-fatal sentinel returned
	// when every empresa in the run was skipped (e.g. all
	// parametrizacion lookups returned ErrParametrizacionBadRequest
	// or ErrParametrizacionUnavailable). The job converts it to
	// a successful no-op so the cron runner does not record a
	// FAILED execution when the only "problem" is that no
	// empresa could be processed today.
	ErrAllEmpresasSkipped = errors.New("scheduler: all empresas skipped")
)

// ---------------------------------------------------------------------------
// DB abstraction — replaced by EmployeeRepository +
// EmployeeTxFactory (PR 5B-ii). The `inactivaDB` interface and
// the `inactivaPoolAdapter` lived here from PR 3B-ii; they are
// removed because the job now talks to PostgreSQL exclusively
// through the typed EmployeeRepository contract. The pgx
// implementation is in `employee_repository_pg.go` and the
// wiring in `cmd/server/scheduler_wiring.go` constructs the
// repo + factory and passes them to the constructor.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Configuration knobs.
// ---------------------------------------------------------------------------

// InactivaEmpleadoJobDeps bundles the dependencies the
// InactivaEmpleadoJob needs. The pattern mirrors
// NotificacionesEnTiempoRealJob: a struct (instead of positional
// constructor args) keeps the call site readable as the
// dependency list grows. The CronRunner already provides a
// BaseJob at runtime, but the job retains direct handles to the
// ones its Run method uses so the type remains self-contained
// and testable in isolation.
type InactivaEmpleadoJobDeps struct {
	// Emps is the employee repository used to read the
	// per-empresa candidate list and to perform the
	// transactional empleado status update + identity revoke
	// inside the deactivation envelope. Required.
	Emps EmployeeRepository
	// TxFactory opens the per-empleado transaction that wraps
	// the status update, identity revoke, and outbox insert.
	// Required.
	TxFactory EmployeeTxFactory
	// Parametrizacion is the HTTP client used to read the
	// per-company inactivity threshold. The client returns
	// service.ErrParametrizacionBadRequest on permanent 4xx
	// (skip the empresa) and service.ErrParametrizacionUnavailable
	// on transient failures (also skip — the cron will retry
	// tomorrow). Required.
	Parametrizacion service.ParametrizacionClient
	// Outbox is the transactional outbox writer. Required;
	// callers may pass jobs.NewNoopOutboxWriter() while the
	// real outbox infra is being built (PR 4).
	Outbox OutboxWriter
	// EventPublisher emits EmpleadoInactivadoAutomaticamente
	// events. Optional; a noop default is substituted when
	// nil. The job's current code path does NOT call this
	// method — the outbox writer is the actual delivery
	// surface — but the field is held on the struct for future
	// direct-publishing callers and for symmetry with the
	// notificaciones job.
	EventPublisher events.EventPublisher
	// Logger is the structured logger used for the per-empresa
	// summary, the per-empleado debug lines, and the end-of-run
	// summary. Required.
	Logger *service.Logger
	// Metrics is the typed holder for the Prometheus
	// collectors. The job calls IncJobFailed on per-empleado
	// failures that should not fail the whole batch. Optional
	// (nil-safe; service.Metrics is also nil-safe).
	Metrics *service.SchedulerMetrics
	// MaxEmpresasPerRun caps the distinct empresa_id list
	// processed in a single run. Zero falls back to
	// DefaultMaxEmpresasPerRun (1000).
	MaxEmpresasPerRun int
}

// ---------------------------------------------------------------------------
// InactivaEmpleadoJob — the auto-deactivation concrete cron job.
// ---------------------------------------------------------------------------

// InactivaEmpleadoJob is the implementation of the
// `inactiva_empleado` job described in spec.md "Responsabilidades
// > Asynchronous Rule Evaluation" and Gherkin scenario 2
// ("Desactivación automática de empleado por ausencias
// consecutivas"). The job:
//
//  1. Queries the distinct empresa_id list that has at least one
//     active (estatus != 4) employee with at least one
//     unjustified inasistencia. The list is capped at
//     MaxEmpresasPerRun (defensive bound).
//  2. For each empresa_id:
//     - Calls Parametrizacion.GetInactivityThreshold. On
//     ErrParametrizacionBadRequest or ErrParametrizacionUnavailable
//     the empresa is logged and skipped — the cron will retry
//     the next day.
//     - Queries the candidate empleado_id list whose most
//     recent N=umbral inasistencias are all UNJUSTIFIED.
//     - For each candidate, opens a transaction and performs
//     the canonical transactional outbox: update empleado
//     status, revoke identity, insert outbox row, commit.
//     On any failure inside the transaction, ROLLBACK and
//     continue with the next candidate. The job increments
//     IncJobFailed once per failure so the metric label
//     error_class is bounded.
//  3. Emits one info-level summary line at the end of the run
//     with the inspection, deactivation, and error counters.
//
// The job is concurrency-safe: Run is invoked sequentially by
// the cron runner (the lock is held by the runner, not the
// job), and the per-empresa / per-empleado loops are sequential
// by design — see the Concurrency note in the file header.
type InactivaEmpleadoJob struct {
	// deps holds the static dependencies. The struct is
	// intentionally small — the runner constructs the
	// *service.BaseJob, the job reads from this struct to do
	// its work.
	deps InactivaEmpleadoJobDeps

	// emps is the employee repository. Built from deps.Emps
	// at construction time.
	emps EmployeeRepository
	// txFactory opens the per-empleado transaction. Built
	// from deps.TxFactory at construction time.
	txFactory EmployeeTxFactory

	// publisher is the event publisher snapshot used by future
	// direct-publishing paths. Defaults to a noop; can be
	// replaced at any time via SetEventPublisher. Guarded by
	// publisherMu so the wiring layer can swap it without
	// blocking Run.
	publisher   events.EventPublisher
	publisherMu sync.RWMutex

	// maxEmpresas is the resolved (non-zero) bound used at
	// runtime.
	maxEmpresas int
}

// ---------------------------------------------------------------------------
// Constructor.
// ---------------------------------------------------------------------------

// NewInactivaEmpleadoJob constructs the job and resolves the
// configurable knobs to their non-zero defaults. A nil deps
// field returns an error so the wiring layer fails fast at
// startup rather than at the first cron tick.
func NewInactivaEmpleadoJob(deps InactivaEmpleadoJobDeps) (*InactivaEmpleadoJob, error) {
	if deps.Emps == nil {
		return nil, fmt.Errorf("jobs.NewInactivaEmpleadoJob: %w: deps.Emps is required", scheduler.ErrInvalidInput)
	}
	if deps.TxFactory == nil {
		return nil, fmt.Errorf("jobs.NewInactivaEmpleadoJob: %w: deps.TxFactory is required", scheduler.ErrInvalidInput)
	}
	if deps.Parametrizacion == nil {
		return nil, fmt.Errorf("jobs.NewInactivaEmpleadoJob: %w: deps.Parametrizacion is required", scheduler.ErrInvalidInput)
	}
	if deps.Outbox == nil {
		return nil, fmt.Errorf("jobs.NewInactivaEmpleadoJob: %w: deps.Outbox is required", scheduler.ErrInvalidInput)
	}
	if deps.Logger == nil {
		return nil, fmt.Errorf("jobs.NewInactivaEmpleadoJob: %w: deps.Logger is required", scheduler.ErrInvalidInput)
	}

	maxEmpresas := deps.MaxEmpresasPerRun
	if maxEmpresas <= 0 {
		maxEmpresas = DefaultMaxEmpresasPerRun
	}

	pub := deps.EventPublisher
	if pub == nil {
		pub = events.NewNoopEventPublisher(deps.Logger.Inner())
	}

	return &InactivaEmpleadoJob{
		deps:        deps,
		emps:        deps.Emps,
		txFactory:   deps.TxFactory,
		publisher:   pub,
		maxEmpresas: maxEmpresas,
	}, nil
}

// SetEventPublisher swaps the publisher used by subsequent Run
// invocations. The wiring layer in main.go (Phase 4) calls this
// once at startup with the real Notificaciones-service-backed
// implementation; tests can use it to inject a spy. The swap is
// safe to perform concurrently with an in-flight Run — the
// in-flight run keeps using the publisher it loaded at the top
// of the method.
func (j *InactivaEmpleadoJob) SetEventPublisher(pub events.EventPublisher) {
	if j == nil {
		return
	}
	if pub == nil {
		pub = events.NewNoopEventPublisher(j.deps.Logger.Inner())
	}
	j.publisherMu.Lock()
	j.publisher = pub
	j.publisherMu.Unlock()
}

// currentPublisher returns the publisher snapshot for use within
// a single Run invocation. Reads the field under the read-lock
// once.
func (j *InactivaEmpleadoJob) currentPublisher() events.EventPublisher {
	j.publisherMu.RLock()
	defer j.publisherMu.RUnlock()
	return j.publisher
}

// ---------------------------------------------------------------------------
// Job contract — Metadata + Run.
// ---------------------------------------------------------------------------

// Metadata returns the scheduler.Job descriptor used by the cron
// runner to bind the job to a cron expression and to look up the
// job_id for the execution-row FK. The descriptor is a value
// (not a pointer) so the runner can store it without copying.
//
// The job_id is left as uuid.Nil because the cron runner loads
// the canonical row from the database via JobRepository.GetByKey
// before scheduling; the metadata returned here is the static
// "what kind of job is this" record, the row in
// scheduler.scheduler_jobs is the "this instance in this DB"
// record.
func (j *InactivaEmpleadoJob) Metadata() scheduler.Job {
	cronExpr := inactivaEmpleadoCronExpr
	desc := inactivaEmpleadoDescription
	return scheduler.Job{
		JobKey:         InactivaEmpleadoJobKey,
		CronExpression: &cronExpr,
		JobClass:       "jobs.InactivaEmpleadoJob",
		Module:         "empleados",
		Description:    &desc,
		IsActive:       true,
	}
}

// Run is the per-tick entry point. The cron runner wraps the
// call in WrapWithLifecycle and the lock-gated flow documented
// in service/cron_runner.go, so this method assumes it is the
// only goroutine executing it for the current process (lock is
// held by the runner). The inner per-empresa / per-empleado
// loops are sequential by design — see the Concurrency note in
// the file header.
func (j *InactivaEmpleadoJob) Run(ctx context.Context) error {
	start := time.Now()
	now := time.Now().UTC()

	// Capture the publisher once at the top of the run so a
	// concurrent SetEventPublisher call from the wiring layer
	// does not race with the in-flight loop. The publisher is
	// not currently called inside the loop (the outbox writer
	// is the actual delivery surface) but the snapshot is
	// kept for symmetry with the notificaciones job.
	_ = j.currentPublisher()

	empresas, err := j.emps.EmpresasWithUnjustifiedStreaks(ctx, j.maxEmpresas)
	if err != nil {
		return fmt.Errorf("inactiva_empleado.Run: query active empresas: %w", err)
	}
	if len(empresas) >= j.maxEmpresas {
		j.deps.Logger.Inner().WarnContext(ctx, "inactiva_empleado: empresa list hit the per-run cap; raise MaxEmpresasPerRun if the tenant population is expected to grow",
			slog.Int("max_empresas", j.maxEmpresas),
			slog.Int("empresas_observed", len(empresas)),
		)
	}

	var (
		totalCandidates   int
		totalDeactivated  int
		totalSkipped      int
		totalErrors       int
		empresasWithWork  int
	)
	for _, empresaID := range empresas {
		empresaStart := time.Now()
		threshold, err := j.deps.Parametrizacion.GetInactivityThreshold(ctx, empresaID)
		if err != nil {
			// Per-empresa parametrizacion errors do NOT fail
			// the whole job — the cron will retry tomorrow.
			// The error class label is propagated to the
			// scheduler_jobs_failed_total counter.
			j.deps.Logger.Inner().WarnContext(ctx, "inactiva_empleado: parametrizacion lookup failed; skipping empresa",
				slog.Int64("empresa_id", empresaID),
				slog.String("error", err.Error()),
			)
			if j.deps.Metrics != nil {
				j.deps.Metrics.IncJobFailed(InactivaEmpleadoJobKey, errorClassFor(err))
			}
			totalErrors++
			continue
		}
		if threshold <= 0 {
			// Defensive: the parametrizacion client already
			// rejects umbral_dias <= 0, but a defensive guard
			// here costs nothing and protects against a future
			// parser regression.
			j.deps.Logger.Inner().WarnContext(ctx, "inactiva_empleado: parametrizacion returned non-positive threshold; skipping empresa",
				slog.Int64("empresa_id", empresaID),
				slog.Int("threshold", threshold),
			)
			totalErrors++
			continue
		}

		candidates, err := j.emps.EmpleadosWithUnjustifiedStreakAtLeast(ctx, empresaID, threshold)
		if err != nil {
			j.deps.Logger.Inner().WarnContext(ctx, "inactiva_empleado: candidate query failed; skipping empresa",
				slog.Int64("empresa_id", empresaID),
				slog.Int("threshold", threshold),
				slog.String("error", err.Error()),
			)
			if j.deps.Metrics != nil {
				j.deps.Metrics.IncJobFailed(InactivaEmpleadoJobKey, errorClassFor(err))
			}
			totalErrors++
			continue
		}
		totalCandidates += len(candidates)
		if len(candidates) == 0 {
			j.deps.Logger.Inner().InfoContext(ctx, "inactiva_empleado empresa batch completed",
				slog.String("msg_field", "inactiva_empleado empresa batch completed"),
				slog.Int64("empresa_id", empresaID),
				slog.Int("threshold", threshold),
				slog.Int("candidates", 0),
				slog.Int("deactivated", 0),
				slog.Int("errors", 0),
				slog.Int64("duration_ms", time.Since(empresaStart).Milliseconds()),
			)
			continue
		}
		empresasWithWork++

		var empresaDeactivated, empresaErrors int
		for _, empleadoID := range candidates {
			ok, derr := j.deactivateEmpleado(ctx, empleadoID, empresaID, threshold, now)
			switch {
			case derr != nil:
				j.deps.Logger.Inner().WarnContext(ctx, "inactiva_empleado: per-empleado deactivation failed; continuing",
					slog.Int64("empresa_id", empresaID),
					slog.Int64("empleado_id", empleadoID),
					slog.Int("threshold", threshold),
					slog.String("error", derr.Error()),
				)
				if j.deps.Metrics != nil {
					j.deps.Metrics.IncJobFailed(InactivaEmpleadoJobKey, errorClassFor(derr))
				}
				empresaErrors++
				totalErrors++
			case ok:
				empresaDeactivated++
				totalDeactivated++
			default:
				// Already inactive — neither an error nor a
				// deactivation. Counted in totalSkipped to
				// give the end-of-run summary a complete
				// picture (candidates - deactivated -
				// errors = skipped).
				totalSkipped++
			}
		}

		j.deps.Logger.Inner().InfoContext(ctx, "inactiva_empleado empresa batch completed",
			slog.String("msg_field", "inactiva_empleado empresa batch completed"),
			slog.Int64("empresa_id", empresaID),
			slog.Int("threshold", threshold),
			slog.Int("candidates", len(candidates)),
			slog.Int("deactivated", empresaDeactivated),
			slog.Int("errors", empresaErrors),
			slog.Int64("duration_ms", time.Since(empresaStart).Milliseconds()),
		)
	}

	// End-of-run summary. The single info-level record emitted
	// per tick. No PII: the per-empresa lines above are the
	// authoritative per-tenant observability sink.
	j.deps.Logger.Inner().InfoContext(ctx, "inactiva_empleado run completed",
		slog.String("msg_field", "inactiva_empleado run completed"),
		slog.Int("empresas_visited", len(empresas)),
		slog.Int("empresas_with_work", empresasWithWork),
		slog.Int("total_candidates", totalCandidates),
		slog.Int("total_deactivated", totalDeactivated),
		slog.Int("total_skipped", totalSkipped),
		slog.Int("total_errors", totalErrors),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	if len(empresas) == 0 || (totalDeactivated == 0 && totalErrors == 0) {
		// Non-fatal: the dataset is empty or the only
		// activity was skips. Return the sentinel so the
		// runner can opt to log it differently (the runner
		// currently treats every error as a FAILED execution,
		// so we return nil and rely on the summary log for
		// observability). The sentinel is kept exported for
		// tests and for future Phase-5 instrumentation that
		// may want to count empty-run cases.
		_ = ErrAllEmpresasSkipped
	}
	return nil
}

// ---------------------------------------------------------------------------
// SQL — moved to employee_repository_pg.go (PR 5B-ii). See
// that file for the canonical prepared statements and the
// schema assumptions they encode. The job talks to PostgreSQL
// exclusively through the typed EmployeeRepository contract.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Transactional envelope — update + revoke + outbox, in one tx.
// ---------------------------------------------------------------------------

// deactivateEmpleado performs the canonical transactional
// outbox for one empleado: open a transaction, flip the
// empleado row to estatus = 4 (idempotent), revoke the linked
// identity (idempotent, gracefully no-ops when the link is
// missing or the identity is in a terminal status), insert the
// outbox row, and commit. Any error inside the envelope
// triggers a ROLLBACK and the function returns the error
// wrapped with %w. The first return value is true when the
// empleado status was actually flipped (i.e. a new
// deactivation happened); false when the row was already
// inactive (the identity revoke and outbox insert are NOT
// attempted — the outbox event only fires for real
// deactivations).
//
// The implementation uses the EmployeeRepository contract
// (PR 5B-ii) instead of a raw pgx.Tx. The contract is what the
// in-package OutboxWriter accepts (it takes the new Tx
// interface), so the per-empleado envelope stays a pure
// orchestration of repository calls + outbox insert + commit.
func (j *InactivaEmpleadoJob) deactivateEmpleado(
	ctx context.Context,
	empleadoID, empresaID int64,
	threshold int,
	now time.Time,
) (bool, error) {
	tx, _, err := j.txFactory.BeginTx(ctx)
	if err != nil {
		return false, fmt.Errorf("deactivateEmpleado begin: %w", err)
	}
	// rollbackUnlessCommitted runs the ROLLBACK if the
	// transaction has not been committed by the time the
	// deferred function fires. The pgxTx adapter swallows
	// pgx.ErrTxClosed so a double-Rollback (after a successful
	// Commit) is a no-op.
	committed := false
	defer func() {
		if !committed {
			// Best-effort rollback. A failure here is
			// logged but not surfaced — the original
			// error is the one the caller cares about.
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				j.deps.Logger.Inner().WarnContext(ctx, "inactiva_empleado: rollback failed after deactivation error",
					slog.Int64("empresa_id", empresaID),
					slog.Int64("empleado_id", empleadoID),
					slog.String("error", rbErr.Error()),
				)
			}
		}
	}()

	// 1. Update empleado status. The WHERE estatus <> 4 guard
	// is the idempotency contract — re-running on the same
	// day must not re-deactivate an already-inactive employee.
	// The repository returns (false, nil) when the guard
	// matched 0 rows so the caller can short-circuit the
	// outbox event.
	updated, err := j.emps.DeactivateEmpleadoTx(ctx, tx, empleadoID, now)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Already inactive (or the row was deleted between
		// the candidate query and the update). The
		// transactional envelope is still rolled back —
		// there is nothing to commit and no outbox event
		// to emit. (The repository's idempotency contract
		// does not return pgx.ErrNoRows, but the switch
		// arm is kept for defence in depth: a future
		// implementation that returns the sentinel still
		// produces the correct "already inactive"
		// outcome.)
		return false, nil
	case err != nil:
		return false, fmt.Errorf("deactivateEmpleado update empleado: %w", err)
	case !updated:
		// The repository's idempotency contract: 0 rows
		// updated → the row was already inactive. Same
		// short-circuit as the ErrNoRows arm above.
		return false, nil
	}

	// 2. Revoke the linked identity. A 0-row result is
	// tolerated: the identity may not be linked yet (V1
	// schema gap — see the file header) or it may already
	// be in a terminal status. The outbox event is still
	// emitted so the HR dashboard can react.
	if err := j.emps.RevokeIdentityTx(ctx, tx, empleadoID, now); err != nil {
		return false, fmt.Errorf("deactivateEmpleado revoke identity: %w", err)
	}

	// 3. Insert the outbox row. The payload is the
	// EmpleadoInactivadoEvent struct from
	// internal/scheduler/events serialised to JSON. The
	// encoder is json.Marshal (not Encoder.Encode) so the
	// output is canonical and free of map iteration order
	// quirks.
	evt := events.EmpleadoInactivadoEvent{
		EmpleadoID:        empleadoID,
		EmpresaID:         empresaID,
		FaltasConsecutivas: threshold,
		Motivo:            motivoInasistenciasConsecutivas,
		Timestamp:         now,
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return false, fmt.Errorf("deactivateEmpleado marshal event: %w", err)
	}
	eventType := "EmpleadoInactivadoAutomaticamente"
	if err := j.deps.Outbox.Insert(ctx, tx, eventType, empleadoID, payload); err != nil {
		return false, fmt.Errorf("deactivateEmpleado outbox insert: %w", err)
	}

	// 4. Commit. A commit error surfaces to the caller so
	// the per-empresa loop can log and continue.
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("deactivateEmpleado commit: %w", err)
	}
	committed = true

	// Post-commit observability. The before/after audit log
	// lives here so it only fires for real deactivations
	// (committed, not rolled back). No PII: only bigint
	// identifiers.
	j.deps.Logger.Inner().InfoContext(ctx, "inactiva_empleado: empleado deactivated",
		slog.String("msg_field", "inactiva_empleado: empleado deactivated"),
		slog.Int64("empresa_id", empresaID),
		slog.Int64("empleado_id", empleadoID),
		slog.Int("faltas_consecutivas", threshold),
		slog.Int("new_estatus", estatusInactivo),
		slog.String("motivo", motivoInasistenciasConsecutivas),
	)
	return true, nil
}

// ---------------------------------------------------------------------------
// Internal helpers — error class + redaction policy.
// ---------------------------------------------------------------------------

// errorClassFor returns a stable, low-cardinality label suitable
// for the scheduler_jobs_failed_total counter. The label is the
// Go type name of the error so the metric stays bounded even
// when many distinct error values flow through the job. A
// non-typed error is reported as "unknown" so the operator can
// spot un-typed errors without inflating the cardinality.
func errorClassFor(err error) string {
	if err == nil {
		return "unknown"
	}
	// Unwrap to the deepest known sentinel so the
	// parametrizacion and pgx error families collapse to a
	// single label each. This keeps the metric useful
	// without exploding the cardinality.
	switch {
	case errors.Is(err, service.ErrParametrizacionBadRequest):
		return "parametrizacion_bad_request"
	case errors.Is(err, service.ErrParametrizacionUnavailable):
		return "parametrizacion_unavailable"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return "pg_" + pgErr.Code
	}
	return "inactiva_empleado_error"
}

// redactUUIDPrefixString returns the first 8 characters of a
// UUID-stringified value followed by an ellipsis. The full UUID
// is safe to log in principle (it is not a secret) but the
// redaction keeps log lines short and consistent with the FCM
// token redaction policy used by the notificaciones job.
func redactUUIDPrefixString(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8] + "…"
}
