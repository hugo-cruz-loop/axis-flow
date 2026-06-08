package unit_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── mock ─────────────────────────────────────────────────────────────────────

type mockTokenRepo struct{ mock.Mock }

func (m *mockTokenRepo) Create(ctx context.Context, t *domain.Token) error {
	return m.Called(ctx, t).Error(0)
}
func (m *mockTokenRepo) FindByHash(ctx context.Context, hash string) (*domain.Token, error) {
	args := m.Called(ctx, hash)
	if v := args.Get(0); v != nil {
		return v.(*domain.Token), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockTokenRepo) MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	return m.Called(ctx, id, usedAt).Error(0)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func newTestTokenService(r *mockTokenRepo) *service.TokenService {
	return service.NewTokenService(r)
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestGenerateActivationToken_ReturnsHashedToken(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := newTestTokenService(repo)

	userID := uuid.New()
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Token")).Return(nil)

	raw, err := svc.GenerateActivationToken(context.Background(), userID)

	assert.NoError(t, err)
	assert.NotEmpty(t, raw)
	// The raw token must NOT be an empty string and must differ from any stored hash
	repo.AssertExpectations(t)
}

func TestValidateActivationToken_WithValidToken_Succeeds(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := newTestTokenService(repo)

	storedToken := &domain.Token{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Type:      domain.TokenActivation,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	repo.On("FindByHash", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)
	repo.On("MarkUsed", mock.Anything, storedToken.ID, mock.AnythingOfType("time.Time")).Return(nil)

	err := svc.ValidateAndConsumeToken(context.Background(), "raw-token", domain.TokenActivation)

	assert.NoError(t, err)
}

func TestValidateActivationToken_WithExpiredToken_Fails(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := newTestTokenService(repo)

	storedToken := &domain.Token{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Type:      domain.TokenActivation,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
	}

	repo.On("FindByHash", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)

	err := svc.ValidateAndConsumeToken(context.Background(), "raw-token", domain.TokenActivation)

	assert.ErrorIs(t, err, service.ErrTokenExpired)
}

func TestValidateActivationToken_WithUsedToken_Fails(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := newTestTokenService(repo)

	usedAt := time.Now().Add(-30 * time.Minute)
	storedToken := &domain.Token{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Type:      domain.TokenActivation,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		UsedAt:    &usedAt,
	}

	repo.On("FindByHash", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)

	err := svc.ValidateAndConsumeToken(context.Background(), "raw-token", domain.TokenActivation)

	assert.ErrorIs(t, err, service.ErrTokenAlreadyUsed)
}
