package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"axis-flow-back/internal/atencionseguimiento"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// QuejaRepository — interface (port adapter target).
// ---------------------------------------------------------------------------

// QuejaRepository defines persistence operations for queja entities.
// It is the concrete interface used by the pgx and in-memory adapters below.
type QuejaRepository interface {
	Create(ctx context.Context, s *atencionseguimiento.SolicitudQueja) error
	GetByID(ctx context.Context, id uuid.UUID, empleadoID int64) (*atencionseguimiento.SolicitudQueja, error)
	ListByEmpresa(ctx context.Context, empresaID uuid.UUID, estatus *int, page, pageSize int) ([]*atencionseguimiento.SolicitudQueja, int, error)
	CreateRespuesta(ctx context.Context, r *atencionseguimiento.RespuestaQueja) error
	GetRespuestas(ctx context.Context, solicitudID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.RespuestaQueja, int, error)
	SuspendQuejasByEmpleado(ctx context.Context, empleadoID int64) error
}

// ---------------------------------------------------------------------------
// PgxQuejaRepository — PostgreSQL implementation.
// ---------------------------------------------------------------------------

type quejaDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgxQuejaRepository is a PostgreSQL-backed implementation of QuejaRepository.
type PgxQuejaRepository struct {
	db          quejaDB
	cache       *RedisAtencionCacheInvalidator
}

// NewPgxQuejaRepository creates a PostgreSQL queja repository.
// Pass a nil redis.Client to disable cache invalidation.
func NewPgxQuejaRepository(pool *pgxpool.Pool, redisClient redis.Cmdable) *PgxQuejaRepository {
	var cache *RedisAtencionCacheInvalidator
	if redisClient != nil {
		cache = NewRedisAtencionCacheInvalidator(redisClient)
	}
	return &PgxQuejaRepository{db: pool, cache: cache}
}

// Create inserts a new SolicitudQueja row.
// fecha_vigencia is computed by a DB trigger — it must NOT be set by the caller.
func (r *PgxQuejaRepository) Create(ctx context.Context, s *atencionseguimiento.SolicitudQueja) error {
	const q = `
		INSERT INTO atencion_seguimiento.solicitudes_queja
		    (id, empresa_id, empleado_id, tipo_queja_id, titulo, descripcion, estatus, ultima_resp, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())`
	_, err := r.db.Exec(ctx, q,
		s.ID, s.EmpresaID, s.EmpleadoID, s.TipoQuejaID,
		s.Titulo, s.Descripcion, s.Estatus, s.UltimaResp,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return atencionseguimiento.ErrConflict
		}
		return fmt.Errorf("queja_repository.Create: %w", err)
	}
	return nil
}

// GetByID fetches a SolicitudQueja by primary key.
// If empleadoID != 0 and the record's empleado_id does not match, ErrForbidden is returned.
func (r *PgxQuejaRepository) GetByID(ctx context.Context, id uuid.UUID, empleadoID int64) (*atencionseguimiento.SolicitudQueja, error) {
	const q = `
		SELECT id, empresa_id, empleado_id, tipo_queja_id, titulo, descripcion,
		       estatus, fecha_vigencia, ultima_resp, created_at, updated_at
		FROM atencion_seguimiento.solicitudes_queja
		WHERE id = $1`
	s, err := scanSolicitudQueja(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, atencionseguimiento.ErrNotFound
		}
		return nil, fmt.Errorf("queja_repository.GetByID: %w", err)
	}
	if empleadoID != 0 && s.EmpleadoID != empleadoID {
		return nil, atencionseguimiento.ErrForbidden
	}
	return s, nil
}

