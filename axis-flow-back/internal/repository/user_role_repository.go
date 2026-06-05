package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// PgxUserRoleRepository manages user-role assignments with Redis cache invalidation.
type PgxUserRoleRepository struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewPgxUserRoleRepository creates a new PgxUserRoleRepository.
func NewPgxUserRoleRepository(pool *pgxpool.Pool, rc *redis.Client) *PgxUserRoleRepository {
	return &PgxUserRoleRepository{pool: pool, redis: rc}
}

// Assign adds a role to a user and invalidates the user's role/permission cache.
func (r *PgxUserRoleRepository) Assign(ctx context.Context, userID, roleID uuid.UUID, assignedBy uuid.UUID) error {
	const q = `
		INSERT INTO users.identity_user_roles (user_id, role_id, assigned_by, assigned_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT DO NOTHING`

	if _, err := r.pool.Exec(ctx, q, userID, roleID, assignedBy); err != nil {
		return fmt.Errorf("user_role_repository.Assign: %w", err)
	}

	r.invalidateCache(ctx, userID)
	return nil
}

// Revoke removes a role from a user and invalidates the user's role/permission cache.
func (r *PgxUserRoleRepository) Revoke(ctx context.Context, userID, roleID uuid.UUID) error {
	const q = `DELETE FROM users.identity_user_roles WHERE user_id = $1 AND role_id = $2`
	if _, err := r.pool.Exec(ctx, q, userID, roleID); err != nil {
		return fmt.Errorf("user_role_repository.Revoke: %w", err)
	}

	r.invalidateCache(ctx, userID)
	return nil
}

// invalidateCache deletes the user's role and permission cache keys.
// Errors are logged and swallowed (fail open) to avoid blocking the response.
func (r *PgxUserRoleRepository) invalidateCache(ctx context.Context, userID uuid.UUID) {
	keys := []string{
		fmt.Sprintf("user:%s:roles", userID),
		fmt.Sprintf("user:%s:permissions", userID),
	}
	if err := r.redis.Del(ctx, keys...).Err(); err != nil {
		slog.WarnContext(ctx, "user_role_repository: redis invalidation failed",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
	}
}
