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
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── mock activation service ───────────────────────────────────────────────────

type mockActivationService struct{ mock.Mock }

func (m *mockActivationService) ValidateAndConsumeToken(ctx context.Context, raw string, t domain.TokenType) error {
	return m.Called(ctx, raw, t).Error(0)
}
func (m *mockActivationService) ActivateUser(ctx context.Context, userID uuid.UUID, newPasswordHash string) error {
	return m.Called(ctx, userID, newPasswordHash).Error(0)
}
func (m *mockActivationService) FindUserByTokenHash(ctx context.Context, hash string) (*domain.User, error) {
	args := m.Called(ctx, hash)
	if v := args.Get(0); v != nil {
		return v.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestPatchResetPass_WithValidToken_Returns200(t *testing.T) {
	svc := &mockActivationService{}
	h := handler.NewActivationHandler(svc)

	userID := uuid.New()
	user := &domain.User{ID: userID, Status: domain.StatusPendingActivation}

	svc.On("FindUserByTokenHash", mock.Anything, mock.AnythingOfType("string")).Return(user, nil)
	svc.On("ValidateAndConsumeToken", mock.Anything, "valid-token", domain.TokenActivation).Return(nil)
	svc.On("ActivateUser", mock.Anything, userID, mock.AnythingOfType("string")).Return(nil)

	body, _ := json.Marshal(map[string]string{"token": "valid-token", "password": "new-secure-pass"})
	req := httptest.NewRequest(http.MethodPatch, "/api/users/reset-pass/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ResetPass(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPatchResetPass_WithExpiredToken_Returns400(t *testing.T) {
	svc := &mockActivationService{}
	h := handler.NewActivationHandler(svc)

	userID := uuid.New()
	user := &domain.User{ID: userID, Status: domain.StatusPendingActivation}

	svc.On("FindUserByTokenHash", mock.Anything, mock.AnythingOfType("string")).Return(user, nil)
	svc.On("ValidateAndConsumeToken", mock.Anything, "expired-token", domain.TokenActivation).Return(service.ErrTokenExpired)

	body, _ := json.Marshal(map[string]string{"token": "expired-token", "password": "new-secure-pass"})
	req := httptest.NewRequest(http.MethodPatch, "/api/users/reset-pass/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ResetPass(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchResetPass_WithAlreadyUsedToken_Returns400(t *testing.T) {
	svc := &mockActivationService{}
	h := handler.NewActivationHandler(svc)

	userID := uuid.New()
	user := &domain.User{ID: userID, Status: domain.StatusPendingActivation}

	svc.On("FindUserByTokenHash", mock.Anything, mock.AnythingOfType("string")).Return(user, nil)
	svc.On("ValidateAndConsumeToken", mock.Anything, "used-token", domain.TokenActivation).Return(service.ErrTokenAlreadyUsed)

	body, _ := json.Marshal(map[string]string{"token": "used-token", "password": "new-secure-pass"})
	req := httptest.NewRequest(http.MethodPatch, "/api/users/reset-pass/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ResetPass(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
