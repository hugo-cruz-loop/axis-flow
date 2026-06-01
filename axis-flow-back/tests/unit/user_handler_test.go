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
	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── user service mock ─────────────────────────────────────────────────────────

type mockUserService struct{ mock.Mock }

func (m *mockUserService) CreateUser(ctx context.Context, req service.CreateUserRequest) (domain.User, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(domain.User), args.Error(1)
}
func (m *mockUserService) ListByRole(ctx context.Context, roleCode string, page, pageSize int) ([]domain.User, error) {
	args := m.Called(ctx, roleCode, page, pageSize)
	if v := args.Get(0); v != nil {
		return v.([]domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockUserService) GetLastCreatedID(ctx context.Context) (uuid.UUID, error) {
	args := m.Called(ctx)
	return args.Get(0).(uuid.UUID), args.Error(1)
}
func (m *mockUserService) UpdateFirebaseToken(ctx context.Context, userID uuid.UUID, token string) error {
	return m.Called(ctx, userID, token).Error(0)
}
func (m *mockUserService) GetFirebaseToken(ctx context.Context, userID uuid.UUID) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}
func (m *mockUserService) DeleteAllData(ctx context.Context, actorID uuid.UUID) error {
	return m.Called(ctx, actorID).Error(0)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ctxWithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, middleware.ContextKeyRole, role)
}

func ctxWithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, handler.CtxUserID, id)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestPostUsers_WithAdminRole_Returns201(t *testing.T) {
	svc := &mockUserService{}
	h := handler.NewUserHandler(svc, false)

	actorID := uuid.New()
	created := domain.User{
		ID:     uuid.New(),
		Email:  "new@example.com",
		Status: domain.StatusPendingActivation,
	}
	svc.On("CreateUser", mock.Anything, mock.AnythingOfType("service.CreateUserRequest")).
		Return(created, nil)

	body, _ := json.Marshal(map[string]string{
		"email":      "new@example.com",
		"first_name": "Test",
		"last_name":  "User",
		"role_code":  "Cliente",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/users/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := ctxWithRole(ctxWithUserID(req.Context(), actorID.String()), "Administrador")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.CreateUser(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestPostUsers_WithoutAdminRole_Returns403(t *testing.T) {
	svc := &mockUserService{}
	h := handler.NewUserHandler(svc, false)

	body, _ := json.Marshal(map[string]string{
		"email":     "new@example.com",
		"role_code": "Cliente",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/users/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := ctxWithRole(req.Context(), "Cliente") // non-admin role
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.CreateUser(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetFiltradoUser_WithValidRole_Returns200(t *testing.T) {
	svc := &mockUserService{}
	h := handler.NewUserHandler(svc, false)

	users := []domain.User{{ID: uuid.New(), Email: "a@example.com"}}
	svc.On("ListByRole", mock.Anything, "Cliente", 1, 20).Return(users, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/users/filtradoUser/Cliente/", nil)
	req.SetPathValue("role", "Cliente")
	ctx := ctxWithRole(req.Context(), "Administrador")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.ListByRole(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetUltimoID_Returns200(t *testing.T) {
	svc := &mockUserService{}
	h := handler.NewUserHandler(svc, false)

	id := uuid.New()
	svc.On("GetLastCreatedID", mock.Anything).Return(id, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/users/ultimoID/", nil)
	ctx := ctxWithRole(req.Context(), "Administrador")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.GetLastCreatedID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, id.String(), resp["id"])
}

func TestPatchUpdateFirebaseToken_WithValidJWT_Returns200(t *testing.T) {
	svc := &mockUserService{}
	h := handler.NewUserHandler(svc, false)

	userID := uuid.New()
	svc.On("UpdateFirebaseToken", mock.Anything, userID, "fcm-xyz").Return(nil)

	body, _ := json.Marshal(map[string]string{"firebase_token": "fcm-xyz"})
	req := httptest.NewRequest(http.MethodPatch, "/api/users/update-firebase-token/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := ctxWithRole(ctxWithUserID(req.Context(), userID.String()), "Cliente")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.UpdateFirebaseToken(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetFirebaseToken_WithAdminRole_Returns200(t *testing.T) {
	svc := &mockUserService{}
	h := handler.NewUserHandler(svc, false)

	targetID := uuid.New()
	svc.On("GetFirebaseToken", mock.Anything, targetID).Return("fcm-token-value", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/users/get-firebase-token/"+targetID.String()+"/", nil)
	req.SetPathValue("id", targetID.String())
	ctx := ctxWithRole(req.Context(), "Administrador")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.GetFirebaseToken(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteAllData_InDevelopment_Returns200(t *testing.T) {
	svc := &mockUserService{}
	// deleteEnabled = true simulates non-production env with feature flag
	h := handler.NewUserHandler(svc, true)

	actorID := uuid.New()
	svc.On("DeleteAllData", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/delete-all-data/", nil)
	ctx := ctxWithRole(ctxWithUserID(req.Context(), actorID.String()), "Administrador")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.DeleteAllData(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteAllData_InProduction_Returns403OrNotFound(t *testing.T) {
	svc := &mockUserService{}
	// deleteEnabled = false simulates production guard
	h := handler.NewUserHandler(svc, false)

	actorID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/users/delete-all-data/", nil)
	ctx := ctxWithRole(ctxWithUserID(req.Context(), actorID.String()), "Administrador")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.DeleteAllData(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
