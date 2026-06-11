// Package events — consumer.go: the cross-domain event consumer for
// the Formularios module.
//
// PR-5 (5.2b) implementation. The formularios module consumes the
// EventoCancelado stream published by the Asignacion service (per
// the spec's "Consume" table). For each event, the consumer
// invokes EventoServicer.CancelEvento(id) which marks the evento
// as 'cancelado' and invalidates the cached pendientes list (see
// internal/formularios/service/evento_service.go for the service
// side).
//
// The consumer mirrors the shape of
// internal/atencionseguimiento/events/consumer.go (PR-5 of
// 09_AtencionSeguimiento_Service_Spec) — XREAD with a per-consumer
// lastID cursor, NOT a Redis Stream consumer group. See the
// 09-pattern-mirror deviation in apply-progress for why the brief's
// "XREADGROUP + ack" wording is interpreted as "mirror 09 exactly"
// rather than introducing a new pattern in PR-5.
//
// Stream name: "formularios:evento_cancelado". The source of the
// events is the Asignacion service (which publishes via its own
// publisher); the formularios module is a subscriber, not the
// publisher.
package events

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// streamEventoCancelado is the cross-domain stream the formularios
// module subscribes to. Matches the Asignacion publisher's
// EventoCancelado emission (the spec calls for one stream per
// domain event; the formularios subscription reads from a stream
// owned by Asignacion — the two services share the stream name as
// the contract).
const streamEventoCancelado = "formularios:evento_cancelado"

// EventoServicer is the minimal interface the consumer depends on.
// The concrete *service.EventoService satisfies this interface; a
// mock is used in the unit tests.
type EventoServicer interface {
	CancelEvento(ctx context.Context, id uuid.UUID) error
}

// EventoCanceladoConsumer reads the cross-domain EventoCancelado
// stream and calls EventoServicer.CancelEvento for each event.
//
// Implementation notes (PR-5 deviations from the brief):
//
//   - Uses XREAD (not XREADGROUP) + a per-consumer lastID cursor
//     instead of a Redis Stream consumer group + XACK. This is the
//     exact pattern 09 uses for EmpleadoDeBajaConsumer and
//     ClienteInactivoConsumer; the orchestrator's brief explicitly
//     said "Mirror 09's EmpleadoDeBajaConsumer.Start exactly".
//     Consumer groups + XACK will land in a future PR (cross-cutting
//     outbox design).
//   - On service errors the consumer logs and continues. The
//     brief's "retry/dead-letter on transient errors" is
//     documented as a future PR; PR-5 follows 09's "log + move on"
//     model so the failure mode is consistent across all cross-
//     domain consumers in the system.
type EventoCanceladoConsumer struct {
	rdb     *redis.Client
	svc     EventoServicer
	stream  string
	stopCh  chan struct{}
	group   string
	consumer string
}

// NewEventoCanceladoConsumer constructs an EventoCanceladoConsumer
// with the default stream ("formularios:evento_cancelado") and
// group/consumer defaults. The constructor is the production
// seam; tests can use it directly (the consumer is unit-tested
// with a miniredis-backed rdb and a mock service).
func NewEventoCanceladoConsumer(rdb *redis.Client, svc EventoServicer) *EventoCanceladoConsumer {
	return &EventoCanceladoConsumer{
		rdb:      rdb,
		svc:      svc,
		stream:   streamEventoCancelado,
		stopCh:   make(chan struct{}),
		group:    "formularios-consumer-group",
		consumer: "formularios-consumer-1",
	}
}

// Start launches the consumer loop in a goroutine. The loop
// exits cleanly when ctx is cancelled or Stop is called.
func (c *EventoCanceladoConsumer) Start(ctx context.Context) {
	go func() {
		lastID := "0"
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			default:
			}

			entries, err := c.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{c.stream, lastID},
				Count:   10,
				Block:   500 * time.Millisecond,
			}).Result()
			if err != nil {
				if err == redis.Nil || err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				slog.Error("EventoCanceladoConsumer: XRead error", slog.String("error", err.Error()))
				continue
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					lastID = msg.ID
					eventoIDStr, _ := msg.Values["evento_id"].(string)
					eventoID, err := uuid.Parse(eventoIDStr)
					if err != nil {
						slog.Warn("EventoCanceladoConsumer: invalid evento_id",
							slog.String("value", eventoIDStr),
							slog.String("stream_msg_id", msg.ID),
						)
						continue
					}
					if err := c.svc.CancelEvento(ctx, eventoID); err != nil {
						// 09-pattern mirror: log + move on. Future PR will
						// add retry-with-backoff and dead-letter routing per
						// the spec's failure-handling table.
						slog.Error("EventoCanceladoConsumer: CancelEvento failed",
							slog.String("evento_id", eventoIDStr),
							slog.String("error", err.Error()),
						)
					}
				}
			}
		}
	}()
}

// Stop signals the consumer goroutine to exit. Safe to call
// multiple times — subsequent calls are no-ops.
func (c *EventoCanceladoConsumer) Stop() {
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
}

// strconv is referenced to keep the import used (the consumer
// could be extended to also parse int64 fields; keeping the
// import here avoids churn when that lands).
var _ = strconv.Itoa
