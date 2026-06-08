package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// UserRoleRepositorier is the consumer-package interface for user-role assignments.
type UserRoleRepositorier interface {
	Assign(ctx context.Context, userID, roleID uuid.UUID, assignedBy uuid.UUID) error
	Revoke(ctx context.Context, userID, roleID uuid.UUID) error
}

// UserRoleHandler handles /api/v1/users/{id}/roles endpoints.
type UserRoleHandler struct {
	repo  UserRoleRepositorier
	audit AuditAppender
}

// NewUserRoleHandler creates a UserRoleHandler.
func NewUserRoleHandler(repo UserRoleRepositorier, audit AuditAppender) *UserRoleHandler {
	return &UserRoleHandler{repo: repo, audit: audit}
}

// AssignRole handles POST /api/v1/users/{id}/roles
func (h *UserRoleHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req struct {
		RoleID string `json:"role_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RoleID == "" {
		writeError(w, "role_id is required", http.StatusBadRequest)
		return
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		writeError(w, "invalid role_id", http.StatusBadRequest)
		return
	}

	actorID := actorFromContext(r)
	var assignedBy uuid.UUID
	if actorID != nil {
		assignedBy = *actorID
	}

	if err := h.repo.Assign(r.Context(), userID, roleID, assignedBy); err != nil {
		slog.ErrorContext(r.Context(), "user_role_handler.AssignRole", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID:  actorID,
		TargetUserID: &userID,
		Action:       "USER_ROLE_ASSIGNED",
		Metadata:     map[string]any{"user_id": userID.String(), "role_id": roleID.String()},
		IPAddress:    clientIP(r),
		TraceID:      middleware.TraceIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusCreated, map[string]string{"status": "assigned"})
}

// RevokeRole handles DELETE /api/v1/users/{id}/roles/{role_id}
func (h *UserRoleHandler) RevokeRole(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid user id", http.StatusBadRequest)
		return
	}
	roleID, err := uuid.Parse(chi.URLParam(r, "role_id"))
	if err != nil {
		writeError(w, "invalid role_id", http.StatusBadRequest)
		return
	}

	if err := h.repo.Revoke(r.Context(), userID, roleID); err != nil {
		slog.ErrorContext(r.Context(), "user_role_handler.RevokeRole", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID:  actorID,
		TargetUserID: &userID,
		Action:       "USER_ROLE_REVOKED",
		Metadata:     map[string]any{"user_id": userID.String(), "role_id": roleID.String()},
		IPAddress:    clientIP(r),
		TraceID:      middleware.TraceIDFromContext(r.Context()),
	})

	w.WriteHeader(http.StatusNoContent)
}
