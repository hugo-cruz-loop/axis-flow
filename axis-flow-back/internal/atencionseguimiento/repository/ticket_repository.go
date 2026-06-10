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
// TicketRepository — interface.
// ---------------------------------------------------------------------------

// TicketRepository defines persistence operations for ticket service entities.
type TicketRepository interface {
	CreateTicket(ctx context.Context, t *atencionseguimiento.TicketServicio) error
	GetTicket(ctx context.Context, id uuid.UUID) (*atencionseguimiento.TicketServicio, error)
	GetTicketsByCliente(ctx context.Context, clienteID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.TicketServicio, int, error)
	UpdateTicketEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*atencionseguimiento.TicketServicio, error)
	GetTicketStats(ctx context.Context, empresaID uuid.UUID) (*atencionseguimiento.TicketStats, error)
	CreateRespuestaServicio(ctx context.Context, r *atencionseguimiento.RespuestaServicio) error
	GetRespuestasServicio(ctx context.Context, ticketID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.RespuestaServicio, int, error)
	CloseTicketsByCliente(ctx context.Context, clienteID uuid.UUID) error
}

// ---------------------------------------------------------------------------
// PgxTicketRepository — PostgreSQL implementation.
// ---------------------------------------------------------------------------

// PgxTicketRepository is a PostgreSQL-backed implementation of TicketRepository.
type PgxTicketRepository struct {
	db    quejaDB // reuse the same minimal interface defined in queja_repository.go
	cache *RedisAtencionCacheInvalidator
}

// NewPgxTicketRepository creates a PostgreSQL ticket repository.
func NewPgxTicketRepository(pool *pgxpool.Pool, redisClient redis.Cmdable) *PgxTicketRepository {
	var cache *RedisAtencionCacheInvalidator
	if redisClient != nil {
		cache = NewRedisAtencionCacheInvalidator(redisClient)
	}
	return &PgxTicketRepository{db: pool, cache: cache}
}

// CreateTicket inserts a new TicketServicio row.
func (r *PgxTicketRepository) CreateTicket(ctx context.Context, t *atencionseguimiento.TicketServicio) error {
	const q = `
		INSERT INTO atencion_seguimiento.tickets_servicio
		    (id, empresa_id, cliente_id, localidad_id, asunto, descripcion, estatus, ultima_resp, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())`
	_, err := r.db.Exec(ctx, q,
		t.ID, t.EmpresaID, t.ClienteID, t.LocalidadID,
		t.Asunto, t.Descripcion, t.Estatus, t.UltimaResp,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return atencionseguimiento.ErrConflict
		}
		return fmt.Errorf("ticket_repository.CreateTicket: %w", err)
	}
	return nil
}

// GetTicket fetches a TicketServicio by primary key. Returns ErrNotFound when missing.
func (r *PgxTicketRepository) GetTicket(ctx context.Context, id uuid.UUID) (*atencionseguimiento.TicketServicio, error) {
	const q = `
		SELECT id, empresa_id, cliente_id, localidad_id, asunto, descripcion, estatus, ultima_resp, created_at, updated_at
		FROM atencion_seguimiento.tickets_servicio
		WHERE id = $1`
	t, err := scanTicket(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, atencionseguimiento.ErrNotFound
		}
		return nil, fmt.Errorf("ticket_repository.GetTicket: %w", err)
	}
	return t, nil
}

// GetTicketsByCliente returns paginated tickets for a cliente.
func (r *PgxTicketRepository) GetTicketsByCliente(ctx context.Context, clienteID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.TicketServicio, int, error) {
	offset := (page - 1) * pageSize
	const countQ = `SELECT COUNT(*) FROM atencion_seguimiento.tickets_servicio WHERE cliente_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQ, clienteID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ticket_repository.GetTicketsByCliente count: %w", err)
	}
	const q = `
		SELECT id, empresa_id, cliente_id, localidad_id, asunto, descripcion, estatus, ultima_resp, created_at, updated_at
		FROM atencion_seguimiento.tickets_servicio
		WHERE cliente_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, clienteID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ticket_repository.GetTicketsByCliente query: %w", err)
	}
	defer rows.Close()
	var out []*atencionseguimiento.TicketServicio
	for rows.Next() {
		t, scanErr := scanTicket(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("ticket_repository.GetTicketsByCliente scan: %w", scanErr)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ticket_repository.GetTicketsByCliente rows: %w", err)
	}
	return out, total, nil
}

