package formularios

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// InMemRespuestaRepository — goroutine-safe, in-memory adapter.
//
// PR-2 (Repositories) — task 2.4.
//
// The repository stores VARCHAR paths for evidence URLs only. The S3 upload
// itself is a service-layer concern (PR-3); this adapter does NOT touch S3.
// Mirrors the SQL CHECK constraint on geolocation lat/lon so the constraint
// is testable without a live database.
// ---------------------------------------------------------------------------

// InMemRespuestaRepository is a goroutine-safe, in-memory RespuestaRepository.
type InMemRespuestaRepository struct {
	mu         sync.RWMutex
	respuestas map[uuid.UUID]*Respuesta
}

// NewInMemRespuestaRepository creates an empty in-memory respuesta repository.
func NewInMemRespuestaRepository() *InMemRespuestaRepository {
	return &InMemRespuestaRepository{respuestas: make(map[uuid.UUID]*Respuesta)}
}

// validRespuestaLat mirrors chk_formularios_respuestas_geo_lat.
func validRespuestaLat(lat *float64) bool {
	if lat == nil {
		return true
	}
	return *lat >= -90.0 && *lat <= 90.0
}

// validRespuestaLon mirrors chk_formularios_respuestas_geo_lon.
func validRespuestaLon(lon *float64) bool {
	if lon == nil {
		return true
	}
	return *lon >= -180.0 && *lon <= 180.0
}

// Create inserts a new Respuesta. The repository does not enforce that the
// parent evento_iniciado / pregunta exist in memory — those FKs are guarded
// by the SQL schema and the service layer is expected to have validated them
// before calling Create. Evidence URLs are stored as plain VARCHAR paths
// (JSON-declared or pre-uploaded S3 keys); the repository does NOT touch S3.
//
// If ID is uuid.Nil, a fresh ID is generated. CreatedAt / UpdatedAt are
// stamped if zero.
func (r *InMemRespuestaRepository) Create(_ context.Context, resp *Respuesta) error {
	if !validRespuestaLat(resp.GeolocalizacionRespuestaLat) {
		return fmt.Errorf("%w: geolocalizacion_respuesta_lat out of range", ErrInvalidInput)
	}
	if !validRespuestaLon(resp.GeolocalizacionRespuestaLon) {
		return fmt.Errorf("%w: geolocalizacion_respuesta_lon out of range", ErrInvalidInput)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if resp.ID == uuid.Nil {
		resp.ID = uuid.New()
	}
	now := time.Now().UTC()
	if resp.CreatedAt.IsZero() {
		resp.CreatedAt = now
	}
	if resp.UpdatedAt.IsZero() {
		resp.UpdatedAt = now
	}
	cp := *resp
	r.respuestas[resp.ID] = &cp
	return nil
}

// ListByIniciado returns all respuestas for the given iniciado, ordered by
// created_at ASC. Returns an empty slice (not nil) for unknown iniciados.
func (r *InMemRespuestaRepository) ListByIniciado(_ context.Context, iniciadoID uuid.UUID) ([]*Respuesta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*Respuesta
	for _, resp := range r.respuestas {
		if resp.EventoIniciadoID != iniciadoID {
			continue
		}
		cp := *resp
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	if out == nil {
		return []*Respuesta{}, nil
	}
	return out, nil
}
