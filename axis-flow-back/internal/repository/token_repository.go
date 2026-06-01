package repository

import (
	"context"
	"fmt"
	"time"

	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxTokenRepository is a PostgreSQL implementation for one-use Token persistence.
type PgxTokenRepository struct {
	pool *pgxpool.Pool
}

// NewPgxTokenRepository creates a new PgxTokenRepository.
func NewPgxTokenRepository(pool *pgxpool.Pool) *PgxTokenRepository {
	return &PgxTokenRepository{pool: pool}
}

// Create persists a new one-use token.
func (r *PgxTokenRepository) Create(ctx context.Context, t *domain.Token) error {
	const q = `
		INSERT INTO users.identity_tokens (id, user_id, token_hash, type, expires_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.pool.Exec(ctx, q, t.ID, t.UserID, t.TokenHash, string(t.Type), t.ExpiresAt)
	if err != nil {
		return fmt.Errorf("token_repository.Create: %w", err)
	}
	return nil
}

// FindByHash returns the token record matching the given hash, or an error.
func (r *PgxTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.Token, error) {
	const q = `
		SELECT id, user_id, token_hash, type, expires_at, used_at, revoked_at
		FROM users.identity_tokens
		WHERE token_hash = $1
		LIMIT 1`

	t := &domain.Token{}
	err := r.pool.QueryRow(ctx, q, hash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.Type, &t.ExpiresAt, &t.UsedAt, &t.RevokedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("token_repository.FindByHash: %w", err)
	}
	return t, nil
}

// MarkUsed sets the used_at timestamp on the token.
func (r *PgxTokenRepository) MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	const q = `UPDATE users.identity_tokens SET used_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, usedAt, id)
	if err != nil {
		return fmt.Errorf("token_repository.MarkUsed: %w", err)
	}
	return nil
}
