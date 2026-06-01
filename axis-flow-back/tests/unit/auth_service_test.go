package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// ── mocks ────────────────────────────────────────────────────────────────────

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if v := args.Get(0); v != nil {
		return v.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockUserRepo) UpdateStatus(ctx context.Context, id uuid.UUID, s domain.UserStatus) error {
	return m.Called(ctx, id, s).Error(0)
}
func (m *mockUserRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error {
	return m.Called(ctx, id, at).Error(0)
}
func (m *mockUserRepo) IncrementFailedAttempts(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockUserRepo) ResetFailedAttempts(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockUserRepo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	return m.Called(ctx, id, hash).Error(0)
}
func (m *mockUserRepo) FindPrimaryRoleCode(_ context.Context, _ uuid.UUID) (string, error) {
	return "ADMIN_CHECK_ON", nil
}

type mockSessionRepo struct{ mock.Mock }

func (m *mockSessionRepo) Create(ctx context.Context, s *domain.Session) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockSessionRepo) FindByRefreshTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	args := m.Called(ctx, hash)
	if v := args.Get(0); v != nil {
		return v.(*domain.Session), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockSessionRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func newTestAuthService(u *mockUserRepo, s *mockSessionRepo) *service.AuthService {
	return service.NewAuthService(u, s, "test-secret-key-32-bytes-longXXX", 15*time.Minute, 7*24*time.Hour)
}

func hashPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func activeUser(t *testing.T) *domain.User {
	t.Helper()
	return &domain.User{
		ID:           uuid.New(),
		TenantID:     uuid.New(),
		Email:        "user@example.com",
		PasswordHash: hashPassword(t, "correct-password"),
		FirstName:    "John",
		LastName:     "Doe",
		Status:       domain.StatusActive,
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestLogin_WithValidActiveUser_ReturnsTokenPair(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	svc := newTestAuthService(userRepo, sessRepo)

	u := activeUser(t)
	userRepo.On("FindByEmail", mock.Anything, u.Email).Return(u, nil)
	userRepo.On("UpdateLastLogin", mock.Anything, u.ID, mock.Anything).Return(nil)
	userRepo.On("ResetFailedAttempts", mock.Anything, u.ID).Return(nil)
	sessRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)

	result, err := svc.Login(context.Background(), u.Email, "correct-password", "WEB", "", "127.0.0.1")

	assert.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	userRepo.AssertExpectations(t)
	sessRepo.AssertExpectations(t)
}

func TestLogin_WithInactiveUser_ReturnsUnauthorized(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	svc := newTestAuthService(userRepo, sessRepo)

	u := activeUser(t)
	u.Status = domain.StatusInactive
	userRepo.On("FindByEmail", mock.Anything, u.Email).Return(u, nil)

	_, err := svc.Login(context.Background(), u.Email, "correct-password", "WEB", "", "127.0.0.1")

	assert.ErrorIs(t, err, service.ErrUnauthorized)
}

func TestLogin_WithWrongPassword_ReturnsUnauthorized(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	svc := newTestAuthService(userRepo, sessRepo)

	u := activeUser(t)
	userRepo.On("FindByEmail", mock.Anything, u.Email).Return(u, nil)
	userRepo.On("IncrementFailedAttempts", mock.Anything, u.ID).Return(nil)

	_, err := svc.Login(context.Background(), u.Email, "wrong-password", "WEB", "", "127.0.0.1")

	assert.ErrorIs(t, err, service.ErrUnauthorized)
}

func TestRefreshToken_WithValidToken_ReturnsNewAccessToken(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	svc := newTestAuthService(userRepo, sessRepo)

	u := activeUser(t)
	sessionID := uuid.New()
	sess := &domain.Session{
		ID:               sessionID,
		UserID:           u.ID,
		RefreshTokenHash: "placeholder", // will be replaced by matcher
		DeviceType:       domain.DeviceWeb,
		ExpiresAt:        time.Now().Add(7 * 24 * time.Hour),
	}

	// The service hashes the incoming token before lookup; use AnythingOfType
	sessRepo.On("FindByRefreshTokenHash", mock.Anything, mock.AnythingOfType("string")).Return(sess, nil)
	userRepo.On("FindByID", mock.Anything, u.ID).Return(u, nil)
	sessRepo.On("Revoke", mock.Anything, sessionID).Return(nil)
	sessRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)

	result, err := svc.RefreshToken(context.Background(), "some-raw-refresh-token")

	assert.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
}

func TestRefreshToken_WithRevokedToken_ReturnsUnauthorized(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	svc := newTestAuthService(userRepo, sessRepo)

	revokedAt := time.Now().Add(-1 * time.Hour)
	sess := &domain.Session{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		RevokedAt: &revokedAt,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	sessRepo.On("FindByRefreshTokenHash", mock.Anything, mock.AnythingOfType("string")).Return(sess, nil)

	_, err := svc.RefreshToken(context.Background(), "revoked-token")

	assert.ErrorIs(t, err, service.ErrUnauthorized)
	_ = errors.Is(err, service.ErrUnauthorized) // satisfy linter
}