// ListByEmpresa returns paginated quejas for an empresa, optionally filtered by estatus.
func (r *PgxQuejaRepository) ListByEmpresa(ctx context.Context, empresaID uuid.UUID, estatus *int, page, pageSize int) ([]*atencionseguimiento.SolicitudQueja, int, error) {
	offset := (page - 1) * pageSize

	baseQ := `FROM atencion_seguimiento.solicitudes_queja WHERE empresa_id = $1`
	args := []any{empresaID}
	if estatus != nil {
		baseQ += fmt.Sprintf(" AND estatus = $%d", len(args)+1)
		args = append(args, *estatus)
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) "+baseQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("queja_repository.ListByEmpresa count: %w", err)
	}

	selectQ := "SELECT id, empresa_id, empleado_id, tipo_queja_id, titulo, descripcion, estatus, fecha_vigencia, ultima_resp, created_at, updated_at " +
		baseQ + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("queja_repository.ListByEmpresa query: %w", err)
	}
	defer rows.Close()

	var out []*atencionseguimiento.SolicitudQueja
	for rows.Next() {
		s, scanErr := scanSolicitudQueja(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("queja_repository.ListByEmpresa scan: %w", scanErr)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("queja_repository.ListByEmpresa rows: %w", err)
	}
	return out, total, nil
}

