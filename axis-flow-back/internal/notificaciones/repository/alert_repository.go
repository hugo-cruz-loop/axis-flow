package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"axis-flow-back/internal/notificaciones"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// AlertRepository — interface.
// ---------------------------------------------------------------------------

// AlertRepository defines persistence operations for in-app atencion notifications.
type AlertRepository interface {
	// InsertAlert records a new notification and returns the persisted row.
	InsertAlert(ctx context.Context, n notificaciones.NotificacionAtencion) (notificaciones.NotificacionAtencion, error)
	// GetByID returns a single notification by its ID. Returns ErrNotFound when absent.
	GetByID(ctx context.Context, id uuid.UUID) (notificaciones.NotificacionAtencion, error)
	// UpdateEstatus changes the read status of a notification by ID.
	UpdateEstatus(ctx context.Context, id uuid.UUID, estatus int) (notificaciones.NotificacionAtencion, error)
	// CountUnread returns the number of unread notifications for a user.
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
	// ListByUser returns paginated notifications for a user, ordered by created_at DESC.
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionAtencion, int, error)
}

// ---------------------------------------------------------------------------
// pgAlertRepository — PostgreSQL implementation.
// ---------------------------------------------------------------------------

// pgAlertRepository is a PostgreSQL-backed AlertRepository.
type pgAlertRepository struct {
	db notiDB
}

// NewPgAlertRepository creates a PostgreSQL alert repository.
func NewPgAlertRepository(pool *pgxpool.Pool) AlertRepository {
	return &pgAlertRepository{db: &pgxPoolAdapter{pool: pool}}
}

// InsertAlert inserts a NotificacionAtencion row and returns the persisted record.
func (r *pgAlertRepository) InsertAlert(ctx context.Context, n notificaciones.NotificacionAtencion) (notificaciones.NotificacionAtencion, error) {
	const q = `
		INSERT INTO notificaciones.notificacionatencionserviciocliente
		    (user_id, ticket_id, queja_id, mensaje, estatus, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, user_id, ticket_id, queja_id, mensaje, estatus, created_at, updated_at`

	var out notificaciones.NotificacionAtencion
	err := r.db.QueryRow(ctx, q, n.UserID, n.TicketID, n.QuejaID, n.Mensaje, int(n.Estatus)).
		Scan(&out.ID, &out.UserID, &out.TicketID, &out.QuejaID, &out.Mensaje, &out.Estatus, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return notificaciones.NotificacionAtencion{}, fmt.Errorf("alert_repository.InsertAlert: %w", err)
	}
	return out, nil
}

// GetByID returns a single notification by its primary key.
// Returns ErrNotFound when the record does not exist.
func (r *pgAlertRepository) GetByID(ctx context.Context, id uuid.UUID) (notificaciones.NotificacionAtencion, error) {
	const q = `
		SELECT id, user_id, ticket_id, queja_id, mensaje, estatus, created_at, updated_at
		FROM notificaciones.notificacionatencionserviciocliente
		WHERE id = $1`

	var out notificaciones.NotificacionAtencion
	err := r.db.QueryRow(ctx, q, id).
		Scan(&out.ID, &out.UserID, &out.TicketID, &out.QuejaID, &out.Mensaje, &out.Estatus, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return notificaciones.NotificacionAtencion{}, notificaciones.ErrNotFound
		}
		return notificaciones.NotificacionAtencion{}, fmt.Errorf("alert_repository.GetByID: %w", err)
	}
	return out, nil
}

// UpdateEstatus sets a new estatus on the given notification and returns the updated row.
// Returns ErrNotFound when the record does not exist.
func (r *pgAlertRepository) UpdateEstatus(ctx context.Context, id uuid.UUID, estatus int) (notificaciones.NotificacionAtencion, error) {
	const q = `
		UPDATE notificaciones.notificacionatencionserviciocliente
		SET estatus = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, ticket_id, queja_id, mensaje, estatus, created_at, updated_at`

	var out notificaciones.NotificacionAtencion
	err := r.db.QueryRow(ctx, q, id, estatus).
		Scan(&out.ID, &out.UserID, &out.TicketID, &out.QuejaID, &out.Mensaje, &out.Estatus, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return notificaciones.NotificacionAtencion{}, notificaciones.ErrNotFound
		}
		return notificaciones.NotificacionAtencion{}, fmt.Errorf("alert_repository.UpdateEstatus: %w", err)
	}
	return out, nil
}

