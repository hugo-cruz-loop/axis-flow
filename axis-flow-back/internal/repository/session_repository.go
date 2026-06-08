package repository

import (
	"context"
	"fmt"

	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxSessionRepository is a PostgreSQL implementation for Session persistence.
type PgxSessionRepository struct {
	pool *pgxpool.Pool
}

// NewPgxSessionRepository creates a new PgxSessionRepository.
func NewPgxSessionRepository(pool *pgxpool.Pool) *PgxSessionRepository {
	return &PgxSessionRepository{pool: pool}
}

// Create persists a new session record.
func (r *PgxSessionRepository) Create(ctx context.Context, s *domain.Session) error {
	const q = `
		INSERT INTO users.identity_sessions
		    (id, user_id, refresh_token_hash, device_id, device_type, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.pool.Exec(ctx, q,
		s.ID, s.UserID, s.RefreshTokenHash, s.DeviceID,
		string(s.DeviceType), s.IPAddress, s.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("session_repository.Create: %w", err)
	}
	return nil
}

// FindByRefreshTokenHash returns the session matching the given hash, or an error.
func (r *PgxSessionRepository) FindByRefreshTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	const q = `
		SELECT id, user_id, refresh_token_hash, COALESCE(device_id,''), device_type,
		       COALESCE(CAST(ip_address AS TEXT),''), revoked_at, expires_at, last_used_at
		FROM users.identity_sessions
		WHERE refresh_token_hash = $1
		LIMIT 1`

	sess := &domain.Session{}
	err := r.pool.QueryRow(ctx, q, hash).Scan(
		&sess.ID, &sess.UserID, &sess.RefreshTokenHash, &sess.DeviceID,
		&sess.DeviceType, &sess.IPAddress, &sess.RevokedAt, &sess.ExpiresAt, &sess.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("session_repository.FindByRefreshTokenHash: %w", err)
	}
	return sess, nil
}

// Revoke marks a session as revoked by setting revoked_at to now.
func (r *PgxSessionRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users.identity_sessions SET revoked_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("session_repository.Revoke: %w", err)
	}
	return nil
}
