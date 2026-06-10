package events

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisStreamPublisher implements EventPublisher using Redis Streams (XADD).
type RedisStreamPublisher struct {
	rdb *redis.Client
}

// NewRedisStreamPublisher constructs a RedisStreamPublisher.
func NewRedisStreamPublisher(rdb *redis.Client) *RedisStreamPublisher {
	return &RedisStreamPublisher{rdb: rdb}
}

// Publish sends the payload as an XADD to the given stream.
// Payload values are serialised as string key-value pairs.
func (p *RedisStreamPublisher) Publish(ctx context.Context, stream string, payload map[string]any) error {
	values := make(map[string]any, len(payload))
	for k, v := range payload {
		values[k] = fmt.Sprintf("%v", v)
	}
	args := &redis.XAddArgs{
		Stream: stream,
		ID:     "*",
		Values: values,
	}
	return p.rdb.XAdd(ctx, args).Err()
}

// Compile-time check.
var _ EventPublisher = (*RedisStreamPublisher)(nil)
