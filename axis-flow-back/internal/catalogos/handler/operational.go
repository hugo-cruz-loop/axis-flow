package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	catalogoscache "axis-flow-back/internal/catalogos/cache"
	"axis-flow-back/internal/catalogos/domain"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// ── Consumer interfaces ───────────────────────────────────────────────────────

type WorkflowStatusRepositorier interface {
	List(ctx context.Context) ([]domain.WorkflowStatus, error)
	ListByRoleID(ctx context.Context, roleID int16) ([]domain.WorkflowStatus, error)
	FindByID(ctx context.Context, id int64) (*domain.WorkflowStatus, error)
	Create(ctx context.Context, w *domain.WorkflowStatus) error
	Update(ctx context.Context, w *domain.WorkflowStatus) error
	Delete(ctx context.Context, id int64) error
}

type ComplaintTypeRepositorier interface {
	List(ctx context.Context) ([]domain.ComplaintType, error)
	FindByID(ctx context.Context, id int64) (*domain.ComplaintType, error)
	Create(ctx context.Context, c *domain.ComplaintType) error
	Update(ctx context.Context, c *domain.ComplaintType) error
	Delete(ctx context.Context, id int64) error
}

type ServiceRepositorier interface {
	List(ctx context.Context) ([]domain.Service, error)
	FindByID(ctx context.Context, id int64) (*domain.Service, error)
	Create(ctx context.Context, s *domain.Service) error
	Update(ctx context.Context, s *domain.Service) error
	Delete(ctx context.Context, id int64) error
}

type SubscriptionPlanRepositorier interface {
	List(ctx context.Context) ([]domain.SubscriptionPlan, error)
	FindByID(ctx context.Context, id int64) (*domain.SubscriptionPlan, error)
	Create(ctx context.Context, p *domain.SubscriptionPlan) error
	Update(ctx context.Context, p *domain.SubscriptionPlan) error
	Delete(ctx context.Context, id int64) error
}

type DatePeriodicityRepositorier interface {
	List(ctx context.Context) ([]domain.DatePeriodicity, error)
	FindByID(ctx context.Context, id int64) (*domain.DatePeriodicity, error)
	Create(ctx context.Context, d *domain.DatePeriodicity) error
	Update(ctx context.Context, d *domain.DatePeriodicity) error
	Delete(ctx context.Context, id int64) error
}

// ── Handler ───────────────────────────────────────────────────────────────────

type OperationalHandler struct {
	workflowStatuses  WorkflowStatusRepositorier
	complaintTypes    ComplaintTypeRepositorier
	services          ServiceRepositorier
	subscriptionPlans SubscriptionPlanRepositorier
	datePeriodicities DatePeriodicityRepositorier
	rdb               *redis.Client
}

func NewOperationalHandler(
	workflowStatuses WorkflowStatusRepositorier,
	complaintTypes ComplaintTypeRepositorier,
	services ServiceRepositorier,
	subscriptionPlans SubscriptionPlanRepositorier,
	datePeriodicities DatePeriodicityRepositorier,
	rdb *redis.Client,
) *OperationalHandler {
	return &OperationalHandler{
		workflowStatuses:  workflowStatuses,
		complaintTypes:    complaintTypes,
		services:          services,
		subscriptionPlans: subscriptionPlans,
		datePeriodicities: datePeriodicities,
		rdb:               rdb,
	}
}

// ── Workflow Statuses ─────────────────────────────────────────────────────────

func (h *OperationalHandler) ListWorkflowStatuses(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:workflow_statuses:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.WorkflowStatus, error) { return h.workflowStatuses.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *OperationalHandler) FilterStatusByRole(w http.ResponseWriter, r *http.Request) {
	roleStr := chi.URLParam(r, "rol")
	roleID, err := strconv.ParseInt(roleStr, 10, 16)
	if err != nil {
		writeError(w, "invalid rol — must be a numeric role id", http.StatusBadRequest)
		return
	}
	key := fmt.Sprintf("catalog:workflow_statuses:by_role:%d", roleID)
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.WorkflowStatus, error) {
			return h.workflowStatuses.ListByRoleID(ctx, int16(roleID))
		})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *OperationalHandler) CreateWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	var ws domain.WorkflowStatus
	if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.workflowStatuses.Create(r.Context(), &ws); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb,
		"catalog:workflow_statuses:all",
		fmt.Sprintf("catalog:workflow_statuses:by_role:%d", ws.RoleID),
	)
	writeJSON(w, http.StatusCreated, ws)
}

func (h *OperationalHandler) UpdateWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var ws domain.WorkflowStatus
	if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	ws.ID = id
	if err := h.workflowStatuses.Update(r.Context(), &ws); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb,
		"catalog:workflow_statuses:all",
		fmt.Sprintf("catalog:workflow_statuses:by_role:%d", ws.RoleID),
	)
	writeJSON(w, http.StatusOK, ws)
}

func (h *OperationalHandler) DeleteWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.workflowStatuses.Delete(r.Context(), id); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:workflow_statuses:all")
	w.WriteHeader(http.StatusNoContent)
}

// ── Complaint Types ───────────────────────────────────────────────────────────

func (h *OperationalHandler) ListComplaintTypes(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:complaint_types:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.ComplaintType, error) { return h.complaintTypes.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *OperationalHandler) CreateComplaintType(w http.ResponseWriter, r *http.Request) {
	var c domain.ComplaintType
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.complaintTypes.Create(r.Context(), &c); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:complaint_types:all")
	writeJSON(w, http.StatusCreated, c)
}

// ── Services ──────────────────────────────────────────────────────────────────

func (h *OperationalHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:services:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.Service, error) { return h.services.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *OperationalHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	var s domain.Service
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.services.Create(r.Context(), &s); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:services:all")
	writeJSON(w, http.StatusCreated, s)
}

// ── Subscription Plans ────────────────────────────────────────────────────────

func (h *OperationalHandler) ListSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:subscription_plans:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.SubscriptionPlan, error) { return h.subscriptionPlans.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *OperationalHandler) CreateSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	var p domain.SubscriptionPlan
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.subscriptionPlans.Create(r.Context(), &p); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:subscription_plans:all")
	writeJSON(w, http.StatusCreated, p)
}

// ── Date Periodicities ────────────────────────────────────────────────────────

func (h *OperationalHandler) ListDatePeriodicities(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:date_periodicities:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.DatePeriodicity, error) { return h.datePeriodicities.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *OperationalHandler) CreateDatePeriodicity(w http.ResponseWriter, r *http.Request) {
	var d domain.DatePeriodicity
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.datePeriodicities.Create(r.Context(), &d); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:date_periodicities:all")
	writeJSON(w, http.StatusCreated, d)
}
