// Package repository provides pgx-backed implementations of the service repository interfaces.
package repository

import (
	"context"
	"fmt"
	"time"

	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxUserRepository is a PostgreSQL implementation backed by pgx/v5.
type PgxUserRepository struct {
	pool *pgxpool.Pool
}

// NewPgxUserRepository creates a new PgxUserRepository.
func NewPgxUserRepository(pool *pgxpool.Pool) *PgxUserRepository {
	return &PgxUserRepository{pool: pool}
}

// FindByEmail returns the user with the given email, or an error if not found.
func (r *PgxUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, tenant_id, email, password_hash, first_name, last_name,
		       COALESCE(phone,''), status, failed_login_attempts,
		       locked_until, last_login_at, created_at, updated_at,
		       deleted_at, created_by, updated_by
		FROM users.identity_users
		WHERE email = $1 AND deleted_at IS NULL
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, email)
	return scanUser(row)
}

// FindByID returns the user with the given ID, or an error if not found.
func (r *PgxUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `
		SELECT id, tenant_id, email, password_hash, first_name, last_name,
		       COALESCE(phone,''), status, failed_login_attempts,
		       locked_until, last_login_at, created_at, updated_at,
		       deleted_at, created_by, updated_by
		FROM users.identity_users
		WHERE id = $1 AND deleted_at IS NULL
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, id)
	return scanUser(row)
}

// FindPrimaryRoleCode returns the code of the first role assigned to the user.
// Returns an empty string (no error) when the user has no roles yet.
func (r *PgxUserRepository) FindPrimaryRoleCode(ctx context.Context, id uuid.UUID) (string, error) {
	const q = `
		SELECT ro.code
		FROM users.identity_user_roles ur
		JOIN users.identity_roles ro ON ro.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY ur.assigned_at ASC
		LIMIT 1`

	var code string
	err := r.pool.QueryRow(ctx, q, id).Scan(&code)
	if err != nil {
		// pgx returns pgx.ErrNoRows when there are no roles — treat as "no role"
		return "", nil
	}
	return code, nil
}

// ListPermissionCodes returns all permission codes assigned to the user's roles.
func (r *PgxUserRepository) ListPermissionCodes(ctx context.Context, id uuid.UUID) ([]string, error) {
	const q = `
		SELECT DISTINCT p.code
		FROM users.identity_permissions p
		JOIN users.identity_role_permissions rp ON rp.permission_id = p.id
		JOIN users.identity_user_roles ur ON ur.role_id = rp.role_id
		WHERE ur.user_id = $1
		ORDER BY p.code ASC`

	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("user_repository.ListPermissionCodes: %w", err)
	}
	defer rows.Close()

	codes := []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("user_repository.ListPermissionCodes scan: %w", err)
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user_repository.ListPermissionCodes rows: %w", err)
	}
	return codes, nil
}

// UpdateStatus changes the status of the given user.
func (r *PgxUserRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus) error {
	const q = `UPDATE users.identity_users SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, string(status), id)
	if err != nil {
		return fmt.Errorf("user_repository.UpdateStatus: %w", err)
	}
	return nil
}

// UpdateLastLogin sets the last_login_at timestamp for the given user.
func (r *PgxUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error {
	const q = `UPDATE users.identity_users SET last_login_at = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, at, id)
	if err != nil {
		return fmt.Errorf("user_repository.UpdateLastLogin: %w", err)
	}
	return nil
}

// UpdatePasswordHash persists a new bcrypt password hash for the user.
func (r *PgxUserRepository) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	const q = `UPDATE users.identity_users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, hash, id)
	if err != nil {
		return fmt.Errorf("user_repository.UpdatePasswordHash: %w", err)
	}
	return nil
}

// IncrementFailedAttempts increments the failed login counter by 1.
func (r *PgxUserRepository) IncrementFailedAttempts(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users.identity_users SET failed_login_attempts = failed_login_attempts + 1, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("user_repository.IncrementFailedAttempts: %w", err)
	}
	return nil
}

// ResetFailedAttempts zeroes the failed login counter.
func (r *PgxUserRepository) ResetFailedAttempts(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users.identity_users SET failed_login_attempts = 0, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("user_repository.ResetFailedAttempts: %w", err)
	}
	return nil
}

