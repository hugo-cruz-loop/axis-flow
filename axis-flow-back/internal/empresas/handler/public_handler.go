package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"axis-flow-back/internal/empresas"
	"axis-flow-back/internal/empresas/service"
)

// OnboardingServicer is the interface the PublicHandler depends on for onboarding.
type OnboardingServicer interface {
	Register(ctx context.Context, req service.OnboardingRequest) (service.OnboardingResult, error)
}

// StripeServicer is the interface the PublicHandler depends on for Stripe operations.
type StripeServicer interface {
	CreateCheckoutSession(ctx context.Context, planID int64, clavePago, successURL, cancelURL string) (string, error)
	ProcessWebhook(ctx context.Context, payload []byte, sigHeader string) error
}

// PublicHandler handles unauthenticated empresa endpoints.
type PublicHandler struct {
	onboarding OnboardingServicer
	stripe     StripeServicer
}

// NewPublicHandler creates a new PublicHandler.
func NewPublicHandler(onboarding OnboardingServicer, stripe StripeServicer) *PublicHandler {
	return &PublicHandler{onboarding: onboarding, stripe: stripe}
}

// Alta handles POST /api/v1/empresa/alta
func (h *PublicHandler) Alta(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Nombre        string  `json:"nombre"`
		Direccion     string  `json:"direccion"`
		Telefono      string  `json:"telefono"`
		PlanID        int64   `json:"plan_id"`
		Monto         float64 `json:"monto"`
		Representante struct {
			Email           string `json:"email"`
			Nombre          string `json:"nombre"`
			ApellidoPaterno string `json:"apellido_paterno"`
			ApellidoMaterno string `json:"apellido_materno"`
		} `json:"representante"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Nombre == "" {
		writeError(w, "nombre is required", http.StatusBadRequest)
		return
	}
	if body.PlanID == 0 {
		writeError(w, "plan_id is required", http.StatusBadRequest)
		return
	}

	req := service.OnboardingRequest{
		Nombre:                 body.Nombre,
		Direccion:              body.Direccion,
		Telefono:               body.Telefono,
		PlanID:                 body.PlanID,
		PlanMonto:              body.Monto,
		RepresentanteEmail:     body.Representante.Email,
		RepresentanteNombre:    body.Representante.Nombre,
		RepresentanteApPaterno: body.Representante.ApellidoPaterno,
		RepresentanteApMaterno: body.Representante.ApellidoMaterno,
	}

	result, err := h.onboarding.Register(r.Context(), req)
	if err != nil {
		if errors.Is(err, empresas.ErrDuplicateRFC) {
			writeError(w, "RFC already registered", http.StatusConflict)
			return
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"empresa_id":  result.EmpresaID,
			"user_id":     result.UserID,
			"empleado_id": result.EmpleadoID,
			"clave_pago":  result.ClavePago,
		},
	})
}

// Checkout handles POST /api/v1/empresa/checkout
func (h *PublicHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PlanID     int64  `json:"plan_id"`
		ClavePago  string `json:"clave_pago"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	url, err := h.stripe.CreateCheckoutSession(r.Context(), body.PlanID, body.ClavePago, body.SuccessURL, body.CancelURL)
	if err != nil {
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"checkout_url": url,
		},
	})
}

// Webhook handles POST /api/v1/empresa/webhook
func (h *PublicHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	// Must read raw body before any parsing — Stripe signature covers the raw bytes
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, "could not read body", http.StatusBadRequest)
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if err := h.stripe.ProcessWebhook(r.Context(), payload, sigHeader); err != nil {
		if errors.Is(err, empresas.ErrInvalidStripeSignature) {
			writeError(w, "invalid stripe signature", http.StatusBadRequest)
			return
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"status": "processed",
		},
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, status, map[string]string{"error": msg})
}
