package unit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/handler"
	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── auth handler mock service ─────────────────────────────────────────────────

type mockAuthService struct{ mock.Mock }

func (m *mockAuthService) Login(ctx context.Context, email, password, deviceType, deviceID, ip string) (service.TokenPair, error) {
	args := m.Called(ctx, email, password, deviceType, deviceID, ip)
	return args.Get(0).(service.TokenPair), args.Error(1)
}
func (m *mockAuthService) RefreshToken(ctx context.Context, raw string) (service.TokenPair, error) {
	args := m.Called(ctx, raw)
	return args.Get(0).(service.TokenPair), args.Error(1)
}
func (m *mockAuthService) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if v := args.Get(0); v != nil {
		return v.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockAuthService) GetPermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	args := m.Called(ctx, userID)
	if v := args.Get(0); v != nil {
		return v.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestPostLogin_StoresRemoteAddrHostWithoutPort(t *testing.T) {
	svc := &mockAuthService{}
	h := handler.NewAuthHandler(svc)

	svc.On("Login", mock.Anything, "user@example.com", "pass", "WEB", "", "::1").
		Return(service.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil)

	body, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "pass", "device_type": "WEB"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/", bytes.NewReader(body))
	req.RemoteAddr = "[::1]:53869"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestPostLogin_UsesFirstForwardedIP(t *testing.T) {
	svc := &mockAuthService{}
	h := handler.NewAuthHandler(svc)

	svc.On("Login", mock.Anything, "user@example.com", "pass", "WEB", "", "203.0.113.10").
		Return(service.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil)

	body, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "pass", "device_type": "WEB"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/", bytes.NewReader(body))
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.2")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestPostLogin_WithValidCredentials_Returns200(t *testing.T) {
	svc := &mockAuthService{}
	h := handler.NewAuthHandler(svc)

	svc.On("Login", mock.Anything, "user@example.com", "pass", "WEB", "", mock.AnythingOfType("string")).
		Return(service.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil)

	body, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "pass", "device_type": "WEB"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "access", resp["access_token"])
	assert.Equal(t, "refresh", resp["refresh_token"])
}

func TestPostLogin_WithInvalidCredentials_Returns401(t *testing.T) {
	svc := &mockAuthService{}
	h := handler.NewAuthHandler(svc)

	svc.On("Login", mock.Anything, "user@example.com", "wrong", "WEB", "", mock.AnythingOfType("string")).
		Return(service.TokenPair{}, service.ErrUnauthorized)

	body, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "wrong", "device_type": "WEB"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetMe_WithValidJWT_Returns200(t *testing.T) {
	svc := &mockAuthService{}
	h := handler.NewAuthHandler(svc)

	userID := uuid.New()
	u := &domain.User{
		ID:        userID,
		TenantID:  uuid.New(),
		Email:     "user@example.com",
		FirstName: "John",
		LastName:  "Doe",
		Status:    domain.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	svc.On("GetProfile", mock.Anything, userID).Return(u, nil)
	svc.On("GetPermissionCodes", mock.Anything, userID).Return([]string{"roles:update"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me/", nil)
	// Inject user ID into context as the middleware would
	ctx := context.WithValue(req.Context(), middleware.ContextKeyUserID, userID.String())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Me(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetMe_WithoutJWT_Returns401(t *testing.T) {
	svc := &mockAuthService{}
	h := handler.NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me/", nil)
	// No user in context — simulates a request that bypassed the JWT middleware
	w := httptest.NewRecorder()

	h.Me(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPostRefresh_WithValidToken_Returns200(t *testing.T) {
	svc := &mockAuthService{}
	h := handler.NewAuthHandler(svc)

	svc.On("RefreshToken", mock.Anything, "valid-refresh-token").
		Return(service.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}, nil)

	body, _ := json.Marshal(map[string]string{"refresh_token": "valid-refresh-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/token/refresh/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.RefreshToken(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "new-access", resp["access_token"])
}
