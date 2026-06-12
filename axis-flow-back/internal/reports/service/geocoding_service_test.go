package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/service"
)

// ---------------------------------------------------------------------------
// Mock GeocodingCache
// ---------------------------------------------------------------------------

type mockGeoCache struct {
	mu      sync.RWMutex
	data    map[string]string
	setCalls int
}

func newMockGeoCache() *mockGeoCache {
	return &mockGeoCache{data: make(map[string]string)}
}

func cacheKey(lat, lng float64) string {
	return service.CacheKeyForTest(lat, lng)
}

func (m *mockGeoCache) Get(_ context.Context, lat, lng float64) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[cacheKey(lat, lng)]
	return v, ok
}

func (m *mockGeoCache) Set(_ context.Context, lat, lng float64, direccion string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[cacheKey(lat, lng)] = direccion
	m.setCalls++
	return nil
}

func (m *mockGeoCache) DeleteByPattern(_ context.Context, _ string) error { return nil }

// ---------------------------------------------------------------------------
// Mock GeocodingRepo
// ---------------------------------------------------------------------------

type mockGeoRepo struct {
	mu        sync.RWMutex
	data      map[string]*reports.DireccionCache
	storeCalls int
}

func newMockGeoRepo() *mockGeoRepo {
	return &mockGeoRepo{data: make(map[string]*reports.DireccionCache)}
}

func repoKey(lat, lng float64) string {
	return service.CacheKeyForTest(lat, lng)
}

func (m *mockGeoRepo) GetByCoords(_ context.Context, lat, lng float64) (*reports.DireccionCache, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.data[repoKey(lat, lng)]
	if !ok {
		return nil, reports.ErrNotFound
	}
	cp := *d
	return &cp, nil
}

func (m *mockGeoRepo) Store(_ context.Context, d reports.DireccionCache) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := d
	m.data[repoKey(d.Latitud, d.Longitud)] = &cp
	m.storeCalls++
	return nil
}

