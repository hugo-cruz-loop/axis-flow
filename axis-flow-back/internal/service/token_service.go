package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"axis-flow-back/internal/crypto"
	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
)

var (
	// ErrTokenExpired is returned when a one-use token is past its expiry.
	ErrTokenExpired = errors.New("token has expired")
	// ErrTokenAlreadyUsed is returned when a one-use token has already been consumed.
	ErrTokenAlreadyUsed = errors.New("token has already been used")
	// ErrTokenNotFound is returned when no token matches the provided raw value.
	ErrTokenNotFound = errors.New("token not found")
)

// TokenService handles generation and validation of one-use tokens
// (activation, password-reset, email-verification).
type TokenService struct {
	tokens TokenRepository
}

// NewTokenService creates a TokenService backed by the given repository.
func NewTokenService(tokens TokenRepository) *TokenService {
	return &TokenService{tokens: tokens}
}

// GenerateActivationToken creates a random activation token for the given user,
// persists its hash, and returns the raw (unhashed) token to be sent out-of-band.
func (s *TokenService) GenerateActivationToken(ctx context.Context, userID uuid.UUID) (string, error) {
	raw, err := crypto.GenerateRandomToken()
	if err != nil {
		return "", fmt.Errorf("generate activation token: %w", err)
	}

	t := &domain.Token{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: crypto.HashToken(raw),
		Type:      domain.TokenActivation,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := s.tokens.Create(ctx, t); err != nil {
		return "", fmt.Errorf("persist activation token: %w", err)
	}

	return raw, nil
}

// ValidateAndConsumeToken looks up the token by its hash, checks expiry and
// single-use constraints, then marks it as used.
func (s *TokenService) ValidateAndConsumeToken(ctx context.Context, rawToken string, expectedType domain.TokenType) error {
	hash := crypto.HashToken(rawToken)

	t, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		return ErrTokenNotFound
	}

	if t.UsedAt != nil {
		return ErrTokenAlreadyUsed
	}

	if t.RevokedAt != nil {
		return ErrTokenAlreadyUsed
	}

	if time.Now().After(t.ExpiresAt) {
		return ErrTokenExpired
	}

	if t.Type != expectedType {
		return ErrTokenNotFound
	}

	if err := s.tokens.MarkUsed(ctx, t.ID, time.Now()); err != nil {
		return fmt.Errorf("mark token used: %w", err)
	}

	return nil
}
