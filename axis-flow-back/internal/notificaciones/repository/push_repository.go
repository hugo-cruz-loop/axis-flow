// Package repository provides persistence implementations for the notificaciones module.
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
// DB interface — minimal surface used by pg implementations.
// ---------------------------------------------------------------------------

// notiDB is the minimal pgx interface required by both pg repositories.
type notiDB interface {
	Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// pgxPoolAdapter adapts *pgxpool.Pool to notiDB.
type pgxPoolAdapter struct{ pool *pgxpool.Pool }

func (a *pgxPoolAdapter) Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error) {
	return a.pool.Exec(ctx, sql, args...)
}
func (a *pgxPoolAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return a.pool.Query(ctx, sql, args...)
}
func (a *pgxPoolAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return a.pool.QueryRow(ctx, sql, args...)
}

// ---------------------------------------------------------------------------
// Tx interface — minimal surface for pgx.Tx used by InsertPush.
// ---------------------------------------------------------------------------

// notiTx is the minimal interface for a pgx transaction (or nil for no tx).
type notiTx interface {
	Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ---------------------------------------------------------------------------
// PushRepository — interface.
// ---------------------------------------------------------------------------

// PushRepository defines persistence operations for push notification records.
type PushRepository interface {
	// InsertPush records a sent push notification. tx may be nil for non-transactional inserts.
	InsertPush(ctx context.Context, tx pgx.Tx, n notificaciones.NotificacionEnviada) (notificaciones.NotificacionEnviada, error)
	// ListPushByUser returns paginated push records for a user, ordered by created_at DESC.
	ListPushByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionEnviada, int, error)
}

// ---------------------------------------------------------------------------
// pgPushRepository — PostgreSQL implementation.
// ---------------------------------------------------------------------------

// pgPushRepository is a PostgreSQL-backed PushRepository.
type pgPushRepository struct {
	db notiDB
}

// NewPgPushRepository creates a PostgreSQL push repository.
func NewPgPushRepository(pool *pgxpool.Pool) PushRepository {
	return &pgPushRepository{db: &pgxPoolAdapter{pool: pool}}
}

// InsertPush inserts a NotificacionEnviada row and returns the persisted record.
// When tx is non-nil the insert runs inside that transaction.
func (r *pgPushRepository) InsertPush(ctx context.Context, tx pgx.Tx, n notificaciones.NotificacionEnviada) (notificaciones.NotificacionEnviada, error) {
	const q = `
		INSERT INTO notificaciones.notificacionenviada
		    (user_id, token, noti_id, device_type, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, user_id, token, noti_id, device_type, created_at`

	var querier interface {
		QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	}
	if tx != nil {
		querier = tx
	} else {
		querier = r.db
	}

	var out notificaciones.NotificacionEnviada
	err := querier.QueryRow(ctx, q, n.UserID, n.Token, n.NotiID, string(n.DeviceType)).
		Scan(&out.ID, &out.UserID, &out.Token, &out.NotiID, &out.DeviceType, &out.CreatedAt)
	if err != nil {
		return notificaciones.NotificacionEnviada{}, fmt.Errorf("push_repository.InsertPush: %w", err)
	}
	return out, nil
}

// ListPushByUser returns paginated push notification records for a given user.
func (r *pgPushRepository) ListPushByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionEnviada, int, error) {
	offset := (page - 1) * pageSize

	const countQ = `SELECT COUNT(*) FROM notificaciones.notificacionenviada WHERE user_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("push_repository.ListPushByUser count: %w", err)
	}

	const q = `
		SELECT id, user_id, token, noti_id, device_type, created_at
		FROM notificaciones.notificacionenviada
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("push_repository.ListPushByUser query: %w", err)
	}
	defer rows.Close()

	var out []notificaciones.NotificacionEnviada
	for rows.Next() {
		var n notificaciones.NotificacionEnviada
		if scanErr := rows.Scan(&n.ID, &n.UserID, &n.Token, &n.NotiID, &n.DeviceType, &n.CreatedAt); scanErr != nil {
			return nil, 0, fmt.Errorf("push_repository.ListPushByUser scan: %w", scanErr)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("push_repository.ListPushByUser rows: %w", err)
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// InMemPushRepository — goroutine-safe in-memory implementation for tests.
// ---------------------------------------------------------------------------

// InMemPushRepository is a goroutine-safe, in-memory PushRepository for unit tests.
type InMemPushRepository struct {
	mu      sync.RWMutex
	records []notificaciones.NotificacionEnviada
}

// NewInMemPushRepository creates an empty in-memory push repository.
func NewInMemPushRepository() *InMemPushRepository {
	return &InMemPushRepository{}
}

// InsertPush stores a push record and assigns a new UUID and CreatedAt timestamp.
// The tx argument is accepted for interface compatibility but is ignored by this implementation.
func (r *InMemPushRepository) InsertPush(_ context.Context, _ pgx.Tx, n notificaciones.NotificacionEnviada) (notificaciones.NotificacionEnviada, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	cp := n
	r.records = append(r.records, cp)
	return cp, nil
}

// ListPushByUser returns paginated push records for the given userID, ordered by created_at DESC.
func (r *InMemPushRepository) ListPushByUser(_ context.Context, userID uuid.UUID, page, pageSize int) ([]notificaciones.NotificacionEnviada, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []notificaciones.NotificacionEnviada
	for _, rec := range r.records {
		if rec.UserID == userID {
			filtered = append(filtered, rec)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []notificaciones.NotificacionEnviada{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}
