package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"axis-flow-back/internal/formularios"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// PgxRespuestaRepository — PostgreSQL implementation of RespuestaRepository.
// ---------------------------------------------------------------------------

// PgxRespuestaRepository is a PostgreSQL-backed RespuestaRepository.
type PgxRespuestaRepository struct {
	db formulariosDB
}

// NewPgxRespuestaRepository creates a PostgreSQL respuesta repository.
func NewPgxRespuestaRepository(pool *pgxpool.Pool) *PgxRespuestaRepository {
	return &PgxRespuestaRepository{db: pool}
}

// Create inserts a new formularios_respuestas row. Evidence URLs are stored as
// plain VARCHAR paths (JSON-declared or pre-uploaded S3 keys) — this adapter
// does NOT touch S3; the upload itself is a service-layer concern (PR-3).
// The DB CHECK on geolocation lat/lon enforces the [-90, 90] / [-180, 180]
// ranges; we do not duplicate the check here.
func (r *PgxRespuestaRepository) Create(ctx context.Context, resp *formularios.Respuesta) error {
	const q = `
		INSERT INTO formularios.formularios_respuestas
		    (id, evento_iniciado_id, pregunta_id, respuesta_texto, respuesta_lista,
		     evidencia1, evidencia2, evidencia3, documento_url,
		     geolocalizacion_respuesta_lat, geolocalizacion_respuesta_lon,
		     created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())`
	var rawLista any
	if len(resp.RespuestaLista) > 0 {
		rawLista = []byte(resp.RespuestaLista)
	}
	_, err := r.db.Exec(ctx, q,
		resp.ID, resp.EventoIniciadoID, resp.PreguntaID,
		nullStr(resp.RespuestaTexto), rawLista,
		nullStr(resp.Evidencia1), nullStr(resp.Evidencia2), nullStr(resp.Evidencia3),
		nullStr(resp.DocumentoURL),
		nullFloat(resp.GeolocalizacionRespuestaLat),
		nullFloat(resp.GeolocalizacionRespuestaLon),
	)
	if mapped := mapPgError(err); mapped != nil {
		if errors.Is(mapped, formularios.ErrInvalidInput) || errors.Is(mapped, formularios.ErrConflict) || errors.Is(mapped, formularios.ErrNotFound) {
			return mapped
		}
		return fmt.Errorf("respuesta_repository.Create: %w", err)
	}
	return nil
}

// ListByIniciado returns respuestas for the given iniciado, ordered by
// created_at ASC. IDOR is enforced at the service layer (the parent iniciado
// must be fetched and scoped to the caller's empresa first).
func (r *PgxRespuestaRepository) ListByIniciado(ctx context.Context, iniciadoID uuid.UUID) ([]*formularios.Respuesta, error) {
	const q = `
		SELECT id, evento_iniciado_id, pregunta_id,
		       COALESCE(respuesta_texto, ''),
		       COALESCE(respuesta_lista, '{}'::jsonb),
		       COALESCE(evidencia1, ''), COALESCE(evidencia2, ''), COALESCE(evidencia3, ''),
		       COALESCE(documento_url, ''),
		       geolocalizacion_respuesta_lat, geolocalizacion_respuesta_lon,
		       created_at, updated_at
		FROM formularios.formularios_respuestas
		WHERE evento_iniciado_id = $1
		ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, iniciadoID)
	if err != nil {
		return nil, fmt.Errorf("respuesta_repository.ListByIniciado query: %w", err)
	}
	defer rows.Close()

	var out []*formularios.Respuesta
	for rows.Next() {
		resp, scanErr := scanRespuesta(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("respuesta_repository.ListByIniciado scan: %w", scanErr)
		}
		out = append(out, resp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("respuesta_repository.ListByIniciado rows: %w", err)
	}
	if out == nil {
		return []*formularios.Respuesta{}, nil
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// scanners
// ---------------------------------------------------------------------------

type respuestaRowScanner interface {
	Scan(dest ...any) error
}

func scanRespuesta(row respuestaRowScanner) (*formularios.Respuesta, error) {
	r := &formularios.Respuesta{}
	var rawLista []byte
	var lat, lon *float64
	err := row.Scan(
		&r.ID, &r.EventoIniciadoID, &r.PreguntaID,
		&r.RespuestaTexto, &rawLista,
		&r.Evidencia1, &r.Evidencia2, &r.Evidencia3, &r.DocumentoURL,
		&lat, &lon, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, formularios.ErrNotFound
		}
		return nil, err
	}
	if len(rawLista) > 0 {
		r.RespuestaLista = append(r.RespuestaLista, rawLista...)
	}
	r.GeolocalizacionRespuestaLat = lat
	r.GeolocalizacionRespuestaLon = lon
	return r, nil
}

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
	respuestas map[uuid.UUID]*formularios.Respuesta
}

// NewInMemRespuestaRepository creates an empty in-memory respuesta repository.
func NewInMemRespuestaRepository() *InMemRespuestaRepository {
	return &InMemRespuestaRepository{respuestas: make(map[uuid.UUID]*formularios.Respuesta)}
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
func (r *InMemRespuestaRepository) Create(_ context.Context, resp *formularios.Respuesta) error {
	if !validRespuestaLat(resp.GeolocalizacionRespuestaLat) {
		return fmt.Errorf("%w: geolocalizacion_respuesta_lat out of range", formularios.ErrInvalidInput)
	}
	if !validRespuestaLon(resp.GeolocalizacionRespuestaLon) {
		return fmt.Errorf("%w: geolocalizacion_respuesta_lon out of range", formularios.ErrInvalidInput)
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
func (r *InMemRespuestaRepository) ListByIniciado(_ context.Context, iniciadoID uuid.UUID) ([]*formularios.Respuesta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*formularios.Respuesta
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
		return []*formularios.Respuesta{}, nil
	}
	return out, nil
}
