package cache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/catalogos/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func TestAside_OnCacheHit_DoesNotCallFetch(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	const key = "test:aside:hit"

	// Pre-populate cache
	require.NoError(t, cache.Set(ctx, rdb, key, []string{"cached"}, time.Minute))

	calls := 0
	result, err := cache.Aside(ctx, rdb, key, time.Minute, func(ctx context.Context) ([]string, error) {
		calls++
		return []string{"from-db"}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"cached"}, result)
	assert.Equal(t, 0, calls, "fetch should not be called on cache hit")
}

func TestAside_OnCacheMiss_CallsFetchAndStores(t *testing.T) {
	_, rdb := newTestRedis(t)

	ctx := context.Background()
	const key = "test:aside:miss"

	calls := 0
	result, err := cache.Aside(ctx, rdb, key, time.Minute, func(ctx context.Context) ([]string, error) {
		calls++
		return []string{"from-db"}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"from-db"}, result)
	assert.Equal(t, 1, calls, "fetch must be called on cache miss")

	// Verify it was stored in cache
	cached, ok, err := cache.Get[[]string](ctx, rdb, key)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"from-db"}, cached)
}

func TestAside_OnRedisError_FailsOpenAndCallsFetch(t *testing.T) {
	mr, rdb := newTestRedis(t)

	ctx := context.Background()
	const key = "test:aside:error"

	// Close miniredis to cause connection errors
	mr.Close()

	calls := 0
	result, err := cache.Aside(ctx, rdb, key, time.Minute, func(ctx context.Context) ([]string, error) {
		calls++
		return []string{"from-db"}, nil
	})

	require.NoError(t, err, "Aside must fail open — Redis error should not propagate")
	assert.Equal(t, []string{"from-db"}, result)
	assert.Equal(t, 1, calls, "fetch must be called when Redis is unavailable")
}

func TestAside_WhenFetchFails_ReturnsError(t *testing.T) {
	_, rdb := newTestRedis(t)

	ctx := context.Background()
	const key = "test:aside:fetcherr"

	fetchErr := errors.New("db connection failed")
	_, err := cache.Aside(ctx, rdb, key, time.Minute, func(ctx context.Context) ([]string, error) {
		return nil, fetchErr
	})

	assert.ErrorIs(t, err, fetchErr)
}
