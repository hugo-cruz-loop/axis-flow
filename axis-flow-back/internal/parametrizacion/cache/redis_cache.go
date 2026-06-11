package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements parametrizacion.CachePort using JSON values in Redis.
type RedisCache struct {
	client redis.Cmdable
}

// NewRedisCache creates a Redis-backed cache-aside adapter.
func NewRedisCache(client redis.Cmdable) *RedisCache {
	return &RedisCache{client: client}
}

func (c *RedisCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	raw, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("parametrizacion cache get %s: %w", key, err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, fmt.Errorf("parametrizacion cache unmarshal %s: %w", key, err)
	}
	return true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("parametrizacion cache marshal %s: %w", key, err)
	}
	if err := c.client.Set(ctx, key, body, ttl).Err(); err != nil {
		return fmt.Errorf("parametrizacion cache set %s: %w", key, err)
	}
	return nil
}

func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := c.client.Unlink(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("parametrizacion cache unlink: %w", err)
	}
	return nil
}
