package dashboardsdata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultCacheTTL is the default TTL for cached dashboard metrics.
const DefaultCacheTTL = 15 * time.Minute

// CacheClient handles storing and retrieving serialized dashboard payloads from Redis.
type CacheClient struct {
	client *redis.Client
}

// NewCacheClient creates a new dashboard cache client.
func NewCacheClient(client *redis.Client) *CacheClient {
	return &CacheClient{client: client}
}

// FormatKey generates a cache key based on the pattern: dashboard:{role}:{entity_id}:{metric_name}
func (c *CacheClient) FormatKey(role string, entityID string, metricName string) string {
	return fmt.Sprintf("dashboard:%s:%s:%s", role, entityID, metricName)
}

// Get retrieves cached data for the given key and unmarshals it into dest.
// Returns (true, nil) on hit, (false, nil) on miss.
func (c *CacheClient) Get(ctx context.Context, key string, dest any) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("dashboard cache get %s: %w", key, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("dashboard cache unmarshal %s: %w", key, err)
	}
	return true, nil
}

// Set marshals val and stores it in Redis under the given key with default 15m TTL.
func (c *CacheClient) Set(ctx context.Context, key string, val any) error {
	return c.SetWithTTL(ctx, key, val, DefaultCacheTTL)
}

// SetWithTTL marshals val and stores it in Redis under the given key with a custom TTL.
func (c *CacheClient) SetWithTTL(ctx context.Context, key string, val any, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("dashboard cache marshal %s: %w", key, err)
	}
	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("dashboard cache set %s: %w", key, err)
	}
	return nil
}

// Delete removes the cached entry for key.
func (c *CacheClient) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("dashboard cache delete %s: %w", key, err)
	}
	return nil
}
