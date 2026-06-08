// Package cache provides Redis cache-aside helpers for the Catalogos module.
// TTL default: 86400 seconds (24h) per spec.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultTTL is the cache TTL for catalog entries (24 hours).
const DefaultTTL = 86400 * time.Second

// Get retrieves a JSON-encoded value from Redis.
// Returns (zero, false, nil) on cache miss; fails open on Redis error.
func Get[T any](ctx context.Context, rdb *redis.Client, key string) (T, bool, error) {
	var zero T
	data, err := rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, fmt.Errorf("catalogos/cache.Get %s: %w", key, err)
	}
	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return zero, false, fmt.Errorf("catalogos/cache.Get unmarshal %s: %w", key, err)
	}
	return val, true, nil
}

// Set stores a JSON-encoded value in Redis with the given TTL.
func Set(ctx context.Context, rdb *redis.Client, key string, val any, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("catalogos/cache.Set marshal %s: %w", key, err)
	}
	if err := rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("catalogos/cache.Set %s: %w", key, err)
	}
	return nil
}

// Del removes one or more keys from Redis. Fails open — callers should log the error.
func Del(ctx context.Context, rdb *redis.Client, keys ...string) error {
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("catalogos/cache.Del: %w", err)
	}
	return nil
}
