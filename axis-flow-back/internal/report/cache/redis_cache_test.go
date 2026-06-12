// Package cache provides a Redis-backed implementation of ReportCache.
package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	report "axis-flow-back/internal/report"
	"axis-flow-back/internal/report/cache"
)

func newTestCache(t *testing.T) (cache.ReportCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return cache.NewRedisReportCache(rdb), mr
}

func TestRedisReportCache(t *testing.T) {
	ctx := context.Background()

	t.Run("Store then Fetch returns same bytes", func(t *testing.T) {
		c, _ := newTestCache(t)
		data := []byte("hello report")

		if err := c.Store(ctx, "key1", data, time.Minute); err != nil {
			t.Fatalf("Store: unexpected error: %v", err)
		}

		got, err := c.Fetch(ctx, "key1")
		if err != nil {
			t.Fatalf("Fetch: unexpected error: %v", err)
		}
		if string(got) != string(data) {
			t.Errorf("Fetch returned %q; want %q", got, data)
		}
	})

	t.Run("Fetch on missing key returns ErrReportNotFound", func(t *testing.T) {
		c, _ := newTestCache(t)

		_, err := c.Fetch(ctx, "nonexistent")
		if err != report.ErrReportNotFound {
			t.Errorf("Fetch: got err %v; want ErrReportNotFound", err)
		}
	})

	t.Run("Fetch after TTL expiry returns ErrReportNotFound", func(t *testing.T) {
		c, mr := newTestCache(t)
		data := []byte("expiring data")

		if err := c.Store(ctx, "key-ttl", data, 5*time.Second); err != nil {
			t.Fatalf("Store: unexpected error: %v", err)
		}

		// Advance miniredis clock past the TTL.
		mr.FastForward(6 * time.Second)

		_, err := c.Fetch(ctx, "key-ttl")
		if err != report.ErrReportNotFound {
			t.Errorf("Fetch after expiry: got err %v; want ErrReportNotFound", err)
		}
	})

	t.Run("Fetch does not delete key — second Fetch still succeeds", func(t *testing.T) {
		c, _ := newTestCache(t)
		data := []byte("persistent data")

		if err := c.Store(ctx, "key-persist", data, time.Minute); err != nil {
			t.Fatalf("Store: unexpected error: %v", err)
		}

		if _, err := c.Fetch(ctx, "key-persist"); err != nil {
			t.Fatalf("First Fetch: unexpected error: %v", err)
		}

		got, err := c.Fetch(ctx, "key-persist")
		if err != nil {
			t.Fatalf("Second Fetch: unexpected error: %v", err)
		}
		if string(got) != string(data) {
			t.Errorf("Second Fetch returned %q; want %q", got, data)
		}
	})
}
