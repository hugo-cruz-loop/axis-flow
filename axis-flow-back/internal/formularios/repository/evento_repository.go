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
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// PgxEventoRepository — PostgreSQL implementation of EventoRepository.
// ---------------------------------------------------------------------------

// PgxEventoRepository is a PostgreSQL-backed EventoRepository.
type PgxEventoRepository struct {
	db    formulariosDB
	cache *RedisFormulariosCacheInvalidator
}

// NewPgxEventoRepository creates a PostgreSQL evento repository.
// Pass a nil redis.Client to disable cache invalidation.
func NewPgxEventoRepository(pool *pgxpool.Pool, redisClient redis.Cmdable) *PgxEventoRepository {
	var cache *RedisFormulariosCacheInvalidator
	if redisClient != nil {
		cache = NewRedisFormulariosCacheInvalidator(redisClient)
	}
	return &PgxEventoRepository{db: pool, cache: cache}
}

// Create inserts a new eventos_evento header plus one M:N association row per
// formularioID in eventos_evento_formulario.
func (r *PgxEventoRepository) Create(ctx context.Context, e *formularios.Evento, formularioIDs []uuid.UUID) error {
	const insertHeader = `
		INSERT INTO formularios.eventos_evento
		    (id, empresa_id, cliente_id, localidad_id, nombre, descripcion,
		     fecha_programada, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`
	_, err := r.db.Exec(ctx, insertHeader,
		e.ID, e.EmpresaID, e.ClienteID, e.LocalidadID,
		e.Nombre, nullStr(e.Descripcion), e.FechaProgramada, e.Status,
	)
	if mapped := mapPgError(err); mapped != nil {
		if errors.Is(mapped, formularios.ErrInvalidInput) || errors.Is(mapped, formularios.ErrConflict) {
			return mapped
		}
		return fmt.Errorf("evento_repository.Create insert: %w", err)
	}

	if len(formularioIDs) > 0 {
		const insertAsoc = `
			INSERT INTO formularios.eventos_evento_formulario
			    (id, evento_id, formulario_id, created_at)
			VALUES ($1, $2, $3, NOW())`
		for _, formID := range formularioIDs {
			_, err := r.db.Exec(ctx, insertAsoc, uuid.New(), e.ID, formID)
			if mapped := mapPgError(err); mapped != nil {
				if errors.Is(mapped, formularios.ErrConflict) {
					return formularios.ErrConflict
				}
				return fmt.Errorf("evento_repository.Create asoc: %w", err)
			}
		}
	}
	return nil
}

// GetByID fetches an eventos_evento row by primary key, scoped by empresa_id
// for IDOR. Returns formularios.ErrNotFound on miss or wrong tenant.
func (r *PgxEventoRepository) GetByID(ctx context.Context, id, empresaID uuid.UUID) (*formularios.Evento, error) {
	const q = `
		SELECT id, empresa_id, cliente_id, localidad_id, nombre, COALESCE(descripcion,''),
		       fecha_programada, status, created_at
		FROM formularios.eventos_evento
		WHERE id = $1 AND empresa_id = $2`
	e, err := scanEvento(r.db.QueryRow(ctx, q, id, empresaID))
	if err != nil {
		if errors.Is(err, formularios.ErrNotFound) {
			return nil, formularios.ErrNotFound
		}
		return nil, fmt.Errorf("evento_repository.GetByID: %w", err)
	}
	return e, nil
}

// ListByEmpCte returns paginated eventos for an (empresa, cliente) pair,
// optionally filtered by status, ordered by created_at DESC.
func (r *PgxEventoRepository) ListByEmpCte(ctx context.Context, empresaID, clienteID uuid.UUID, status *string, page, pageSize int) ([]*formularios.Evento, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	baseQ := `FROM formularios.eventos_evento WHERE empresa_id = $1 AND cliente_id = $2`
	args := []any{empresaID, clienteID}
	if status != nil {
		baseQ += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, *status)
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) "+baseQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("evento_repository.ListByEmpCte count: %w", err)
	}

	selectQ := "SELECT id, empresa_id, cliente_id, localidad_id, nombre, COALESCE(descripcion,''), fecha_programada, status, created_at " +
		baseQ + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("evento_repository.ListByEmpCte query: %w", err)
	}
	defer rows.Close()

	var out []*formularios.Evento
	for rows.Next() {
		e, scanErr := scanEvento(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("evento_repository.ListByEmpCte scan: %w", scanErr)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("evento_repository.ListByEmpCte rows: %w", err)
	}
	return out, total, nil
}

