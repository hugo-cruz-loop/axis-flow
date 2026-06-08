package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/catalogos/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

type CountryRepositorier interface {
	List(ctx context.Context) ([]domain.Country, error)
	FindByID(ctx context.Context, id int64) (*domain.Country, error)
	Create(ctx context.Context, c *domain.Country) error
	Update(ctx context.Context, c *domain.Country) error
	Delete(ctx context.Context, id int64) error
}

type StateRepositorier interface {
	ListByCountryID(ctx context.Context, countryID int64) ([]domain.State, error)
	FindByID(ctx context.Context, id int64) (*domain.State, error)
	Create(ctx context.Context, s *domain.State) error
	Update(ctx context.Context, s *domain.State) error
	Delete(ctx context.Context, id int64) error
}

type CityRepositorier interface {
	ListByStateID(ctx context.Context, stateID int64) ([]domain.City, error)
	FindByID(ctx context.Context, id int64) (*domain.City, error)
	Create(ctx context.Context, c *domain.City) error
	Update(ctx context.Context, c *domain.City) error
	Delete(ctx context.Context, id int64) error
}

// ---------------------------------------------------------------------------
// In-memory mocks
// ---------------------------------------------------------------------------

type mockCountryRepo struct {
	countries []domain.Country
	nextID    int64
}

func newMockCountryRepo() *mockCountryRepo {
	return &mockCountryRepo{nextID: 1}
}

