package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/empresas"
	"axis-flow-back/internal/empresas/handler"
	"axis-flow-back/internal/empresas/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubOnboardingSvc struct {
	result service.OnboardingResult
	err    error
}

func (s *stubOnboardingSvc) Register(ctx context.Context, req service.OnboardingRequest) (service.OnboardingResult, error) {
	return s.result, s.err
}

type stubStripeSvc struct {
	checkoutURL string
	checkoutErr error
	webhookErr  error
}

func (s *stubStripeSvc) CreateCheckoutSession(ctx context.Context, planID int64, clavePago, successURL, cancelURL string) (string, error) {
	return s.checkoutURL, s.checkoutErr
}

func (s *stubStripeSvc) ProcessWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	return s.webhookErr
}

// ── helpers ───────────────────────────────────────────────────────────────────

func postJSON(t *testing.T, h http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestPostAlta_WithValidBody_Returns201(t *testing.T) {
	fixedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	onb := &stubOnboardingSvc{
		result: service.OnboardingResult{
			EmpresaID:  1,
			UserID:     fixedUserID,
			EmpleadoID: 1,
			ClavePago:  "tok_abc",
		},
	}
	h := handler.NewPublicHandler(onb, &stubStripeSvc{})

	w := postJSON(t, h.Alta, map[string]any{
		"nombre":    "Test SA",
		"direccion": "Av Test 1",
		"telefono":  "5551234567",
		"plan_id":   1,
		"monto":     999.0,
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(1), data["empresa_id"])
	assert.Equal(t, fixedUserID.String(), data["user_id"])
	assert.Equal(t, float64(1), data["empleado_id"])
	assert.Equal(t, "tok_abc", data["clave_pago"])
}

func TestPostAlta_WithMissingNombre_Returns400(t *testing.T) {
	onb := &stubOnboardingSvc{}
	h := handler.NewPublicHandler(onb, &stubStripeSvc{})

	w := postJSON(t, h.Alta, map[string]any{
		"direccion": "Av Test 1",
		"plan_id":   1,
		"monto":     999.0,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostCheckout_WithValidBody_Returns200(t *testing.T) {
	stripe := &stubStripeSvc{checkoutURL: "https://checkout.stripe.com/abc"}
	h := handler.NewPublicHandler(&stubOnboardingSvc{}, stripe)

	w := postJSON(t, h.Checkout, map[string]any{
		"plan_id":     1,
		"clave_pago":  "tok_abc",
		"success_url": "https://example.com/ok",
		"cancel_url":  "https://example.com/cancel",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "https://checkout.stripe.com/abc", data["checkout_url"])
}

func TestPostWebhook_WithInvalidSignature_Returns400(t *testing.T) {
	stripe := &stubStripeSvc{webhookErr: empresas.ErrInvalidStripeSignature}
	h := handler.NewPublicHandler(&stubOnboardingSvc{}, stripe)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Stripe-Signature", "invalid")
	w := httptest.NewRecorder()
	h.Webhook(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostWebhook_WithValidEvent_Returns200(t *testing.T) {
	stripe := &stubStripeSvc{webhookErr: nil}
	h := handler.NewPublicHandler(&stubOnboardingSvc{}, stripe)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Stripe-Signature", "valid")
	w := httptest.NewRecorder()
	h.Webhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "processed", data["status"])
}
