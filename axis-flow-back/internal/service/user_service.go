package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"
)

// ErrDuplicateEmail is returned when a user with the same email already exists.
var ErrDuplicateEmail = errors.New("email already in use")

// ErrForbidden is returned when the caller lacks the required permission.
var ErrForbidden = errors.New("forbidden")

// TokenServicer abstracts the token generation needed by UserService.
type TokenServicer interface {
	GenerateActivationToken(ctx context.Context, userID uuid.UUID) (string, error)
}

// CreateUserRequest carries the input for creating a new user.
type CreateUserRequest struct {
	ActorUserID uuid.UUID
	TenantID    uuid.UUID
	Email       string
	FirstName   string
	LastName    string
	Phone       string
	RoleCode    string
	// InitialPassword is optional; if empty a random placeholder is used.
	InitialPassword string
}

// UserService handles user-management use cases.
type UserService struct {
	users     ExtendedUserRepository
	audit     AuditRepository
	tokensSvc TokenServicer
}

// NewUserService creates a UserService with the given dependencies.
func NewUserService(
	users ExtendedUserRepository,
	audit AuditRepository,
	tokensSvc TokenServicer,
) *UserService {
	return &UserService{
		users:     users,
		audit:     audit,
		tokensSvc: tokensSvc,
	}
}

// CreateUser inserts a new user, assigns their role, records an audit entry,
// and triggers an activation token.
// The user insert and role assignment are logically atomic via sequential repo calls;
// the repo layer handles the DB transaction via the pool.
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (domain.User, error) {
	// Duplicate check
	existing, err := s.users.FindByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return domain.User{}, ErrDuplicateEmail
	}

	// Derive a placeholder password hash when none is supplied
	passwordInput := req.InitialPassword
	if passwordInput == "" {
		raw, genErr := generateRandomToken()
		if genErr != nil {
			return domain.User{}, fmt.Errorf("create user: generate placeholder password: %w", genErr)
		}
		passwordInput = raw
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(passwordInput), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("create user: hash password: %w", err)
	}

	actorID := req.ActorUserID
	u := &domain.User{
		ID:           uuid.New(),
		TenantID:     req.TenantID,
		Email:        req.Email,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Phone:        req.Phone,
		Status:       domain.StatusPendingActivation,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		CreatedBy:    &actorID,
	}
	if actorID == uuid.Nil {
		u.CreatedBy = nil
	}

	if err := s.users.CreateUser(ctx, u); err != nil {
		return domain.User{}, fmt.Errorf("create user: persist: %w", err)
	}

	if err := s.users.AssignRole(ctx, u.ID, req.RoleCode); err != nil {
		return domain.User{}, fmt.Errorf("create user: assign role: %w", err)
	}

	// Audit
	entry := domain.AuditEntry{
		ID:           uuid.New(),
		ActorUserID:  nil,
		TargetUserID: &u.ID,
		Action:       "USER_CREATED",
		Metadata: map[string]any{
			"role":  req.RoleCode,
			"email": req.Email,
		},
	}
	if actorID != uuid.Nil {
		entry.ActorUserID = &actorID
	}
	traceID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()

	if appendErr := s.audit.Append(ctx, entry); appendErr != nil {
		slog.WarnContext(ctx, "create user: audit append failed",
			slog.String("user_id", u.ID.String()),
			slog.String("error", appendErr.Error()),
			slog.String("trace_id", traceID),
		)
	}

	// Trigger activation token
	if _, tokenErr := s.tokensSvc.GenerateActivationToken(ctx, u.ID); tokenErr != nil {
		slog.WarnContext(ctx, "create user: activation token generation failed",
			slog.String("user_id", u.ID.String()),
			slog.String("error", tokenErr.Error()),
			slog.String("trace_id", traceID),
		)
	}

	slog.InfoContext(ctx, "user created",
		slog.String("user_id", u.ID.String()),
		slog.String("email", u.Email),
		slog.String("role", req.RoleCode),
		slog.String("trace_id", traceID),
	)

	return *u, nil
}

// ListByRole returns paginated users with the given role.
func (s *UserService) ListByRole(ctx context.Context, roleCode string, page, pageSize int) ([]domain.User, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	users, err := s.users.ListByRole(ctx, roleCode, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list by role: %w", err)
	}
	return users, nil
}

// GetLastCreatedID returns the ID of the most recently created user.
func (s *UserService) GetLastCreatedID(ctx context.Context) (uuid.UUID, error) {
	id, err := s.users.GetLastCreatedID(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get last created id: %w", err)
	}
	return id, nil
}

// UpdateFirebaseToken stores an FCM token for a user.
// The token value is NEVER written to logs.
func (s *UserService) UpdateFirebaseToken(ctx context.Context, userID uuid.UUID, token string) error {
	if err := s.users.UpdateFirebaseToken(ctx, userID, token); err != nil {
		return fmt.Errorf("update firebase token: %w", err)
	}

	entry := domain.AuditEntry{
		ID:           uuid.New(),
		TargetUserID: &userID,
		Action:       "FCM_TOKEN_UPDATED",
		Metadata:     map[string]any{},
	}
	if appendErr := s.audit.Append(ctx, entry); appendErr != nil {
		slog.WarnContext(ctx, "update firebase token: audit append failed",
			slog.String("user_id", userID.String()),
			slog.String("error", appendErr.Error()),
		)
	}

	slog.InfoContext(ctx, "firebase token updated", slog.String("user_id", userID.String()))
	return nil
}

// GetFirebaseToken retrieves an FCM token for a user.
// The token value is NEVER written to logs.
func (s *UserService) GetFirebaseToken(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := s.users.GetFirebaseToken(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get firebase token: %w", err)
	}
	return token, nil
}

// DeleteAllData removes all data (dev/test only — guard must be enforced at startup).
func (s *UserService) DeleteAllData(ctx context.Context, actorID uuid.UUID) error {
	if err := s.users.DeleteAllData(ctx); err != nil {
		return fmt.Errorf("delete all data: %w", err)
	}

	entry := domain.AuditEntry{
		ID:     uuid.New(),
		Action: "DELETE_ALL_DATA",
		Metadata: map[string]any{
			"actor_user_id": actorID.String(),
		},
	}
	if actorID != uuid.Nil {
		entry.ActorUserID = &actorID
	}
	_ = s.audit.Append(ctx, entry)

	slog.WarnContext(ctx, "delete all data executed", slog.String("actor_id", actorID.String()))
	return nil
}
