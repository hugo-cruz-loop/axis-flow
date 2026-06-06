package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"axis-flow-back/internal/catalogos/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory mocks — HR
// ---------------------------------------------------------------------------

type mockJobCategoryRepo struct {
	categories []domain.JobCategory
	nextID     int64
}

func newMockJobCategoryRepo() *mockJobCategoryRepo { return &mockJobCategoryRepo{nextID: 1} }

func (m *mockJobCategoryRepo) List(_ context.Context) ([]domain.JobCategory, error) {
	var out []domain.JobCategory
	for _, jc := range m.categories {
		if jc.DeletedAt == nil {
			out = append(out, jc)
		}
	}
	return out, nil
}

func (m *mockJobCategoryRepo) Create(_ context.Context, jc *domain.JobCategory) error {
	jc.Name = strings.ToLower(jc.Name)
	for _, existing := range m.categories {
		if existing.Name == jc.Name && existing.DeletedAt == nil {
			return domain.ErrDuplicateCode
		}
	}
	jc.ID = m.nextID
	m.nextID++
	jc.CreatedAt = time.Now()
	jc.UpdatedAt = time.Now()
	m.categories = append(m.categories, *jc)
	return nil
}

// ---------------------------------------------------------------------------

type mockJobTypeRepo struct {
	types  []domain.JobType
	nextID int64
}

func newMockJobTypeRepo() *mockJobTypeRepo { return &mockJobTypeRepo{nextID: 1} }

func (m *mockJobTypeRepo) Create(_ context.Context, jt *domain.JobType) error {
	jt.Name = strings.ToLower(jt.Name)
	jt.ID = m.nextID
	m.nextID++
	m.types = append(m.types, *jt)
	return nil
}

// ---------------------------------------------------------------------------

type mockHrAbsenceTypeRepo struct {
	types  []domain.HrAbsenceType
	nextID int64
}

func newMockHrAbsenceTypeRepo() *mockHrAbsenceTypeRepo { return &mockHrAbsenceTypeRepo{nextID: 1} }

func (m *mockHrAbsenceTypeRepo) List(_ context.Context) ([]domain.HrAbsenceType, error) {
	var out []domain.HrAbsenceType
	for _, h := range m.types {
		if h.DeletedAt == nil {
			out = append(out, h)
		}
	}
	return out, nil
}

func (m *mockHrAbsenceTypeRepo) Create(_ context.Context, h *domain.HrAbsenceType) error {
	h.ID = m.nextID
	m.nextID++
	h.CreatedAt = time.Now()
	h.UpdatedAt = time.Now()
	m.types = append(m.types, *h)
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestJobCategoryRepo_Create_ForcesLowercase(t *testing.T) {
	repo := newMockJobCategoryRepo()
	ctx := context.Background()

	jc := domain.JobCategory{Code: "DEV", Name: "Software Developer"}
	require.NoError(t, repo.Create(ctx, &jc))

	assert.Equal(t, "software developer", jc.Name)
}

func TestJobCategoryRepo_Create_WithDuplicateName_ReturnsErrDuplicateCode(t *testing.T) {
	repo := newMockJobCategoryRepo()
	ctx := context.Background()

	jc1 := domain.JobCategory{Code: "DEV1", Name: "developer"}
	require.NoError(t, repo.Create(ctx, &jc1))

	jc2 := domain.JobCategory{Code: "DEV2", Name: "Developer"} // same name, different case
	err := repo.Create(ctx, &jc2)
	assert.ErrorIs(t, err, domain.ErrDuplicateCode)
}

func TestJobTypeRepo_Create_ForcesLowercase(t *testing.T) {
	repo := newMockJobTypeRepo()
	ctx := context.Background()

	jt := domain.JobType{Code: "FT", Name: "Full Time"}
	require.NoError(t, repo.Create(ctx, &jt))

	assert.Equal(t, "full time", jt.Name)
}

func TestHrAbsenceTypeRepo_List_ReturnsOnlyNonDeleted(t *testing.T) {
	repo := newMockHrAbsenceTypeRepo()
	ctx := context.Background()

	now := time.Now()
	h1 := domain.HrAbsenceType{Code: "VAC", Name: "Vacation", RequiresJustification: false}
	h2 := domain.HrAbsenceType{Code: "SICK", Name: "Sick Leave", RequiresJustification: true}

	require.NoError(t, repo.Create(ctx, &h1))
	require.NoError(t, repo.Create(ctx, &h2))

	// Soft-delete the second
	repo.types[1].DeletedAt = &now

	got, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "VAC", got[0].Code)
}
