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
