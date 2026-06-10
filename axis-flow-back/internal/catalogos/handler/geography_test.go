package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/catalogos/cache"
	"axis-flow-back/internal/catalogos/domain"
	"axis-flow-back/internal/catalogos/handler"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mock repos ────────────────────────────────────────────────────────────────

type mockCountryRepo struct {
	listFn   func(ctx context.Context) ([]domain.Country, error)
	createFn func(ctx context.Context, c *domain.Country) error
}

func (m *mockCountryRepo) List(ctx context.Context) ([]domain.Country, error) {
	return m.listFn(ctx)
}
func (m *mockCountryRepo) FindByID(_ context.Context, _ int64) (*domain.Country, error) {
	return nil, domain.ErrNotFound
}
func (m *mockCountryRepo) Create(ctx context.Context, c *domain.Country) error {
	return m.createFn(ctx, c)
}
func (m *mockCountryRepo) Update(_ context.Context, _ *domain.Country) error { return nil }
func (m *mockCountryRepo) Delete(_ context.Context, _ int64) error           { return nil }

type mockStateRepo struct{}

func (m *mockStateRepo) ListByCountryID(_ context.Context, _ int64) ([]domain.State, error) {
	return nil, nil
}
func (m *mockStateRepo) FindByID(_ context.Context, _ int64) (*domain.State, error) {
	return nil, domain.ErrNotFound
}
func (m *mockStateRepo) Create(_ context.Context, _ *domain.State) error { return nil }
func (m *mockStateRepo) Update(_ context.Context, _ *domain.State) error { return nil }
func (m *mockStateRepo) Delete(_ context.Context, _ int64) error         { return nil }

type mockCityRepo struct {
	listFn func(ctx context.Context, stateID int64) ([]domain.City, error)
}

func (m *mockCityRepo) List(_ context.Context) ([]domain.City, error) {
	return nil, nil
}
func (m *mockCityRepo) ListByStateID(ctx context.Context, stateID int64) ([]domain.City, error) {
	return m.listFn(ctx, stateID)
}
func (m *mockCityRepo) FindByID(_ context.Context, _ int64) (*domain.City, error) {
	return nil, domain.ErrNotFound
}
func (m *mockCityRepo) Create(_ context.Context, _ *domain.City) error { return nil }
func (m *mockCityRepo) Update(_ context.Context, _ *domain.City) error { return nil }
func (m *mockCityRepo) Delete(_ context.Context, _ int64) error        { return nil }

type mockLocalityTypeRepo struct{}

func (m *mockLocalityTypeRepo) List(_ context.Context) ([]domain.LocalityType, error) {
	return nil, nil
}
func (m *mockLocalityTypeRepo) Create(_ context.Context, _ *domain.LocalityType) error { return nil }

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestRdb(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func buildGeoHandler(countries handler.CountryRepositorier, cities handler.CityRepositorier, rdb *redis.Client) *handler.GeographyHandler {
	return handler.NewGeographyHandler(countries, &mockStateRepo{}, cities, &mockLocalityTypeRepo{}, rdb)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestListCountries_Returns200_WithCachedData(t *testing.T) {
	mr, rdb := newTestRdb(t)
	defer mr.Close()

	ctx := context.Background()
	const key = "catalog:countries:all"
	cached := []domain.Country{{ID: 1, Code: "MX", Name: "Mexico"}}
	require.NoError(t, cache.Set(ctx, rdb, key, cached, cache.DefaultTTL))

	fetchCalled := false
	countryRepo := &mockCountryRepo{
		listFn: func(_ context.Context) ([]domain.Country, error) {
			fetchCalled = true
			return nil, nil
		},
	}
	h := buildGeoHandler(countryRepo, &mockCityRepo{listFn: func(_ context.Context, _ int64) ([]domain.City, error) { return nil, nil }}, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pais", nil)
	w := httptest.NewRecorder()
	h.ListCountries(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, fetchCalled, "repo must not be called on cache hit")

	var got []domain.Country
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
	assert.Equal(t, "MX", got[0].Code)
}

func TestListCountries_Returns200_WithDBFallback(t *testing.T) {
	_, rdb := newTestRdb(t)

	countries := []domain.Country{{ID: 2, Code: "AR", Name: "Argentina"}}
	countryRepo := &mockCountryRepo{
		listFn: func(_ context.Context) ([]domain.Country, error) { return countries, nil },
	}
	h := buildGeoHandler(countryRepo, &mockCityRepo{listFn: func(_ context.Context, _ int64) ([]domain.City, error) { return nil, nil }}, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pais", nil)
	w := httptest.NewRecorder()
	h.ListCountries(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var got []domain.Country
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
	assert.Equal(t, "AR", got[0].Code)
}

func TestGetCitiesByState_WithInvalidStateID_Returns400(t *testing.T) {
	_, rdb := newTestRdb(t)
	h := buildGeoHandler(
		&mockCountryRepo{listFn: func(_ context.Context) ([]domain.Country, error) { return nil, nil }},
		&mockCityRepo{listFn: func(_ context.Context, _ int64) ([]domain.City, error) { return nil, nil }},
		rdb,
	)

	// Use chi router to inject URL params
	r := chi.NewRouter()
	r.Get("/ciudad/byedo/{estadoId}", h.ListCitiesByState)

	req := httptest.NewRequest(http.MethodGet, "/ciudad/byedo/notanumber", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetCitiesByState_WithNonExistentState_Returns404(t *testing.T) {
	_, rdb := newTestRdb(t)
	cityRepo := &mockCityRepo{
		listFn: func(_ context.Context, _ int64) ([]domain.City, error) {
			return nil, domain.ErrNotFound
		},
	}
	h := buildGeoHandler(
		&mockCountryRepo{listFn: func(_ context.Context) ([]domain.Country, error) { return nil, nil }},
		cityRepo,
		rdb,
	)

	r := chi.NewRouter()
	r.Get("/ciudad/byedo/{estadoId}", h.ListCitiesByState)

	req := httptest.NewRequest(http.MethodGet, "/ciudad/byedo/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateCountry_WithValidBody_Returns201(t *testing.T) {
	_, rdb := newTestRdb(t)
	countryRepo := &mockCountryRepo{
		createFn: func(_ context.Context, c *domain.Country) error {
			c.ID = 10
			return nil
		},
	}
	h := buildGeoHandler(countryRepo, &mockCityRepo{listFn: func(_ context.Context, _ int64) ([]domain.City, error) { return nil, nil }}, rdb)

	body, _ := json.Marshal(domain.Country{Code: "BR", Name: "Brazil", PhoneCode: "+55"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pais", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateCountry(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var got domain.Country
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, int64(10), got.ID)
}

func TestCreateCountry_WithDuplicateCode_Returns409(t *testing.T) {
	_, rdb := newTestRdb(t)
	countryRepo := &mockCountryRepo{
		createFn: func(_ context.Context, _ *domain.Country) error {
			return domain.ErrDuplicateCode
		},
	}
	h := buildGeoHandler(countryRepo, &mockCityRepo{listFn: func(_ context.Context, _ int64) ([]domain.City, error) { return nil, nil }}, rdb)

	body, _ := json.Marshal(domain.Country{Code: "MX", Name: "Mexico"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pais", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateCountry(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}
