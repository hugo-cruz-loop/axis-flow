package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"axis-flow-back/internal/reports"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// GeocodingRepo interface
// ---------------------------------------------------------------------------

// GeocodingRepo persists reverse-geocoding results in reports.reports_direcciones.
type GeocodingRepo interface {
	// GetByCoords fetches a cached direccion by exact coordinate match.
	// Returns reports.ErrNotFound when no row exists.
	GetByCoords(ctx context.Context, lat, lng float64) (*reports.DireccionCache, error)
	// Store upserts a DireccionCache entry. On conflict (latitud, longitud)
	// the direccion and updated_at are updated.
	Store(ctx context.Context, d reports.DireccionCache) error
	// DeleteByTenant is called on EmpresaDeBaja events. Because geocoding
	// cache is global (not tenant-scoped), this is intentionally a no-op.
	// A warning is logged to make the no-op explicit in observability.
	DeleteByTenant(ctx context.Context, empresaID string) error
}

// ---------------------------------------------------------------------------
// pgGeocodingRepo — PostgreSQL implementation
// ---------------------------------------------------------------------------

type geocodingDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type pgGeocodingRepo struct {
	db geocodingDB
}

// NewPgGeocodingRepo creates a PostgreSQL-backed GeocodingRepo.
func NewPgGeocodingRepo(pool *pgxpool.Pool) GeocodingRepo {
	return &pgGeocodingRepo{db: pool}
}

func (r *pgGeocodingRepo) GetByCoords(ctx context.Context, lat, lng float64) (*reports.DireccionCache, error) {
	const q = `
		SELECT latitud, longitud, direccion,
		       created_at::text, updated_at::text
		FROM reports.reports_direcciones
		WHERE latitud = $1 AND longitud = $2`

	d := &reports.DireccionCache{}
	err := r.db.QueryRow(ctx, q, lat, lng).Scan(
		&d.Latitud, &d.Longitud, &d.Direccion, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, reports.ErrNotFound
		}
		return nil, fmt.Errorf("geocoding_repository.GetByCoords: %w", err)
	}
	return d, nil
}

func (r *pgGeocodingRepo) Store(ctx context.Context, d reports.DireccionCache) error {
	const q = `
		INSERT INTO reports.reports_direcciones (latitud, longitud, direccion)
		VALUES ($1, $2, $3)
		ON CONFLICT (latitud, longitud)
		DO UPDATE SET direccion = EXCLUDED.direccion, updated_at = now()`
	if _, err := r.db.Exec(ctx, q, d.Latitud, d.Longitud, d.Direccion); err != nil {
		return fmt.Errorf("geocoding_repository.Store: %w", err)
	}
	return nil
}

// DeleteByTenant is a deliberate no-op: geocoding cache is not tenant-scoped.
// Deleting entries on EmpresaDeBaja would evict shared data for other tenants.
func (r *pgGeocodingRepo) DeleteByTenant(_ context.Context, empresaID string) error {
	log.Printf("WARNING geocoding_repository.DeleteByTenant: geocoding cache is global, skipping tenant-scoped delete for empresa_id=%s", empresaID)
	return nil
}

// ---------------------------------------------------------------------------
// InMemGeocodingRepo — in-memory stub for unit tests
// ---------------------------------------------------------------------------

type inMemGeocodingEntry struct {
	d reports.DireccionCache
}

// InMemGeocodingRepo is a goroutine-safe, in-memory GeocodingRepo for tests.
type InMemGeocodingRepo struct {
	mu      sync.RWMutex
	entries map[string]inMemGeocodingEntry
}

// NewInMemGeocodingRepo creates an empty in-memory GeocodingRepo.
func NewInMemGeocodingRepo() *InMemGeocodingRepo {
	return &InMemGeocodingRepo{entries: make(map[string]inMemGeocodingEntry)}
}

func coordKey(lat, lng float64) string {
	return fmt.Sprintf("%.6f,%.6f", lat, lng)
}

func (r *InMemGeocodingRepo) GetByCoords(_ context.Context, lat, lng float64) (*reports.DireccionCache, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[coordKey(lat, lng)]
	if !ok {
		return nil, reports.ErrNotFound
	}
	cp := entry.d
	return &cp, nil
}

func (r *InMemGeocodingRepo) Store(_ context.Context, d reports.DireccionCache) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[coordKey(d.Latitud, d.Longitud)] = inMemGeocodingEntry{d: d}
	return nil
}

func (r *InMemGeocodingRepo) DeleteByTenant(_ context.Context, empresaID string) error {
	log.Printf("WARNING geocoding_repository.DeleteByTenant: geocoding cache is global, skipping tenant-scoped delete for empresa_id=%s", empresaID)
	return nil
}