// CountUnread returns the count of unread (estatus=1) notifications for a user.
func (r *pgAlertRepository) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM notificaciones.notificacionatencionserviciocliente
		WHERE user_id = $1 AND estatus = $2`

	var count int
	if err := r.db.QueryRow(ctx, q, userID, int(notificaciones.EstatusUnread)).Scan(&count); err != nil {
		return 0, fmt.Errorf("alert_repository.CountUnread: %w", err)
	}
	return count, nil
}

// ListByUser returns paginated atencion notifications for a user, ordered by created_at DESC.
func (r *pgAlertRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionAtencion, int, error) {
	offset := (page - 1) * pageSize

	const countQ = `
		SELECT COUNT(*)
		FROM notificaciones.notificacionatencionserviciocliente
		WHERE user_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("alert_repository.ListByUser count: %w", err)
	}

	const q = `
		SELECT id, user_id, ticket_id, queja_id, mensaje, estatus, created_at, updated_at
		FROM notificaciones.notificacionatencionserviciocliente
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("alert_repository.ListByUser query: %w", err)
	}
	defer rows.Close()

	var out []notificaciones.NotificacionAtencion
	for rows.Next() {
		var n notificaciones.NotificacionAtencion
		if scanErr := rows.Scan(&n.ID, &n.UserID, &n.TicketID, &n.QuejaID, &n.Mensaje, &n.Estatus, &n.CreatedAt, &n.UpdatedAt); scanErr != nil {
			return nil, 0, fmt.Errorf("alert_repository.ListByUser scan: %w", scanErr)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("alert_repository.ListByUser rows: %w", err)
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// InMemAlertRepository — goroutine-safe in-memory implementation for tests.
// ---------------------------------------------------------------------------

// InMemAlertRepository is a goroutine-safe, in-memory AlertRepository for unit tests.
type InMemAlertRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]notificaciones.NotificacionAtencion
}

// NewInMemAlertRepository creates an empty in-memory alert repository.
func NewInMemAlertRepository() *InMemAlertRepository {
	return &InMemAlertRepository{records: make(map[uuid.UUID]notificaciones.NotificacionAtencion)}
}

// InsertAlert stores an alert and assigns a new UUID and timestamps.
func (r *InMemAlertRepository) InsertAlert(_ context.Context, n notificaciones.NotificacionAtencion) (notificaciones.NotificacionAtencion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	now := time.Now().UTC()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	r.records[n.ID] = n
	return n, nil
}

// GetByID returns a single notification by ID or ErrNotFound.
func (r *InMemAlertRepository) GetByID(_ context.Context, id uuid.UUID) (notificaciones.NotificacionAtencion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.records[id]
	if !ok {
		return notificaciones.NotificacionAtencion{}, notificaciones.ErrNotFound
	}
	return n, nil
}

// UpdateEstatus changes the estatus on the given record.
// Returns ErrNotFound when the ID is unknown.
func (r *InMemAlertRepository) UpdateEstatus(_ context.Context, id uuid.UUID, estatus int) (notificaciones.NotificacionAtencion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.records[id]
	if !ok {
		return notificaciones.NotificacionAtencion{}, notificaciones.ErrNotFound
	}
	n.Estatus = notificaciones.EstatusNotif(estatus)
	n.UpdatedAt = time.Now().UTC()
	r.records[id] = n
	return n, nil
}

// CountUnread returns the number of unread notifications for a user.
func (r *InMemAlertRepository) CountUnread(_ context.Context, userID uuid.UUID) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, n := range r.records {
		if n.UserID == userID && n.Estatus == notificaciones.EstatusUnread {
			count++
		}
	}
	return count, nil
}

// ListByUser returns paginated notifications for the given userID, ordered by created_at DESC.
func (r *InMemAlertRepository) ListByUser(_ context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionAtencion, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []notificaciones.NotificacionAtencion
	for _, n := range r.records {
		if n.UserID == userID {
			filtered = append(filtered, n)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []notificaciones.NotificacionAtencion{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}
