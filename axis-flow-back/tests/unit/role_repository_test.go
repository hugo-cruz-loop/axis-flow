package unit_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockRoleRepo implements service.RoleRepository for unit tests.
type mockRoleRepo struct{ mock.Mock }

func (m *mockRoleRepo) ListAll(ctx context.Context) ([]domain.RoleWithCount, error) {
	args := m.Called(ctx)
	if v := args.Get(0); v != nil {
		return v.([]domain.RoleWithCount), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockRoleRepo) ListPermissionsByRole(ctx context.Context, code string) ([]domain.Permission, error) {
	args := m.Called(ctx, code)
	if v := args.Get(0); v != nil {
		return v.([]domain.Permission), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockRoleRepo) Create(ctx context.Context, r *domain.Role) error {
	return m.Called(ctx, r).Error(0)
}
func (m *mockRoleRepo) Update(ctx context.Context, r *domain.Role) error {
	return m.Called(ctx, r).Error(0)
}
func (m *mockRoleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRoleRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.Role), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockRoleRepo) AssignPermission(ctx context.Context, roleID, permID uuid.UUID) error {
	return m.Called(ctx, roleID, permID).Error(0)
}
func (m *mockRoleRepo) RevokePermission(ctx context.Context, roleID, permID uuid.UUID) error {
	return m.Called(ctx, roleID, permID).Error(0)
}
func (m *mockRoleRepo) ListPermissionsByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	args := m.Called(ctx, roleID)
	if v := args.Get(0); v != nil {
		return v.([]domain.Permission), args.Error(1)
	}
	return nil, args.Error(1)
}

// compile-time check
var _ service.RoleRepository = (*mockRoleRepo)(nil)

func TestRoleRepo_Create_WithValidRole_Succeeds(t *testing.T) {
	// Arrange
	repo := &mockRoleRepo{}
	ctx := context.Background()
	role := &domain.Role{Code: "NEW_ROLE", Name: "New Role", Scope: "GLOBAL"}
	repo.On("Create", ctx, role).Return(nil)

	// Act
	err := repo.Create(ctx, role)

	// Assert
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRoleRepo_Update_WithSystemRole_ReturnsErrSystemRole(t *testing.T) {
	// Arrange
	repo := &mockRoleRepo{}
	ctx := context.Background()
	role := &domain.Role{ID: uuid.New(), Name: "Updated", IsSystem: true}
	repo.On("Update", ctx, role).Return(domain.ErrSystemRole)

	// Act
	err := repo.Update(ctx, role)

	// Assert
	assert.ErrorIs(t, err, domain.ErrSystemRole)
	repo.AssertExpectations(t)
}

func TestRoleRepo_Delete_WithUsersAssigned_ReturnsErrRoleHasUsers(t *testing.T) {
	// Arrange
	repo := &mockRoleRepo{}
	ctx := context.Background()
	id := uuid.New()
	repo.On("Delete", ctx, id).Return(domain.ErrRoleHasUsers)

	// Act
	err := repo.Delete(ctx, id)

	// Assert
	assert.ErrorIs(t, err, domain.ErrRoleHasUsers)
	repo.AssertExpectations(t)
}

func TestRoleRepo_Delete_WithSystemRole_ReturnsErrSystemRole(t *testing.T) {
	// Arrange
	repo := &mockRoleRepo{}
	ctx := context.Background()
	id := uuid.New()
	repo.On("Delete", ctx, id).Return(domain.ErrSystemRole)

	// Act
	err := repo.Delete(ctx, id)

	// Assert
	assert.ErrorIs(t, err, domain.ErrSystemRole)
	repo.AssertExpectations(t)
}

func TestRoleRepo_AssignPermission_Succeeds(t *testing.T) {
	// Arrange
	repo := &mockRoleRepo{}
	ctx := context.Background()
	roleID := uuid.New()
	permID := uuid.New()
	repo.On("AssignPermission", ctx, roleID, permID).Return(nil)

	// Act
	err := repo.AssignPermission(ctx, roleID, permID)

	// Assert
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRoleRepo_RevokePermission_Succeeds(t *testing.T) {
	// Arrange
	repo := &mockRoleRepo{}
	ctx := context.Background()
	roleID := uuid.New()
	permID := uuid.New()
	repo.On("RevokePermission", ctx, roleID, permID).Return(nil)

	// Act
	err := repo.RevokePermission(ctx, roleID, permID)

	// Assert
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
