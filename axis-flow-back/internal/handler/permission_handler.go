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

// PermissionRepositorier is the consumer-package interface for permission persistence.
type PermissionRepositorier interface {
	ListAll(ctx context.Context, module string) ([]domain.Permission, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error)
	Create(ctx context.Context, p *domain.Permission) error
	Update(ctx context.Context, p *domain.Permission) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PermissionHandler handles /api/v1/permissions endpoints.
type PermissionHandler struct {
	repo  PermissionRepositorier
	audit AuditAppender
}

// NewPermissionHandler creates a PermissionHandler.
func NewPermissionHandler(repo PermissionRepositorier, audit AuditAppender) *PermissionHandler {
	return &PermissionHandler{repo: repo, audit: audit}
}

// ListPermissions handles GET /api/v1/permissions?module=...
func (h *PermissionHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	module := r.URL.Query().Get("module")

	perms, err := h.repo.ListAll(r.Context(), module)
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

// CreatePermission handles POST /api/v1/permissions
func (h *PermissionHandler) CreatePermission(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Module      string `json:"module"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Code == "" || req.Name == "" {
		writeError(w, "code and name are required", http.StatusBadRequest)
		return
	}

	p := &domain.Permission{
		Code:        req.Code,
		Name:        req.Name,
		Module:      req.Module,
		Description: req.Description,
	}
	if err := h.repo.Create(r.Context(), p); err != nil {
		if errors.Is(err, domain.ErrDuplicateCode) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "code_already_exists"})
			return
		}
		slog.ErrorContext(r.Context(), "permission_handler.CreatePermission", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID: actorID,
		Action:      "PERMISSION_CREATED",
		Metadata:    map[string]any{"permission_id": p.ID.String(), "code": p.Code},
		IPAddress:   clientIP(r),
		TraceID:     middleware.TraceIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": p.ID.String()})
}

// UpdatePermission handles PUT /api/v1/permissions/{id}
func (h *PermissionHandler) UpdatePermission(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid permission id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Module      string `json:"module"`
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

	p := &domain.Permission{ID: id, Name: req.Name, Module: req.Module, Description: req.Description}
	if err := h.repo.Update(r.Context(), p); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		slog.ErrorContext(r.Context(), "permission_handler.UpdatePermission", slog.String("error", err.Error()))
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID: actorID,
		Action:      "PERMISSION_UPDATED",
		Metadata:    map[string]any{"permission_id": id.String()},
		IPAddress:   clientIP(r),
		TraceID:     middleware.TraceIDFromContext(r.Context()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"id": id.String()})
}

// DeletePermission handles DELETE /api/v1/permissions/{id}
func (h *PermissionHandler) DeletePermission(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "invalid permission id", http.StatusBadRequest)
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, domain.ErrPermissionInUse):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "permission_in_use"})
		case errors.Is(err, domain.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		default:
			slog.ErrorContext(r.Context(), "permission_handler.DeletePermission", slog.String("error", err.Error()))
			writeError(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	actorID := actorFromContext(r)
	_ = h.audit.Append(r.Context(), domain.AuditEntry{
		ActorUserID: actorID,
		Action:      "PERMISSION_DELETED",
		Metadata:    map[string]any{"permission_id": id.String()},
		IPAddress:   clientIP(r),
		TraceID:     middleware.TraceIDFromContext(r.Context()),
	})

	w.WriteHeader(http.StatusNoContent)
}
