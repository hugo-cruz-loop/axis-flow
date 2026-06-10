// Package events contains real transport implementations for Asignacion
// domain events. The current slice ships a Redis Streams publisher; future
// slices may add Kafka, NATS, or webhook transports.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"axis-flow-back/internal/asignacion"
)

const (
	defaultStreamKey        = "asignacion:events"
	defaultStreamMaxLen     = int64(10000)
	defaultPublishTimeout   = 2 * time.Second
	defaultOperationTimeout = 2 * time.Second
)

// RedisStreamsConfig tunes the Redis Streams publisher.
type RedisStreamsConfig struct {
	// StreamKey is the Redis Streams key events are appended to.
	// Empty defaults to "asignacion:events".
	StreamKey string
	// MaxLen is the approximate cap on the stream. XAdd uses Approx trim so
	// the actual length may briefly exceed this. Empty defaults to 10000.
	MaxLen int64
	// OperationTimeout bounds a single XADD. Empty defaults to 2s.
	OperationTimeout time.Duration
}

// RedisStreamsAssignmentEventPublisher publishes Asignacion domain events
// to a Redis Stream with at-least-once semantics. Consumers must use
// consumer groups and XACK to confirm delivery.
type RedisStreamsAssignmentEventPublisher struct {
	client redis.Cmdable
	cfg    RedisStreamsConfig
}

// NewRedisStreamsAssignmentEventPublisher creates a publisher that writes
// events to the configured Redis Stream. Defaults are applied for empty
// configuration values.
func NewRedisStreamsAssignmentEventPublisher(client redis.Cmdable, cfg RedisStreamsConfig) *RedisStreamsAssignmentEventPublisher {
	if cfg.StreamKey == "" {
		cfg.StreamKey = defaultStreamKey
	}
	if cfg.MaxLen <= 0 {
		cfg.MaxLen = defaultStreamMaxLen
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = defaultOperationTimeout
	}
	return &RedisStreamsAssignmentEventPublisher{client: client, cfg: cfg}
}

// PublishAsignacionModificada implements AssignmentEventPublisher.
func (p *RedisStreamsAssignmentEventPublisher) PublishAsignacionModificada(ctx context.Context, e asignacion.AsignacionModificadaEvent) error {
	return p.publish(ctx, e.Name(), e)
}

// PublishEvidenciaCargada implements AssignmentEventPublisher.
func (p *RedisStreamsAssignmentEventPublisher) PublishEvidenciaCargada(ctx context.Context, e asignacion.EvidenciaCargadaEvent) error {
	return p.publish(ctx, e.Name(), e)
}

// PublishEmpleadoEvaluado implements AssignmentEventPublisher.
func (p *RedisStreamsAssignmentEventPublisher) PublishEmpleadoEvaluado(ctx context.Context, e asignacion.EmpleadoEvaluadoEvent) error {
	return p.publish(ctx, e.Name(), e)
}

func (p *RedisStreamsAssignmentEventPublisher) publish(ctx context.Context, eventName string, payload any) error {
	opCtx, cancel := context.WithTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("redis_publisher.%s marshal: %w", eventName, err)
	}

	args := &redis.XAddArgs{
		Stream: p.cfg.StreamKey,
		MaxLen: p.cfg.MaxLen,
		Approx: true,
		Values: map[string]any{
			"event":        eventName,
			"published_at": time.Now().UTC().Format(time.RFC3339Nano),
			"payload":      string(body),
		},
	}
	if err := p.client.XAdd(opCtx, args).Err(); err != nil {
		return fmt.Errorf("redis_publisher.%s xadd: %w", eventName, err)
	}
	return nil
}
