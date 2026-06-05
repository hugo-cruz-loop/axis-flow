package unit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/handler"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockPermHandlerRepo implements handler.PermissionRepositorier.
type mockPermHandlerRepo struct{ mock.Mock }

func (m *mockPermHandlerRepo) ListAll(ctx context.Context, module string) ([]domain.Permission, error) {
	args := m.Called(ctx, module)
	if v := args.Get(0); v != nil {
		return v.([]domain.Permission), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockPermHandlerRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.Permission), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockPermHandlerRepo) Create(ctx context.Context, p *domain.Permission) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockPermHandlerRepo) Update(ctx context.Context, p *domain.Permission) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockPermHandlerRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func TestGetPermissions_Returns200(t *testing.T) {
	// Arrange
	repo := &mockPermHandlerRepo{}
	h := handler.NewPermissionHandler(repo, &mockAudit{})

	expected := []domain.Permission{
		{ID: uuid.New(), Code: "users:read", Name: "Read Users", Module: "users"},
	}
	repo.On("ListAll", mock.Anything, "").Return(expected, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/permissions", nil)
	w := httptest.NewRecorder()

	// Act
	h.ListPermissions(w, r)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	var resp []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Len(t, resp, 1)
	repo.AssertExpectations(t)
}

func TestPostPermissions_WithValidData_Returns201(t *testing.T) {
	// Arrange
	repo := &mockPermHandlerRepo{}
	h := handler.NewPermissionHandler(repo, &mockAudit{})

	body := map[string]string{"code": "users:write", "name": "Write Users", "module": "users"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/permissions", bytes.NewReader(b))
	w := httptest.NewRecorder()

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Permission")).Return(nil)

	// Act
	h.CreatePermission(w, r)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestDeletePermission_WhenAssignedToRole_Returns409(t *testing.T) {
	// Arrange
	repo := &mockPermHandlerRepo{}
	h := handler.NewPermissionHandler(repo, &mockAudit{})

	id := uuid.New()
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/permissions/"+id.String(), nil)
	r = chiContext(r, map[string]string{"id": id.String()})
	w := httptest.NewRecorder()

	repo.On("Delete", mock.Anything, id).Return(domain.ErrPermissionInUse)

	// Act
	h.DeletePermission(w, r)

	// Assert
	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}
