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

// mockPermissionRepo implements service.PermissionRepository for unit tests.
type mockPermissionRepo struct{ mock.Mock }

func (m *mockPermissionRepo) ListAll(ctx context.Context, module string) ([]domain.Permission, error) {
	args := m.Called(ctx, module)
	if v := args.Get(0); v != nil {
		return v.([]domain.Permission), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockPermissionRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.Permission), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockPermissionRepo) Create(ctx context.Context, p *domain.Permission) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockPermissionRepo) Update(ctx context.Context, p *domain.Permission) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockPermissionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// compile-time check
var _ service.PermissionRepository = (*mockPermissionRepo)(nil)

func TestPermissionRepo_Create_Succeeds(t *testing.T) {
	// Arrange
	repo := &mockPermissionRepo{}
	ctx := context.Background()
	p := &domain.Permission{Code: "users:read", Name: "Read Users", Module: "users"}
	repo.On("Create", ctx, p).Return(nil)

	// Act
	err := repo.Create(ctx, p)

	// Assert
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPermissionRepo_Delete_WhenAssignedToRole_ReturnsErrPermissionInUse(t *testing.T) {
	// Arrange
	repo := &mockPermissionRepo{}
	ctx := context.Background()
	id := uuid.New()
	repo.On("Delete", ctx, id).Return(domain.ErrPermissionInUse)

	// Act
	err := repo.Delete(ctx, id)

	// Assert
	assert.ErrorIs(t, err, domain.ErrPermissionInUse)
	repo.AssertExpectations(t)
}

func TestPermissionRepo_ListAll_FiltersByModule(t *testing.T) {
	// Arrange
	repo := &mockPermissionRepo{}
	ctx := context.Background()
	expected := []domain.Permission{
		{ID: uuid.New(), Code: "users:read", Name: "Read Users", Module: "users"},
	}
	repo.On("ListAll", ctx, "users").Return(expected, nil)

	// Act
	perms, err := repo.ListAll(ctx, "users")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, perms, 1)
	assert.Equal(t, "users", perms[0].Module)
	repo.AssertExpectations(t)
}
