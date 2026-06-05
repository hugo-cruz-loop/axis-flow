// Package domain defines the core business entities for the identity service.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// UserStatus represents the lifecycle state of a user account.
type UserStatus string

const (
	StatusPendingActivation UserStatus = "PENDING_ACTIVATION"
	StatusActive            UserStatus = "ACTIVE"
	StatusInactive          UserStatus = "INACTIVE"
	StatusSuspended         UserStatus = "SUSPENDED"
	StatusLocked            UserStatus = "LOCKED"
	StatusDeleted           UserStatus = "DELETED"
)

// DeviceType represents the type of device used to create a session.
type DeviceType string

const (
	DeviceWeb     DeviceType = "WEB"
	DeviceAndroid DeviceType = "ANDROID"
	DeviceIOS     DeviceType = "IOS"
)

// TokenType represents the purpose of a one-use token.
type TokenType string

const (
	TokenActivation        TokenType = "ACTIVATION"
	TokenPasswordReset     TokenType = "PASSWORD_RESET"
	TokenEmailVerification TokenType = "EMAIL_VERIFICATION"
)

var (
	// ErrInvalidStatus is returned when a user status string is not recognised.
	ErrInvalidStatus = errors.New("invalid user status")
	// ErrUserNotActive is returned when a login is attempted on a non-active account.
	ErrUserNotActive = errors.New("user account is not active")

	// RBAC sentinel errors.
	ErrSystemRole      = errors.New("cannot modify or delete a system role")
	ErrRoleHasUsers    = errors.New("cannot delete role: users are assigned to it")
	ErrPermissionInUse = errors.New("cannot delete permission: assigned to one or more roles")
	ErrDuplicateCode   = errors.New("code already exists")
	ErrNotFound        = errors.New("not found")
)

// ValidStatuses contains every accepted UserStatus value.
var ValidStatuses = map[UserStatus]struct{}{
	StatusPendingActivation: {},
	StatusActive:            {},
	StatusInactive:          {},
	StatusSuspended:         {},
	StatusLocked:            {},
	StatusDeleted:           {},
}

// ValidateStatus returns an error if s is not a recognised UserStatus.
func ValidateStatus(s UserStatus) error {
	if _, ok := ValidStatuses[s]; !ok {
		return ErrInvalidStatus
	}
	return nil
}

// CanLogin reports whether a user with the given status is allowed to log in.
func CanLogin(s UserStatus) bool {
	return s == StatusActive
}

// User is the core identity entity.
type User struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	Email               string
	PasswordHash        string
	FirstName           string
	LastName            string
	Phone               string
	Status              UserStatus
	FailedLoginAttempts int
	LockedUntil         *time.Time
	LastLoginAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
	CreatedBy           *uuid.UUID
	UpdatedBy           *uuid.UUID
}

// Role represents an assignable role in the system.
type Role struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	Scope       string
	IsSystem    bool
}

// Session represents an active refresh-token session for a user.
type Session struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	DeviceID         string
	DeviceType       DeviceType
	IPAddress        string
	RevokedAt        *time.Time
	ExpiresAt        time.Time
	LastUsedAt       *time.Time
}

// Token represents a one-use token (activation, password-reset, etc.).
type Token struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	Type      TokenType
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
}

// Permission represents a fine-grained capability that can be assigned to roles.
type Permission struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Module      string
	Description string
}

// RoleWithCount wraps a Role with its assigned permission count.
type RoleWithCount struct {
	Role
	PermissionCount int
}

// RoleFilter holds optional filters for paginated role listing.
type RoleFilter struct {
	Scope    string // "GLOBAL" | "TENANT" | "" (all)
	IsSystem *bool
	Page     int
	PageSize int
}

// PermissionFilter holds optional filters for paginated permission listing.
type PermissionFilter struct {
	Module   string
	Page     int
	PageSize int
}

// AuditEntry records an action performed by or on a user.
type AuditEntry struct {
	ID           uuid.UUID
	ActorUserID  *uuid.UUID
	TargetUserID *uuid.UUID
	Action       string
	Metadata     map[string]any
	IPAddress    string
	TraceID      string
	CreatedAt    time.Time
}
