// Package events — publisher.go: the Redis Streams XADD publisher
// for the Formularios module.
//
// PR-5 (5.1) implementation. Mirrors the
// internal/atencionseguimiento/events/publisher.go shape (PR-5 of
// 09_AtencionSeguimiento_Service_Spec) and adds XADD MAXLEN ~ N
// bounded retention so the stream never grows past the cap (the
// Asignacion publisher in the same repo already uses this
// pattern; this brings the formularios publisher to parity).
//
// The publisher is fire-and-forget at the call site (postWriteHook
// in service/hooks.go swallows the error). The outbox pattern
// (PR-5 of the cross-cutting design) is the source of truth for
// at-least-once delivery — the publisher is the transport.
package events

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// defaultStreamMaxLen is the approximate cap on every formularios
// stream. XADD with MaxLen+Approx may keep MaxLen+1 entries
// transiently (Redis's trim is opportunistic).
const defaultStreamMaxLen = int64(10000)

// RedisStreamPublisher implements EventPublisher using Redis
// Streams (XADD). It applies MAXLEN ~ N to every XADD so the
// stream never grows unbounded.
type RedisStreamPublisher struct {
	rdb    *redis.Client
	maxLen int64
}

// NewRedisStreamPublisher constructs a RedisStreamPublisher with
// the default MaxLen (10_000). Callers that need a different cap
// can use NewRedisStreamPublisherWithMaxLen.
func NewRedisStreamPublisher(rdb *redis.Client) *RedisStreamPublisher {
	return NewRedisStreamPublisherWithMaxLen(rdb, defaultStreamMaxLen)
}

// NewRedisStreamPublisherWithMaxLen constructs a RedisStreamPublisher
// with a custom MaxLen. Use a non-positive value to get the default.
func NewRedisStreamPublisherWithMaxLen(rdb *redis.Client, maxLen int64) *RedisStreamPublisher {
	if maxLen <= 0 {
		maxLen = defaultStreamMaxLen
	}
	return &RedisStreamPublisher{rdb: rdb, maxLen: maxLen}
}

// Publish sends the payload as an XADD to the given stream. Field
// values are coerced to strings via fmt.Sprintf("%v", v); this
// matches the 09 publisher and is required by the Redis Streams
// field-value model (values are always strings on the wire).
//
// An empty payload is preserved as an XADD with a single synthetic
// "_empty" field so the entry round-trips (XADD with zero fields
// would fail on real Redis). Call sites that want to publish
// "nothing meaningful happened" can pass an empty map.
//
// XADD is wrapped with MAXLEN ~ maxLen so the stream never grows
// past the cap (Approx trim is opportunistic and may let the
// stream briefly hold maxLen+1 entries).
func (p *RedisStreamPublisher) Publish(ctx context.Context, stream string, payload map[string]any) error {
	values := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		values[k] = fmt.Sprintf("%v", v)
	}
	if len(values) == 0 {
		// XADD with zero fields would be rejected; emit a synthetic
		// marker so the entry round-trips. The marker is intentionally
		// not a real field name a consumer would key on.
		values["_empty"] = "true"
	}
	args := &redis.XAddArgs{
		Stream: stream,
		ID:     "*",
		MaxLen: p.maxLen,
		Approx: true,
		Values: values,
	}
	return p.rdb.XAdd(ctx, args).Err()
}

// Compile-time check that the concrete type satisfies the port.
var _ EventPublisher = (*RedisStreamPublisher)(nil)
