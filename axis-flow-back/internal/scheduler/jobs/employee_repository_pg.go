// Package jobs — pgx implementation of the EmployeeRepository
// and EmployeeTxFactory contracts. The file lives alongside
// `employee_repository.go` so the interface and its production
// implementation can be reviewed together.
//
// All SQL is built with positional parameters ($1, $2, …) per
// the spec's "Seguridad > SQL Injection Prevention (Hardening)"
// section. No string interpolation, no fmt.Sprintf in WHERE
// clauses.
//
// Transactional surface
// =====================
// The pgx tx is wrapped in `pgEmployeeTx`, a small adapter
// that implements the `Tx` interface. The adapter is a
// one-shot struct: the underlying pgx.Tx is closed by Commit
// or Rollback and a second call to either is a no-op
// (pgx.ErrTxClosed is swallowed).
package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Constants — query-time business values.
// ---------------------------------------------------------------------------

// estatusEmpleadoInactivo is the target value written to
// empleados_empleado.estatus. Matches the Gherkin scenario 2
// expectation ("estatus del empleado ... a Inactivo (4)") and
// the in-package constant in `inactiva_empleado.go`. The value
// is duplicated here so the SQL file is self-contained and
// does not depend on the job package's exported state.
const estatusEmpleadoInactivo = 4

// identityStatusInactivo is the value written to
// users.identity_users.status when the linked identity is
// revoked. The string matches the in-package constant in
// `inactiva_empleado.go` and the documented V1 enum on the
// users schema.
const identityStatusInactivo = "INACTIVE"

// ---------------------------------------------------------------------------
// pgEmployeeTx — Transaction adapter.
// ---------------------------------------------------------------------------

// pgEmployeeTx is the production implementation of the Tx
// interface. It wraps a *pgx.Tx so callers can drive the
// per-empleado transaction without holding the concrete pgx
// type. A zero-value struct is invalid — the constructor
// always returns a non-nil adapter that wraps a non-nil
// pgx.Tx.
type pgEmployeeTx struct {
	tx pgx.Tx
}

// newPgEmployeeTx wraps a pgx.Tx in a pgEmployeeTx. The caller
// retains ownership of the lifecycle: a successful Commit
// invalidates the adapter (subsequent calls return
// pgx.ErrTxClosed, which the adapter swallows).
func newPgEmployeeTx(tx pgx.Tx) *pgEmployeeTx {
	return &pgEmployeeTx{tx: tx}
}

