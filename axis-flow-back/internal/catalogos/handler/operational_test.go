package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/catalogos/domain"
	"axis-flow-back/internal/catalogos/handler"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mock repos ────────────────────────────────────────────────────────────────

type mockWorkflowStatusRepo struct {
	listFn         func(ctx context.Context) ([]domain.WorkflowStatus, error)
	listByRoleIDFn func(ctx context.Context, roleID int16) ([]domain.WorkflowStatus, error)
}

func (m *mockWorkflowStatusRepo) List(ctx context.Context) ([]domain.WorkflowStatus, error) {
	return m.listFn(ctx)
}
func (m *mockWorkflowStatusRepo) ListByRoleID(ctx context.Context, roleID int16) ([]domain.WorkflowStatus, error) {
	return m.listByRoleIDFn(ctx, roleID)
}
func (m *mockWorkflowStatusRepo) FindByID(_ context.Context, _ int64) (*domain.WorkflowStatus, error) {
	return nil, domain.ErrNotFound
}
func (m *mockWorkflowStatusRepo) Create(_ context.Context, _ *domain.WorkflowStatus) error { return nil }
func (m *mockWorkflowStatusRepo) Update(_ context.Context, _ *domain.WorkflowStatus) error { return nil }
func (m *mockWorkflowStatusRepo) Delete(_ context.Context, _ int64) error                  { return nil }

type mockComplaintTypeRepo2 struct{}

func (m *mockComplaintTypeRepo2) List(_ context.Context) ([]domain.ComplaintType, error) {
	return nil, nil
}
func (m *mockComplaintTypeRepo2) FindByID(_ context.Context, _ int64) (*domain.ComplaintType, error) {
	return nil, domain.ErrNotFound
}
func (m *mockComplaintTypeRepo2) Create(_ context.Context, _ *domain.ComplaintType) error { return nil }
func (m *mockComplaintTypeRepo2) Update(_ context.Context, _ *domain.ComplaintType) error { return nil }
func (m *mockComplaintTypeRepo2) Delete(_ context.Context, _ int64) error                 { return nil }

type mockServiceRepo2 struct{}

func (m *mockServiceRepo2) List(_ context.Context) ([]domain.Service, error)           { return nil, nil }
func (m *mockServiceRepo2) FindByID(_ context.Context, _ int64) (*domain.Service, error) {
	return nil, domain.ErrNotFound
}
func (m *mockServiceRepo2) Create(_ context.Context, _ *domain.Service) error { return nil }
func (m *mockServiceRepo2) Update(_ context.Context, _ *domain.Service) error { return nil }
func (m *mockServiceRepo2) Delete(_ context.Context, _ int64) error           { return nil }

type mockSubscriptionPlanRepo2 struct{}

func (m *mockSubscriptionPlanRepo2) List(_ context.Context) ([]domain.SubscriptionPlan, error) {
	return nil, nil
}
func (m *mockSubscriptionPlanRepo2) FindByID(_ context.Context, _ int64) (*domain.SubscriptionPlan, error) {
	return nil, domain.ErrNotFound
}
func (m *mockSubscriptionPlanRepo2) Create(_ context.Context, _ *domain.SubscriptionPlan) error {
	return nil
}
func (m *mockSubscriptionPlanRepo2) Update(_ context.Context, _ *domain.SubscriptionPlan) error {
	return nil
}
func (m *mockSubscriptionPlanRepo2) Delete(_ context.Context, _ int64) error { return nil }

type mockDatePeriodicityRepo2 struct{}

func (m *mockDatePeriodicityRepo2) List(_ context.Context) ([]domain.DatePeriodicity, error) {
	return nil, nil
}
func (m *mockDatePeriodicityRepo2) FindByID(_ context.Context, _ int64) (*domain.DatePeriodicity, error) {
	return nil, domain.ErrNotFound
}
func (m *mockDatePeriodicityRepo2) Create(_ context.Context, _ *domain.DatePeriodicity) error {
	return nil
}
func (m *mockDatePeriodicityRepo2) Update(_ context.Context, _ *domain.DatePeriodicity) error {
	return nil
}
func (m *mockDatePeriodicityRepo2) Delete(_ context.Context, _ int64) error { return nil }

// ── helper ────────────────────────────────────────────────────────────────────

func buildOpHandler(wsRepo handler.WorkflowStatusRepositorier) *handler.OperationalHandler {
	_, rdb := newTestRdb(&testing.T{})
	return handler.NewOperationalHandler(
		wsRepo,
		&mockComplaintTypeRepo2{},
		&mockServiceRepo2{},
		&mockSubscriptionPlanRepo2{},
		&mockDatePeriodicityRepo2{},
		rdb,
	)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestGetWorkflowStatuses_Returns200(t *testing.T) {
	statuses := []domain.WorkflowStatus{{ID: 1, Code: "OPEN", Name: "Open"}}
	wsRepo := &mockWorkflowStatusRepo{
		listFn:         func(_ context.Context) ([]domain.WorkflowStatus, error) { return statuses, nil },
		listByRoleIDFn: func(_ context.Context, _ int16) ([]domain.WorkflowStatus, error) { return nil, nil },
	}

	_, rdb := newTestRdb(t)
	h := handler.NewOperationalHandler(wsRepo, &mockComplaintTypeRepo2{}, &mockServiceRepo2{},
		&mockSubscriptionPlanRepo2{}, &mockDatePeriodicityRepo2{}, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()
	h.ListWorkflowStatuses(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var got []domain.WorkflowStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
}

func TestFilterStatusByRole_WithValidRole_Returns200(t *testing.T) {
	statuses := []domain.WorkflowStatus{{ID: 2, Code: "ASSIGNED", Name: "Assigned", RoleID: 3}}
	wsRepo := &mockWorkflowStatusRepo{
		listFn: func(_ context.Context) ([]domain.WorkflowStatus, error) { return nil, nil },
		listByRoleIDFn: func(_ context.Context, roleID int16) ([]domain.WorkflowStatus, error) {
			if roleID == 3 {
				return statuses, nil
			}
			return nil, nil
		},
	}

	_, rdb := newTestRdb(t)
	h := handler.NewOperationalHandler(wsRepo, &mockComplaintTypeRepo2{}, &mockServiceRepo2{},
		&mockSubscriptionPlanRepo2{}, &mockDatePeriodicityRepo2{}, rdb)

	r := chi.NewRouter()
	r.Get("/status/filterStatus/{rol}", h.FilterStatusByRole)

	req := httptest.NewRequest(http.MethodGet, "/status/filterStatus/3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var got []domain.WorkflowStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
	assert.Equal(t, "ASSIGNED", got[0].Code)
}

func TestFilterStatusByRole_WithInvalidRole_Returns400(t *testing.T) {
	wsRepo := &mockWorkflowStatusRepo{
		listFn:         func(_ context.Context) ([]domain.WorkflowStatus, error) { return nil, nil },
		listByRoleIDFn: func(_ context.Context, _ int16) ([]domain.WorkflowStatus, error) { return nil, nil },
	}

	_, rdb := newTestRdb(t)
	h := handler.NewOperationalHandler(wsRepo, &mockComplaintTypeRepo2{}, &mockServiceRepo2{},
		&mockSubscriptionPlanRepo2{}, &mockDatePeriodicityRepo2{}, rdb)

	r := chi.NewRouter()
	r.Get("/status/filterStatus/{rol}", h.FilterStatusByRole)

	req := httptest.NewRequest(http.MethodGet, "/status/filterStatus/notanumber", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
