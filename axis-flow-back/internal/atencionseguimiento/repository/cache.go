// Package repository provides pgx-backed and in-memory implementations of the
// atencionseguimiento repository interfaces, plus a Redis cache invalidator.
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Redis cache key patterns (using prefix atencion_seguimiento:).
// ---------------------------------------------------------------------------

const (
	keyEmpleadoUnreadComplaints = "atencion_seguimiento:empleado:%s:unread_complaints"
	keyEmpresaUnreadComplaints  = "atencion_seguimiento:empresa:%s:unread_complaints"
	keyClienteUnreadTickets     = "atencion_seguimiento:cliente:%s:unread_tickets"
	keyEmpresaUnreadTickets     = "atencion_seguimiento:empresa:%s:unread_tickets"
)

// RedisAtencionCacheInvalidator handles async cache invalidation via UNLINK.
type RedisAtencionCacheInvalidator struct {
	client redis.Cmdable
}

// NewRedisAtencionCacheInvalidator creates a Redis-backed cache invalidator.
func NewRedisAtencionCacheInvalidator(client redis.Cmdable) *RedisAtencionCacheInvalidator {
	return &RedisAtencionCacheInvalidator{client: client}
}

// UnlinkQuejaRespuestaKeys removes unread-complaint cache keys for empleado and empresa.
// Uses UNLINK (async) to avoid blocking the caller.
func (c *RedisAtencionCacheInvalidator) UnlinkQuejaRespuestaKeys(ctx context.Context, empleadoID, empresaID uuid.UUID) error {
	keys := []string{
		fmt.Sprintf(keyEmpleadoUnreadComplaints, empleadoID),
		fmt.Sprintf(keyEmpresaUnreadComplaints, empresaID),
	}
	if err := c.client.Unlink(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache.UnlinkQuejaRespuestaKeys: %w", err)
	}
	return nil
}

// UnlinkTicketRespuestaKeys removes unread-ticket cache keys for cliente and empresa.
// Uses UNLINK (async) to avoid blocking the caller.
func (c *RedisAtencionCacheInvalidator) UnlinkTicketRespuestaKeys(ctx context.Context, clienteID, empresaID uuid.UUID) error {
	keys := []string{
		fmt.Sprintf(keyClienteUnreadTickets, clienteID),
		fmt.Sprintf(keyEmpresaUnreadTickets, empresaID),
	}
	if err := c.client.Unlink(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache.UnlinkTicketRespuestaKeys: %w", err)
	}
	return nil
}
