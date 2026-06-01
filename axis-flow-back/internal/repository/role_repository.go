// Package repository provides pgx-backed implementations of the service repository interfaces.
package repository

import (
	"context"
	"fmt"

	"axis-flow-back/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RoleWithCount wraps a Role with its assigned permission count.
type RoleWithCount struct {
	domain.Role
	PermissionCount int
}

// PgxRoleRepository is a PostgreSQL implementation for role queries.
type PgxRoleRepository struct {
	pool *pgxpool.Pool
}

// NewPgxRoleRepository creates a new PgxRoleRepository.
func NewPgxRoleRepository(pool *pgxpool.Pool) *PgxRoleRepository {
	return &PgxRoleRepository{pool: pool}
}

// ListAll returns all roles with their assigned permission count.
func (r *PgxRoleRepository) ListAll(ctx context.Context) ([]RoleWithCount, error) {
	const q = `
		SELECT r.id, r.code, r.name, COALESCE(r.description,''), r.scope, r.is_system,
		       COUNT(rp.permission_id) AS permission_count
		FROM users.identity_roles r
		LEFT JOIN users.identity_role_permissions rp ON rp.role_id = r.id
		GROUP BY r.id, r.code, r.name, r.description, r.scope, r.is_system
		ORDER BY r.is_system DESC, r.name ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("role_repository.ListAll: %w", err)
	}
	defer rows.Close()

	var roles []RoleWithCount
	for rows.Next() {
		var rc RoleWithCount
		if err := rows.Scan(
			&rc.ID, &rc.Code, &rc.Name, &rc.Description, &rc.Scope, &rc.IsSystem,
			&rc.PermissionCount,
		); err != nil {
			return nil, fmt.Errorf("role_repository.ListAll scan: %w", err)
		}
		roles = append(roles, rc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("role_repository.ListAll rows: %w", err)
	}
	return roles, nil
}

// ListPermissionsByRole returns all permissions assigned to a role identified by its code.
func (r *PgxRoleRepository) ListPermissionsByRole(ctx context.Context, roleCode string) ([]domain.Permission, error) {
	const q = `
		SELECT p.id, p.code, p.name, COALESCE(p.module,''), COALESCE(p.description,'')
		FROM users.identity_permissions p
		JOIN users.identity_role_permissions rp ON rp.permission_id = p.id
		JOIN users.identity_roles r ON r.id = rp.role_id
		WHERE r.code = $1
		ORDER BY p.module ASC, p.name ASC`

	rows, err := r.pool.Query(ctx, q, roleCode)
	if err != nil {
		return nil, fmt.Errorf("role_repository.ListPermissionsByRole: %w", err)
	}
	defer rows.Close()

	var perms []domain.Permission
	for rows.Next() {
		var p domain.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Module, &p.Description); err != nil {
			return nil, fmt.Errorf("role_repository.ListPermissionsByRole scan: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("role_repository.ListPermissionsByRole rows: %w", err)
	}
	return perms, nil
}
