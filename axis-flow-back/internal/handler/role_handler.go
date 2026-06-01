package handler

import (
	"context"
	"net/http"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/repository"

	"github.com/go-chi/chi/v5"
)

// RoleRepositorier is the interface the RoleHandler depends on.
type RoleRepositorier interface {
	ListAll(ctx context.Context) ([]repository.RoleWithCount, error)
	ListPermissionsByRole(ctx context.Context, roleCode string) ([]domain.Permission, error)
}

// RoleHandler handles HTTP endpoints for role management.
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
