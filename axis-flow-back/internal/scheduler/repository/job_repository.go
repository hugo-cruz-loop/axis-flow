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
// JobRepository — interface.
// ---------------------------------------------------------------------------

// JobRepository defines persistence operations for scheduler.scheduler_jobs.
type JobRepository interface {
	// GetByKey returns a job by its unique (active) job_key.
	// Returns scheduler.ErrNotFound when the job is absent.
	GetByKey(ctx context.Context, key string) (scheduler.Job, error)
	// GetByID returns a job by its UUID primary key.
	// Returns scheduler.ErrNotFound when the job is absent.
	GetByID(ctx context.Context, id uuid.UUID) (scheduler.Job, error)
	// List returns jobs matching the optional filter, paginated.
	List(ctx context.Context, filter scheduler.JobListFilter) ([]scheduler.Job, int, error)
	// ListActive returns all active (non-soft-deleted) jobs ordered by job_key.
	ListActive(ctx context.Context) ([]scheduler.Job, error)
	// Create persists a new job and returns the stored record.
	Create(ctx context.Context, job scheduler.Job) (scheduler.Job, error)
	// UpdatePause toggles the is_active flag of a job.
	// Returns scheduler.ErrNotFound when the job is absent.
	UpdatePause(ctx context.Context, id uuid.UUID, paused bool) (scheduler.Job, error)
	// UpdateLastRun records the last and next run timestamps for a job.
	// Both arguments are optional pointers: pass nil to leave the column
	// unchanged. Returns scheduler.ErrNotFound when the job is absent.
	UpdateLastRun(ctx context.Context, id uuid.UUID, last, next *time.Time) (scheduler.Job, error)
}

// ---------------------------------------------------------------------------
// pgJobRepository — PostgreSQL implementation.
// ---------------------------------------------------------------------------

// pgJobRepository is a PostgreSQL-backed JobRepository.
type pgJobRepository struct {
	db jobDB
}

// NewPgJobRepository creates a PostgreSQL job repository.
func NewPgJobRepository(pool *pgxpool.Pool) JobRepository {
	return &pgJobRepository{db: &pgxJobPoolAdapter{pool: pool}}
}

const jobSelectColumns = `job_id, job_key, cron_expression, interval_seconds, job_class,
    module, description, is_active, metadata, last_run_at, next_run_at,
    created_at, updated_at, deleted_at`

func (r *pgJobRepository) GetByKey(ctx context.Context, key string) (scheduler.Job, error) {
	const q = `SELECT ` + jobSelectColumns + `
        FROM scheduler.scheduler_jobs
        WHERE job_key = $1 AND deleted_at IS NULL`
	var j scheduler.Job
	err := r.db.QueryRow(ctx, q, key).Scan(
		&j.JobID, &j.JobKey, &j.CronExpression, &j.IntervalSeconds,
		&j.JobClass, &j.Module, &j.Description, &j.IsActive, &j.Metadata,
		&j.LastRunAt, &j.NextRunAt, &j.CreatedAt, &j.UpdatedAt, &j.DeletedAt,
	)
	if err != nil {
		return scheduler.Job{}, mapNotFound("job_repository.GetByKey", err)
	}
	return j, nil
}

func (r *pgJobRepository) GetByID(ctx context.Context, id uuid.UUID) (scheduler.Job, error) {
	const q = `SELECT ` + jobSelectColumns + `
        FROM scheduler.scheduler_jobs
        WHERE job_id = $1 AND deleted_at IS NULL`
	var j scheduler.Job
	err := r.db.QueryRow(ctx, q, id).Scan(
		&j.JobID, &j.JobKey, &j.CronExpression, &j.IntervalSeconds,
		&j.JobClass, &j.Module, &j.Description, &j.IsActive, &j.Metadata,
		&j.LastRunAt, &j.NextRunAt, &j.CreatedAt, &j.UpdatedAt, &j.DeletedAt,
	)
	if err != nil {
		return scheduler.Job{}, mapNotFound("job_repository.GetByID", err)
	}
	return j, nil
}

func (r *pgJobRepository) List(ctx context.Context, filter scheduler.JobListFilter) ([]scheduler.Job, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Build dynamic WHERE clause with positional parameters. Using a
	// small slice of conditions keeps the query static while still
	// supporting optional filters.
	conds := []string{"deleted_at IS NULL"}
	args := []any{}
	idx := 1
	if filter.Module != nil {
		conds = append(conds, fmt.Sprintf("module = $%d", idx))
		args = append(args, *filter.Module)
		idx++
	}
	if filter.IsActive != nil {
		conds = append(conds, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *filter.IsActive)
		idx++
	}
	where := " WHERE " + joinAnd(conds)

	countQ := `SELECT COUNT(*) FROM scheduler.scheduler_jobs` + where
	var total int
	if err := r.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("job_repository.List count: %w", err)
	}

	listQ := fmt.Sprintf(`SELECT %s FROM scheduler.scheduler_jobs%s
        ORDER BY job_key
        LIMIT $%d OFFSET $%d`, jobSelectColumns, where, idx, idx+1)
	listArgs := append(args, limit, offset)
	rows, err := r.db.Query(ctx, listQ, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("job_repository.List query: %w", err)
	}
	defer rows.Close()

	out := []scheduler.Job{}
	for rows.Next() {
		var j scheduler.Job
		if scanErr := rows.Scan(
			&j.JobID, &j.JobKey, &j.CronExpression, &j.IntervalSeconds,
			&j.JobClass, &j.Module, &j.Description, &j.IsActive, &j.Metadata,
			&j.LastRunAt, &j.NextRunAt, &j.CreatedAt, &j.UpdatedAt, &j.DeletedAt,
		); scanErr != nil {
			return nil, 0, fmt.Errorf("job_repository.List scan: %w", scanErr)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("job_repository.List rows: %w", err)
	}
	return out, total, nil
}

