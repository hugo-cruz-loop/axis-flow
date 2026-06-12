// Package repository provides PostgreSQL, Redis, and in-memory implementations
// of the reports repository interfaces.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// GeocodingCache interface
// ---------------------------------------------------------------------------

// GeocodingCache is the Redis-backed cache for reverse-geocoding results.
// Keys follow the pattern: reports:geocoding:{lat},{lng} (6 decimal places).
type GeocodingCache interface {
	// Get returns the cached direccion and true if found; ("", false) on miss.
	Get(ctx context.Context, lat, lng float64) (string, bool)
	// Set stores the direccion under the coordinate key with the given TTL.
	Set(ctx context.Context, lat, lng float64, direccion string, ttl time.Duration) error
	// DeleteByPattern removes all keys matching pattern using Redis SCAN + UNLINK
	// (async, non-blocking). Use "reports:geocoding:*" to evict the entire cache.
	DeleteByPattern(ctx context.Context, pattern string) error
}

// ---------------------------------------------------------------------------
// redisGeocodingCache — Redis implementation
// ---------------------------------------------------------------------------

type redisGeocodingCache struct {
	client redis.UniversalClient
}

// NewRedisGeocodingCache creates a Redis-backed GeocodingCache.
func NewRedisGeocodingCache(client redis.UniversalClient) GeocodingCache {
	return &redisGeocodingCache{client: client}
}

func geoKey(lat, lng float64) string {
	return fmt.Sprintf("reports:geocoding:%.6f,%.6f", lat, lng)
}

func (c *redisGeocodingCache) Get(ctx context.Context, lat, lng float64) (string, bool) {
	val, err := c.client.Get(ctx, geoKey(lat, lng)).Result()
	if err != nil {
		// redis.Nil means key not found; any other error is treated as a miss.
		return "", false
	}
	return val, true
}

func (c *redisGeocodingCache) Set(ctx context.Context, lat, lng float64, direccion string, ttl time.Duration) error {
	if err := c.client.Set(ctx, geoKey(lat, lng), direccion, ttl).Err(); err != nil {
		return fmt.Errorf("geocoding_cache.Set: %w", err)
	}
	return nil
}

// DeleteByPattern scans Redis for keys matching pattern and removes them with
// UNLINK (async) so eviction never blocks the caller.
func (c *redisGeocodingCache) DeleteByPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("geocoding_cache.DeleteByPattern scan: %w", err)
		}
		if len(keys) > 0 {
			if err := c.client.Unlink(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("geocoding_cache.DeleteByPattern unlink: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}
