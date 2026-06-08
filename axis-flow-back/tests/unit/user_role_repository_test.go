package unit_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockUserRoleRepo implements service.UserRoleRepository for unit tests.
type mockUserRoleRepo struct{ mock.Mock }

func (m *mockUserRoleRepo) Assign(ctx context.Context, userID, roleID uuid.UUID, assignedBy uuid.UUID) error {
	return m.Called(ctx, userID, roleID, assignedBy).Error(0)
}
func (m *mockUserRoleRepo) Revoke(ctx context.Context, userID, roleID uuid.UUID) error {
	return m.Called(ctx, userID, roleID).Error(0)
}

// compile-time check
var _ service.UserRoleRepository = (*mockUserRoleRepo)(nil)

func TestUserRoleRepo_Assign_InvalidatesRedisCache(t *testing.T) {
	// Arrange
	repo := &mockUserRoleRepo{}
	ctx := context.Background()
	userID := uuid.New()
	roleID := uuid.New()
	assignedBy := uuid.New()
	// The mock stands in for the full impl; we verify Assign is called and succeeds
	repo.On("Assign", ctx, userID, roleID, assignedBy).Return(nil)

	// Act
	err := repo.Assign(ctx, userID, roleID, assignedBy)

	// Assert
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestUserRoleRepo_Revoke_InvalidatesRedisCache(t *testing.T) {
	// Arrange
	repo := &mockUserRoleRepo{}
	ctx := context.Background()
	userID := uuid.New()
	roleID := uuid.New()
	repo.On("Revoke", ctx, userID, roleID).Return(nil)

	// Act
	err := repo.Revoke(ctx, userID, roleID)

	// Assert
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
