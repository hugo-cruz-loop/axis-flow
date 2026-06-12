// Package service provides the business-logic layer for the Reports module.
// Service methods coordinate caching, external API calls, and PII masking.
// No SQL lives in this package — all persistence is delegated to repository interfaces.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/repository"
)

// ---------------------------------------------------------------------------
// GeocodingService interface
// ---------------------------------------------------------------------------

// GeocodingService resolves (lat, lng) pairs to human-readable street addresses
// using a 3-layer cache: Redis → PostgreSQL → GCP Geocoding API.
type GeocodingService interface {
	// Reverse converts coordinates to a street address.
	// Returns GeocodingResult{Cached: true} when served from any cache layer.
	// Returns ErrRateLimitExceeded when the live-API rate limit is exhausted.
	// Returns ErrGeocodingUnavailable when GCP returns no usable results.
	Reverse(ctx context.Context, lat, lng float64) (reports.GeocodingResult, error)
}

// ---------------------------------------------------------------------------
// gcpGeocodingService — 3-layer cache implementation
// ---------------------------------------------------------------------------

// gcpGeocodeResponse mirrors the subset of the GCP Geocoding API response
// that the service needs.
type gcpGeocodeResponse struct {
	Status  string `json:"status"`
	Results []struct {
		FormattedAddress string `json:"formatted_address"`
	} `json:"results"`
}

type gcpGeocodingService struct {
	cache   repository.GeocodingCache
	repo    repository.GeocodingRepo
	cfg     reports.Config
	baseURL string // injectable for testing; defaults to GCP endpoint

	// Rate limiter fields — atomic counter + mutex-guarded window reset.
	mu          sync.Mutex
	rpmCounter  int64
	windowStart time.Time

	httpClient *http.Client
}

// CacheKeyForTest exports the coordinate normalization key so test helpers can
// pre-populate mock caches with the exact key the service uses internally.
// This is a test-support export — do not use in production code paths.
func CacheKeyForTest(lat, lng float64) string {
	return fmt.Sprintf("%.6f,%.6f", normCoord(lat), normCoord(lng))
}

// normCoord rounds a coordinate to 6 decimal places to avoid float equality
// issues when comparing coordinates stored at different precision levels.
func normCoord(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

const gcpBaseURL = "https://maps.googleapis.com/maps/api/geocode/json"

// NewGeocodingService creates a GeocodingService that calls the real GCP endpoint.
func NewGeocodingService(
	cache repository.GeocodingCache,
	repo repository.GeocodingRepo,
	cfg reports.Config,
) GeocodingService {
	return NewGeocodingServiceWithBaseURL(cache, repo, cfg, gcpBaseURL)
}

// NewGeocodingServiceWithBaseURL creates a GeocodingService with an injectable
// base URL so tests can point to an httptest.Server without hitting GCP.
func NewGeocodingServiceWithBaseURL(
	cache repository.GeocodingCache,
	repo repository.GeocodingRepo,
	cfg reports.Config,
	baseURL string,
) GeocodingService {
	return &gcpGeocodingService{
		cache:       cache,
		repo:        repo,
		cfg:         cfg,
		baseURL:     baseURL,
		windowStart: time.Now(),
		httpClient:  &http.Client{Timeout: 3 * time.Second},
	}
}

// Reverse implements GeocodingService.
func (s *gcpGeocodingService) Reverse(ctx context.Context, lat, lng float64) (reports.GeocodingResult, error) {
	// Step 1 — normalize coordinates to 6 decimal places.
	lat = normCoord(lat)
	lng = normCoord(lng)

	// Step 2 — check Redis cache.
	if direccion, ok := s.cache.Get(ctx, lat, lng); ok {
		return reports.GeocodingResult{
			Latitud:   lat,
			Longitud:  lng,
			Direccion: direccion,
			Cached:    true,
		}, nil
	}

	// Step 3 — check PostgreSQL cache.
	if d, err := s.repo.GetByCoords(ctx, lat, lng); err == nil {
		// Backfill Redis so subsequent requests skip PG.
		ttl := time.Duration(s.cfg.GeocodingCacheTTLDays) * 24 * time.Hour
		_ = s.cache.Set(ctx, lat, lng, d.Direccion, ttl)
		return reports.GeocodingResult{
			Latitud:   lat,
			Longitud:  lng,
			Direccion: d.Direccion,
			Cached:    true,
		}, nil
	}

	// Step 4 — rate limit check before any live GCP call.
	if !s.allowRequest() {
		slog.WarnContext(ctx, "reports: geocoding rate limit exceeded",
			"lat", lat, "lng", lng)
		return reports.GeocodingResult{}, reports.ErrRateLimitExceeded
	}

	// Step 5 — call GCP Geocoding API.
	direccion, err := s.callGCP(ctx, lat, lng)
	if err != nil {
		return reports.GeocodingResult{}, err
	}

	// Step 6 — store result in both caches.
	ttl := time.Duration(s.cfg.GeocodingCacheTTLDays) * 24 * time.Hour
	_ = s.cache.Set(ctx, lat, lng, direccion, ttl)
	_ = s.repo.Store(ctx, reports.DireccionCache{
		Latitud:   lat,
		Longitud:  lng,
		Direccion: direccion,
	})

	return reports.GeocodingResult{
		Latitud:   lat,
		Longitud:  lng,
		Direccion: direccion,
		Cached:    false,
	}, nil
}

// allowRequest checks and increments the rate-limit counter.
// Returns true if the request is allowed, false if the RPM window is full.
// A GeocodingRateLimitRPM of 0 means no live calls are permitted at all.
func (s *gcpGeocodingService) allowRequest() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if now.Sub(s.windowStart) >= time.Minute {
		// Reset window.
		atomic.StoreInt64(&s.rpmCounter, 0)
		s.windowStart = now
	}

	current := atomic.LoadInt64(&s.rpmCounter)
	if int(current) >= s.cfg.GeocodingRateLimitRPM {
		return false
	}
	atomic.AddInt64(&s.rpmCounter, 1)
	return true
}

// callGCP performs the HTTP request to the GCP Geocoding API.
// The API key is intentionally excluded from all log fields.
func (s *gcpGeocodingService) callGCP(ctx context.Context, lat, lng float64) (string, error) {
	// Build URL with latlng and key parameters. The key is NEVER logged.
	url := fmt.Sprintf("%s?latlng=%.6f,%.6f&key=%s", s.baseURL, lat, lng, s.cfg.GCPGeocodingAPIKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("geocoding_service.callGCP: build request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "reports: GCP geocoding HTTP error",
			"lat", lat, "lng", lng, "error", err.Error())
		return "", reports.ErrGeocodingUnavailable
	}
	defer resp.Body.Close()

	var gcpResp gcpGeocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&gcpResp); err != nil {
		slog.WarnContext(ctx, "reports: GCP geocoding decode error",
			"lat", lat, "lng", lng, "error", err.Error())
		return "", reports.ErrGeocodingUnavailable
	}

	if len(gcpResp.Results) == 0 {
		slog.InfoContext(ctx, "reports: GCP geocoding returned no results",
			"lat", lat, "lng", lng, "status", gcpResp.Status)
		return "", reports.ErrGeocodingUnavailable
	}

	return gcpResp.Results[0].FormattedAddress, nil
}
