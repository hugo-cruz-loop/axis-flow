// Package repository provides pgx-backed implementations of the service repository interfaces.
package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// PgxRoleRepository is a PostgreSQL implementation for role queries.
type PgxRoleRepository struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewPgxRoleRepository creates a new PgxRoleRepository.
func NewPgxRoleRepository(pool *pgxpool.Pool, rc *redis.Client) *PgxRoleRepository {
	return &PgxRoleRepository{pool: pool, redis: rc}
}

// ListAll returns all roles with their assigned permission count.
func (r *PgxRoleRepository) ListAll(ctx context.Context) ([]domain.RoleWithCount, error) {
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

	var roles []domain.RoleWithCount
	for rows.Next() {
		var rc domain.RoleWithCount
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

// Create inserts a new role. Returns domain.ErrDuplicateCode on unique constraint violation.
func (r *PgxRoleRepository) Create(ctx context.Context, role *domain.Role) error {
	if role.ID == uuid.Nil {
		role.ID = uuid.New()
	}
	const q = `
		INSERT INTO users.identity_roles (id, code, name, description, scope, is_system)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, q,
		role.ID, role.Code, role.Name, role.Description, role.Scope, role.IsSystem,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("role_repository.Create: %w", err)
	}
	return nil
}

// Update updates the name and description of a role.
// Returns domain.ErrSystemRole if the role is a system role.
func (r *PgxRoleRepository) Update(ctx context.Context, role *domain.Role) error {
	var isSystem bool
	const checkQ = `SELECT is_system FROM users.identity_roles WHERE id = $1`
	if err := r.pool.QueryRow(ctx, checkQ, role.ID).Scan(&isSystem); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("role_repository.Update check: %w", err)
	}
	if isSystem {
		return domain.ErrSystemRole
	}

	const q = `
		UPDATE users.identity_roles
		SET name = $2, description = $3, updated_at = NOW()
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, role.ID, role.Name, role.Description)
	if err != nil {
		return fmt.Errorf("role_repository.Update: %w", err)
	}
	return nil
}

// Delete removes a role.
// Returns domain.ErrSystemRole if is_system=true, domain.ErrRoleHasUsers if users are assigned.
func (r *PgxRoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var isSystem bool
	const checkSystemQ = `SELECT is_system FROM users.identity_roles WHERE id = $1`
	if err := r.pool.QueryRow(ctx, checkSystemQ, id).Scan(&isSystem); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("role_repository.Delete check system: %w", err)
	}
	if isSystem {
		return domain.ErrSystemRole
	}

	var count int
	const checkUsersQ = `SELECT COUNT(*) FROM users.identity_user_roles WHERE role_id = $1`
	if err := r.pool.QueryRow(ctx, checkUsersQ, id).Scan(&count); err != nil {
		return fmt.Errorf("role_repository.Delete check users: %w", err)
	}
	if count > 0 {
		return domain.ErrRoleHasUsers
	}

	const q = `DELETE FROM users.identity_roles WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("role_repository.Delete: %w", err)
	}
	return nil
}

// FindByID returns a role by its UUID.
func (r *PgxRoleRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	const q = `SELECT id, code, name, COALESCE(description,''), scope, is_system
	           FROM users.identity_roles WHERE id = $1`
	var role domain.Role
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&role.ID, &role.Code, &role.Name, &role.Description, &role.Scope, &role.IsSystem,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("role_repository.FindByID: %w", err)
	}
	return &role, nil
}

// invalidateRoleUsersCache deletes role/permission cache keys for all users that have roleID.
// Errors are logged and swallowed (fail-open) to avoid blocking the response.
func (r *PgxRoleRepository) invalidateRoleUsersCache(ctx context.Context, roleID uuid.UUID) {
	const q = `SELECT user_id FROM users.identity_user_roles WHERE role_id = $1`
	rows, err := r.pool.Query(ctx, q, roleID)
	if err != nil {
		slog.WarnContext(ctx, "role_repo: failed to query role users for cache invalidation",
			slog.String("error", err.Error()))
		return
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			continue
		}
		uid := userID.String()
		keys = append(keys, "user:"+uid+":roles", "user:"+uid+":permissions")
	}

	if len(keys) == 0 {
		return
	}

	pipe := r.redis.Pipeline()
	for _, k := range keys {
		pipe.Del(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		slog.WarnContext(ctx, "role_repo: cache invalidation failed (fail-open)",
			slog.String("error", err.Error()),
			slog.String("role_id", roleID.String()))
	}
}

// AssignPermission adds a role-permission mapping.
// Uses SELECT ... FOR UPDATE on the role to prevent concurrent updates.
func (r *PgxRoleRepository) AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("role_repository.AssignPermission begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const lockQ = `SELECT id FROM users.identity_roles WHERE id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, lockQ, roleID).Scan(new(uuid.UUID)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("role_repository.AssignPermission lock: %w", err)
	}

	const q = `
		INSERT INTO users.identity_role_permissions (role_id, permission_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`
	if _, err := tx.Exec(ctx, q, roleID, permissionID); err != nil {
		return fmt.Errorf("role_repository.AssignPermission insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("role_repository.AssignPermission commit: %w", err)
	}

	r.invalidateRoleUsersCache(ctx, roleID)
	return nil
}

// RevokePermission removes a role-permission mapping.
func (r *PgxRoleRepository) RevokePermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	const q = `DELETE FROM users.identity_role_permissions WHERE role_id = $1 AND permission_id = $2`
	_, err := r.pool.Exec(ctx, q, roleID, permissionID)
	if err != nil {
		return fmt.Errorf("role_repository.RevokePermission: %w", err)
	}

	r.invalidateRoleUsersCache(ctx, roleID)
	return nil
}

// ListPermissionsByRoleID returns permissions for a role by its UUID.
func (r *PgxRoleRepository) ListPermissionsByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	const q = `
		SELECT p.id, p.code, p.name, COALESCE(p.module,''), COALESCE(p.description,'')
		FROM users.identity_permissions p
		JOIN users.identity_role_permissions rp ON rp.permission_id = p.id
		WHERE rp.role_id = $1
		ORDER BY p.module ASC, p.name ASC`

	rows, err := r.pool.Query(ctx, q, roleID)
	if err != nil {
		return nil, fmt.Errorf("role_repository.ListPermissionsByRoleID: %w", err)
	}
	defer rows.Close()

	var perms []domain.Permission
	for rows.Next() {
		var p domain.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Module, &p.Description); err != nil {
			return nil, fmt.Errorf("role_repository.ListPermissionsByRoleID scan: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("role_repository.ListPermissionsByRoleID rows: %w", err)
	}
	return perms, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique constraint violation (code 23505).
func isUniqueViolation(err error) bool {
	return err != nil && containsString(err.Error(), "23505")
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
