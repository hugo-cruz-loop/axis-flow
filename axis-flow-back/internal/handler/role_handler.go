package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RoleRepositorier is the interface the RoleHandler depends on (read-only, legacy /api/roles).
type RoleRepositorier interface {
	ListAll(ctx context.Context) ([]domain.RoleWithCount, error)
	ListPermissionsByRole(ctx context.Context, roleCode string) ([]domain.Permission, error)
}

// RoleHandler handles HTTP endpoints for role management (legacy read-only /api/roles).
type RoleHandler struct {
	repo RoleRepositorier
}

// NewRoleHandler creates a RoleHandler backed by the given repository.
func NewRoleHandler(repo RoleRepositorier) *RoleHandler {
	return &RoleHandler{repo: repo}
}

// roleResponse is the JSON shape for a role with permission count.
type roleResponse struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Scope           string `json:"scope"`
	IsSystem        bool   `json:"is_system"`
	PermissionCount int    `json:"permission_count"`
}

// permissionResponse is the JSON shape for a permission.
type permissionResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Module      string `json:"module"`
	Description string `json:"description"`
}

// ListRoles handles GET /api/roles/
func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.repo.ListAll(r.Context())
	if err != nil {
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := make([]roleResponse, 0, len(roles))
	for _, rc := range roles {
		resp = append(resp, roleResponse{
			ID:              rc.ID.String(),
			Code:            rc.Code,
			Name:            rc.Name,
			Scope:           rc.Scope,
			IsSystem:        rc.IsSystem,
			PermissionCount: rc.PermissionCount,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListPermissions handles GET /api/roles/{code}/permissions
func (h *RoleHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		writeError(w, "role code is required", http.StatusBadRequest)
		return
	}

	perms, err := h.repo.ListPermissionsByRole(r.Context(), code)
	if err != nil {
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := make([]permissionResponse, 0, len(perms))
	for _, p := range perms {
		resp = append(resp, permissionResponse{
			ID:          p.ID.String(),
			Code:        p.Code,
			Name:        p.Name,
			Module:      p.Module,
			Description: p.Description,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// ─── v1 CRUD ─────────────────────────────────────────────────────────────────

// RoleRepositorierV1 extends the read-only interface with CRUD + assignment operations.
type RoleRepositorierV1 interface {
	RoleRepositorier
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	Create(ctx context.Context, r *domain.Role) error
	Update(ctx context.Context, r *domain.Role) error
	Delete(ctx context.Context, id uuid.UUID) error
	AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID) error
	RevokePermission(ctx context.Context, roleID, permissionID uuid.UUID) error
	ListPermissionsByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error)
}

// AuditAppender is the minimal interface required for audit logging in handlers.
type AuditAppender interface {
	Append(ctx context.Context, entry domain.AuditEntry) error
}

// RoleHandlerV1 handles /api/v1/roles CRUD endpoints.
type RoleHandlerV1 struct {
	repo  RoleRepositorierV1
	audit AuditAppender
}

// NewRoleHandlerV1 creates a RoleHandlerV1.
func NewRoleHandlerV1(repo RoleRepositorierV1, audit AuditAppender) *RoleHandlerV1 {
	return &RoleHandlerV1{repo: repo, audit: audit}
}

// guardSystemRole returns ErrSystemRole if isSystem is true.
func guardSystemRole(isSystem bool) error {
	if isSystem {
		return domain.ErrSystemRole
	}
	return nil
}

// ListRoles handles GET /api/v1/roles
func (h *RoleHandlerV1) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.repo.ListAll(r.Context())
	if err != nil {
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := make([]roleResponse, 0, len(roles))
	for _, rc := range roles {
		resp = append(resp, roleResponse{
			ID:              rc.ID.String(),
			Code:            rc.Code,
			Name:            rc.Name,
			Scope:           rc.Scope,
			IsSystem:        rc.IsSystem,
			PermissionCount: rc.PermissionCount,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateRole handles POST /api/v1/roles
func (h *RoleHandlerV1) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Code == "" || req.Name == "" {
		writeError(w, "code and name are required", http.StatusBadRequest)
		return
	}
	if req.Scope == "" {
		req.Scope = "GLOBAL"
	}

	role := &domain.Role{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Scope:       req.Scope,
	}
	if err := h.repo.Create(r.Context(), role); err != nil {
		if errors.Is(err, domain.ErrDuplicateCode) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "code_already_exists"})
			return
		}
		slog.ErrorContext(r.Context(), "role_handler.CreateRole", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID: actorID,
		Action:      "ROLE_CREATED",
		Metadata:    map[string]any{"role_id": role.ID.String(), "code": role.Code},
		IPAddress:   clientIP(r),
		TraceID:     middleware.TraceIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": role.ID.String()})
}

// UpdateRole handles PUT /api/v1/roles/{id}
func (h *RoleHandlerV1) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid role id", http.StatusBadRequest)
		return
	}

	// Fetch role to check is_system at the handler layer (before any mutation)
	existing, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		slog.ErrorContext(r.Context(), "role_handler.UpdateRole FindByID", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := guardSystemRole(existing.IsSystem); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "system_role_protected"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		writeError(w, "name is required", http.StatusBadRequest)
		return
	}

	role := &domain.Role{ID: id, Name: req.Name, Description: req.Description}
	if err := h.repo.Update(r.Context(), role); err != nil {
		switch {
		case errors.Is(err, domain.ErrSystemRole):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "system_role_protected"})
		case errors.Is(err, domain.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		default:
			slog.ErrorContext(r.Context(), "role_handler.UpdateRole", slog.String("error", err.Error()))
			writeError(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID: actorID,
		Action:      "ROLE_UPDATED",
		Metadata:    map[string]any{"role_id": id.String()},
		IPAddress:   clientIP(r),
		TraceID:     middleware.TraceIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"id": id.String()})
}

// DeleteRole handles DELETE /api/v1/roles/{id}
func (h *RoleHandlerV1) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid role id", http.StatusBadRequest)
		return
	}

	// Fetch role to check is_system at the handler layer (before any mutation)
	existing, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		slog.ErrorContext(r.Context(), "role_handler.DeleteRole FindByID", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := guardSystemRole(existing.IsSystem); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "system_role_protected"})
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, domain.ErrSystemRole):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "system_role_protected"})
		case errors.Is(err, domain.ErrRoleHasUsers):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "role_has_users"})
		case errors.Is(err, domain.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		default:
			slog.ErrorContext(r.Context(), "role_handler.DeleteRole", slog.String("error", err.Error()))
			writeError(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID: actorID,
		Action:      "ROLE_DELETED",
		Metadata:    map[string]any{"role_id": id.String()},
		IPAddress:   clientIP(r),
		TraceID:     middleware.TraceIDFromContext(r.Context()),
	})

	w.WriteHeader(http.StatusNoContent)
}

