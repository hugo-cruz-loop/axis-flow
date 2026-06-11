package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"axis-flow-back/internal/notificaciones"
	"axis-flow-back/internal/notificaciones/events"
	"axis-flow-back/internal/notificaciones/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// NotificationHandler handles HTTP requests for the Notificaciones module.
type NotificationHandler struct {
	svc    service.NotificationService
	email  service.EmailService
	events events.EventPublisher
}

// NewNotificationHandler constructs a NotificationHandler.
func NewNotificationHandler(
	svc service.NotificationService,
	email service.EmailService,
	pub events.EventPublisher,
) *NotificationHandler {
	return &NotificationHandler{svc: svc, email: email, events: pub}
}

// ---------------------------------------------------------------------------
// POST /api/v1/notificaciones/recuperar-password (public)
// ---------------------------------------------------------------------------

type recuperarPasswordRequest struct {
	Email string `json:"email"`
}

// RecuperarPassword triggers a password-recovery email. No authentication required.
func (h *NotificationHandler) RecuperarPassword(w http.ResponseWriter, r *http.Request) {
	var req recuperarPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "email is required")
		return
	}

	if err := h.email.TriggerRecovery(r.Context(), req.Email); err != nil {
		status, code := mapNotifError(err)
		respondError(w, status, code, "failed to queue recovery email")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":  "queued",
		"message": "Email de recuperación encolado",
	})
}

// ---------------------------------------------------------------------------
// POST /api/v1/notificaciones/bienvenida (Admin JWT required)
// ---------------------------------------------------------------------------

type bienvenidaRequest struct {
	UserID uuid.UUID `json:"userId"`
	Email  string    `json:"email"`
	Name   string    `json:"name"`
}

// Bienvenida triggers a welcome email. Requires ADMIN role.
func (h *NotificationHandler) Bienvenida(w http.ResponseWriter, r *http.Request) {
	role := extractRole(r)
	if role != "ADMIN" && role != "ADMINISTRADOR" && role != "AdminCheckOn" && role != "ADMIN_CHECK_ON" {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "admin role required")
		return
	}

	var req bienvenidaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}
	if req.UserID == uuid.Nil || strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Name) == "" {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "userId, email, and name are required")
		return
	}

	if err := h.email.TriggerWelcome(r.Context(), req.UserID, req.Email, req.Name); err != nil {
		status, code := mapNotifError(err)
		respondError(w, status, code, "failed to queue welcome email")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":  "queued",
		"message": "Email de bienvenida encolado",
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/notificaciones/usuario/{userId} (JWT owner or admin)
// ---------------------------------------------------------------------------

// ListPushHistory returns paginated push notification records for a user.
func (h *NotificationHandler) ListPushHistory(w http.ResponseWriter, r *http.Request) {
	callerID, err := extractUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid token")
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid userId")
		return
	}

	// IDOR: caller must be the owner or admin
	role := extractRole(r)
	if callerID != targetID && role != "ADMIN" && role != "ADMINISTRADOR" && role != "AdminCheckOn" && role != "ADMIN_CHECK_ON" {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "access denied")
		return
	}

	page := queryInt(r, "page", 1)
	pageSize := queryInt(r, "pageSize", 20)

	records, total, err := h.svc.ListPushByUser(r.Context(), targetID, page, pageSize)
	if err != nil {
		status, code := mapNotifError(err)
		respondError(w, status, code, err.Error())
		return
	}

	respondPaginated(w, http.StatusOK, records, page, pageSize, total)
}

// ---------------------------------------------------------------------------
// PATCH /api/v1/notificaciones/{id}/estatus (JWT)
// ---------------------------------------------------------------------------

type updateEstatusRequest struct {
	Estatus int `json:"estatus"`
}

// UpdateEstatus marks a notification as read or unread.
func (h *NotificationHandler) UpdateEstatus(w http.ResponseWriter, r *http.Request) {
	callerID, err := extractUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid token")
		return
	}

	notifID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid notification id")
		return
	}

	var req updateEstatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}

	// Validate estatus in (1,2)
	if req.Estatus != int(notificaciones.EstatusUnread) && req.Estatus != int(notificaciones.EstatusRead) {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "estatus must be 1 (unread) or 2 (read)")
		return
	}

	updated, err := h.svc.MarkAsRead(r.Context(), callerID, notifID)
	if err != nil {
		status, code := mapNotifError(err)
		respondError(w, status, code, err.Error())
		return
	}

	// Publish event (best-effort, non-blocking)
	if payload, encErr := json.Marshal(map[string]any{
		"notif_id": updated.ID,
		"user_id":  updated.UserID,
		"estatus":  updated.Estatus,
	}); encErr == nil {
		_ = h.events.Publish(r.Context(), events.StreamNotificacionLeida, payload)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"id":        updated.ID,
		"estatus":   updated.Estatus,
		"updatedAt": updated.UpdatedAt,
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/notificaciones/usuario/{userId}/no-leidas/count (JWT)
// ---------------------------------------------------------------------------

// CountUnread returns the number of unread notifications for a user.
func (h *NotificationHandler) CountUnread(w http.ResponseWriter, r *http.Request) {
	callerID, err := extractUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid token")
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid userId")
		return
	}

	// IDOR: caller must be the owner
	if callerID != targetID {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "access denied")
		return
	}

	count, err := h.svc.CountUnread(r.Context(), targetID)
	if err != nil {
		status, code := mapNotifError(err)
		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"userId":      targetID,
		"unreadCount": count,
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func queryInt(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}
