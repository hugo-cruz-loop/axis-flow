package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"axis-flow-back/internal/crypto"
	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ActivationServicer is the interface the ActivationHandler depends on.
type ActivationServicer interface {
	FindUserByTokenHash(ctx context.Context, hash string) (*domain.User, error)
	ValidateAndConsumeToken(ctx context.Context, rawToken string, tokenType domain.TokenType) error
	ActivateUser(ctx context.Context, userID uuid.UUID, newPasswordHash string) error
}

// ActivationHandler handles account activation and password-reset flows.
type ActivationHandler struct {
	svc ActivationServicer
}

// NewActivationHandler creates an ActivationHandler backed by the given service.
func NewActivationHandler(svc ActivationServicer) *ActivationHandler {
	return &ActivationHandler{svc: svc}
}

// ResetPass handles PATCH /api/users/reset-pass/
// It validates a one-use activation token, hashes the new password, and activates the account.
func (h *ActivationHandler) ResetPass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.Password == "" {
		writeError(w, "token and password are required", http.StatusBadRequest)
		return
	}

	// Derive hash to look up the user without persisting raw token
	tokenHash := crypto.HashToken(req.Token)

	user, err := h.svc.FindUserByTokenHash(r.Context(), tokenHash)
	if err != nil {
		writeError(w, "invalid token", http.StatusBadRequest)
		return
	}

	if err := h.svc.ValidateAndConsumeToken(r.Context(), req.Token, domain.TokenActivation); err != nil {
		switch {
		case errors.Is(err, service.ErrTokenExpired):
			writeError(w, "token has expired", http.StatusBadRequest)
		case errors.Is(err, service.ErrTokenAlreadyUsed):
			writeError(w, "token has already been used", http.StatusBadRequest)
		default:
			writeError(w, "invalid token", http.StatusBadRequest)
		}
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(r.Context(), "reset-pass: hash password", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.svc.ActivateUser(r.Context(), user.ID, string(hash)); err != nil {
		slog.ErrorContext(r.Context(), "reset-pass: activate user",
			slog.String("user_id", user.ID.String()),
			slog.String("error", err.Error()),
		)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "reset-pass: account activated", slog.String("user_id", user.ID.String()))
	writeJSON(w, http.StatusOK, map[string]string{"message": "account activated successfully"})
}

