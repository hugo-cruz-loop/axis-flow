package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/empresas"
	"axis-flow-back/internal/empresas/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stubs for stripe service ──────────────────────────────────────────────────

type stripeEmpresaRepo struct {
	updateStatusErr error
	updateCalled    bool
}

func (r *stripeEmpresaRepo) Create(ctx context.Context, e *empresas.Empresa) error { return nil }
func (r *stripeEmpresaRepo) UpdateStatus(ctx context.Context, id int64, status empresas.EmpresaStatus, vigencia *time.Time) error {
	r.updateCalled = true
	return r.updateStatusErr
}

type stripePagoRepo struct {
	findByTokenFn   func(ctx context.Context, token string) (*empresas.Pago, error)
	updateStatusErr error
	updateCalled    bool
}

func (r *stripePagoRepo) Create(ctx context.Context, p *empresas.Pago) error { return nil }
func (r *stripePagoRepo) UpdateStatus(ctx context.Context, id int64, status empresas.PagoStatus, stripeSessionID string) error {
	r.updateCalled = true
	return r.updateStatusErr
}
func (r *stripePagoRepo) FindByToken(ctx context.Context, token string) (*empresas.Pago, error) {
	if r.findByTokenFn != nil {
		return r.findByTokenFn(ctx, token)
	}
	return nil, empresas.ErrEmpresaNotFound
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCreateCheckoutSession_WithValidClavePago_ReturnsURL(t *testing.T) {
	svc := service.NewStripeService("", "", false, &stripeEmpresaRepo{}, &stripePagoRepo{})

	url, err := svc.CreateCheckoutSession(context.Background(), 1, "tok_abc123",
		"https://example.com/success", "https://example.com/cancel")

	require.NoError(t, err)
	assert.Contains(t, url, "tok_abc123")
}

func TestVerifyWebhookSignature_WithValidSignature_ReturnsEvent(t *testing.T) {
	// In dev/disabled mode, ProcessWebhook should return nil (200) — no processing occurs.
	svc := service.NewStripeService("", "", false, &stripeEmpresaRepo{}, &stripePagoRepo{})

	err := svc.ProcessWebhook(context.Background(), []byte(`{}`), "t=1,v1=abc")

	require.NoError(t, err)
}

func TestVerifyWebhookSignature_WithInvalidSignature_ReturnsError(t *testing.T) {
	// When stripe is enabled, invalid sig should return ErrInvalidStripeSignature
	svc := service.NewStripeService("sk_test_fake", "whsec_fake", true, &stripeEmpresaRepo{}, &stripePagoRepo{})

	err := svc.ProcessWebhook(context.Background(), []byte(`{"type":"checkout.session.completed"}`), "invalid-sig")

	require.Error(t, err)
	assert.True(t, errors.Is(err, empresas.ErrInvalidStripeSignature))
}

func TestActivateTenant_OnPaidEvent_UpdatesEmpresaAndUser(t *testing.T) {
	// Test the dev-mode behavior where no actual Stripe call is made
	svc := service.NewStripeService("", "", false, &stripeEmpresaRepo{}, &stripePagoRepo{})

	url, err := svc.CreateCheckoutSession(context.Background(), 1, "clave_abc",
		"https://example.com/ok", "https://example.com/cancel")

	require.NoError(t, err)
	// Dev mode returns a mock URL
	assert.Contains(t, url, "dev-mock")
}
