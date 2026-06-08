package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Aside executes a cache-aside read: tries Redis first, on miss calls fetch, stores result.
// Fails open on Redis errors — always returns the DB result.
func Aside[T any](
	ctx context.Context,
	rdb *redis.Client,
	key string,
	ttl time.Duration,
	fetch func(context.Context) (T, error),
) (T, error) {
	if rdb != nil {
		if val, ok, err := Get[T](ctx, rdb, key); ok {
			return val, nil
		} else if err != nil {
			slog.WarnContext(ctx, "catalogos cache get error (fail-open)",
				slog.String("key", key), slog.String("error", err.Error()))
		}
	}
	result, err := fetch(ctx)
	if err != nil {
		return result, err
	}
	if rdb != nil {
		if err := Set(ctx, rdb, key, result, ttl); err != nil {
			slog.WarnContext(ctx, "catalogos cache set error (fail-open)",
				slog.String("key", key), slog.String("error", err.Error()))
		}
	}
	return result, nil
}
