// Package repository contains the PostgreSQL persistence layer for the
// scheduler module. All SQL uses prepared statements with positional
// parameters (no string interpolation) per the service's SQL-injection
// hardening policy.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"axis-flow-back/internal/scheduler"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// ExecutionRepository — interface.
// ---------------------------------------------------------------------------

// ExecutionRepository defines persistence operations for
// scheduler.scheduler_executions.
type ExecutionRepository interface {
	// Insert records a new execution row (typically status=RUNNING) and
	// returns the persisted record including server-assigned id and
	// timestamps.
	Insert(ctx context.Context, exec scheduler.Execution) (scheduler.Execution, error)
	// Finalize updates an existing execution with its terminal status,
	// ended_at and optional error_log. Returns scheduler.ErrNotFound when
	// the row is absent.
	Finalize(ctx context.Context, execID uuid.UUID, status scheduler.ExecutionStatus, errorLog *string) (scheduler.Execution, error)
	// List returns executions matching the optional filter, paginated.
	List(ctx context.Context, filter scheduler.ExecutionListFilter) ([]scheduler.Execution, int, error)
}

// ---------------------------------------------------------------------------
// pgExecutionRepository — PostgreSQL implementation.
// ---------------------------------------------------------------------------

// pgExecutionRepository is a PostgreSQL-backed ExecutionRepository.
type pgExecutionRepository struct {
	db execDB
}

// NewPgExecutionRepository creates a PostgreSQL execution repository.
func NewPgExecutionRepository(pool *pgxpool.Pool) ExecutionRepository {
	return &pgExecutionRepository{db: &pgxExecPoolAdapter{pool: pool}}
}

const execSelectColumns = `execution_id, job_id, status, started_at, ended_at,
    error_log, execution_metadata, created_at, duration_seconds`

func (r *pgExecutionRepository) Insert(ctx context.Context, exec scheduler.Execution) (scheduler.Execution, error) {
	metadata := exec.ExecutionMetadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	startedAt := exec.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	const q = `INSERT INTO scheduler.scheduler_executions
        (job_id, status, started_at, ended_at, error_log, execution_metadata)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING ` + execSelectColumns
	var out scheduler.Execution
	err := r.db.QueryRow(ctx, q,
		exec.JobID, string(exec.Status), startedAt, exec.EndedAt, exec.ErrorLog, metadata,
	).Scan(
		&out.ExecutionID, &out.JobID, &out.Status, &out.StartedAt, &out.EndedAt,
		&out.ErrorLog, &out.ExecutionMetadata, &out.CreatedAt, &out.DurationSeconds,
	)
	if err != nil {
		if isFKViolation(err) {
			return scheduler.Execution{}, fmt.Errorf("execution_repository.Insert: %w", scheduler.ErrNotFound)
		}
		return scheduler.Execution{}, fmt.Errorf("execution_repository.Insert: %w", err)
	}
	return out, nil
}

func (r *pgExecutionRepository) Finalize(ctx context.Context, execID uuid.UUID, status scheduler.ExecutionStatus, errorLog *string) (scheduler.Execution, error) {
	endedAt := time.Now().UTC()
	const q = `UPDATE scheduler.scheduler_executions
        SET status = $2,
            ended_at = $3,
            error_log = $4
        WHERE execution_id = $1
        RETURNING ` + execSelectColumns
	var out scheduler.Execution
	err := r.db.QueryRow(ctx, q, execID, string(status), endedAt, errorLog).Scan(
		&out.ExecutionID, &out.JobID, &out.Status, &out.StartedAt, &out.EndedAt,
		&out.ErrorLog, &out.ExecutionMetadata, &out.CreatedAt, &out.DurationSeconds,
	)
	if err != nil {
		return scheduler.Execution{}, mapNotFound("execution_repository.Finalize", err)
	}
	return out, nil
}

func (r *pgExecutionRepository) List(ctx context.Context, filter scheduler.ExecutionListFilter) ([]scheduler.Execution, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	conds := []string{}
	args := []any{}
	idx := 1
	if filter.JobID != nil {
		conds = append(conds, fmt.Sprintf("job_id = $%d", idx))
		args = append(args, *filter.JobID)
		idx++
	}
	if filter.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(*filter.Status))
		idx++
	}
	if filter.StartDate != nil {
		conds = append(conds, fmt.Sprintf("started_at >= $%d", idx))
		args = append(args, *filter.StartDate)
		idx++
	}
	if filter.EndDate != nil {
		conds = append(conds, fmt.Sprintf("started_at <= $%d", idx))
		args = append(args, *filter.EndDate)
		idx++
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + joinAnd(conds)
	}

	countQ := `SELECT COUNT(*) FROM scheduler.scheduler_executions` + where
	var total int
	if err := r.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("execution_repository.List count: %w", err)
	}

	listQ := fmt.Sprintf(`SELECT %s FROM scheduler.scheduler_executions%s
        ORDER BY started_at DESC
        LIMIT $%d OFFSET $%d`, execSelectColumns, where, idx, idx+1)
	listArgs := append(args, limit, offset)
	rows, err := r.db.Query(ctx, listQ, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("execution_repository.List query: %w", err)
	}
	defer rows.Close()

	out := []scheduler.Execution{}
	for rows.Next() {
		var e scheduler.Execution
		if scanErr := rows.Scan(
			&e.ExecutionID, &e.JobID, &e.Status, &e.StartedAt, &e.EndedAt,
			&e.ErrorLog, &e.ExecutionMetadata, &e.CreatedAt, &e.DurationSeconds,
		); scanErr != nil {
			return nil, 0, fmt.Errorf("execution_repository.List scan: %w", scanErr)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("execution_repository.List rows: %w", err)
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// Internal helpers — adapters and error mapping.
// ---------------------------------------------------------------------------

// execDB abstracts the pgx surface used by the repository so tests can plug
// in transaction-bound runners or in-memory fakes.
type execDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// pgxExecPoolAdapter adapts a *pgxpool.Pool to the execDB interface.
type pgxExecPoolAdapter struct{ pool *pgxpool.Pool }

func (a *pgxExecPoolAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return a.pool.QueryRow(ctx, sql, args...)
}
func (a *pgxExecPoolAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return a.pool.Query(ctx, sql, args...)
}

func isFKViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}
