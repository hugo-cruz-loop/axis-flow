package cache_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"axis-flow-back/internal/clientes/cache"
)

func newTestClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestClienteCacheKey(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	key := cache.ClienteCacheKey(42, id)
	assert.Equal(t, "clientes:42:00000000-0000-0000-0000-000000000001", key)
}

func TestLocalidadCacheKey(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	key := cache.LocalidadCacheKey(id)
	assert.Equal(t, "clientes:localidad:00000000-0000-0000-0000-000000000002", key)
}

func TestActivationFlagKey(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	key := cache.ActivationFlagKey(id)
	assert.Equal(t, "clientes:activation:00000000-0000-0000-0000-000000000003", key)
}

func TestInvalidateCliente(t *testing.T) {
	mr, rdb := newTestClient(t)
	ctx := context.Background()

	empresaID := int64(7)
	clienteID := uuid.New()
	key := cache.ClienteCacheKey(empresaID, clienteID)

	// Pre-set the key.
	require.NoError(t, rdb.Set(ctx, key, "data", 0).Err())
	mr.CheckGet(t, key, "data")

	err := cache.InvalidateCliente(ctx, rdb, empresaID, clienteID)
	require.NoError(t, err)

	// Key must be gone after invalidation — Get returns error when key is missing.
	_, getErr := mr.Get(key)
	assert.Error(t, getErr, "key should be removed after invalidation")
}

func TestInvalidateLocalidad(t *testing.T) {
	mr, rdb := newTestClient(t)
	ctx := context.Background()

	localidadID := uuid.New()
	key := cache.LocalidadCacheKey(localidadID)

	require.NoError(t, rdb.Set(ctx, key, "localidad-data", 0).Err())
	mr.CheckGet(t, key, "localidad-data")

	err := cache.InvalidateLocalidad(ctx, rdb, localidadID)
	require.NoError(t, err)

	_, getErr := mr.Get(key)
	assert.Error(t, getErr, "localidad key should be removed after invalidation")
}
