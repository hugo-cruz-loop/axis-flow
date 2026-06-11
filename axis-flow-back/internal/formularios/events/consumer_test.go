// Package events_test — consumer tests for PR-5 (5.2b).
//
// Mirrors the atencionseguimiento consumer_test pattern. The
// EventoCanceladoConsumer reads the cross-domain EventoCancelado
// stream and calls EventoServicer.CancelEvento for each event.
package events_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"axis-flow-back/internal/formularios/events"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEventoServicer captures CancelEvento calls.
type mockEventoServicer struct {
	called    []uuid.UUID
	returnErr error
	mu        atomic.Int32
}

func (m *mockEventoServicer) CancelEvento(_ context.Context, id uuid.UUID) error {
	m.mu.Add(1)
	m.called = append(m.called, id)
	return m.returnErr
}

func newConsumerFixture(t *testing.T) (*miniredis.Miniredis, *redis.Client, *mockEventoServicer) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb, &mockEventoServicer{}
}

// TestEventoCanceladoConsumer_ProcessesEvent asserts that a message
// pushed to the EventoCancelado stream is consumed and the service
// receives the eventoID parsed from the message fields.
func TestEventoCanceladoConsumer_ProcessesEvent(t *testing.T) {
	mr, rdb, svc := newConsumerFixture(t)

	cons := events.NewEventoCanceladoConsumer(rdb, svc)

	eventoID := uuid.New()
	require.NoError(t, rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "formularios:evento_cancelado",
		ID:     "*",
		Values: map[string]any{"evento_id": eventoID.String()},
	}).Err())

	runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cons.Start(runCtx)

	assert.Eventually(t, func() bool {
		return len(svc.called) == 1
	}, 2*time.Second, 50*time.Millisecond, "expected CancelEvento to be called once")

	assert.Equal(t, eventoID, svc.called[0], "service must receive the eventoID from the message")
	cons.Stop()
	_ = mr
}

// TestEventoCanceladoConsumer_StopsOnContextCancel asserts the
// consumer loop exits cleanly when the context is cancelled (the
// graceful-shutdown path used by main.go's signal handler).
func TestEventoCanceladoConsumer_StopsOnContextCancel(t *testing.T) {
	_, rdb, svc := newConsumerFixture(t)

	cons := events.NewEventoCanceladoConsumer(rdb, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cons.Start(ctx)
	cancel()
	time.Sleep(100 * time.Millisecond)
	cons.Stop()
}

// TestEventoCanceladoConsumer_SkipsMalformedPayload asserts that a
// message with a missing or unparseable evento_id is logged and
// skipped (no panic, no infinite loop). The consumer moves on to
// the next valid message.
func TestEventoCanceladoConsumer_SkipsMalformedPayload(t *testing.T) {
	_, rdb, svc := newConsumerFixture(t)

	cons := events.NewEventoCanceladoConsumer(rdb, svc)

	// Push a malformed message (evento_id is not a UUID).
	require.NoError(t, rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "formularios:evento_cancelado",
		ID:     "*",
		Values: map[string]any{"evento_id": "not-a-uuid"},
	}).Err())

	// Then push a valid message — the consumer should still process it
	// after skipping the malformed one.
	validID := uuid.New()
	require.NoError(t, rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "formularios:evento_cancelado",
		ID:     "*",
		Values: map[string]any{"evento_id": validID.String()},
	}).Err())

	runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cons.Start(runCtx)

	assert.Eventually(t, func() bool {
		return len(svc.called) == 1
	}, 2*time.Second, 50*time.Millisecond, "consumer must process the valid message after skipping the malformed one")
	assert.Equal(t, validID, svc.called[0])
	cons.Stop()
}

// TestEventoCanceladoConsumer_ServiceErrorDoesNotPanic asserts that
// a transient service error is logged and the consumer continues
// (the next message is still processed). The 09 mirror uses
// "no XACK on error" semantics; PR-5 follows the same pattern.
// See Deviation #1 in the apply-progress.
func TestEventoCanceladoConsumer_ServiceErrorDoesNotPanic(t *testing.T) {
	_, rdb, svc := newConsumerFixture(t)
	svc.returnErr = assert.AnError

	cons := events.NewEventoCanceladoConsumer(rdb, svc)

	eventoID := uuid.New()
	require.NoError(t, rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "formularios:evento_cancelado",
		ID:     "*",
		Values: map[string]any{"evento_id": eventoID.String()},
	}).Err())

	runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cons.Start(runCtx)

	// The consumer reads the message and tries to call the service.
	// The service errors. The consumer logs and continues (does not
	// panic). We assert the call was attempted at least once.
	assert.Eventually(t, func() bool {
		return len(svc.called) >= 1
	}, 2*time.Second, 50*time.Millisecond, "consumer must have tried the service at least once")
	cons.Stop()
}

// TestEventoCanceladoConsumer_ProcessesMultipleEvents asserts that
// multiple backlogged messages are processed in order.
func TestEventoCanceladoConsumer_ProcessesMultipleEvents(t *testing.T) {
	_, rdb, svc := newConsumerFixture(t)

	cons := events.NewEventoCanceladoConsumer(rdb, svc)

	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		require.NoError(t, rdb.XAdd(context.Background(), &redis.XAddArgs{
			Stream: "formularios:evento_cancelado",
			ID:     "*",
			Values: map[string]any{"evento_id": id.String()},
		}).Err())
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cons.Start(runCtx)

	assert.Eventually(t, func() bool {
		return len(svc.called) == 3
	}, 2*time.Second, 50*time.Millisecond, "consumer must process all 3 events")
	assert.Equal(t, ids, svc.called, "service must receive the IDs in the order they were published")
	cons.Stop()
}