// CreateRespuesta inserts a RespuestaQueja and updates ultima_resp + updated_at on the parent solicitud.
// After a successful insert, it invalidates related Redis keys.
func (r *PgxQuejaRepository) CreateRespuesta(ctx context.Context, resp *atencionseguimiento.RespuestaQueja) error {
	const insertQ = `
		INSERT INTO atencion_seguimiento.respuestas_queja
		    (id, solicitud_id, remitente_id, rol_respuesta, mensaje, archivo_adjunto_url, leido, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`
	const updateQ = `
		UPDATE atencion_seguimiento.solicitudes_queja
		SET ultima_resp = $2, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(ctx, insertQ,
		resp.ID, resp.SolicitudID, resp.RemitenteID, resp.RolRespuesta,
		nullStr(resp.Mensaje), nullStr(resp.ArchivoAdjuntoURL), resp.Leido,
	)
	if err != nil {
		return fmt.Errorf("queja_repository.CreateRespuesta insert: %w", err)
	}
	if _, err := r.db.Exec(ctx, updateQ, resp.SolicitudID, resp.RolRespuesta); err != nil {
		return fmt.Errorf("queja_repository.CreateRespuesta update: %w", err)
	}
	return nil
}

// GetRespuestas returns paginated responses for a solicitud, ordered by created_at ASC.
func (r *PgxQuejaRepository) GetRespuestas(ctx context.Context, solicitudID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.RespuestaQueja, int, error) {
	offset := (page - 1) * pageSize
	const countQ = `SELECT COUNT(*) FROM atencion_seguimiento.respuestas_queja WHERE solicitud_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQ, solicitudID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("queja_repository.GetRespuestas count: %w", err)
	}
	const q = `
		SELECT id, solicitud_id, remitente_id, rol_respuesta, mensaje, COALESCE(archivo_adjunto_url,''), leido, created_at
		FROM atencion_seguimiento.respuestas_queja
		WHERE solicitud_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, solicitudID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("queja_repository.GetRespuestas query: %w", err)
	}
	defer rows.Close()
	var out []*atencionseguimiento.RespuestaQueja
	for rows.Next() {
		resp := &atencionseguimiento.RespuestaQueja{}
		if scanErr := rows.Scan(&resp.ID, &resp.SolicitudID, &resp.RemitenteID, &resp.RolRespuesta,
			&resp.Mensaje, &resp.ArchivoAdjuntoURL, &resp.Leido, &resp.CreatedAt); scanErr != nil {
			return nil, 0, fmt.Errorf("queja_repository.GetRespuestas scan: %w", scanErr)
		}
		out = append(out, resp)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("queja_repository.GetRespuestas rows: %w", err)
	}
	return out, total, nil
}

// SuspendQuejasByEmpleado sets estatus=EstatusFinalizado on all non-finalized quejas for an employee.
// Designed for the EmpleadoDeBaja domain event.
func (r *PgxQuejaRepository) SuspendQuejasByEmpleado(ctx context.Context, empleadoID int64) error {
	const q = `
		UPDATE atencion_seguimiento.solicitudes_queja
		SET estatus = $2, updated_at = NOW()
		WHERE empleado_id = $1 AND estatus != $2`
	if _, err := r.db.Exec(ctx, q, empleadoID, atencionseguimiento.EstatusFinalizado); err != nil {
		return fmt.Errorf("queja_repository.SuspendQuejasByEmpleado: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// scanners
// ---------------------------------------------------------------------------

type quejaRowScanner interface {
	Scan(dest ...any) error
}

func scanSolicitudQueja(row quejaRowScanner) (*atencionseguimiento.SolicitudQueja, error) {
	s := &atencionseguimiento.SolicitudQueja{}
	err := row.Scan(
		&s.ID, &s.EmpresaID, &s.EmpleadoID, &s.TipoQuejaID,
		&s.Titulo, &s.Descripcion, &s.Estatus, &s.FechaVigencia,
		&s.UltimaResp, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// ---------------------------------------------------------------------------
// InMemQuejaRepository — in-memory implementation for unit tests.
// ---------------------------------------------------------------------------

// InMemQuejaRepository is a goroutine-safe, in-memory QuejaRepository for unit tests.
type InMemQuejaRepository struct {
	mu         sync.RWMutex
	quejas     map[uuid.UUID]*atencionseguimiento.SolicitudQueja
	respuestas map[uuid.UUID][]*atencionseguimiento.RespuestaQueja
}

// NewInMemQuejaRepository creates an empty in-memory queja repository.
func NewInMemQuejaRepository() *InMemQuejaRepository {
	return &InMemQuejaRepository{
		quejas:     make(map[uuid.UUID]*atencionseguimiento.SolicitudQueja),
		respuestas: make(map[uuid.UUID][]*atencionseguimiento.RespuestaQueja),
	}
}

func (r *InMemQuejaRepository) Create(_ context.Context, s *atencionseguimiento.SolicitudQueja) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	cp := *s
	r.quejas[s.ID] = &cp
	return nil
}

func (r *InMemQuejaRepository) GetByID(_ context.Context, id uuid.UUID, empleadoID int64) (*atencionseguimiento.SolicitudQueja, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.quejas[id]
	if !ok {
		return nil, atencionseguimiento.ErrNotFound
	}
	if empleadoID != 0 && s.EmpleadoID != empleadoID {
		return nil, atencionseguimiento.ErrForbidden
	}
	cp := *s
	return &cp, nil
}

func (r *InMemQuejaRepository) ListByEmpresa(_ context.Context, empresaID uuid.UUID, estatus *int, page, pageSize int) ([]*atencionseguimiento.SolicitudQueja, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*atencionseguimiento.SolicitudQueja
	for _, s := range r.quejas {
		if s.EmpresaID != empresaID {
			continue
		}
		if estatus != nil && s.Estatus != *estatus {
			continue
		}
		cp := *s
		filtered = append(filtered, &cp)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	total := len(filtered)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []*atencionseguimiento.SolicitudQueja{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}

func (r *InMemQuejaRepository) CreateRespuesta(_ context.Context, resp *atencionseguimiento.RespuestaQueja) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if resp.ID == uuid.Nil {
		resp.ID = uuid.New()
	}
	resp.CreatedAt = time.Now().UTC()
	cp := *resp
	r.respuestas[resp.SolicitudID] = append(r.respuestas[resp.SolicitudID], &cp)
	// update ultima_resp on parent
	if s, ok := r.quejas[resp.SolicitudID]; ok {
		s.UltimaResp = resp.RolRespuesta
		s.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (r *InMemQuejaRepository) GetRespuestas(_ context.Context, solicitudID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.RespuestaQueja, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := r.respuestas[solicitudID]
	total := len(all)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []*atencionseguimiento.RespuestaQueja{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (r *InMemQuejaRepository) SuspendQuejasByEmpleado(_ context.Context, empleadoID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.quejas {
		if s.EmpleadoID == empleadoID && s.Estatus != atencionseguimiento.EstatusFinalizado {
			s.Estatus = atencionseguimiento.EstatusFinalizado
			s.UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}
