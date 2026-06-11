package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
func (m *mockUserRepo) ListPermissionCodes(ctx context.Context, id uuid.UUID) ([]string, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.([]string), args.Error(1)
	}
	return nil, args.Error(1)
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
	return service.NewAuthService(u, s, "test-secret-key-32-bytes-longXXX", 15*time.Minute, 7*24*time.Hour, nil)
}

// newTestAuthServiceWithLookup is the PR-5 (5.0) variant: the test
// passes a mock EmpleadoLookup so the login flow's enrichment can be
// asserted.
func newTestAuthServiceWithLookup(u *mockUserRepo, s *mockSessionRepo, e service.EmpleadoLookup) *service.AuthService {
	return service.NewAuthService(u, s, "test-secret-key-32-bytes-longXXX", 15*time.Minute, 7*24*time.Hour, e)
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

// ---------------------------------------------------------------------------
// PR-5 (5.0) — EmpleadoID claim enrichment in the access token.
// ---------------------------------------------------------------------------

// mockEmpleadoLookup satisfies service.EmpleadoLookup with a
// configurable id and a recording call slice.
type mockEmpleadoLookup struct {
	mock.Mock
}

func (m *mockEmpleadoLookup) GetEmpleadoIDByUserID(ctx context.Context, userID, empresaID uuid.UUID) (int64, error) {
	args := m.Called(ctx, userID, empresaID)
	return args.Get(0).(int64), args.Error(1)
}

// TestLogin_EnrichesAccessTokenWithEmpleadoID asserts that a successful
// login calls the EmpleadoLookup seam and the resulting access token's
// EmpleadoID claim equals the lookup's return value.
func TestLogin_EnrichesAccessTokenWithEmpleadoID(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	lookup := &mockEmpleadoLookup{}
	svc := newTestAuthServiceWithLookup(userRepo, sessRepo, lookup)

	u := activeUser(t)
	userRepo.On("FindByEmail", mock.Anything, u.Email).Return(u, nil)
	userRepo.On("UpdateLastLogin", mock.Anything, u.ID, mock.Anything).Return(nil)
	userRepo.On("ResetFailedAttempts", mock.Anything, u.ID).Return(nil)
	sessRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)
	lookup.On("GetEmpleadoIDByUserID", mock.Anything, u.ID, u.TenantID).Return(int64(42), nil)

	pair, err := svc.Login(context.Background(), u.Email, "correct-password", "WEB", "", "127.0.0.1")
	require.NoError(t, err)

	// Parse the access token and assert the EmpleadoID claim is 42.
	claims := &service.Claims{}
	parsed, err := jwt.ParseWithClaims(pair.AccessToken, claims, func(_ *jwt.Token) (any, error) {
		return []byte("test-secret-key-32-bytes-longXXX"), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	assert.Equal(t, int64(42), claims.EmpleadoID, "EmpleadoID must be enriched from the lookup")
	lookup.AssertExpectations(t)
}

// TestLogin_NoLinkedEmpleado_IssuesTokenWithZeroEmpleadoID asserts the
// "no linked empleado" path: lookup returns 0, the token is still
// issued successfully with EmpleadoID=0. Non-formularios endpoints
// continue to work; the formularios POST /evento_iniciado will see
// 0 and return 401 (extracted handler invariant — tested in PR-4 AMEND).
func TestLogin_NoLinkedEmpleado_IssuesTokenWithZeroEmpleadoID(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	lookup := &mockEmpleadoLookup{}
	svc := newTestAuthServiceWithLookup(userRepo, sessRepo, lookup)

	u := activeUser(t)
	userRepo.On("FindByEmail", mock.Anything, u.Email).Return(u, nil)
	userRepo.On("UpdateLastLogin", mock.Anything, u.ID, mock.Anything).Return(nil)
	userRepo.On("ResetFailedAttempts", mock.Anything, u.ID).Return(nil)
	sessRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)
	lookup.On("GetEmpleadoIDByUserID", mock.Anything, u.ID, u.TenantID).Return(int64(0), nil)

	pair, err := svc.Login(context.Background(), u.Email, "correct-password", "WEB", "", "127.0.0.1")
	require.NoError(t, err, "login must succeed even with no linked empleado")

	claims := &service.Claims{}
	parsed, err := jwt.ParseWithClaims(pair.AccessToken, claims, func(_ *jwt.Token) (any, error) {
		return []byte("test-secret-key-32-bytes-longXXX"), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	assert.Equal(t, int64(0), claims.EmpleadoID, "EmpleadoID must be 0 when no empleado is linked")
}

// TestLogin_EmpleadoLookupError_StillIssuesToken asserts the
// fail-open contract: a DB error from the lookup does NOT block
// login. The token is issued with EmpleadoID=0 and the user can
// re-authenticate. The formularios endpoint will see 0 and return
// 401 — a clear signal that the user-employee link is broken.
func TestLogin_EmpleadoLookupError_StillIssuesToken(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	lookup := &mockEmpleadoLookup{}
	svc := newTestAuthServiceWithLookup(userRepo, sessRepo, lookup)

	u := activeUser(t)
	userRepo.On("FindByEmail", mock.Anything, u.Email).Return(u, nil)
	userRepo.On("UpdateLastLogin", mock.Anything, u.ID, mock.Anything).Return(nil)
	userRepo.On("ResetFailedAttempts", mock.Anything, u.ID).Return(nil)
	sessRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)
	lookup.On("GetEmpleadoIDByUserID", mock.Anything, u.ID, u.TenantID).Return(int64(0), assert.AnError)

	pair, err := svc.Login(context.Background(), u.Email, "correct-password", "WEB", "", "127.0.0.1")
	require.NoError(t, err, "login must succeed even when empleado lookup errors (fail-open)")

	claims := &service.Claims{}
	parsed, err := jwt.ParseWithClaims(pair.AccessToken, claims, func(_ *jwt.Token) (any, error) {
		return []byte("test-secret-key-32-bytes-longXXX"), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	assert.Equal(t, int64(0), claims.EmpleadoID, "lookup errors degrade to EmpleadoID=0")
}

// TestLogin_NilEmpleadoLookup_DoesNotPanic asserts that the nil
// EmpleadoLookup seam (used by the legacy 5-arg NewAuthService call
// sites that don't need the claim) does not crash the login flow.
func TestLogin_NilEmpleadoLookup_DoesNotPanic(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	svc := newTestAuthService(userRepo, sessRepo) // nil lookup

	u := activeUser(t)
	userRepo.On("FindByEmail", mock.Anything, u.Email).Return(u, nil)
	userRepo.On("UpdateLastLogin", mock.Anything, u.ID, mock.Anything).Return(nil)
	userRepo.On("ResetFailedAttempts", mock.Anything, u.ID).Return(nil)
	sessRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)

	pair, err := svc.Login(context.Background(), u.Email, "correct-password", "WEB", "", "127.0.0.1")
	require.NoError(t, err)

	claims := &service.Claims{}
	parsed, err := jwt.ParseWithClaims(pair.AccessToken, claims, func(_ *jwt.Token) (any, error) {
		return []byte("test-secret-key-32-bytes-longXXX"), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	assert.Equal(t, int64(0), claims.EmpleadoID, "nil lookup → EmpleadoID=0")
}

// TestRefreshToken_EnrichesAccessTokenWithEmpleadoID asserts that the
// refresh path re-issues an access token with the same EmpleadoID
// enrichment. The brief explicitly requires this.
func TestRefreshToken_EnrichesAccessTokenWithEmpleadoID(t *testing.T) {
	userRepo := &mockUserRepo{}
	sessRepo := &mockSessionRepo{}
	lookup := &mockEmpleadoLookup{}
	svc := newTestAuthServiceWithLookup(userRepo, sessRepo, lookup)

	u := activeUser(t)
	sessionID := uuid.New()
	sess := &domain.Session{
		ID:               sessionID,
		UserID:           u.ID,
		RefreshTokenHash: "placeholder",
		DeviceType:       domain.DeviceWeb,
		ExpiresAt:        time.Now().Add(7 * 24 * time.Hour),
	}

	sessRepo.On("FindByRefreshTokenHash", mock.Anything, mock.AnythingOfType("string")).Return(sess, nil)
	userRepo.On("FindByID", mock.Anything, u.ID).Return(u, nil)
	sessRepo.On("Revoke", mock.Anything, sessionID).Return(nil)
	sessRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Session")).Return(nil)
	lookup.On("GetEmpleadoIDByUserID", mock.Anything, u.ID, u.TenantID).Return(int64(99), nil)

	pair, err := svc.RefreshToken(context.Background(), "some-raw-refresh-token")
	require.NoError(t, err)

	claims := &service.Claims{}
	parsed, err := jwt.ParseWithClaims(pair.AccessToken, claims, func(_ *jwt.Token) (any, error) {
		return []byte("test-secret-key-32-bytes-longXXX"), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	assert.Equal(t, int64(99), claims.EmpleadoID, "refresh path must re-issue with enriched EmpleadoID")
	lookup.AssertExpectations(t)
}
