// Package repository_test covers the Redis cache invalidator behaviour.
//
// PR-2 amend (Repositories) — added the cache invalidator alongside the
// Pgx adapters so the cache boundary is testable in isolation. Mirrors the
// miniredis-backed test pattern from
// internal/atencionseguimiento/repository/queja_repository_test.go.
package repository_test

import (
	"context"
	"fmt"
	"testing"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRedis spins up a miniredis instance and returns it alongside a real
// redis.Client pointed at it. Mirrors 09's helper in
// internal/atencionseguimiento/repository/queja_repository_test.go.
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

// ---------------------------------------------------------------------------
// UnlinkFormulariosByEmpresa
// ---------------------------------------------------------------------------

func TestRedisFormulariosCacheInvalidatorUnlinkFormulariosByEmpresa(t *testing.T) {
	mr, client := newTestRedis(t)
	ctx := context.Background()

	empresaID := uuid.New()
	key := fmt.Sprintf(formularios.KeyFormularioByEmpresa, empresaID)
	mr.Set(key, "cached-list")

	invalidator := repository.NewRedisFormulariosCacheInvalidator(client)

	require.NoError(t, invalidator.UnlinkFormulariosByEmpresa(ctx, empresaID))

	assert.False(t, mr.Exists(key), "expected cache key %q to be unlinked", key)
}

// ---------------------------------------------------------------------------
// UnlinkEventosByEmpCte
// ---------------------------------------------------------------------------

func TestRedisFormulariosCacheInvalidatorUnlinkEventosByEmpCte(t *testing.T) {
	mr, client := newTestRedis(t)
	ctx := context.Background()

	empresaID := uuid.New()
	clienteID := uuid.New()
	key := fmt.Sprintf(formularios.KeyEventoByEmpCte, empresaID, clienteID)
	mr.Set(key, "cached-list")

	invalidator := repository.NewRedisFormulariosCacheInvalidator(client)

	require.NoError(t, invalidator.UnlinkEventosByEmpCte(ctx, empresaID, clienteID))

	assert.False(t, mr.Exists(key), "expected cache key %q to be unlinked", key)
}

// ---------------------------------------------------------------------------
// UnlinkRespuestasByIniciado
// ---------------------------------------------------------------------------

func TestRedisFormulariosCacheInvalidatorUnlinkRespuestasByIniciado(t *testing.T) {
	mr, client := newTestRedis(t)
	ctx := context.Background()

	iniciadoID := uuid.New()
	key := fmt.Sprintf(formularios.KeyRespuestasByIniciado, iniciadoID)
	mr.Set(key, "cached-list")

	invalidator := repository.NewRedisFormulariosCacheInvalidator(client)

	require.NoError(t, invalidator.UnlinkRespuestasByIniciado(ctx, iniciadoID))

	assert.False(t, mr.Exists(key), "expected cache key %q to be unlinked", key)
}

// ---------------------------------------------------------------------------
// Triangulate: a pre-set unrelated key must NOT be touched by any Unlink
// method. This guards against over-broad invalidation (e.g. someone
// rewriting the method to "unlink all formularios: keys").
// ---------------------------------------------------------------------------

func TestRedisFormulariosCacheInvalidatorDoesNotTouchUnrelatedKeys(t *testing.T) {
	mr, client := newTestRedis(t)
	ctx := context.Background()

	empresaID := uuid.New()
	clienteID := uuid.New()
	iniciadoID := uuid.New()

	unrelatedKey := fmt.Sprintf(formularios.KeyFormularioByEmpresa, uuid.New()) // different empresa
	mr.Set(unrelatedKey, "should-survive")

	invalidator := repository.NewRedisFormulariosCacheInvalidator(client)

	require.NoError(t, invalidator.UnlinkFormulariosByEmpresa(ctx, empresaID))
	require.NoError(t, invalidator.UnlinkEventosByEmpCte(ctx, empresaID, clienteID))
	require.NoError(t, invalidator.UnlinkRespuestasByIniciado(ctx, iniciadoID))

	assert.True(t, mr.Exists(unrelatedKey), "unrelated key must NOT be unlinked")
}
