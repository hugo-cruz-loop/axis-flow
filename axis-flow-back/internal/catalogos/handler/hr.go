package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	catalogoscache "axis-flow-back/internal/catalogos/cache"
	"axis-flow-back/internal/catalogos/domain"

	"github.com/redis/go-redis/v9"
)

// ── Consumer interfaces ───────────────────────────────────────────────────────

type JobCategoryRepositorier interface {
	List(ctx context.Context) ([]domain.JobCategory, error)
	FindByID(ctx context.Context, id int64) (*domain.JobCategory, error)
	Create(ctx context.Context, jc *domain.JobCategory) error
	Update(ctx context.Context, jc *domain.JobCategory) error
	Delete(ctx context.Context, id int64) error
}

type JobTypeRepositorier interface {
	List(ctx context.Context) ([]domain.JobType, error)
	FindByID(ctx context.Context, id int64) (*domain.JobType, error)
	Create(ctx context.Context, jt *domain.JobType) error
	Update(ctx context.Context, jt *domain.JobType) error
	Delete(ctx context.Context, id int64) error
}

type HrAbsenceTypeRepositorier interface {
	List(ctx context.Context) ([]domain.HrAbsenceType, error)
	FindByID(ctx context.Context, id int64) (*domain.HrAbsenceType, error)
	Create(ctx context.Context, h *domain.HrAbsenceType) error
	Update(ctx context.Context, h *domain.HrAbsenceType) error
	Delete(ctx context.Context, id int64) error
}

// ── Handler ───────────────────────────────────────────────────────────────────

type HRHandler struct {
	jobCategories  JobCategoryRepositorier
	jobTypes       JobTypeRepositorier
	hrAbsenceTypes HrAbsenceTypeRepositorier
	rdb            *redis.Client
}

func NewHRHandler(
	jobCategories JobCategoryRepositorier,
	jobTypes JobTypeRepositorier,
	hrAbsenceTypes HrAbsenceTypeRepositorier,
	rdb *redis.Client,
) *HRHandler {
	return &HRHandler{
		jobCategories:  jobCategories,
		jobTypes:       jobTypes,
		hrAbsenceTypes: hrAbsenceTypes,
		rdb:            rdb,
	}
}

// ── Job Categories ────────────────────────────────────────────────────────────

func (h *HRHandler) ListJobCategories(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:job_categories:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.JobCategory, error) { return h.jobCategories.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *HRHandler) CreateJobCategory(w http.ResponseWriter, r *http.Request) {
	var jc domain.JobCategory
	if err := json.NewDecoder(r.Body).Decode(&jc); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	jc.Name = strings.ToLower(jc.Name)
	if err := h.jobCategories.Create(r.Context(), &jc); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:job_categories:all")
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     jc.ID,
		"nombre": jc.Name,
	})
}

// ── Job Types ─────────────────────────────────────────────────────────────────

func (h *HRHandler) ListJobTypes(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:job_types:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.JobType, error) { return h.jobTypes.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *HRHandler) CreateJobType(w http.ResponseWriter, r *http.Request) {
	var jt domain.JobType
	if err := json.NewDecoder(r.Body).Decode(&jt); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	jt.Name = strings.ToLower(jt.Name)
	if err := h.jobTypes.Create(r.Context(), &jt); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:job_types:all")
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     jt.ID,
		"nombre": jt.Name,
	})
}

// ── HR Absence Types ──────────────────────────────────────────────────────────

func (h *HRHandler) ListHrAbsenceTypes(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:hr_absence_types:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.HrAbsenceType, error) { return h.hrAbsenceTypes.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *HRHandler) CreateHrAbsenceType(w http.ResponseWriter, r *http.Request) {
	var hat domain.HrAbsenceType
	if err := json.NewDecoder(r.Body).Decode(&hat); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.hrAbsenceTypes.Create(r.Context(), &hat); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:hr_absence_types:all")
	writeJSON(w, http.StatusCreated, hat)
}