func (m *mockGeoRepo) DeleteByTenant(_ context.Context, _ string) error { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// gcpResponse builds the JSON body that the real GCP Geocoding API returns.
func gcpResponse(formatted string) interface{} {
	return map[string]interface{}{
		"status": "OK",
		"results": []map[string]interface{}{
			{"formatted_address": formatted},
		},
	}
}

func emptyGCPResponse() interface{} {
	return map[string]interface{}{
		"status":  "ZERO_RESULTS",
		"results": []interface{}{},
	}
}

func newGCPServer(t *testing.T, body interface{}, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// Test 1: Redis hit — returns Cached: true, no GCP call needed.
func TestGeocodingService_RedisHit(t *testing.T) {
	cache := newMockGeoCache()
	repo := newMockGeoRepo()

	// Pre-populate Redis cache.
	_ = cache.Set(context.Background(), 19.432600, -99.133200, "Calle Falsa 123", time.Hour)
	prevSetCalls := cache.setCalls

	cfg := reports.Config{GeocodingCacheTTLDays: 30, GeocodingRateLimitRPM: 60}
	svc := service.NewGeocodingService(cache, repo, cfg)

	result, err := svc.Reverse(context.Background(), 19.432600, -99.133200)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Cached {
		t.Error("expected Cached=true on Redis hit")
	}
	if result.Direccion != "Calle Falsa 123" {
		t.Errorf("unexpected Direccion: %q", result.Direccion)
	}
	// No additional Set calls should have happened.
	if cache.setCalls != prevSetCalls {
		t.Errorf("expected no extra cache Set calls, got %d new", cache.setCalls-prevSetCalls)
	}
}

// Test 2: Redis miss, PG hit — returns Cached: true, backfills Redis.
func TestGeocodingService_PGHit_BackfillsRedis(t *testing.T) {
	cache := newMockGeoCache()
	repo := newMockGeoRepo()

	// Pre-populate PG repo only.
	_ = repo.Store(context.Background(), reports.DireccionCache{
		Latitud:   19.432600,
		Longitud:  -99.133200,
		Direccion: "Av. Insurgentes 123",
	})
	prevSetCalls := cache.setCalls

	cfg := reports.Config{GeocodingCacheTTLDays: 30, GeocodingRateLimitRPM: 60}
	svc := service.NewGeocodingService(cache, repo, cfg)

	result, err := svc.Reverse(context.Background(), 19.432600, -99.133200)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Cached {
		t.Error("expected Cached=true on PG hit")
	}
	if result.Direccion != "Av. Insurgentes 123" {
		t.Errorf("unexpected Direccion: %q", result.Direccion)
	}
	// Should have backfilled Redis.
	if cache.setCalls <= prevSetCalls {
		t.Error("expected Redis backfill (Set call) after PG hit")
	}
	// Confirm the value is now in Redis.
	v, ok := cache.Get(context.Background(), 19.432600, -99.133200)
	if !ok || v != "Av. Insurgentes 123" {
		t.Errorf("Redis not backfilled correctly, got %q, ok=%v", v, ok)
	}
}

// Test 3: Redis miss, PG miss — calls GCP, stores in both, returns Cached: false.
func TestGeocodingService_GCPFallback_StoresBothCaches(t *testing.T) {
	srv := newGCPServer(t, gcpResponse("Plaza Mayor 1"), http.StatusOK)
	defer srv.Close()

	cache := newMockGeoCache()
	repo := newMockGeoRepo()

	cfg := reports.Config{
		GCPGeocodingAPIKey:    "test-api-key",
		GeocodingCacheTTLDays: 30,
		GeocodingRateLimitRPM: 60,
	}
	svc := service.NewGeocodingServiceWithBaseURL(cache, repo, cfg, srv.URL)

	result, err := svc.Reverse(context.Background(), 19.432600, -99.133200)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Cached {
		t.Error("expected Cached=false on GCP fallback")
	}
	if result.Direccion != "Plaza Mayor 1" {
		t.Errorf("unexpected Direccion: %q", result.Direccion)
	}
	// Should have stored in PG.
	if repo.storeCalls != 1 {
		t.Errorf("expected 1 repo Store call, got %d", repo.storeCalls)
	}
	// Should have stored in Redis.
	v, ok := cache.Get(context.Background(), 19.432600, -99.133200)
	if !ok || v != "Plaza Mayor 1" {
		t.Errorf("Redis not populated after GCP call, got %q, ok=%v", v, ok)
	}
}

// Test 4: Rate limit exceeded — returns ErrRateLimitExceeded before GCP call.
func TestGeocodingService_RateLimitExceeded(t *testing.T) {
	gcpCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gcpCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cache := newMockGeoCache()
	repo := newMockGeoRepo()

	// RPM = 1, so immediately exhausted after first call that hits GCP.
	// We use RPM = 0 to pre-exhaust without needing a real GCP call.
	cfg := reports.Config{
		GCPGeocodingAPIKey:    "test-api-key",
		GeocodingCacheTTLDays: 30,
		GeocodingRateLimitRPM: 0, // zero means no GCP calls allowed
	}
	svc := service.NewGeocodingServiceWithBaseURL(cache, repo, cfg, srv.URL)

	_, err := svc.Reverse(context.Background(), 19.432600, -99.133200)
	if !errors.Is(err, reports.ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded, got %v", err)
	}
	if gcpCalled {
		t.Error("GCP must not be called when rate limit is exceeded")
	}
}

// Test 5: GCP returns empty results — returns ErrGeocodingUnavailable.
func TestGeocodingService_GCPEmptyResults(t *testing.T) {
	srv := newGCPServer(t, emptyGCPResponse(), http.StatusOK)
	defer srv.Close()

	cache := newMockGeoCache()
	repo := newMockGeoRepo()

	cfg := reports.Config{
		GCPGeocodingAPIKey:    "test-api-key",
		GeocodingCacheTTLDays: 30,
		GeocodingRateLimitRPM: 60,
	}
	svc := service.NewGeocodingServiceWithBaseURL(cache, repo, cfg, srv.URL)

	_, err := svc.Reverse(context.Background(), 19.432600, -99.133200)
	if !errors.Is(err, reports.ErrGeocodingUnavailable) {
		t.Errorf("expected ErrGeocodingUnavailable, got %v", err)
	}
}

// Test 6: Coordinate normalization — lat=19.4326001 normalizes to 19.432600.
func TestGeocodingService_CoordinateNormalization(t *testing.T) {
	cache := newMockGeoCache()
	repo := newMockGeoRepo()

	// Store at the normalized coordinate.
	_ = cache.Set(context.Background(), 19.432600, -99.133200, "Normalizada 1", time.Hour)

	cfg := reports.Config{GeocodingCacheTTLDays: 30, GeocodingRateLimitRPM: 60}
	svc := service.NewGeocodingService(cache, repo, cfg)

	// Call with slightly different precision — should hit cache after normalization.
	result, err := svc.Reverse(context.Background(), 19.4326001, -99.1332001)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Cached {
		t.Error("expected Cached=true — normalization should hit Redis cache")
	}
	if result.Direccion != "Normalizada 1" {
		t.Errorf("unexpected Direccion: %q", result.Direccion)
	}
}
