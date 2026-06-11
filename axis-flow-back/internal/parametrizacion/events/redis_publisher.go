package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"axis-flow-back/internal/parametrizacion"

	"github.com/redis/go-redis/v9"
)

const (
	defaultStreamKey        = "parametrizacion:events"
	defaultStreamMaxLen     = int64(10000)
	defaultOperationTimeout = 2 * time.Second
)

// RedisPublisherConfig tunes the Redis Streams publisher.
type RedisPublisherConfig struct {
	StreamKey        string
	MaxLen           int64
	OperationTimeout time.Duration
}

// RedisPublisher publishes parametrizacion domain events to Redis Streams.
type RedisPublisher struct {
	client redis.Cmdable
	cfg    RedisPublisherConfig
}

// NewRedisPublisher creates a Redis Streams event publisher.
func NewRedisPublisher(client redis.Cmdable, cfg RedisPublisherConfig) *RedisPublisher {
	if cfg.StreamKey == "" {
		cfg.StreamKey = defaultStreamKey
	}
	if cfg.MaxLen <= 0 {
		cfg.MaxLen = defaultStreamMaxLen
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = defaultOperationTimeout
	}
	return &RedisPublisher{client: client, cfg: cfg}
}

func (p *RedisPublisher) Publish(ctx context.Context, event parametrizacion.DomainEvent) error {
	opCtx, cancel := context.WithTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()
	body, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("parametrizacion publisher marshal %s: %w", event.Name, err)
	}
	publishedAt := event.OccurredAt
	if publishedAt.IsZero() {
		publishedAt = time.Now().UTC()
	}
	args := &redis.XAddArgs{
		Stream: p.cfg.StreamKey,
		MaxLen: p.cfg.MaxLen,
		Approx: true,
		Values: map[string]any{
			"event":        event.Name,
			"published_at": publishedAt.Format(time.RFC3339Nano),
			"payload":      string(body),
		},
	}
	if err := p.client.XAdd(opCtx, args).Err(); err != nil {
		return fmt.Errorf("parametrizacion publisher xadd %s: %w", event.Name, err)
	}
	return nil
}
