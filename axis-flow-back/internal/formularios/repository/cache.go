// Package repository — cache.go: Redis cache invalidator for formularios
// cached lists.
//
// The 3 UnlinkXxxKeys methods below are the only public surface of this
// file. They are called by the Pgx repository adapters (and ultimately the
// service layer in PR-3) whenever the underlying table changes. The
// invalidator uses UNLINK (async) so the caller's transaction is not
// blocked by Redis latency.
//
// Cache key patterns are defined in internal/formularios/domain.go:
//   - formularios.KeyFormularioByEmpresa
//   - formularios.KeyEventoByEmpCte
//   - formularios.KeyRespuestasByIniciado
//
// Mirrors the structure of internal/atencionseguimiento/repository/cache.go
// (PR-2 of 09_AtencionSeguimiento_Service_Spec): one invalidator type with
// one Unlink method per logical invalidation boundary, and a `redis.Cmdable`
// (not `*redis.Client`) so the constructor accepts both a real client and a
// miniredis-backed test client.

package repository

import (
	"context"
	"fmt"

	"axis-flow-back/internal/formularios"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// RedisFormulariosCacheInvalidator
// ---------------------------------------------------------------------------

// RedisFormulariosCacheInvalidator handles async cache invalidation via
// UNLINK. Constructed with a redis.Cmdable so tests can inject a
// miniredis-backed client.
type RedisFormulariosCacheInvalidator struct {
	client redis.Cmdable
}

// NewRedisFormulariosCacheInvalidator creates a Redis-backed cache
// invalidator for the formularios service.
func NewRedisFormulariosCacheInvalidator(client redis.Cmdable) *RedisFormulariosCacheInvalidator {
	return &RedisFormulariosCacheInvalidator{client: client}
}

// ---------------------------------------------------------------------------
// Unlink methods — one per logical invalidation boundary.
// ---------------------------------------------------------------------------

// UnlinkFormulariosByEmpresa removes the cached list of formularios for an
// empresa. Uses UNLINK (async) to avoid blocking the caller.
func (c *RedisFormulariosCacheInvalidator) UnlinkFormulariosByEmpresa(ctx context.Context, empresaID uuid.UUID) error {
	key := fmt.Sprintf(formularios.KeyFormularioByEmpresa, empresaID)
	if err := c.client.Unlink(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache.UnlinkFormulariosByEmpresa: %w", err)
	}
	return nil
}

// UnlinkEventosByEmpCte removes the cached list of eventos for an
// (empresa, cliente) pair. Uses UNLINK (async) to avoid blocking the caller.
func (c *RedisFormulariosCacheInvalidator) UnlinkEventosByEmpCte(ctx context.Context, empresaID, clienteID uuid.UUID) error {
	key := fmt.Sprintf(formularios.KeyEventoByEmpCte, empresaID, clienteID)
	if err := c.client.Unlink(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache.UnlinkEventosByEmpCte: %w", err)
	}
	return nil
}

// UnlinkRespuestasByIniciado removes the cached list of respuestas for an
// iniciado (employee check-in). Uses UNLINK (async) to avoid blocking the
// caller.
func (c *RedisFormulariosCacheInvalidator) UnlinkRespuestasByIniciado(ctx context.Context, iniciadoID uuid.UUID) error {
	key := fmt.Sprintf(formularios.KeyRespuestasByIniciado, iniciadoID)
	if err := c.client.Unlink(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache.UnlinkRespuestasByIniciado: %w", err)
	}
	return nil
}
