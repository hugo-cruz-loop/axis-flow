package service

import (
	"context"
	"fmt"

	"axis-flow-back/internal/crypto"
	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
)

// ActivationService handles account-activation and password-reset flows.
// It wraps TokenService for token validation and UserRepository for status updates.
type ActivationService struct {
	tokens TokenService
	users  UserRepository
}

// NewActivationService creates an ActivationService.
func NewActivationService(tokens TokenService, users UserRepository) *ActivationService {
	return &ActivationService{tokens: tokens, users: users}
}

// FindUserByTokenHash looks up the user associated with a token identified by its hash.
func (s *ActivationService) FindUserByTokenHash(ctx context.Context, hash string) (*domain.User, error) {
	t, err := s.tokens.tokens.FindByHash(ctx, hash)
	if err != nil {
		return nil, ErrTokenNotFound
	}
	user, err := s.users.FindByID(ctx, t.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user for token: %w", err)
	}
	return user, nil
}

// ValidateAndConsumeToken delegates to the embedded TokenService.
func (s *ActivationService) ValidateAndConsumeToken(ctx context.Context, rawToken string, tokenType domain.TokenType) error {
	return s.tokens.ValidateAndConsumeToken(ctx, rawToken, tokenType)
}

// ActivateUser sets the user status to ACTIVE and persists the new password hash.
func (s *ActivationService) ActivateUser(ctx context.Context, userID uuid.UUID, newPasswordHash string) error {
	if err := s.users.UpdateStatus(ctx, userID, domain.StatusActive); err != nil {
		return fmt.Errorf("activate user: update status: %w", err)
	}
	if err := s.users.UpdatePasswordHash(ctx, userID, newPasswordHash); err != nil {
		return fmt.Errorf("activate user: update password: %w", err)
	}
	return nil
}

// GenerateActivationToken generates and persists an activation token for the user.
func (s *ActivationService) GenerateActivationToken(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.tokens.GenerateActivationToken(ctx, userID)
}

// HashToken is a convenience wrapper exposed for the handler layer.
func HashToken(raw string) string {
	return crypto.HashToken(raw)
}
