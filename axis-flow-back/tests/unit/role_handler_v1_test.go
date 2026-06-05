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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockRoleRepoV1 implements handler.RoleRepositorierV1.
type mockRoleRepoV1 struct{ mock.Mock }

func (m *mockRoleRepoV1) ListAll(ctx context.Context) ([]domain.RoleWithCount, error) {
	args := m.Called(ctx)
	if v := args.Get(0); v != nil {
		return v.([]domain.RoleWithCount), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockRoleRepoV1) ListPermissionsByRole(ctx context.Context, code string) ([]domain.Permission, error) {
	args := m.Called(ctx, code)
	if v := args.Get(0); v != nil {
		return v.([]domain.Permission), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockRoleRepoV1) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.Role), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockRoleRepoV1) Create(ctx context.Context, r *domain.Role) error {
	return m.Called(ctx, r).Error(0)
}
func (m *mockRoleRepoV1) Update(ctx context.Context, r *domain.Role) error {
	return m.Called(ctx, r).Error(0)
}
func (m *mockRoleRepoV1) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRoleRepoV1) AssignPermission(ctx context.Context, roleID, permID uuid.UUID) error {
	return m.Called(ctx, roleID, permID).Error(0)
}
func (m *mockRoleRepoV1) RevokePermission(ctx context.Context, roleID, permID uuid.UUID) error {
	return m.Called(ctx, roleID, permID).Error(0)
}
func (m *mockRoleRepoV1) ListPermissionsByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	args := m.Called(ctx, roleID)
	if v := args.Get(0); v != nil {
		return v.([]domain.Permission), args.Error(1)
	}
	return nil, args.Error(1)
}

// mockAudit is a no-op audit appender.
type mockAudit struct{ mock.Mock }

func (m *mockAudit) Append(ctx context.Context, entry domain.AuditEntry) error {
	return nil
}

// chiContext injects chi URL params into a request.
func chiContext(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestPostRoles_WithValidData_Returns201(t *testing.T) {
	// Arrange
	repo := &mockRoleRepoV1{}
	h := handler.NewRoleHandlerV1(repo, &mockAudit{})

	body := map[string]string{"code": "NEW_ROLE", "name": "New Role", "scope": "GLOBAL"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewReader(b))
	w := httptest.NewRecorder()

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Role")).Return(nil)

	// Act
	h.CreateRole(w, r)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestPostRoles_WithDuplicateCode_Returns409(t *testing.T) {
	// Arrange
	repo := &mockRoleRepoV1{}
	h := handler.NewRoleHandlerV1(repo, &mockAudit{})

	body := map[string]string{"code": "EXISTING", "name": "Existing"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewReader(b))
	w := httptest.NewRecorder()

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Role")).Return(domain.ErrDuplicateCode)

	// Act
	h.CreateRole(w, r)

	// Assert
	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestPutRole_WithSystemRole_Returns403(t *testing.T) {
	// Arrange
	repo := &mockRoleRepoV1{}
	h := handler.NewRoleHandlerV1(repo, &mockAudit{})

	id := uuid.New()
	body := map[string]string{"name": "Attempted Update"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/roles/"+id.String(), bytes.NewReader(b))
	r = chiContext(r, map[string]string{"id": id.String()})
	w := httptest.NewRecorder()

	// Handler now fetches the role first to guard at the handler layer
	repo.On("FindByID", mock.Anything, id).Return(&domain.Role{ID: id, IsSystem: true}, nil)

	// Act
	h.UpdateRole(w, r)

	// Assert
	assert.Equal(t, http.StatusForbidden, w.Code)
	repo.AssertExpectations(t)
}

func TestDeleteRole_WithUsersAssigned_Returns409(t *testing.T) {
	// Arrange
	repo := &mockRoleRepoV1{}
	h := handler.NewRoleHandlerV1(repo, &mockAudit{})

	id := uuid.New()
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/roles/"+id.String(), nil)
	r = chiContext(r, map[string]string{"id": id.String()})
	w := httptest.NewRecorder()

	// Handler fetches role first; role is not a system role so guard passes
	repo.On("FindByID", mock.Anything, id).Return(&domain.Role{ID: id, IsSystem: false}, nil)
	repo.On("Delete", mock.Anything, id).Return(domain.ErrRoleHasUsers)

	// Act
	h.DeleteRole(w, r)

	// Assert
	assert.Equal(t, http.StatusConflict, w.Code)
	repo.AssertExpectations(t)
}

func TestPostRolePermissions_Succeeds_Returns200(t *testing.T) {
	// Arrange
	repo := &mockRoleRepoV1{}
	h := handler.NewRoleHandlerV1(repo, &mockAudit{})

	roleID := uuid.New()
	permID := uuid.New()
	body := map[string]string{"permission_id": permID.String()}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/roles/"+roleID.String()+"/permissions", bytes.NewReader(b))
	r = chiContext(r, map[string]string{"id": roleID.String()})
	w := httptest.NewRecorder()

	repo.On("AssignPermission", mock.Anything, roleID, permID).Return(nil)

	// Act
	h.AssignPermissionToRole(w, r)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestDeleteRolePermissions_Succeeds_Returns204(t *testing.T) {
	// Arrange
	repo := &mockRoleRepoV1{}
	h := handler.NewRoleHandlerV1(repo, &mockAudit{})

	roleID := uuid.New()
	permID := uuid.New()
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/roles/"+roleID.String()+"/permissions/"+permID.String(), nil)
	r = chiContext(r, map[string]string{"id": roleID.String(), "permission_id": permID.String()})
	w := httptest.NewRecorder()

	repo.On("RevokePermission", mock.Anything, roleID, permID).Return(nil)

	// Act
	h.RevokePermissionFromRole(w, r)

	// Assert
	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertExpectations(t)
}