// CreateUser inserts a new user record.
func (r *PgxUserRepository) CreateUser(ctx context.Context, u *domain.User) error {
	const q = `
		INSERT INTO users.identity_users
		    (id, tenant_id, email, password_hash, first_name, last_name, phone, status,
		     failed_login_attempts, created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, NOW(), NOW(), $9)`

	_, err := r.pool.Exec(ctx, q,
		u.ID, u.TenantID, u.Email, u.PasswordHash, u.FirstName, u.LastName,
		u.Phone, string(u.Status), u.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("user_repository.CreateUser: %w", err)
	}
	return nil
}

// AssignRole inserts a role assignment for the given user.
func (r *PgxUserRepository) AssignRole(ctx context.Context, userID uuid.UUID, roleCode string) error {
	const q = `
		INSERT INTO users.identity_user_roles (user_id, role_id, assigned_at)
		SELECT $1, id, NOW()
		FROM users.identity_roles
		WHERE code = $2`

	tag, err := r.pool.Exec(ctx, q, userID, roleCode)
	if err != nil {
		return fmt.Errorf("user_repository.AssignRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user_repository.AssignRole: role %q not found", roleCode)
	}
	return nil
}

// ListByRole returns paginated users with the given role code.
func (r *PgxUserRepository) ListByRole(ctx context.Context, roleCode string, limit, offset int) ([]domain.User, error) {
	const q = `
		SELECT u.id, u.tenant_id, u.email, u.password_hash, u.first_name, u.last_name,
		       COALESCE(u.phone,''), u.status, u.failed_login_attempts,
		       u.locked_until, u.last_login_at, u.created_at, u.updated_at,
		       u.deleted_at, u.created_by, u.updated_by
		FROM users.identity_users u
		JOIN users.identity_user_roles ur ON ur.user_id = u.id
		JOIN users.identity_roles r ON r.id = ur.role_id
		WHERE r.code = $1 AND u.deleted_at IS NULL
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, roleCode, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("user_repository.ListByRole: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		row := rows.Scan
		u, err := scanUser(scannerFunc(row))
		if err != nil {
			return nil, fmt.Errorf("user_repository.ListByRole: %w", err)
		}
		users = append(users, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user_repository.ListByRole: %w", err)
	}
	return users, nil
}

// GetLastCreatedID returns the ID of the most recently created user.
func (r *PgxUserRepository) GetLastCreatedID(ctx context.Context) (uuid.UUID, error) {
	const q = `SELECT id FROM users.identity_users WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT 1`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, q).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("user_repository.GetLastCreatedID: %w", err)
	}
	return id, nil
}

// UpdateFirebaseToken stores the FCM token for the user.
func (r *PgxUserRepository) UpdateFirebaseToken(ctx context.Context, userID uuid.UUID, token string) error {
	const q = `UPDATE users.identity_users SET firebase_token = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, token, userID)
	if err != nil {
		return fmt.Errorf("user_repository.UpdateFirebaseToken: %w", err)
	}
	return nil
}

// GetFirebaseToken retrieves the FCM token for the user.
func (r *PgxUserRepository) GetFirebaseToken(ctx context.Context, userID uuid.UUID) (string, error) {
	const q = `SELECT COALESCE(firebase_token,'') FROM users.identity_users WHERE id = $1 AND deleted_at IS NULL`
	var token string
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&token); err != nil {
		return "", fmt.Errorf("user_repository.GetFirebaseToken: %w", err)
	}
	return token, nil
}

// DeleteAllData removes all users, sessions, tokens, roles and audit entries (dev/test only).
func (r *PgxUserRepository) DeleteAllData(ctx context.Context) error {
	const q = `
		TRUNCATE users.identity_audit_log,
		         users.identity_user_roles,
		         users.identity_tokens,
		         users.identity_sessions,
		         users.identity_users RESTART IDENTITY CASCADE`
	_, err := r.pool.Exec(ctx, q)
	if err != nil {
		return fmt.Errorf("user_repository.DeleteAllData: %w", err)
	}
	return nil
}

// scannerFunc adapts a Rows.Scan method to the rowScanner interface.
type scannerFunc func(dest ...any) error

func (f scannerFunc) Scan(dest ...any) error { return f(dest...) }

// ── scanner helper ────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*domain.User, error) {
	u := &domain.User{}
	err := row.Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.Phone, &u.Status, &u.FailedLoginAttempts,
		&u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
		&u.DeletedAt, &u.CreatedBy, &u.UpdatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}
