package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"axis-flow-back/internal/empresas"

	"github.com/stripe/stripe-go/v76"
	stripeSession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"
)

// StripeService handles Stripe payment session creation and webhook processing.
type StripeService struct {
	secretKey     string
	webhookSecret string
	enabled       bool
	empresaRepo   EmpresaCreatorRepo
	pagoRepo      PagoCreatorRepo
}

// NewStripeService creates a new StripeService.
func NewStripeService(
	secretKey, webhookSecret string,
	enabled bool,
	empresaRepo EmpresaCreatorRepo,
	pagoRepo PagoCreatorRepo,
) *StripeService {
	return &StripeService{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		enabled:       enabled,
		empresaRepo:   empresaRepo,
		pagoRepo:      pagoRepo,
	}
}

// CreateCheckoutSession generates a Stripe checkout URL for the given plan.
// When Stripe is disabled (dev mode), it returns a mock URL.
func (s *StripeService) CreateCheckoutSession(
	ctx context.Context,
	planID int64,
	clavePago, successURL, cancelURL string,
) (string, error) {
	if !s.enabled {
		return "https://checkout.stripe.com/dev-mock?token=" + clavePago, nil
	}

	stripe.Key = s.secretKey
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String("price_placeholder"),
				Quantity: stripe.Int64(1),
			},
		},
		ClientReferenceID: stripe.String(clavePago),
		Metadata: map[string]string{
			"plan_id":    fmt.Sprintf("%d", planID),
			"clave_pago": clavePago,
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
	}
	sess, err := stripeSession.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe.CreateCheckoutSession: %w", err)
	}
	return sess.URL, nil
}

// ProcessWebhook verifies the Stripe signature and activates the tenant if payment succeeded.
func (s *StripeService) ProcessWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	if !s.enabled {
		// Dev mode: Stripe is disabled — skip signature verification and processing.
		slog.WarnContext(ctx, "stripe webhook: dev mode — Stripe disabled, skipping signature verification and processing")
		return nil
	}

	event, err := webhook.ConstructEvent(payload, sigHeader, s.webhookSecret)
	if err != nil {
		return empresas.ErrInvalidStripeSignature
	}

	if event.Type != "checkout.session.completed" {
		return nil
	}

	// Extract clave_pago from metadata
	dataObj, ok := event.Data.Object["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("stripe.ProcessWebhook: missing metadata in event")
	}
	clavePago, _ := dataObj["clave_pago"].(string)
	if clavePago == "" {
		return fmt.Errorf("stripe.ProcessWebhook: missing clave_pago in metadata")
	}

	// Find pago by token
	pago, err := s.pagoRepo.FindByToken(ctx, clavePago)
	if err != nil {
		return fmt.Errorf("stripe.ProcessWebhook find pago: %w", err)
	}

	// Extract stripe session ID from event
	stripeSessionID, _ := event.Data.Object["id"].(string)

	// Update pago → PAID
	if err := s.pagoRepo.UpdateStatus(ctx, pago.ID, empresas.PagoStatusPaid, stripeSessionID); err != nil {
		return fmt.Errorf("stripe.ProcessWebhook update pago: %w", err)
	}

	// Update empresa → ACTIVE with 30-day vigencia
	vigencia := time.Now().AddDate(0, 0, 30)
	if err := s.empresaRepo.UpdateStatus(ctx, pago.EmpresaID, empresas.EmpresaStatusActive, &vigencia); err != nil {
		return fmt.Errorf("stripe.ProcessWebhook update empresa: %w", err)
	}

	return nil
}