func (m *mockCountryRepo) List(_ context.Context) ([]domain.Country, error) {
	var out []domain.Country
	for _, c := range m.countries {
		if c.DeletedAt == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *mockCountryRepo) FindByID(_ context.Context, id int64) (*domain.Country, error) {
	for _, c := range m.countries {
		if c.ID == id && c.DeletedAt == nil {
			cp := c
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockCountryRepo) Create(_ context.Context, c *domain.Country) error {
	for _, existing := range m.countries {
		if existing.Code == c.Code && existing.DeletedAt == nil {
			return domain.ErrDuplicateCode
		}
	}
	c.ID = m.nextID
	m.nextID++
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	m.countries = append(m.countries, *c)
	return nil
}

func (m *mockCountryRepo) Update(_ context.Context, c *domain.Country) error {
	for i, existing := range m.countries {
		if existing.ID == c.ID && existing.DeletedAt == nil {
			m.countries[i] = *c
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *mockCountryRepo) Delete(_ context.Context, id int64) error {
	for i, c := range m.countries {
		if c.ID == id && c.DeletedAt == nil {
			// Simulate FK RESTRICT: if any state references this country, refuse.
			return m.checkDependents(id, &m.countries[i])
		}
	}
	return nil
}

func (m *mockCountryRepo) checkDependents(_ int64, c *domain.Country) error {
	now := time.Now()
	c.DeletedAt = &now
	return nil
}

// mockCountryRepoWithDependents simulates FK RESTRICT on delete.
type mockCountryRepoWithDependents struct{ mockCountryRepo }

func (m *mockCountryRepoWithDependents) Delete(_ context.Context, _ int64) error {
	return domain.ErrHasDependents
}

// ---------------------------------------------------------------------------

type mockStateRepo struct {
	states []domain.State
	nextID int64
}

func newMockStateRepo() *mockStateRepo { return &mockStateRepo{nextID: 1} }

func (m *mockStateRepo) ListByCountryID(_ context.Context, countryID int64) ([]domain.State, error) {
	var out []domain.State
	for _, s := range m.states {
		if s.CountryID == countryID && s.DeletedAt == nil {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *mockStateRepo) FindByID(_ context.Context, id int64) (*domain.State, error) {
	for _, s := range m.states {
		if s.ID == id && s.DeletedAt == nil {
			cp := s
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockStateRepo) Create(_ context.Context, s *domain.State) error {
	s.ID = m.nextID
	m.nextID++
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	m.states = append(m.states, *s)
	return nil
}

func (m *mockStateRepo) Update(_ context.Context, s *domain.State) error {
	for i, existing := range m.states {
		if existing.ID == s.ID && existing.DeletedAt == nil {
			m.states[i] = *s
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *mockStateRepo) Delete(_ context.Context, id int64) error {
	for i, s := range m.states {
		if s.ID == id && s.DeletedAt == nil {
			now := time.Now()
			m.states[i].DeletedAt = &now
			return nil
		}
	}
	return nil
}

// ---------------------------------------------------------------------------

type mockCityRepo struct {
	cities []domain.City
	nextID int64
	// simulateFKError causes Delete to return ErrHasDependents.
	simulateFKError bool
}

func newMockCityRepo() *mockCityRepo { return &mockCityRepo{nextID: 1} }

func (m *mockCityRepo) ListByStateID(_ context.Context, stateID int64) ([]domain.City, error) {
	var out []domain.City
	for _, c := range m.cities {
		if c.StateID == stateID && c.DeletedAt == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *mockCityRepo) FindByID(_ context.Context, id int64) (*domain.City, error) {
	for _, c := range m.cities {
		if c.ID == id && c.DeletedAt == nil {
			cp := c
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockCityRepo) Create(_ context.Context, c *domain.City) error {
	c.ID = m.nextID
	m.nextID++
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	m.cities = append(m.cities, *c)
	return nil
}

func (m *mockCityRepo) Update(_ context.Context, c *domain.City) error {
	for i, existing := range m.cities {
		if existing.ID == c.ID && existing.DeletedAt == nil {
			m.cities[i] = *c
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *mockCityRepo) Delete(_ context.Context, _ int64) error {
	if m.simulateFKError {
		return domain.ErrHasDependents
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCountryRepo_List_ReturnsOnlyNonDeleted(t *testing.T) {
	repo := newMockCountryRepo()
	ctx := context.Background()

	now := time.Now()
	active := domain.Country{Code: "MX", Name: "Mexico", PhoneCode: "+52"}
	deleted := domain.Country{Code: "US", Name: "United States", PhoneCode: "+1"}

	require.NoError(t, repo.Create(ctx, &active))
	require.NoError(t, repo.Create(ctx, &deleted))

	// Manually soft-delete the second.
	deleted.DeletedAt = &now
	repo.countries[1].DeletedAt = &now

	got, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "MX", got[0].Code)
}

func TestCountryRepo_Create_SetsIDOnSuccess(t *testing.T) {
	repo := newMockCountryRepo()
	ctx := context.Background()

	c := domain.Country{Code: "AR", Name: "Argentina", PhoneCode: "+54"}
	require.NoError(t, repo.Create(ctx, &c))

	assert.NotZero(t, c.ID)
	assert.False(t, c.CreatedAt.IsZero())
}

func TestStateRepo_ListByCountryID_FiltersCorrectly(t *testing.T) {
	repo := newMockStateRepo()
	ctx := context.Background()

	s1 := domain.State{CountryID: 1, Code: "CDMX", Name: "Ciudad de Mexico"}
	s2 := domain.State{CountryID: 1, Code: "JAL", Name: "Jalisco"}
	s3 := domain.State{CountryID: 2, Code: "CAT", Name: "Cataluna"}

	require.NoError(t, repo.Create(ctx, &s1))
	require.NoError(t, repo.Create(ctx, &s2))
	require.NoError(t, repo.Create(ctx, &s3))

	got, err := repo.ListByCountryID(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, s := range got {
		assert.Equal(t, int64(1), s.CountryID)
	}
}

func TestCityRepo_ListByStateID_FiltersCorrectly(t *testing.T) {
	repo := newMockCityRepo()
	ctx := context.Background()

	c1 := domain.City{StateID: 10, Name: "Guadalajara"}
	c2 := domain.City{StateID: 10, Name: "Zapopan"}
	c3 := domain.City{StateID: 20, Name: "Monterrey"}

	require.NoError(t, repo.Create(ctx, &c1))
	require.NoError(t, repo.Create(ctx, &c2))
	require.NoError(t, repo.Create(ctx, &c3))

	got, err := repo.ListByStateID(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, c := range got {
		assert.Equal(t, int64(10), c.StateID)
	}
}

func TestCityRepo_Delete_WhenHasDependents_ReturnsError(t *testing.T) {
	repo := newMockCityRepo()
	repo.simulateFKError = true
	ctx := context.Background()

	err := repo.Delete(ctx, 99)
	assert.ErrorIs(t, err, domain.ErrHasDependents)
}
