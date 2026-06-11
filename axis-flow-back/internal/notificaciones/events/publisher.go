package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// RedisEventPublisher implements EventPublisher using Redis Streams (XADD).
// Stream name constants from types.go MUST be used at all call sites; bare
// string literals are forbidden (they silently route to wrong streams).
type RedisEventPublisher struct {
	rdb redis.UniversalClient
}

// NewRedisEventPublisher constructs a RedisEventPublisher.
func NewRedisEventPublisher(rdb redis.UniversalClient) *RedisEventPublisher {
	return &RedisEventPublisher{rdb: rdb}
}

// Publish JSON-marshals payload and appends it to the given stream via XADD.
// Stream must be one of the StreamXxx constants defined in types.go.
func (p *RedisEventPublisher) Publish(ctx context.Context, stream string, payload []byte) error {
	if len(payload) == 0 {
		var err error
		payload, err = json.Marshal(struct{}{})
		if err != nil {
			return fmt.Errorf("events.RedisEventPublisher.Publish: marshal empty payload: %w", err)
		}
	}

	args := &redis.XAddArgs{
		Stream: stream,
		ID:     "*",
		Values: map[string]any{"data": string(payload)},
	}
	if err := p.rdb.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("events.RedisEventPublisher.Publish stream=%s: %w", stream, err)
	}

	slog.Info("event published", "stream", stream)
	return nil
}

// Compile-time interface check.
var _ EventPublisher = (*RedisEventPublisher)(nil)
