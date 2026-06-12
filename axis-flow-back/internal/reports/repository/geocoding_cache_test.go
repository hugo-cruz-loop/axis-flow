package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/reports/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestGeocodingCacheGetMiss(t *testing.T) {
	_, client := newTestRedis(t)
	c := repository.NewRedisGeocodingCache(client)
	ctx := context.Background()

	dir, found := c.Get(ctx, 19.0, -99.0)
	assert.False(t, found)
	assert.Empty(t, dir)
}

func TestGeocodingCacheSetThenGet(t *testing.T) {
	_, client := newTestRedis(t)
	c := repository.NewRedisGeocodingCache(client)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, 19.432608, -99.133209, "Zócalo, CDMX", time.Hour))

	dir, found := c.Get(ctx, 19.432608, -99.133209)
	assert.True(t, found)
	assert.Equal(t, "Zócalo, CDMX", dir)
}

func TestGeocodingCacheKeyFormat(t *testing.T) {
	mr, client := newTestRedis(t)
	c := repository.NewRedisGeocodingCache(client)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, 19.432608, -99.133209, "addr", time.Hour))

	// Verify the key follows the expected format.
	keys := mr.Keys()
	require.Len(t, keys, 1)
	assert.Equal(t, "reports:geocoding:19.432608,-99.133209", keys[0])
}

func TestGeocodingCacheDeleteByPatternUsesUnlink(t *testing.T) {
	_, client := newTestRedis(t)
	c := repository.NewRedisGeocodingCache(client)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, 1.0, 2.0, "addr1", time.Hour))
	require.NoError(t, c.Set(ctx, 3.0, 4.0, "addr2", time.Hour))

	require.NoError(t, c.DeleteByPattern(ctx, "reports:geocoding:*"))

	// After deletion both keys must be gone.
	_, found1 := c.Get(ctx, 1.0, 2.0)
	_, found2 := c.Get(ctx, 3.0, 4.0)
	assert.False(t, found1)
	assert.False(t, found2)
}
