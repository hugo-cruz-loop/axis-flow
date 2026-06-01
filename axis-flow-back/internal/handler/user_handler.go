package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/service"

	"github.com/google/uuid"
)

// adminRoles lists the roles that are considered administrative.
var adminRoles = map[string]struct{}{
	"AdminCheckOn":  {},
	"Administrador": {},
}

// UserServicer is the interface UserHandler depends on.
type UserServicer interface {
	CreateUser(ctx context.Context, req service.CreateUserRequest) (domain.User, error)
	ListByRole(ctx context.Context, roleCode string, page, pageSize int) ([]domain.User, error)
	GetLastCreatedID(ctx context.Context) (uuid.UUID, error)
	UpdateFirebaseToken(ctx context.Context, userID uuid.UUID, token string) error
	GetFirebaseToken(ctx context.Context, userID uuid.UUID) (string, error)
	DeleteAllData(ctx context.Context, actorID uuid.UUID) error
}

// UserHandler handles HTTP endpoints for user management.
type UserHandler struct {
	svc           UserServicer
	deleteEnabled bool // true only when APP_ENV != production AND FEATURE_DELETE_ALL_DATA == true
}

// NewUserHandler creates a UserHandler.
// deleteEnabled must be set to false in production.
func NewUserHandler(svc UserServicer, deleteEnabled bool) *UserHandler {
	return &UserHandler{svc: svc, deleteEnabled: deleteEnabled}
}

// CreateUser handles POST /api/users/
// Requires admin role (AdminCheckOn or Administrador).
// Primary enforcement: RequireRoles middleware in the route chain.
// Defense-in-depth: inline isAdmin check below.
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
		RoleCode  string `json:"role_code"`
		TenantID  string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.RoleCode == "" {
		writeError(w, "email and role_code are required", http.StatusBadRequest)
		return
	}

	actorIDStr, _ := r.Context().Value(CtxUserID).(string)
	actorID, _ := uuid.Parse(actorIDStr)
	tenantID, _ := uuid.Parse(req.TenantID)

	svcReq := service.CreateUserRequest{
		ActorUserID: actorID,
		TenantID:    tenantID,
		Email:       req.Email,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Phone:       req.Phone,
		RoleCode:    req.RoleCode,
	}

	user, err := h.svc.CreateUser(r.Context(), svcReq)
	if err != nil {
		if errors.Is(err, service.ErrDuplicateEmail) {
			writeError(w, "email already in use", http.StatusConflict)
			return
		}
		slog.ErrorContext(r.Context(), "create user handler", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     user.ID,
		"email":  user.Email,
		"status": user.Status,
	})
}

// ListByRole handles GET /api/users/filtradoUser/{role}/
func (h *UserHandler) ListByRole(w http.ResponseWriter, r *http.Request) {
	roleCode := r.PathValue("role")
	if roleCode == "" {
		writeError(w, "role is required", http.StatusBadRequest)
		return
	}

	page := 1
	pageSize := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 {
			pageSize = v
		}
	}

	users, err := h.svc.ListByRole(r.Context(), roleCode, page, pageSize)
	if err != nil {
		slog.ErrorContext(r.Context(), "list by role handler", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	type userView struct {
		ID        uuid.UUID         `json:"id"`
		Email     string            `json:"email"`
		FirstName string            `json:"first_name"`
		LastName  string            `json:"last_name"`
		Status    domain.UserStatus `json:"status"`
	}
	result := make([]userView, 0, len(users))
	for _, u := range users {
		result = append(result, userView{
			ID:        u.ID,
			Email:     u.Email,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Status:    u.Status,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// GetLastCreatedID handles GET /api/users/ultimoID/
func (h *UserHandler) GetLastCreatedID(w http.ResponseWriter, r *http.Request) {
	id, err := h.svc.GetLastCreatedID(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "get last created id handler", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id.String()})
}

// UpdateFirebaseToken handles PATCH /api/users/update-firebase-token/
// FCM token value is NEVER logged.
func (h *UserHandler) UpdateFirebaseToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FirebaseToken string `json:"firebase_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FirebaseToken == "" {
		writeError(w, "firebase_token is required", http.StatusBadRequest)
		return
	}

	rawID, _ := r.Context().Value(CtxUserID).(string)
	userID, err := uuid.Parse(rawID)
	if err != nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.svc.UpdateFirebaseToken(r.Context(), userID, req.FirebaseToken); err != nil {
		slog.ErrorContext(r.Context(), "update firebase token handler",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "firebase token updated"})
}

// GetFirebaseToken handles GET /api/users/get-firebase-token/{id}/
// Requires admin role. FCM token value is NEVER logged.
// Primary enforcement: RequireRoles middleware in the route chain.
// Defense-in-depth: inline isAdmin check below.
func (h *UserHandler) GetFirebaseToken(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	rawID := r.PathValue("id")
	userID, err := uuid.Parse(rawID)
	if err != nil {
		writeError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	token, err := h.svc.GetFirebaseToken(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "get firebase token handler",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"firebase_token": token})
}

// ResetPassApp handles PATCH /api/users/reset-pass-app/{id}/
// Treated as public (no JWT required) per spec TBD decision.
func (h *UserHandler) ResetPassApp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
		writeError(w, "password is required", http.StatusBadRequest)
		return
	}
	// TBD: actual password-reset-app logic (spec deferred)
	writeJSON(w, http.StatusOK, map[string]string{"message": "password reset requested"})
}

// DeleteAllData handles DELETE /api/users/delete-all-data/
// The deleteEnabled flag guards this at startup; this handler also re-checks
// to defend against any routing edge case.
func (h *UserHandler) DeleteAllData(w http.ResponseWriter, r *http.Request) {
	if !h.deleteEnabled {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	if !h.isAdmin(r) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	rawID, _ := r.Context().Value(CtxUserID).(string)
	actorID, _ := uuid.Parse(rawID)

	if err := h.svc.DeleteAllData(r.Context(), actorID); err != nil {
		slog.ErrorContext(r.Context(), "delete all data handler", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "all data deleted"})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (h *UserHandler) isAdmin(r *http.Request) bool {
	role, _ := middleware.RoleFromContext(r.Context())
	_, ok := adminRoles[role]
	return ok
}
