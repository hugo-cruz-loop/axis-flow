package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"axis-flow-back/internal/catalogos/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory mocks — operational
// ---------------------------------------------------------------------------

type mockWorkflowStatusRepo struct {
	statuses []domain.WorkflowStatus
	nextID   int64
}

func newMockWorkflowStatusRepo() *mockWorkflowStatusRepo {
	return &mockWorkflowStatusRepo{nextID: 1}
}

func (m *mockWorkflowStatusRepo) List(_ context.Context) ([]domain.WorkflowStatus, error) {
	var out []domain.WorkflowStatus
	for _, w := range m.statuses {
		if w.DeletedAt == nil {
			out = append(out, w)
		}
	}
	return out, nil
}

func (m *mockWorkflowStatusRepo) ListByRoleID(_ context.Context, roleID int16) ([]domain.WorkflowStatus, error) {
	var out []domain.WorkflowStatus
	for _, w := range m.statuses {
		if w.RoleID == roleID && w.DeletedAt == nil {
			out = append(out, w)
		}
	}
	return out, nil
}

func (m *mockWorkflowStatusRepo) Create(_ context.Context, w *domain.WorkflowStatus) error {
	w.ID = m.nextID
	m.nextID++
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	m.statuses = append(m.statuses, *w)
	return nil
}

// ---------------------------------------------------------------------------

type mockServiceRepo struct {
	services []domain.Service
	nextID   int64
}

func newMockServiceRepo() *mockServiceRepo { return &mockServiceRepo{nextID: 1} }

func (m *mockServiceRepo) Create(_ context.Context, s *domain.Service) error {
	if s.Price < 0 {
		return fmt.Errorf("service: price must be >= 0")
	}
	s.ID = m.nextID
	m.nextID++
	m.services = append(m.services, *s)
	return nil
}

// ---------------------------------------------------------------------------

type mockSubscriptionPlanRepo struct {
	plans  []domain.SubscriptionPlan
	nextID int64
}

func newMockSubscriptionPlanRepo() *mockSubscriptionPlanRepo {
	return &mockSubscriptionPlanRepo{nextID: 1}
}

func (m *mockSubscriptionPlanRepo) List(_ context.Context) ([]domain.SubscriptionPlan, error) {
	var out []domain.SubscriptionPlan
	for _, p := range m.plans {
		if p.DeletedAt == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *mockSubscriptionPlanRepo) Create(_ context.Context, p *domain.SubscriptionPlan) error {
	p.ID = m.nextID
	m.nextID++
	m.plans = append(m.plans, *p)
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestWorkflowStatusRepo_FilterByRole_ReturnsOnlyMatchingRole(t *testing.T) {
	repo := newMockWorkflowStatusRepo()
	ctx := context.Background()

	w1 := domain.WorkflowStatus{Code: "PENDING", Name: "Pending", RoleID: 1}
	w2 := domain.WorkflowStatus{Code: "APPROVED", Name: "Approved", RoleID: 1}
	w3 := domain.WorkflowStatus{Code: "REJECTED", Name: "Rejected", RoleID: 2}

	require.NoError(t, repo.Create(ctx, &w1))
	require.NoError(t, repo.Create(ctx, &w2))
	require.NoError(t, repo.Create(ctx, &w3))

	got, err := repo.ListByRoleID(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, w := range got {
		assert.Equal(t, int16(1), w.RoleID)
	}
}

func TestServiceRepo_Create_WithNegativePrice_Fails(t *testing.T) {
	repo := newMockServiceRepo()
	ctx := context.Background()

	s := domain.Service{Code: "SVC1", Name: "Bad Service", Price: -1.0}
	err := repo.Create(ctx, &s)
	assert.Error(t, err)
}

func TestSubscriptionPlanRepo_List_ReturnsOnlyActive(t *testing.T) {
	repo := newMockSubscriptionPlanRepo()
	ctx := context.Background()

	now := time.Now()
	p1 := domain.SubscriptionPlan{Code: "BASIC", Name: "Basic", Amount: 99.0}
	p2 := domain.SubscriptionPlan{Code: "PRO", Name: "Pro", Amount: 199.0}

	require.NoError(t, repo.Create(ctx, &p1))
	require.NoError(t, repo.Create(ctx, &p2))

	// Soft-delete the second
	repo.plans[1].DeletedAt = &now

	got, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "BASIC", got[0].Code)
}
