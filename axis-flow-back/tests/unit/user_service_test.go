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
)

// ── extended user repo mock ───────────────────────────────────────────────────

type mockExtendedUserRepo struct{ mock.Mock }

func (m *mockExtendedUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if v := args.Get(0); v != nil {
		return v.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockExtendedUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockExtendedUserRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus) error {
	return m.Called(ctx, id, status).Error(0)
}
func (m *mockExtendedUserRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error {
	return m.Called(ctx, id, at).Error(0)
}
func (m *mockExtendedUserRepo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	return m.Called(ctx, id, hash).Error(0)
}
func (m *mockExtendedUserRepo) IncrementFailedAttempts(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockExtendedUserRepo) ResetFailedAttempts(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockExtendedUserRepo) FindPrimaryRoleCode(_ context.Context, _ uuid.UUID) (string, error) {
	return "ADMIN_CHECK_ON", nil
}
func (m *mockExtendedUserRepo) ListPermissionCodes(ctx context.Context, id uuid.UUID) ([]string, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockExtendedUserRepo) CreateUser(ctx context.Context, u *domain.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *mockExtendedUserRepo) AssignRole(ctx context.Context, userID uuid.UUID, roleCode string) error {
	return m.Called(ctx, userID, roleCode).Error(0)
}
func (m *mockExtendedUserRepo) ListByRole(ctx context.Context, roleCode string, limit, offset int) ([]domain.User, error) {
	args := m.Called(ctx, roleCode, limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockExtendedUserRepo) GetLastCreatedID(ctx context.Context) (uuid.UUID, error) {
	args := m.Called(ctx)
	return args.Get(0).(uuid.UUID), args.Error(1)
}
func (m *mockExtendedUserRepo) UpdateFirebaseToken(ctx context.Context, userID uuid.UUID, token string) error {
	return m.Called(ctx, userID, token).Error(0)
}
func (m *mockExtendedUserRepo) GetFirebaseToken(ctx context.Context, userID uuid.UUID) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}
func (m *mockExtendedUserRepo) DeleteAllData(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// ── audit repo mock ───────────────────────────────────────────────────────────

type mockAuditRepo struct{ mock.Mock }

func (m *mockAuditRepo) Append(ctx context.Context, entry domain.AuditEntry) error {
	return m.Called(ctx, entry).Error(0)
}

// ── token service mock ────────────────────────────────────────────────────────

type mockTokenServicer struct{ mock.Mock }

func (m *mockTokenServicer) GenerateActivationToken(ctx context.Context, userID uuid.UUID) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCreateUser_WithValidAdminRequest_CreatesUserAndAssignsRole(t *testing.T) {
	// Arrange
	userRepo := &mockExtendedUserRepo{}
	auditRepo := &mockAuditRepo{}
	tokenSvc := &mockTokenServicer{}

	actorID := uuid.New()
	tenantID := uuid.New()

	userRepo.On("FindByEmail", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))
	userRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)
	userRepo.On("AssignRole", mock.Anything, mock.AnythingOfType("uuid.UUID"), "Cliente").Return(nil)
	auditRepo.On("Append", mock.Anything, mock.AnythingOfType("domain.AuditEntry")).Return(nil)
	tokenSvc.On("GenerateActivationToken", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return("raw-token", nil)

	svc := service.NewUserService(userRepo, auditRepo, tokenSvc)

	req := service.CreateUserRequest{
		ActorUserID: actorID,
		TenantID:    tenantID,
		Email:       "new@example.com",
		FirstName:   "Test",
		LastName:    "User",
		RoleCode:    "Cliente",
	}

	// Act
	user, err := svc.CreateUser(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "new@example.com", user.Email)
	assert.Equal(t, domain.StatusPendingActivation, user.Status)
	userRepo.AssertCalled(t, "CreateUser", mock.Anything, mock.AnythingOfType("*domain.User"))
	userRepo.AssertCalled(t, "AssignRole", mock.Anything, mock.AnythingOfType("uuid.UUID"), "Cliente")
	auditRepo.AssertCalled(t, "Append", mock.Anything, mock.AnythingOfType("domain.AuditEntry"))
}

func TestCreateUser_WithDuplicateEmail_ReturnsConflict(t *testing.T) {
	// Arrange
	userRepo := &mockExtendedUserRepo{}
	auditRepo := &mockAuditRepo{}
	tokenSvc := &mockTokenServicer{}

	existing := &domain.User{ID: uuid.New(), Email: "dup@example.com"}
	userRepo.On("FindByEmail", mock.Anything, "dup@example.com").Return(existing, nil)

	svc := service.NewUserService(userRepo, auditRepo, tokenSvc)

	req := service.CreateUserRequest{
		TenantID:  uuid.New(),
		Email:     "dup@example.com",
		FirstName: "Dup",
		LastName:  "User",
		RoleCode:  "Cliente",
	}

	// Act
	_, err := svc.CreateUser(context.Background(), req)

	// Assert
	assert.ErrorIs(t, err, service.ErrDuplicateEmail)
}

func TestListByRole_WithExistingRole_ReturnsUsers(t *testing.T) {
	// Arrange
	userRepo := &mockExtendedUserRepo{}
	auditRepo := &mockAuditRepo{}
	tokenSvc := &mockTokenServicer{}

	expected := []domain.User{
		{ID: uuid.New(), Email: "a@example.com"},
		{ID: uuid.New(), Email: "b@example.com"},
	}
	userRepo.On("ListByRole", mock.Anything, "Cliente", 20, 0).Return(expected, nil)

	svc := service.NewUserService(userRepo, auditRepo, tokenSvc)

	// Act
	users, err := svc.ListByRole(context.Background(), "Cliente", 1, 20)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUpdateFirebaseToken_WithValidUser_Succeeds(t *testing.T) {
	// Arrange
	userRepo := &mockExtendedUserRepo{}
	auditRepo := &mockAuditRepo{}
	tokenSvc := &mockTokenServicer{}

	userID := uuid.New()
	userRepo.On("UpdateFirebaseToken", mock.Anything, userID, "fcm-token-abc").Return(nil)
	auditRepo.On("Append", mock.Anything, mock.AnythingOfType("domain.AuditEntry")).Return(nil)

	svc := service.NewUserService(userRepo, auditRepo, tokenSvc)

	// Act
	err := svc.UpdateFirebaseToken(context.Background(), userID, "fcm-token-abc")

	// Assert
	assert.NoError(t, err)
	userRepo.AssertCalled(t, "UpdateFirebaseToken", mock.Anything, userID, "fcm-token-abc")
}