func (r *pgJobRepository) ListActive(ctx context.Context) ([]scheduler.Job, error) {
	const q = `SELECT ` + jobSelectColumns + `
        FROM scheduler.scheduler_jobs
        WHERE is_active = TRUE AND deleted_at IS NULL
        ORDER BY job_key`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("job_repository.ListActive: %w", err)
	}
	defer rows.Close()

	out := []scheduler.Job{}
	for rows.Next() {
		var j scheduler.Job
		if scanErr := rows.Scan(
			&j.JobID, &j.JobKey, &j.CronExpression, &j.IntervalSeconds,
			&j.JobClass, &j.Module, &j.Description, &j.IsActive, &j.Metadata,
			&j.LastRunAt, &j.NextRunAt, &j.CreatedAt, &j.UpdatedAt, &j.DeletedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("job_repository.ListActive scan: %w", scanErr)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("job_repository.ListActive rows: %w", err)
	}
	return out, nil
}

func (r *pgJobRepository) Create(ctx context.Context, job scheduler.Job) (scheduler.Job, error) {
	metadata := job.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	const q = `INSERT INTO scheduler.scheduler_jobs
        (job_key, cron_expression, interval_seconds, job_class, module,
         description, is_active, metadata)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING ` + jobSelectColumns
	var j scheduler.Job
	err := r.db.QueryRow(ctx, q,
		job.JobKey, job.CronExpression, job.IntervalSeconds,
		job.JobClass, job.Module, job.Description, job.IsActive, metadata,
	).Scan(
		&j.JobID, &j.JobKey, &j.CronExpression, &j.IntervalSeconds,
		&j.JobClass, &j.Module, &j.Description, &j.IsActive, &j.Metadata,
		&j.LastRunAt, &j.NextRunAt, &j.CreatedAt, &j.UpdatedAt, &j.DeletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return scheduler.Job{}, fmt.Errorf("job_repository.Create: %w", scheduler.ErrConflict)
		}
		return scheduler.Job{}, fmt.Errorf("job_repository.Create: %w", err)
	}
	return j, nil
}

func (r *pgJobRepository) UpdatePause(ctx context.Context, id uuid.UUID, paused bool) (scheduler.Job, error) {
	const q = `UPDATE scheduler.scheduler_jobs
        SET is_active = NOT $2
        WHERE job_id = $1 AND deleted_at IS NULL
        RETURNING ` + jobSelectColumns
	var j scheduler.Job
	err := r.db.QueryRow(ctx, q, id, paused).Scan(
		&j.JobID, &j.JobKey, &j.CronExpression, &j.IntervalSeconds,
		&j.JobClass, &j.Module, &j.Description, &j.IsActive, &j.Metadata,
		&j.LastRunAt, &j.NextRunAt, &j.CreatedAt, &j.UpdatedAt, &j.DeletedAt,
	)
	if err != nil {
		return scheduler.Job{}, mapNotFound("job_repository.UpdatePause", err)
	}
	return j, nil
}

// UpdateLastRun records the last_run_at and next_run_at timestamps for a job.
// Both arguments are optional pointers: pass nil to leave the column unchanged.
func (r *pgJobRepository) UpdateLastRun(ctx context.Context, id uuid.UUID, last, next *time.Time) (scheduler.Job, error) {
	const q = `UPDATE scheduler.scheduler_jobs
        SET last_run_at = $2,
            next_run_at = $3
        WHERE job_id = $1 AND deleted_at IS NULL
        RETURNING ` + jobSelectColumns
	var j scheduler.Job
	err := r.db.QueryRow(ctx, q, id, last, next).Scan(
		&j.JobID, &j.JobKey, &j.CronExpression, &j.IntervalSeconds,
		&j.JobClass, &j.Module, &j.Description, &j.IsActive, &j.Metadata,
		&j.LastRunAt, &j.NextRunAt, &j.CreatedAt, &j.UpdatedAt, &j.DeletedAt,
	)
	if err != nil {
		return scheduler.Job{}, mapNotFound("job_repository.UpdateLastRun", err)
	}
	return j, nil
}

// ---------------------------------------------------------------------------
// Internal helpers — adapters and error mapping.
// ---------------------------------------------------------------------------

// jobDB abstracts the pgx surface used by the repository so tests can plug
// in transaction-bound runners or in-memory fakes.
type jobDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// pgxJobPoolAdapter adapts a *pgxpool.Pool to the jobDB interface.
type pgxJobPoolAdapter struct{ pool *pgxpool.Pool }

func (a *pgxJobPoolAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return a.pool.QueryRow(ctx, sql, args...)
}
func (a *pgxJobPoolAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return a.pool.Query(ctx, sql, args...)
}

func mapNotFound(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return scheduler.ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}
