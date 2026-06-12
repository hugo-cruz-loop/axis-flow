// Package jobs — EmployeeRepository contract used by the
// `inactiva_empleado` job (PR 3B-ii + this PR — 5B-ii). The
// repository owns the SQL for the per-empresa candidate query
// and the transactional empleado-status / identity-revoke /
// outbox envelope. The pgx implementation lives in the sibling
// file `employee_repository_pg.go`.
//
// Tx abstraction
// ==============
// The interface deliberately exposes the transactional surface
// (Commit, Rollback) as a small `Tx` interface so the job can
// drive the per-empleado transaction without holding a
// concrete *pgx type. The actual SQL is performed by the
// repository methods that accept a `Tx` argument — the
// repository "knows" how to issue its queries on a tx-bound
// connection because the pgx implementation casts the
// `Tx` back to the underlying pgx type at the call site. The
// job never sees the pgx type.
//
// Why a TxFactory
// ---------------
// The BeginTx method is exposed on its own interface
// (`EmployeeTxFactory`) so the test mock can return a fake
// `Tx` whose Commit/Rollback the test can assert against. The
// production implementation (`pgEmployeeTxFactory`) is a thin
// adapter around *pgxpool.Pool.Begin.
package jobs

import (
	"context"
	"time"
)

// ---------------------------------------------------------------------------
// Tx — minimal transactional surface.
// ---------------------------------------------------------------------------

// Tx is the transactional surface the job uses to drive a
// per-empleado transaction. The interface exposes ONLY the two
// methods the caller needs (Commit to commit the envelope,
// Rollback to abort). The repository methods that issue SQL
// accept a Tx and cast to the underlying pgx type internally —
// the job never sees the concrete pgx.Tx.
//
// The interface is unexported-by-design at the SQL layer but
// exposed here because the test mock (in
// `inactiva_empleado_test.go`) implements it.
type Tx interface {
	// Commit atomically commits the transaction. A non-nil
	// return signals the caller must treat the operation as a
	// failure (the runner logs and moves to the next empleado).
	Commit(ctx context.Context) error
	// Rollback aborts the transaction. Idempotent: pgx.Tx
	// returns pgx.ErrTxClosed on a second call, and the
	// production adapter swallows that sentinel.
	Rollback(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// EmployeeRepository — public contract.
// ---------------------------------------------------------------------------

// EmployeeRepository is the data-access surface the
// inactiva_empleado job needs. All methods are
// concurrency-safe: the underlying pgx pool is, and the
// interface is the only place the job talks to PostgreSQL.
type EmployeeRepository interface {
	// EmpresasWithUnjustifiedStreaks returns the distinct
	// empresa_id list that has at least one active (estatus
	// <> 4) employee with at least one unjustified
	// inasistencia. The list is sorted and capped at `limit`
	// rows by the caller.
	EmpresasWithUnjustifiedStreaks(ctx context.Context, limit int) ([]int64, error)

	// EmpleadosWithUnjustifiedStreakAtLeast returns the
	// empleado_id list inside the given empresa whose most
	// recent `minStreak` inasistencias are ALL unjustified.
	// The result is sorted by id for deterministic iteration
	// order in the per-empleado loop.
	EmpleadosWithUnjustifiedStreakAtLeast(ctx context.Context, empresaID int64, minStreak int) ([]int64, error)

	// DeactivateEmpleadoTx flips the empleado row to estatus
	// = 4 (Inactivo) on the supplied transaction. The
	// idempotency guard `AND estatus <> 4` returns
	// (false, nil) when the row is already inactive. A
	// non-nil error is returned unwrapped so the job's %w
	// wrapping preserves the underlying pgx error class.
	DeactivateEmpleadoTx(ctx context.Context, tx Tx, empleadoID int64, now time.Time) (bool, error)

	// RevokeIdentityTx flips the linked identity_users row
	// to status = 'INACTIVE' on the supplied transaction.
	// A 0-row result is tolerated (the identity may not be
	// linked yet, may already be in a terminal status, or
	// the link may be missing) and is returned as
	// (uuid.Nil, nil) so the caller can distinguish
	// "no row" from "driver error".
	RevokeIdentityTx(ctx context.Context, tx Tx, empleadoID int64, now time.Time) error
}

// ---------------------------------------------------------------------------
// EmployeeTxFactory — exposes BeginTx for the per-empleado
// envelope.
// ---------------------------------------------------------------------------

// EmployeeTxFactory opens a transaction for the
// per-empleado deactivation envelope. The returned Tx is
// bound to a single connection for the lifetime of the
// transaction; the caller MUST call Commit or Rollback
// exactly once.
//
// The second return value (the EmployeeRepository) is the
// same instance the job already holds on its struct; it is
// returned here for symmetry with the production
// implementation and to keep the BeginTx signature
// future-proof for callers that may want a "tx-scoped
// repository view" (a pattern the codebase does not
// currently need but that the interface leaves open).
type EmployeeTxFactory interface {
	BeginTx(ctx context.Context) (Tx, EmployeeRepository, error)
}