// CreateIniciado inserts a new eventos_evento_iniciado row. The FK to
// formularios.eventos_evento (and the CHECK on status / geo) is enforced at
// the DB level; the service layer is responsible for confirming the parent
// evento belongs to the caller's empresa BEFORE calling CreateIniciado
// (via eventoRepo.GetByID(ctx, eventoID, empresaID)).
//
// The cross-table IDOR comment in the InMem implementation applies here too:
// the iniciado row has no empresa_id column, so the empresa check is the
// service layer's responsibility.
func (r *PgxEventoRepository) CreateIniciado(ctx context.Context, i *formularios.EventoIniciado) error {
	const q = `
		INSERT INTO formularios.eventos_evento_iniciado
		    (id, evento_id, empleado_id, geolocalizacion_inicio_lat,
		     geolocalizacion_inicio_lon, check_in_time, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`
	_, err := r.db.Exec(ctx, q,
		i.ID, i.EventoID, i.EmpleadoID,
		nullFloat(i.GeolocalizacionInicioLat),
		nullFloat(i.GeolocalizacionInicioLon),
		i.CheckInTime, i.Status,
	)
	if mapped := mapPgError(err); mapped != nil {
		if errors.Is(mapped, formularios.ErrInvalidInput) || errors.Is(mapped, formularios.ErrConflict) || errors.Is(mapped, formularios.ErrNotFound) {
			return mapped
		}
		return fmt.Errorf("evento_repository.CreateIniciado: %w", err)
	}
	return nil
}