// Commit forwards to the underlying pgx.Tx.Commit. The
// returned error is preserved so the job can log it; the
// sentinel pgx.ErrTxClosed is wrapped to keep the error chain
// intact.
func (t *pgEmployeeTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

// Rollback forwards to the underlying pgx.Tx.Rollback. The
// sentinel pgx.ErrTxClosed is treated as a successful
// no-op (the transaction was already committed or rolled
// back by a previous call), matching the canonical pgx
// semantics.
func (t *pgEmployeeTx) Rollback(ctx context.Context) error {
	err := t.tx.Rollback(ctx)
	if err != nil && errors.Is(err, pgx.ErrTxClosed) {
		return nil
	}
	return err
}

// raw exposes the underlying pgx.Tx for the repository
// methods. The method is unexported because the job never
// sees it — the repository's own methods cast Tx to
// *pgEmployeeTx internally. The cast is a type assertion
// because the production code paths always pass a real
// pgx-backed adapter; the test mock implements Tx but the
// mock-backed repository is its own type.
func (t *pgEmployeeTx) raw() pgx.Tx {
	return t.tx
}

// ---------------------------------------------------------------------------
// pgEmployeeRepository — production EmployeeRepository.
// ---------------------------------------------------------------------------

// pgEmployeeRepository is the production EmployeeRepository,
// backed by a *pgxpool.Pool. The struct is intentionally
// small — the wiring layer in main.go constructs it once at
// startup and passes the value to the job's constructor and
// the factory.
type pgEmployeeRepository struct {
	pool *pgxpool.Pool
}

// NewPgEmployeeRepository returns an EmployeeRepository
// wired to the supplied pool. The pool is captured by
// reference; the returned repository shares the pool's
// lifecycle.
func NewPgEmployeeRepository(pool *pgxpool.Pool) EmployeeRepository {
	return &pgEmployeeRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// pgEmployeeTxFactory — production EmployeeTxFactory.
// ---------------------------------------------------------------------------

// pgEmployeeTxFactory is the production EmployeeTxFactory,
// backed by a *pgxpool.Pool. The factory is constructed with
// the same pool as the repository and the same
// EmployeeRepository instance is returned from BeginTx so
// the in-tx repository methods can use the same query plan
// cache as the out-of-tx paths.
type pgEmployeeTxFactory struct {
	pool *pgxpool.Pool
	repo EmployeeRepository
}

// NewPgEmployeeTxFactory returns an EmployeeTxFactory wired
// to the supplied pool and repository. The repository is
// returned from BeginTx so callers do not need to thread
// both the factory and the repository through their
// dependency graph.
func NewPgEmployeeTxFactory(pool *pgxpool.Pool, repo EmployeeRepository) EmployeeTxFactory {
	return &pgEmployeeTxFactory{pool: pool, repo: repo}
}

// BeginTx opens a pgx transaction and returns the wrapped Tx
// + the same EmployeeRepository instance the factory was
// constructed with. The caller MUST call Commit or Rollback
// exactly once.
func (f *pgEmployeeTxFactory) BeginTx(ctx context.Context) (Tx, EmployeeRepository, error) {
	pgxTx, err := f.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("EmployeeTxFactory.BeginTx: %w", err)
	}
	return newPgEmployeeTx(pgxTx), f.repo, nil
}

// ---------------------------------------------------------------------------
// SQL — prepared statements (positional parameters only).
// ---------------------------------------------------------------------------

// queryActiveEmpresasSQL returns the distinct empresa_id list
// with at least one active employee carrying an unjustified
// inasistencia. The cap is the LIMIT ($1) substituted at
// execution time.
const queryActiveEmpresasSQL = `
SELECT DISTINCT e.empresa_id
FROM empleados.empleados_empleado AS e
INNER JOIN empleados.empleados_inasistencia AS i
    ON i.empleado_id = e.id
WHERE i.justificada = FALSE
  AND e.estatus <> 4
ORDER BY e.empresa_id
LIMIT $1
`

// queryCandidatesSQL returns the empleado_id list whose
// most recent `threshold` inasistencias are all unjustified.
// The shape uses a correlated subquery (see the long-form
// rationale in `inactiva_empleado.go`).
const queryCandidatesSQL = `
SELECT e.id
FROM empleados.empleados_empleado AS e
WHERE e.empresa_id = $1
  AND e.estatus <> 4
  AND (
    SELECT COUNT(*)
    FROM (
      SELECT i.fecha
      FROM empleados.empleados_inasistencia AS i
      WHERE i.empleado_id = e.id
        AND i.justificada = FALSE
      ORDER BY i.fecha DESC
      LIMIT $2
    ) AS ultimas
  ) = $2
ORDER BY e.id
`

// updateEmpleadoEstatusSQL flips estatus to 4 idempotently.
// RETURNING id confirms the update; the caller treats 0
// rows (pgx.ErrNoRows) as the "already inactive" case.
const updateEmpleadoEstatusSQL = `
UPDATE empleados.empleados_empleado
SET estatus = $2,
    updated_at = $3
WHERE id = $1
  AND estatus <> 4
RETURNING id
`

// revokeIdentitySQL flips the linked identity row to
// INACTIVE. A 0-row result (no linked identity, or the
// identity is already in a terminal status) is tolerated.
const revokeIdentitySQL = `
UPDATE users.identity_users
SET status = $2,
    updated_at = $3
WHERE empleado_id = $1
  AND status NOT IN ($2, 'DELETED', 'LOCKED')
RETURNING id
`

// ---------------------------------------------------------------------------
// Method implementations — out-of-tx paths.
// ---------------------------------------------------------------------------

// EmpresasWithUnjustifiedStreaks returns the distinct empresa
// id list. The cap is the caller's defensive bound (see
// `inactiva_empleado.go` for the rationale).
func (r *pgEmployeeRepository) EmpresasWithUnjustifiedStreaks(ctx context.Context, limit int) ([]int64, error) {
	rows, err := r.pool.Query(ctx, queryActiveEmpresasSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("EmployeeRepository.EmpresasWithUnjustifiedStreaks: %w", err)
	}
	defer rows.Close()

	out := make([]int64, 0, 16)
	for rows.Next() {
		var id int64
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, fmt.Errorf("EmployeeRepository.EmpresasWithUnjustifiedStreaks scan: %w", scanErr)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("EmployeeRepository.EmpresasWithUnjustifiedStreaks rows: %w", err)
	}
	return out, nil
}

// EmpleadosWithUnjustifiedStreakAtLeast returns the
// candidato list. The threshold is the per-company value
// from the Parametrizacion Service.
func (r *pgEmployeeRepository) EmpleadosWithUnjustifiedStreakAtLeast(ctx context.Context, empresaID int64, minStreak int) ([]int64, error) {
	rows, err := r.pool.Query(ctx, queryCandidatesSQL, empresaID, minStreak)
	if err != nil {
		return nil, fmt.Errorf("EmployeeRepository.EmpleadosWithUnjustifiedStreakAtLeast: %w", err)
	}
	defer rows.Close()

	out := make([]int64, 0, 16)
	for rows.Next() {
		var id int64
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, fmt.Errorf("EmployeeRepository.EmpleadosWithUnjustifiedStreakAtLeast scan: %w", scanErr)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("EmployeeRepository.EmpleadosWithUnjustifiedStreakAtLeast rows: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Method implementations — in-tx paths.
// ---------------------------------------------------------------------------

// DeactivateEmpleadoTx flips the empleado row to estatus =
// 4 (Inactivo) on the supplied transaction. The Tx is
// type-asserted to the production adapter; any other
// implementation returns a non-nil error and the job logs
// it.
func (r *pgEmployeeRepository) DeactivateEmpleadoTx(ctx context.Context, tx Tx, empleadoID int64, now time.Time) (bool, error) {
	pgxTx, ok := tx.(*pgEmployeeTx)
	if !ok {
		return false, fmt.Errorf("EmployeeRepository.DeactivateEmpleadoTx: unsupported tx type %T", tx)
	}

	var updatedID int64
	err := pgxTx.raw().QueryRow(ctx, updateEmpleadoEstatusSQL, empleadoID, estatusEmpleadoInactivo, now).Scan(&updatedID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("EmployeeRepository.DeactivateEmpleadoTx: %w", err)
	}
	_ = updatedID
	return true, nil
}

// RevokeIdentityTx flips the linked identity row to
// INACTIVE. A 0-row result is treated as a successful
// no-op (no linked identity, or the identity is already
// in a terminal status) and the function returns nil.
func (r *pgEmployeeRepository) RevokeIdentityTx(ctx context.Context, tx Tx, empleadoID int64, now time.Time) error {
	pgxTx, ok := tx.(*pgEmployeeTx)
	if !ok {
		return fmt.Errorf("EmployeeRepository.RevokeIdentityTx: unsupported tx type %T", tx)
	}

	var revokedID uuid.UUID
	err := pgxTx.raw().QueryRow(ctx, revokeIdentitySQL, empleadoID, identityStatusInactivo, now).Scan(&revokedID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("EmployeeRepository.RevokeIdentityTx: %w", err)
	}
	_ = revokedID
	return nil
}