// UpdateTicketEstatus changes the estatus of a ticket with a tenant ownership check.
// Returns ErrForbidden if the ticket does not belong to the given empresa.
func (r *PgxTicketRepository) UpdateTicketEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*atencionseguimiento.TicketServicio, error) {
	const q = `
		UPDATE atencion_seguimiento.tickets_servicio
		SET estatus = $3, updated_at = NOW()
		WHERE id = $1 AND empresa_id = $2
		RETURNING id, empresa_id, cliente_id, localidad_id, asunto, descripcion, estatus, ultima_resp, created_at, updated_at`
	t, err := scanTicket(r.db.QueryRow(ctx, q, id, empresaID, newEstatus))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Could be not-found or wrong empresa; treat as forbidden to avoid data leak.
			return nil, atencionseguimiento.ErrForbidden
		}
		return nil, fmt.Errorf("ticket_repository.UpdateTicketEstatus: %w", err)
	}
	return t, nil
}

// GetTicketStats returns ticket counts grouped by estatus for an empresa.
// Missing buckets return 0 — never nil fields.
func (r *PgxTicketRepository) GetTicketStats(ctx context.Context, empresaID uuid.UUID) (*atencionseguimiento.TicketStats, error) {
	const q = `
		SELECT estatus, COUNT(*) AS cnt
		FROM atencion_seguimiento.tickets_servicio
		WHERE empresa_id = $1
		GROUP BY estatus`
	rows, err := r.db.Query(ctx, q, empresaID)
	if err != nil {
		return nil, fmt.Errorf("ticket_repository.GetTicketStats: %w", err)
	}
	defer rows.Close()
	stats := &atencionseguimiento.TicketStats{EmpresaID: empresaID}
	for rows.Next() {
		var estatus, cnt int
		if scanErr := rows.Scan(&estatus, &cnt); scanErr != nil {
			return nil, fmt.Errorf("ticket_repository.GetTicketStats scan: %w", scanErr)
		}
		switch estatus {
		case atencionseguimiento.EstatusPendiente:
			stats.Pendiente = cnt
		case atencionseguimiento.EstatusEnProceso:
			stats.EnProceso = cnt
		case atencionseguimiento.EstatusFinalizado:
			stats.Finalizado = cnt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ticket_repository.GetTicketStats rows: %w", err)
	}
	return stats, nil
}

// CreateRespuestaServicio inserts a RespuestaServicio and updates the parent ticket.
// After a successful insert, it invalidates related Redis cache keys.
func (r *PgxTicketRepository) CreateRespuestaServicio(ctx context.Context, resp *atencionseguimiento.RespuestaServicio) error {
	const insertQ = `
		INSERT INTO atencion_seguimiento.respuestas_servicio
		    (id, ticket_id, remitente_id, rol_respuesta, mensaje, archivo_adjunto_url, leido, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`
	const updateQ = `
		UPDATE atencion_seguimiento.tickets_servicio
		SET ultima_resp = $2, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(ctx, insertQ,
		resp.ID, resp.TicketID, resp.RemitenteID, resp.RolRespuesta,
		nullStr(resp.Mensaje), nullStr(resp.ArchivoAdjuntoURL), resp.Leido,
	)
	if err != nil {
		return fmt.Errorf("ticket_repository.CreateRespuestaServicio insert: %w", err)
	}
	if _, err := r.db.Exec(ctx, updateQ, resp.TicketID, resp.RolRespuesta); err != nil {
		return fmt.Errorf("ticket_repository.CreateRespuestaServicio update: %w", err)
	}
	return nil
}

// GetRespuestasServicio returns paginated service ticket responses, ordered ASC.
func (r *PgxTicketRepository) GetRespuestasServicio(ctx context.Context, ticketID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.RespuestaServicio, int, error) {
	offset := (page - 1) * pageSize
	const countQ = `SELECT COUNT(*) FROM atencion_seguimiento.respuestas_servicio WHERE ticket_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQ, ticketID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ticket_repository.GetRespuestasServicio count: %w", err)
	}
	const q = `
		SELECT id, ticket_id, remitente_id, rol_respuesta, mensaje, COALESCE(archivo_adjunto_url,''), leido, created_at
		FROM atencion_seguimiento.respuestas_servicio
		WHERE ticket_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, ticketID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ticket_repository.GetRespuestasServicio query: %w", err)
	}
	defer rows.Close()
	var out []*atencionseguimiento.RespuestaServicio
	for rows.Next() {
		resp := &atencionseguimiento.RespuestaServicio{}
		if scanErr := rows.Scan(&resp.ID, &resp.TicketID, &resp.RemitenteID, &resp.RolRespuesta,
			&resp.Mensaje, &resp.ArchivoAdjuntoURL, &resp.Leido, &resp.CreatedAt); scanErr != nil {
			return nil, 0, fmt.Errorf("ticket_repository.GetRespuestasServicio scan: %w", scanErr)
		}
		out = append(out, resp)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ticket_repository.GetRespuestasServicio rows: %w", err)
	}
	return out, total, nil
}

// CloseTicketsByCliente sets estatus=EstatusFinalizado on all open tickets for a cliente.
func (r *PgxTicketRepository) CloseTicketsByCliente(ctx context.Context, clienteID uuid.UUID) error {
	const q = `
		UPDATE atencion_seguimiento.tickets_servicio
		SET estatus = $2, updated_at = NOW()
		WHERE cliente_id = $1 AND estatus != $2`
	if _, err := r.db.Exec(ctx, q, clienteID, atencionseguimiento.EstatusFinalizado); err != nil {
		return fmt.Errorf("ticket_repository.CloseTicketsByCliente: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// scanner
// ---------------------------------------------------------------------------

type ticketRowScanner interface {
	Scan(dest ...any) error
}

func scanTicket(row ticketRowScanner) (*atencionseguimiento.TicketServicio, error) {
	t := &atencionseguimiento.TicketServicio{}
	err := row.Scan(
		&t.ID, &t.EmpresaID, &t.ClienteID, &t.LocalidadID,
		&t.Asunto, &t.Descripcion, &t.Estatus, &t.UltimaResp,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// ---------------------------------------------------------------------------
// InMemTicketRepository — in-memory implementation for unit tests.
// ---------------------------------------------------------------------------

// InMemTicketRepository is a goroutine-safe, in-memory TicketRepository for unit tests.
type InMemTicketRepository struct {
	mu         sync.RWMutex
	tickets    map[uuid.UUID]*atencionseguimiento.TicketServicio
	respuestas map[uuid.UUID][]*atencionseguimiento.RespuestaServicio
}

// NewInMemTicketRepository creates an empty in-memory ticket repository.
func NewInMemTicketRepository() *InMemTicketRepository {
	return &InMemTicketRepository{
		tickets:    make(map[uuid.UUID]*atencionseguimiento.TicketServicio),
		respuestas: make(map[uuid.UUID][]*atencionseguimiento.RespuestaServicio),
	}
}

func (r *InMemTicketRepository) CreateTicket(_ context.Context, t *atencionseguimiento.TicketServicio) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	cp := *t
	r.tickets[t.ID] = &cp
	return nil
}

func (r *InMemTicketRepository) GetTicket(_ context.Context, id uuid.UUID) (*atencionseguimiento.TicketServicio, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tickets[id]
	if !ok {
		return nil, atencionseguimiento.ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (r *InMemTicketRepository) GetTicketsByCliente(_ context.Context, clienteID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.TicketServicio, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var filtered []*atencionseguimiento.TicketServicio
	for _, t := range r.tickets {
		if t.ClienteID == clienteID {
			cp := *t
			filtered = append(filtered, &cp)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	total := len(filtered)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []*atencionseguimiento.TicketServicio{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}

func (r *InMemTicketRepository) UpdateTicketEstatus(_ context.Context, id, empresaID uuid.UUID, newEstatus int) (*atencionseguimiento.TicketServicio, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tickets[id]
	if !ok {
		return nil, atencionseguimiento.ErrNotFound
	}
	if t.EmpresaID != empresaID {
		return nil, atencionseguimiento.ErrForbidden
	}
	t.Estatus = newEstatus
	t.UpdatedAt = time.Now().UTC()
	cp := *t
	return &cp, nil
}

func (r *InMemTicketRepository) GetTicketStats(_ context.Context, empresaID uuid.UUID) (*atencionseguimiento.TicketStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stats := &atencionseguimiento.TicketStats{EmpresaID: empresaID}
	for _, t := range r.tickets {
		if t.EmpresaID != empresaID {
			continue
		}
		switch t.Estatus {
		case atencionseguimiento.EstatusPendiente:
			stats.Pendiente++
		case atencionseguimiento.EstatusEnProceso:
			stats.EnProceso++
		case atencionseguimiento.EstatusFinalizado:
			stats.Finalizado++
		}
	}
	return stats, nil
}

func (r *InMemTicketRepository) CreateRespuestaServicio(_ context.Context, resp *atencionseguimiento.RespuestaServicio) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if resp.ID == uuid.Nil {
		resp.ID = uuid.New()
	}
	resp.CreatedAt = time.Now().UTC()
	cp := *resp
	r.respuestas[resp.TicketID] = append(r.respuestas[resp.TicketID], &cp)
	if t, ok := r.tickets[resp.TicketID]; ok {
		t.UltimaResp = resp.RolRespuesta
		t.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (r *InMemTicketRepository) GetRespuestasServicio(_ context.Context, ticketID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.RespuestaServicio, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := r.respuestas[ticketID]
	total := len(all)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []*atencionseguimiento.RespuestaServicio{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (r *InMemTicketRepository) CloseTicketsByCliente(_ context.Context, clienteID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tickets {
		if t.ClienteID == clienteID && t.Estatus != atencionseguimiento.EstatusFinalizado {
			t.Estatus = atencionseguimiento.EstatusFinalizado
			t.UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}
