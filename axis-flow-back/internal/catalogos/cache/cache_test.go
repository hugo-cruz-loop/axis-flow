package cache_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/catalogos/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return client, mr
}

func TestGet_OnCacheMiss_ReturnsFalse(t *testing.T) {
	rdb, _ := newTestClient(t)
	val, found, err := cache.Get[string](context.Background(), rdb, "nonexistent-key")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, val)
}

func TestSet_ThenGet_ReturnsStoredValue(t *testing.T) {
	rdb, _ := newTestClient(t)
	ctx := context.Background()
	err := cache.Set(ctx, rdb, "k", "hello-catalogos", time.Minute)
	require.NoError(t, err)
	val, found, err := cache.Get[string](ctx, rdb, "k")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "hello-catalogos", val)
}

func TestDel_RemovesKey(t *testing.T) {
	rdb, _ := newTestClient(t)
	ctx := context.Background()
	_ = cache.Set(ctx, rdb, "del-key", "v", time.Minute)
	require.NoError(t, cache.Del(ctx, rdb, "del-key"))
	_, found, err := cache.Get[string](ctx, rdb, "del-key")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestSet_ThenGet_StructRoundTrips(t *testing.T) {
	type item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	rdb, _ := newTestClient(t)
	ctx := context.Background()
	expected := item{ID: 42, Name: "Mexico"}
	require.NoError(t, cache.Set(ctx, rdb, "struct-key", expected, time.Minute))
	val, found, err := cache.Get[item](ctx, rdb, "struct-key")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, expected, val)
}
