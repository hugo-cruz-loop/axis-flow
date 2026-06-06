package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"axis-flow-back/internal/catalogos/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HrAbsenceTypeRepo handles catalogos.catalog_hr_absence_types.
type HrAbsenceTypeRepo struct{ pool *pgxpool.Pool }

func NewHrAbsenceTypeRepo(pool *pgxpool.Pool) *HrAbsenceTypeRepo {
	return &HrAbsenceTypeRepo{pool: pool}
}

func (r *HrAbsenceTypeRepo) List(ctx context.Context) ([]domain.HrAbsenceType, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, requires_justification, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_hr_absence_types WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("HrAbsenceTypeRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.HrAbsenceType
	for rows.Next() {
		var h domain.HrAbsenceType
		if err := rows.Scan(&h.ID, &h.Code, &h.Name, &h.RequiresJustification,
			&h.DeletedAt, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("HrAbsenceTypeRepo.List scan: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *HrAbsenceTypeRepo) FindByID(ctx context.Context, id int64) (*domain.HrAbsenceType, error) {
	var h domain.HrAbsenceType
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, requires_justification, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_hr_absence_types WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&h.ID, &h.Code, &h.Name, &h.RequiresJustification, &h.DeletedAt, &h.CreatedAt, &h.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("HrAbsenceTypeRepo.FindByID: %w", err)
	}
	return &h, nil
}

func (r *HrAbsenceTypeRepo) Create(ctx context.Context, h *domain.HrAbsenceType) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_hr_absence_types (code, name, requires_justification)
		 VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		h.Code, h.Name, h.RequiresJustification).Scan(&h.ID, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("HrAbsenceTypeRepo.Create: %w", err)
	}
	return nil
}

func (r *HrAbsenceTypeRepo) Update(ctx context.Context, h *domain.HrAbsenceType) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_hr_absence_types SET code=$1, name=$2, requires_justification=$3
		 WHERE id=$4 AND deleted_at IS NULL`,
		h.Code, h.Name, h.RequiresJustification, h.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("HrAbsenceTypeRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *HrAbsenceTypeRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_hr_absence_types SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("HrAbsenceTypeRepo.Delete: %w", err)
	}
	return nil
}

// JobCategoryRepo handles catalogos.catalog_job_categories.
// Name is always lowercased before any DB write.
type JobCategoryRepo struct{ pool *pgxpool.Pool }

func NewJobCategoryRepo(pool *pgxpool.Pool) *JobCategoryRepo { return &JobCategoryRepo{pool: pool} }

func (r *JobCategoryRepo) List(ctx context.Context) ([]domain.JobCategory, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_job_categories WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("JobCategoryRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.JobCategory
	for rows.Next() {
		var jc domain.JobCategory
		if err := rows.Scan(&jc.ID, &jc.Code, &jc.Name, &jc.DeletedAt, &jc.CreatedAt, &jc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("JobCategoryRepo.List scan: %w", err)
		}
		out = append(out, jc)
	}
	return out, rows.Err()
}

func (r *JobCategoryRepo) FindByID(ctx context.Context, id int64) (*domain.JobCategory, error) {
	var jc domain.JobCategory
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_job_categories WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&jc.ID, &jc.Code, &jc.Name, &jc.DeletedAt, &jc.CreatedAt, &jc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("JobCategoryRepo.FindByID: %w", err)
	}
	return &jc, nil
}

func (r *JobCategoryRepo) Create(ctx context.Context, jc *domain.JobCategory) error {
	jc.Name = strings.ToLower(jc.Name)
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_job_categories (code, name)
		 VALUES ($1,$2) RETURNING id, created_at, updated_at`,
		jc.Code, jc.Name).Scan(&jc.ID, &jc.CreatedAt, &jc.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("JobCategoryRepo.Create: %w", err)
	}
	return nil
}

func (r *JobCategoryRepo) Update(ctx context.Context, jc *domain.JobCategory) error {
	jc.Name = strings.ToLower(jc.Name)
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_job_categories SET code=$1, name=$2
		 WHERE id=$3 AND deleted_at IS NULL`,
		jc.Code, jc.Name, jc.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("JobCategoryRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *JobCategoryRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_job_categories SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("JobCategoryRepo.Delete: %w", err)
	}
	return nil
}

// JobTypeRepo handles catalogos.catalog_job_types.
// Name is always lowercased before any DB write.
type JobTypeRepo struct{ pool *pgxpool.Pool }

func NewJobTypeRepo(pool *pgxpool.Pool) *JobTypeRepo { return &JobTypeRepo{pool: pool} }

func (r *JobTypeRepo) List(ctx context.Context) ([]domain.JobType, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_job_types WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("JobTypeRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.JobType
	for rows.Next() {
		var jt domain.JobType
		if err := rows.Scan(&jt.ID, &jt.Code, &jt.Name, &jt.DeletedAt, &jt.CreatedAt, &jt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("JobTypeRepo.List scan: %w", err)
		}
		out = append(out, jt)
	}
	return out, rows.Err()
}

func (r *JobTypeRepo) FindByID(ctx context.Context, id int64) (*domain.JobType, error) {
	var jt domain.JobType
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_job_types WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&jt.ID, &jt.Code, &jt.Name, &jt.DeletedAt, &jt.CreatedAt, &jt.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("JobTypeRepo.FindByID: %w", err)
	}
	return &jt, nil
}

func (r *JobTypeRepo) Create(ctx context.Context, jt *domain.JobType) error {
	jt.Name = strings.ToLower(jt.Name)
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_job_types (code, name)
		 VALUES ($1,$2) RETURNING id, created_at, updated_at`,
		jt.Code, jt.Name).Scan(&jt.ID, &jt.CreatedAt, &jt.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("JobTypeRepo.Create: %w", err)
	}
	return nil
}

func (r *JobTypeRepo) Update(ctx context.Context, jt *domain.JobType) error {
	jt.Name = strings.ToLower(jt.Name)
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_job_types SET code=$1, name=$2
		 WHERE id=$3 AND deleted_at IS NULL`,
		jt.Code, jt.Name, jt.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("JobTypeRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *JobTypeRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_job_types SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("JobTypeRepo.Delete: %w", err)
	}
	return nil
}
