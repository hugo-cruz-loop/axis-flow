package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxPermissionRepository is a PostgreSQL implementation for permission operations.
type PgxPermissionRepository struct {
	pool *pgxpool.Pool
}

// NewPgxPermissionRepository creates a new PgxPermissionRepository.
func NewPgxPermissionRepository(pool *pgxpool.Pool) *PgxPermissionRepository {
	return &PgxPermissionRepository{pool: pool}
}

// ListAll returns all permissions, optionally filtered by module.
func (r *PgxPermissionRepository) ListAll(ctx context.Context, module string) ([]domain.Permission, error) {
	const q = `
		SELECT id, code, name, COALESCE(module,''), COALESCE(description,'')
		FROM users.identity_permissions
		WHERE ($1 = '' OR module = $1)
		ORDER BY module ASC, name ASC`

	rows, err := r.pool.Query(ctx, q, module)
	if err != nil {
		return nil, fmt.Errorf("permission_repository.ListAll: %w", err)
	}
	defer rows.Close()

	var perms []domain.Permission
	for rows.Next() {
		var p domain.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Module, &p.Description); err != nil {
			return nil, fmt.Errorf("permission_repository.ListAll scan: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("permission_repository.ListAll rows: %w", err)
	}
	return perms, nil
}

// FindByID returns a single permission by ID.
func (r *PgxPermissionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	const q = `
		SELECT id, code, name, COALESCE(module,''), COALESCE(description,'')
		FROM users.identity_permissions
		WHERE id = $1`

	var p domain.Permission
	if err := r.pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.Code, &p.Name, &p.Module, &p.Description); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("permission_repository.FindByID: %w", err)
	}
	return &p, nil
}

// Create inserts a new permission. Returns domain.ErrDuplicateCode on unique violation.
func (r *PgxPermissionRepository) Create(ctx context.Context, p *domain.Permission) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	const q = `
		INSERT INTO users.identity_permissions (id, code, name, module, description)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.pool.Exec(ctx, q, p.ID, p.Code, p.Name, p.Module, p.Description)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("permission_repository.Create: %w", err)
	}
	return nil
}

// Update updates the name, module, and description of a permission.
func (r *PgxPermissionRepository) Update(ctx context.Context, p *domain.Permission) error {
	const q = `
		UPDATE users.identity_permissions
		SET name = $2, module = $3, description = $4, updated_at = NOW()
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, p.ID, p.Name, p.Module, p.Description)
	if err != nil {
		return fmt.Errorf("permission_repository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete removes a permission.
// Returns domain.ErrPermissionInUse if the permission is assigned to any role.
func (r *PgxPermissionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var count int
	const checkQ = `SELECT COUNT(*) FROM users.identity_role_permissions WHERE permission_id = $1`
	if err := r.pool.QueryRow(ctx, checkQ, id).Scan(&count); err != nil {
		return fmt.Errorf("permission_repository.Delete check: %w", err)
	}
	if count > 0 {
		return domain.ErrPermissionInUse
	}

	const q = `DELETE FROM users.identity_permissions WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("permission_repository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
