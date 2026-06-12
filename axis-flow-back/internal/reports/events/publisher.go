package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisEventPublisher publishes events to Redis Streams via XADD.
type RedisEventPublisher struct {
	rdb redis.UniversalClient
}

// NewRedisEventPublisher creates a RedisEventPublisher backed by rdb.
func NewRedisEventPublisher(rdb redis.UniversalClient) *RedisEventPublisher {
	return &RedisEventPublisher{rdb: rdb}
}

// Publish serialises payload as JSON and appends it to the named stream.
// It uses the stream name constant — callers should always pass a StreamXxx
// constant, never a bare string.
func (p *RedisEventPublisher) Publish(ctx context.Context, stream string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("events: marshal payload: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"payload": string(data)},
	}
	if err := p.rdb.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("events: XADD %q: %w", stream, err)
	}
	return nil
}
