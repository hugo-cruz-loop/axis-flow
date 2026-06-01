// Package repository provides pgx-backed implementations of the service repository interfaces.
package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"axis-flow-back/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxAuditRepository is a PostgreSQL implementation for audit log entries.
type PgxAuditRepository struct {
	pool *pgxpool.Pool
}

// NewPgxAuditRepository creates a new PgxAuditRepository.
func NewPgxAuditRepository(pool *pgxpool.Pool) *PgxAuditRepository {
	return &PgxAuditRepository{pool: pool}
}

// Append inserts an audit entry into identity_audit_log.
func (r *PgxAuditRepository) Append(ctx context.Context, entry domain.AuditEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}

	meta, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("audit_repository.Append: marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO users.identity_audit_log
		    (id, actor_user_id, target_user_id, action, metadata, ip_address, trace_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`

	_, err = r.pool.Exec(ctx, q,
		entry.ID,
		entry.ActorUserID,
		entry.TargetUserID,
		entry.Action,
		meta,
		entry.IPAddress,
		entry.TraceID,
	)
	if err != nil {
		return fmt.Errorf("audit_repository.Append: %w", err)
	}
	return nil
}
