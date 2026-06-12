// Package cache provides a Redis-backed implementation of ReportCache.
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	report "axis-flow-back/internal/report"
)

const keyPrefix = "report:request:"

// ReportCache defines the contract for storing and retrieving generated report
// binaries by an opaque key.
type ReportCache interface {
	// Store persists data under key for the given TTL duration.
	Store(ctx context.Context, key string, data []byte, ttl time.Duration) error
	// Fetch retrieves the bytes stored under key.
	// Returns ErrReportNotFound when the key is absent or has expired.
	// The key is NOT deleted after a successful Fetch — callers may retry
	// within the original TTL window.
	Fetch(ctx context.Context, key string) ([]byte, error)
}

type redisReportCache struct {
	rdb redis.UniversalClient
}

// NewRedisReportCache returns a ReportCache backed by the provided Redis client.
func NewRedisReportCache(rdb redis.UniversalClient) ReportCache {
	return &redisReportCache{rdb: rdb}
}

func (c *redisReportCache) Store(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, keyPrefix+key, data, ttl).Err()
}

func (c *redisReportCache) Fetch(ctx context.Context, key string) ([]byte, error) {
	data, err := c.rdb.Get(ctx, keyPrefix+key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, report.ErrReportNotFound
		}
		return nil, err
	}
	return data, nil
}