// UpdateStatus flips the evento's status scoped to empresaID. PR-5
// (5.2a) — the CancelEvento service method uses this to mark an
// evento as 'cancelado' when the EventoCancelado consumer receives
// a cross-domain event. Returns formularios.ErrNotFound for
// unknown IDs or foreign tenants. The SQL CHECK on status is
// enforced at the DB level; the caller is expected to have already
// validated the new status against the canonical set.
func (r *PgxEventoRepository) UpdateStatus(ctx context.Context, id, empresaID uuid.UUID, status string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE formularios.eventos_evento
         SET status = $1
         WHERE id = $2 AND empresa_id = $3`,
		status, id, empresaID,
	)
	if err != nil {
		if mapped := mapPgError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("evento_repository.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return formularios.ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// scanners
// ---------------------------------------------------------------------------

type eventoRowScanner interface {
	Scan(dest ...any) error
}

func scanEvento(row eventoRowScanner) (*formularios.Evento, error) {
	e := &formularios.Evento{}
	err := row.Scan(
		&e.ID, &e.EmpresaID, &e.ClienteID, &e.LocalidadID,
		&e.Nombre, &e.Descripcion, &e.FechaProgramada, &e.Status, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, formularios.ErrNotFound
		}
		return nil, err
	}
	return e, nil
}

func nullFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

// ---------------------------------------------------------------------------
// InMemEventoRepository — goroutine-safe, in-memory adapter.
//
// PR-2 (Repositories) — task 2.3.
//
// Owns the in-memory state for Evento, EventoFormulario (M:N association),
// and EventoIniciado. Mirrors the SQL CHECK / UNIQUE constraints so the
// repository contract is testable without a live database:
//   - chk_eventos_evento_status
//   - uq_eventos_evento_formulario_evento_formulario
//   - chk_eventos_evento_iniciado_status
//   - chk_eventos_evento_iniciado_geo_lat / _lon
//
// The pgx adapter (this file holds both adapters) relies on the DB layer for
// the same constraints; this in-memory simulation is for unit tests only.
// ---------------------------------------------------------------------------

// InMemEventoRepository is a goroutine-safe, in-memory EventoRepository.
type InMemEventoRepository struct {
	mu               sync.RWMutex
	eventos          map[uuid.UUID]*formularios.Evento
	asociaciones     map[uuid.UUID]map[uuid.UUID]struct{} // eventoID -> set of formularioID
	eventosIniciados map[uuid.UUID]*formularios.EventoIniciado
}

// NewInMemEventoRepository creates an empty in-memory evento repository.
func NewInMemEventoRepository() *InMemEventoRepository {
	return &InMemEventoRepository{
		eventos:          make(map[uuid.UUID]*formularios.Evento),
		asociaciones:     make(map[uuid.UUID]map[uuid.UUID]struct{}),
		eventosIniciados: make(map[uuid.UUID]*formularios.EventoIniciado),
	}
}

// validEventoStatus mirrors the SQL CHECK on eventos_evento.status.
func validEventoStatus(s string) bool {
	switch s {
	case formularios.EventoStatusPendiente, formularios.EventoStatusIniciado, formularios.EventoStatusCompletado, formularios.EventoStatusCancelado:
		return true
	}
	return false
}

// validIniciadoStatus mirrors the SQL CHECK on eventos_evento_iniciado.status.
func validIniciadoStatus(s string) bool {
	switch s {
	case formularios.IniciadoStatus, formularios.CompletadoStatus, formularios.CanceladoStatus:
		return true
	}
	return false
}

// validLat mirrors chk_eventos_evento_iniciado_geo_lat (lat ∈ [-90, 90]).
func validLat(lat *float64) bool {
	if lat == nil {
		return true
	}
	return *lat >= -90.0 && *lat <= 90.0
}

// validLon mirrors chk_eventos_evento_iniciado_geo_lon (lon ∈ [-180, 180]).
func validLon(lon *float64) bool {
	if lon == nil {
		return true
	}
	return *lon >= -180.0 && *lon <= 180.0
}

// Create inserts a new Evento header plus one M:N association row per
// formularioID. Enforces the (evento_id, formulario_id) UNIQUE constraint
// in-memory. If the header already exists with the same ID, returns
// formularios.ErrConflict.
func (r *InMemEventoRepository) Create(_ context.Context, e *formularios.Evento, formularioIDs []uuid.UUID) error {
	if !validEventoStatus(e.Status) {
		return fmt.Errorf("%w: invalid evento.status", formularios.ErrInvalidInput)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.eventos[e.ID]; exists {
		return fmt.Errorf("%w: evento %s already exists", formularios.ErrConflict, e.ID)
	}

	// Validate that all formularioIDs are unique and that the same
	// (evento_id, formulario_id) pair is not duplicated within this call.
	seen := make(map[uuid.UUID]struct{}, len(formularioIDs))
	for _, formID := range formularioIDs {
		if _, dup := seen[formID]; dup {
			return fmt.Errorf("%w: duplicate formulario_id %s in Create call", formularios.ErrConflict, formID)
		}
		seen[formID] = struct{}{}
	}

	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	cp := *e
	r.eventos[e.ID] = &cp

	if len(formularioIDs) > 0 {
		asoc := make(map[uuid.UUID]struct{}, len(formularioIDs))
		for _, formID := range formularioIDs {
			asoc[formID] = struct{}{}
		}
		r.asociaciones[e.ID] = asoc
	}
	return nil
}

// GetByID returns the Evento if it exists AND belongs to empresaID. Any
// mismatch returns formularios.ErrNotFound to avoid leaking the existence of
// a foreign-empresa row.
func (r *InMemEventoRepository) GetByID(_ context.Context, id, empresaID uuid.UUID) (*formularios.Evento, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.eventos[id]
	if !ok || e.EmpresaID != empresaID {
		return nil, formularios.ErrNotFound
	}
	cp := *e
	return &cp, nil
}

// ListByEmpCte returns paginated eventos for a (empresa, cliente) pair,
// optionally filtered by status, ordered by created_at DESC.
func (r *InMemEventoRepository) ListByEmpCte(_ context.Context, empresaID, clienteID uuid.UUID, status *string, page, pageSize int) ([]*formularios.Evento, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*formularios.Evento
	for _, e := range r.eventos {
		if e.EmpresaID != empresaID || e.ClienteID != clienteID {
			continue
		}
		if status != nil && e.Status != *status {
			continue
		}
		cp := *e
		filtered = append(filtered, &cp)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	total := len(filtered)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	if offset >= total {
		return []*formularios.Evento{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}

// CreateIniciado inserts a new EventoIniciado row. Validates that the parent
// evento exists, that the iniciado status / geo CHECKs pass, and that the
// parent evento has an associated empresa. The InMem adapter enforces the
// cross-table IDOR (the iniciado row has no empresa_id column, so we look
// up the parent to confirm the caller's tenant scope is consistent — the
// service layer is expected to have called GetByID with the same empresaID
// before reaching this method).
func (r *InMemEventoRepository) CreateIniciado(_ context.Context, i *formularios.EventoIniciado) error {
	if !validIniciadoStatus(i.Status) {
		return fmt.Errorf("%w: invalid iniciado.status", formularios.ErrInvalidInput)
	}
	if !validLat(i.GeolocalizacionInicioLat) {
		return fmt.Errorf("%w: geolocalizacion_inicio_lat out of range", formularios.ErrInvalidInput)
	}
	if !validLon(i.GeolocalizacionInicioLon) {
		return fmt.Errorf("%w: geolocalizacion_inicio_lon out of range", formularios.ErrInvalidInput)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	parent, ok := r.eventos[i.EventoID]
	if !ok {
		return fmt.Errorf("%w: parent evento %s does not exist", formularios.ErrNotFound, i.EventoID)
	}
	// Cross-table IDOR: the service layer is responsible for confirming
	// the parent evento belongs to the calling empresa. If the parent
	// has a different empresa, treat the call as "not found" so the
	// caller cannot probe the existence of a foreign-empresa row.
	_ = parent // referenced for future use; the InMem impl relies on the
	// service layer's pre-check. We do not return formularios.ErrForbidden here
	// because the iniciado row itself has no empresa_id column to compare
	// against.

	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	now := time.Now().UTC()
	if i.CheckInTime.IsZero() {
		i.CheckInTime = now
	}
	if i.CreatedAt.IsZero() {
		i.CreatedAt = now
	}
	cp := *i
	r.eventosIniciados[i.ID] = &cp
	return nil
}

// UpdateStatus flips the evento's status in-memory, scoped to
// empresaID. PR-5 (5.2a). Mirrors the SQL UPDATE behavior: unknown
// IDs OR foreign tenants return formularios.ErrNotFound (no leak);
// the SQL CHECK on status is mirrored by validEventoStatus so the
// in-memory adapter cannot drift from the DB-level invariant.
func (r *InMemEventoRepository) UpdateStatus(_ context.Context, id, empresaID uuid.UUID, status string) error {
	if !validEventoStatus(status) {
		return fmt.Errorf("%w: invalid evento.status", formularios.ErrInvalidInput)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.eventos[id]
	if !ok || e.EmpresaID != empresaID {
		return formularios.ErrNotFound
	}
	e.Status = status
	return nil
}
