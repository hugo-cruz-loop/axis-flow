package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/catalogos/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WorkflowStatusRepo handles catalogos.catalog_workflow_statuses.
type WorkflowStatusRepo struct{ pool *pgxpool.Pool }

func NewWorkflowStatusRepo(pool *pgxpool.Pool) *WorkflowStatusRepo {
	return &WorkflowStatusRepo{pool: pool}
}

func (r *WorkflowStatusRepo) List(ctx context.Context) ([]domain.WorkflowStatus, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, role_id, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_workflow_statuses WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("WorkflowStatusRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowStatus
	for rows.Next() {
		var w domain.WorkflowStatus
		if err := rows.Scan(&w.ID, &w.Code, &w.Name, &w.RoleID, &w.DeletedAt, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("WorkflowStatusRepo.List scan: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *WorkflowStatusRepo) ListByRoleID(ctx context.Context, roleID int16) ([]domain.WorkflowStatus, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, role_id, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_workflow_statuses
		 WHERE role_id=$1 AND deleted_at IS NULL ORDER BY name ASC`, roleID)
	if err != nil {
		return nil, fmt.Errorf("WorkflowStatusRepo.ListByRoleID: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowStatus
	for rows.Next() {
		var w domain.WorkflowStatus
		if err := rows.Scan(&w.ID, &w.Code, &w.Name, &w.RoleID, &w.DeletedAt, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("WorkflowStatusRepo.ListByRoleID scan: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *WorkflowStatusRepo) FindByID(ctx context.Context, id int64) (*domain.WorkflowStatus, error) {
	var w domain.WorkflowStatus
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, role_id, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_workflow_statuses WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&w.ID, &w.Code, &w.Name, &w.RoleID, &w.DeletedAt, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("WorkflowStatusRepo.FindByID: %w", err)
	}
	return &w, nil
}

func (r *WorkflowStatusRepo) Create(ctx context.Context, w *domain.WorkflowStatus) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_workflow_statuses (code, name, role_id)
		 VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		w.Code, w.Name, w.RoleID).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("WorkflowStatusRepo.Create: %w", err)
	}
	return nil
}

func (r *WorkflowStatusRepo) Update(ctx context.Context, w *domain.WorkflowStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_workflow_statuses SET code=$1, name=$2, role_id=$3
		 WHERE id=$4 AND deleted_at IS NULL`,
		w.Code, w.Name, w.RoleID, w.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("WorkflowStatusRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *WorkflowStatusRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_workflow_statuses SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("WorkflowStatusRepo.Delete: %w", err)
	}
	return nil
}

// ComplaintTypeRepo handles catalogos.catalog_complaint_types.
type ComplaintTypeRepo struct{ pool *pgxpool.Pool }

func NewComplaintTypeRepo(pool *pgxpool.Pool) *ComplaintTypeRepo {
	return &ComplaintTypeRepo{pool: pool}
}

func (r *ComplaintTypeRepo) List(ctx context.Context) ([]domain.ComplaintType, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, description, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_complaint_types WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("ComplaintTypeRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.ComplaintType
	for rows.Next() {
		var c domain.ComplaintType
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &c.Description, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("ComplaintTypeRepo.List scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ComplaintTypeRepo) FindByID(ctx context.Context, id int64) (*domain.ComplaintType, error) {
	var c domain.ComplaintType
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, description, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_complaint_types WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&c.ID, &c.Code, &c.Name, &c.Description, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ComplaintTypeRepo.FindByID: %w", err)
	}
	return &c, nil
}

func (r *ComplaintTypeRepo) Create(ctx context.Context, c *domain.ComplaintType) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_complaint_types (code, name, description)
		 VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		c.Code, c.Name, c.Description).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("ComplaintTypeRepo.Create: %w", err)
	}
	return nil
}

func (r *ComplaintTypeRepo) Update(ctx context.Context, c *domain.ComplaintType) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_complaint_types SET code=$1, name=$2, description=$3
		 WHERE id=$4 AND deleted_at IS NULL`,
		c.Code, c.Name, c.Description, c.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("ComplaintTypeRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ComplaintTypeRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_complaint_types SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("ComplaintTypeRepo.Delete: %w", err)
	}
	return nil
}

// ServiceRepo handles catalogos.catalog_services.
type ServiceRepo struct{ pool *pgxpool.Pool }

func NewServiceRepo(pool *pgxpool.Pool) *ServiceRepo { return &ServiceRepo{pool: pool} }

func (r *ServiceRepo) List(ctx context.Context) ([]domain.Service, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, description, price, is_active, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_services WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("ServiceRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.Service
	for rows.Next() {
		var s domain.Service
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.Description, &s.Price, &s.IsActive,
			&s.DeletedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("ServiceRepo.List scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ServiceRepo) FindByID(ctx context.Context, id int64) (*domain.Service, error) {
	var s domain.Service
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, description, price, is_active, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_services WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&s.ID, &s.Code, &s.Name, &s.Description, &s.Price, &s.IsActive, &s.DeletedAt, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ServiceRepo.FindByID: %w", err)
	}
	return &s, nil
}

func (r *ServiceRepo) Create(ctx context.Context, s *domain.Service) error {
	if s.Price < 0 {
		return fmt.Errorf("ServiceRepo.Create: price must be >= 0")
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_services (code, name, description, price, is_active)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at, updated_at`,
		s.Code, s.Name, s.Description, s.Price, s.IsActive).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("ServiceRepo.Create: %w", err)
	}
	return nil
}