// AssignPermissionToRole handles POST /api/v1/roles/{id}/permissions
func (h *RoleHandlerV1) AssignPermissionToRole(w http.ResponseWriter, r *http.Request) {
	roleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid role id", http.StatusBadRequest)
		return
	}

	var req struct {
		PermissionID string `json:"permission_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PermissionID == "" {
		writeError(w, "permission_id is required", http.StatusBadRequest)
		return
	}
	permID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		writeError(w, "invalid permission_id", http.StatusBadRequest)
		return
	}

	if err := h.repo.AssignPermission(r.Context(), roleID, permID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		slog.ErrorContext(r.Context(), "role_handler.AssignPermission", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID: actorID,
		Action:      "PERMISSION_ASSIGNED",
		Metadata:    map[string]any{"role_id": roleID.String(), "permission_id": permID.String()},
		IPAddress:   clientIP(r),
		TraceID:     middleware.TraceIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

// RevokePermissionFromRole handles DELETE /api/v1/roles/{id}/permissions/{permission_id}
func (h *RoleHandlerV1) RevokePermissionFromRole(w http.ResponseWriter, r *http.Request) {
	roleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid role id", http.StatusBadRequest)
		return
	}
	permID, err := uuid.Parse(chi.URLParam(r, "permission_id"))
	if err != nil {
		writeError(w, "invalid permission_id", http.StatusBadRequest)
		return
	}

	if err := h.repo.RevokePermission(r.Context(), roleID, permID); err != nil {
		slog.ErrorContext(r.Context(), "role_handler.RevokePermission", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID: actorID,
		Action:      "PERMISSION_REVOKED",
		Metadata:    map[string]any{"role_id": roleID.String(), "permission_id": permID.String()},
		IPAddress:   clientIP(r),
		TraceID:     middleware.TraceIDFromContext(r.Context()),
	})

	w.WriteHeader(http.StatusNoContent)
}

// ListPermissionsByRoleID handles GET /api/v1/roles/{id}/permissions
func (h *RoleHandlerV1) ListPermissionsByRoleID(w http.ResponseWriter, r *http.Request) {
	roleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid role id", http.StatusBadRequest)
		return
	}

	perms, err := h.repo.ListPermissionsByRoleID(r.Context(), roleID)
	if err != nil {
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := make([]permissionResponse, 0, len(perms))
	for _, p := range perms {
		resp = append(resp, permissionResponse{
			ID:          p.ID.String(),
			Code:        p.Code,
			Name:        p.Name,
			Module:      p.Module,
			Description: p.Description,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// actorFromContext extracts the actor user ID from the request context.
func actorFromContext(r *http.Request) *uuid.UUID {
	rawID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || rawID == "" {
		return nil
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return nil
	}
	return &id
}
