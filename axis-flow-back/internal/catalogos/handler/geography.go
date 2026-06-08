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

// ── Consumer interfaces (defined in consumer package) ────────────────────────

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
	List(ctx context.Context) ([]domain.City, error)
	ListByStateID(ctx context.Context, stateID int64) ([]domain.City, error)
	FindByID(ctx context.Context, id int64) (*domain.City, error)
	Create(ctx context.Context, c *domain.City) error
	Update(ctx context.Context, c *domain.City) error
	Delete(ctx context.Context, id int64) error
}

type LocalityTypeRepositorier interface {
	List(ctx context.Context) ([]domain.LocalityType, error)
	Create(ctx context.Context, lt *domain.LocalityType) error
}

// ── Handler ──────────────────────────────────────────────────────────────────

type GeographyHandler struct {
	countries     CountryRepositorier
	states        StateRepositorier
	cities        CityRepositorier
	localityTypes LocalityTypeRepositorier
	rdb           *redis.Client
}

func NewGeographyHandler(
	countries CountryRepositorier,
	states StateRepositorier,
	cities CityRepositorier,
	localityTypes LocalityTypeRepositorier,
	rdb *redis.Client,
) *GeographyHandler {
	return &GeographyHandler{
		countries:     countries,
		states:        states,
		cities:        cities,
		localityTypes: localityTypes,
		rdb:           rdb,
	}
}

// ── Countries ─────────────────────────────────────────────────────────────────

func (h *GeographyHandler) ListCountries(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:countries:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.Country, error) {
			return h.countries.List(ctx)
		})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *GeographyHandler) CreateCountry(w http.ResponseWriter, r *http.Request) {
	var c domain.Country
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.countries.Create(r.Context(), &c); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:countries:all")
	writeJSON(w, http.StatusCreated, c)
}

func (h *GeographyHandler) UpdateCountry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var c domain.Country
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	c.ID = id
	if err := h.countries.Update(r.Context(), &c); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:countries:all")
	writeJSON(w, http.StatusOK, c)
}

func (h *GeographyHandler) DeleteCountry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.countries.Delete(r.Context(), id); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:countries:all")
	w.WriteHeader(http.StatusNoContent)
}

// ── States ────────────────────────────────────────────────────────────────────

func (h *GeographyHandler) ListStates(w http.ResponseWriter, r *http.Request) {
	countryIDStr := r.URL.Query().Get("country_id")
	if countryIDStr == "" {
		writeError(w, "country_id query param is required", http.StatusBadRequest)
		return
	}
	countryID, err := strconv.ParseInt(countryIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid country_id", http.StatusBadRequest)
		return
	}
	key := fmt.Sprintf("catalog:states:by_country:%d", countryID)
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.State, error) {
			return h.states.ListByCountryID(ctx, countryID)
		})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *GeographyHandler) CreateState(w http.ResponseWriter, r *http.Request) {
	var s domain.State
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.states.Create(r.Context(), &s); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb,
		"catalog:states:all",
		fmt.Sprintf("catalog:states:by_country:%d", s.CountryID),
	)
	writeJSON(w, http.StatusCreated, s)
}

func (h *GeographyHandler) UpdateState(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var s domain.State
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	s.ID = id
	if err := h.states.Update(r.Context(), &s); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb,
		"catalog:states:all",
		fmt.Sprintf("catalog:states:by_country:%d", s.CountryID),
	)
	writeJSON(w, http.StatusOK, s)
}

func (h *GeographyHandler) DeleteState(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.states.Delete(r.Context(), id); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:states:all")
	w.WriteHeader(http.StatusNoContent)
}

// ── Cities ────────────────────────────────────────────────────────────────────

func (h *GeographyHandler) ListCities(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:cities:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.City, error) {
			return h.cities.List(ctx)
		})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *GeographyHandler) ListCitiesByState(w http.ResponseWriter, r *http.Request) {
	stateID, err := strconv.ParseInt(chi.URLParam(r, "estadoId"), 10, 64)
	if err != nil {
		writeError(w, "invalid estadoId", http.StatusBadRequest)
		return
	}
	key := fmt.Sprintf("catalog:cities:by_state:%d", stateID)
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.City, error) {
			return h.cities.ListByStateID(ctx, stateID)
		})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *GeographyHandler) CreateCity(w http.ResponseWriter, r *http.Request) {
	var c domain.City
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.cities.Create(r.Context(), &c); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, fmt.Sprintf("catalog:cities:by_state:%d", c.StateID))
	writeJSON(w, http.StatusCreated, c)
}

func (h *GeographyHandler) UpdateCity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var c domain.City
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	c.ID = id
	if err := h.cities.Update(r.Context(), &c); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, fmt.Sprintf("catalog:cities:by_state:%d", c.StateID))
	writeJSON(w, http.StatusOK, c)
}

func (h *GeographyHandler) DeleteCity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.cities.Delete(r.Context(), id); err != nil {
		writeCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Locality Types ────────────────────────────────────────────────────────────

func (h *GeographyHandler) ListLocalityTypes(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:locality_types:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.LocalityType, error) {
			return h.localityTypes.List(ctx)
		})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *GeographyHandler) CreateLocalityType(w http.ResponseWriter, r *http.Request) {
	var lt domain.LocalityType
	if err := json.NewDecoder(r.Body).Decode(&lt); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.localityTypes.Create(r.Context(), &lt); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:locality_types:all")
	writeJSON(w, http.StatusCreated, lt)
}