func (r *ServiceRepo) Update(ctx context.Context, s *domain.Service) error {
	if s.Price < 0 {
		return fmt.Errorf("ServiceRepo.Update: price must be >= 0")
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_services SET code=$1, name=$2, description=$3, price=$4, is_active=$5
		 WHERE id=$6 AND deleted_at IS NULL`,
		s.Code, s.Name, s.Description, s.Price, s.IsActive, s.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("ServiceRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ServiceRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_services SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("ServiceRepo.Delete: %w", err)
	}
	return nil
}

// SubscriptionPlanRepo handles catalogos.catalog_subscription_plans.
type SubscriptionPlanRepo struct{ pool *pgxpool.Pool }

func NewSubscriptionPlanRepo(pool *pgxpool.Pool) *SubscriptionPlanRepo {
	return &SubscriptionPlanRepo{pool: pool}
}

func (r *SubscriptionPlanRepo) List(ctx context.Context) ([]domain.SubscriptionPlan, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, amount, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_subscription_plans WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("SubscriptionPlanRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.SubscriptionPlan
	for rows.Next() {
		var p domain.SubscriptionPlan
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Amount, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("SubscriptionPlanRepo.List scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *SubscriptionPlanRepo) FindByID(ctx context.Context, id int64) (*domain.SubscriptionPlan, error) {
	var p domain.SubscriptionPlan
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, amount, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_subscription_plans WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&p.ID, &p.Code, &p.Name, &p.Amount, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("SubscriptionPlanRepo.FindByID: %w", err)
	}
	return &p, nil
}

func (r *SubscriptionPlanRepo) Create(ctx context.Context, p *domain.SubscriptionPlan) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_subscription_plans (code, name, amount)
		 VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		p.Code, p.Name, p.Amount).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("SubscriptionPlanRepo.Create: %w", err)
	}
	return nil
}

func (r *SubscriptionPlanRepo) Update(ctx context.Context, p *domain.SubscriptionPlan) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_subscription_plans SET code=$1, name=$2, amount=$3
		 WHERE id=$4 AND deleted_at IS NULL`,
		p.Code, p.Name, p.Amount, p.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("SubscriptionPlanRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SubscriptionPlanRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_subscription_plans SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("SubscriptionPlanRepo.Delete: %w", err)
	}
	return nil
}

// DatePeriodicityRepo handles catalogos.catalog_date_periodicities.
type DatePeriodicityRepo struct{ pool *pgxpool.Pool }

func NewDatePeriodicityRepo(pool *pgxpool.Pool) *DatePeriodicityRepo {
	return &DatePeriodicityRepo{pool: pool}
}

func (r *DatePeriodicityRepo) List(ctx context.Context) ([]domain.DatePeriodicity, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_date_periodicities WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("DatePeriodicityRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.DatePeriodicity
	for rows.Next() {
		var d domain.DatePeriodicity
		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.DeletedAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("DatePeriodicityRepo.List scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DatePeriodicityRepo) FindByID(ctx context.Context, id int64) (*domain.DatePeriodicity, error) {
	var d domain.DatePeriodicity
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_date_periodicities WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&d.ID, &d.Code, &d.Name, &d.DeletedAt, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("DatePeriodicityRepo.FindByID: %w", err)
	}
	return &d, nil
}

func (r *DatePeriodicityRepo) Create(ctx context.Context, d *domain.DatePeriodicity) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_date_periodicities (code, name)
		 VALUES ($1,$2) RETURNING id, created_at, updated_at`,
		d.Code, d.Name).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("DatePeriodicityRepo.Create: %w", err)
	}
	return nil
}

func (r *DatePeriodicityRepo) Update(ctx context.Context, d *domain.DatePeriodicity) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_date_periodicities SET code=$1, name=$2
		 WHERE id=$3 AND deleted_at IS NULL`,
		d.Code, d.Name, d.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("DatePeriodicityRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *DatePeriodicityRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_date_periodicities SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("DatePeriodicityRepo.Delete: %w", err)
	}
	return nil
}
