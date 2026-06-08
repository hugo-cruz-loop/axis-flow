// Package service contains the application use-cases for the identity service.
// Repository interfaces are defined here (in the consumer package) following
// the dependency-inversion principle.
package service

import (
	"context"
	"time"

	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
)

// UserRepository abstracts persistence for User entities.
type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	FindPrimaryRoleCode(ctx context.Context, id uuid.UUID) (string, error)
	ListPermissionCodes(ctx context.Context, id uuid.UUID) ([]string, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error
	UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error
	IncrementFailedAttempts(ctx context.Context, id uuid.UUID) error
	ResetFailedAttempts(ctx context.Context, id uuid.UUID) error
}

// SessionRepository abstracts persistence for Session entities.
type SessionRepository interface {
	Create(ctx context.Context, s *domain.Session) error
	FindByRefreshTokenHash(ctx context.Context, hash string) (*domain.Session, error)
	Revoke(ctx context.Context, id uuid.UUID) error
}

// TokenRepository abstracts persistence for one-use Token entities.
type TokenRepository interface {
	Create(ctx context.Context, t *domain.Token) error
	FindByHash(ctx context.Context, hash string) (*domain.Token, error)
	MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error
}

// AuditRepository abstracts append-only persistence for audit log entries.
type AuditRepository interface {
	Append(ctx context.Context, entry domain.AuditEntry) error
}

// RoleRepository provides full CRUD and permission-assignment management for roles.
type RoleRepository interface {
	ListAll(ctx context.Context) ([]domain.RoleWithCount, error)
	ListPermissionsByRole(ctx context.Context, code string) ([]domain.Permission, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	Create(ctx context.Context, r *domain.Role) error
	Update(ctx context.Context, r *domain.Role) error
	Delete(ctx context.Context, id uuid.UUID) error
	AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID) error
	RevokePermission(ctx context.Context, roleID, permissionID uuid.UUID) error
	ListPermissionsByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error)
}

// PermissionRepository provides full CRUD for permissions.
type PermissionRepository interface {
	ListAll(ctx context.Context, module string) ([]domain.Permission, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error)
	Create(ctx context.Context, p *domain.Permission) error
	Update(ctx context.Context, p *domain.Permission) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserRoleRepository manages user-role assignments with Redis cache invalidation.
type UserRoleRepository interface {
	Assign(ctx context.Context, userID, roleID uuid.UUID, assignedBy uuid.UUID) error
	Revoke(ctx context.Context, userID, roleID uuid.UUID) error
}

// ExtendedUserRepository adds PR-2 user management operations.
// Implementations must also satisfy UserRepository.
type ExtendedUserRepository interface {
	UserRepository
	CreateUser(ctx context.Context, u *domain.User) error
	AssignRole(ctx context.Context, userID uuid.UUID, roleCode string) error
	ListByRole(ctx context.Context, roleCode string, limit, offset int) ([]domain.User, error)
	GetLastCreatedID(ctx context.Context) (uuid.UUID, error)
	UpdateFirebaseToken(ctx context.Context, userID uuid.UUID, token string) error
	GetFirebaseToken(ctx context.Context, userID uuid.UUID) (string, error)
	DeleteAllData(ctx context.Context) error
}
