// Package handler provides the HTTP handlers for the identity service.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
)

// CtxUserID is the context key used to read the authenticated user ID from context.
// Exported so handler tests can inject it directly.
const CtxUserID = contextKey("userID")

type contextKey string

// AuthServicer is the interface the AuthHandler depends on.
// Defined here — in the consumer package — following DI principles.
type AuthServicer interface {
	Login(ctx context.Context, email, password, deviceType, deviceID, ipAddress string) (service.TokenPair, error)
	RefreshToken(ctx context.Context, rawRefreshToken string) (service.TokenPair, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	GetPermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// AuthHandler handles HTTP endpoints for authentication.
type AuthHandler struct {
	svc AuthServicer
}

// NewAuthHandler creates an AuthHandler backed by the given service.
func NewAuthHandler(svc AuthServicer) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login handles POST /api/auth/login/
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		DeviceType string `json:"device_type"`
		DeviceID   string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceType == "" {
		req.DeviceType = "WEB"
	}

	ip := clientIP(r)
	pair, err := h.svc.Login(r.Context(), req.Email, req.Password, req.DeviceType, req.DeviceID, ip)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			writeError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		slog.ErrorContext(r.Context(), "login handler error", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
	})
}

func clientIP(r *http.Request) string {
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if comma := strings.Index(forwarded, ","); comma >= 0 {
			return strings.TrimSpace(forwarded[:comma])
		}
		return forwarded
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}

// RefreshToken handles POST /api/auth/token/refresh/
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	pair, err := h.svc.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			writeError(w, "invalid or expired refresh token", http.StatusUnauthorized)
			return
		}
		slog.ErrorContext(r.Context(), "refresh handler error", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
	})
}

// Me handles GET /api/auth/me/
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	rawID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || rawID == "" {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(rawID)
	if err != nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.svc.GetProfile(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "me handler error", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Role is embedded in the JWT by issueTokenPair and injected into context
	// by the JWTAuth middleware — no extra DB query needed here.
	role, _ := middleware.RoleFromContext(r.Context())
	permissions, err := h.svc.GetPermissionCodes(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "me permissions error", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          user.ID,
		"tenant_id":   user.TenantID,
		"email":       user.Email,
		"first_name":  user.FirstName,
		"last_name":   user.LastName,
		"phone":       user.Phone,
		"role":        role,
		"permissions": permissions,
		"status":      user.Status,
		"created_at":  user.CreatedAt,
		"updated_at":  user.UpdatedAt,
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, status, map[string]string{"error": msg})
}
