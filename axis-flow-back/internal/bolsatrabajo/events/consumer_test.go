package events_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/bolsatrabajo/events"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTrabajoServicer captures CloseAllByEmpresa calls.
type mockTrabajoServicer struct {
	called    []uuid.UUID
	returnErr error
}

func (m *mockTrabajoServicer) CloseAllByEmpresa(_ context.Context, empresaID uuid.UUID) error {
	m.called = append(m.called, empresaID)
	return m.returnErr
}

func TestEmpresaDeBajaConsumer_ProcessesEvent(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := &mockTrabajoServicer{}
	consumer := events.NewEmpresaDeBajaConsumer(rdb, svc)

	// Push a fake event to the stream before starting the consumer.
	empresaID := uuid.New()
	ctx := context.Background()
	err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "empresas:empresa_de_baja",
		ID:     "*",
		Values: map[string]any{"empresa_id": empresaID.String()},
	}).Err()
	require.NoError(t, err)

	// Start consumer with a cancellable context.
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	consumer.Start(runCtx)

	// Allow a short window for the goroutine to process.
	assert.Eventually(t, func() bool {
		return len(svc.called) == 1
	}, 2*time.Second, 50*time.Millisecond, "expected CloseAllByEmpresa to be called once")

	assert.Equal(t, empresaID, svc.called[0])
	consumer.Stop()
}

func TestEmpresaDeBajaConsumer_StopsOnContextCancel(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := &mockTrabajoServicer{}
	consumer := events.NewEmpresaDeBajaConsumer(rdb, svc)

	ctx, cancel := context.WithCancel(context.Background())
	consumer.Start(ctx)

	// Cancel immediately — consumer should exit without panic.
	cancel()
	// Give the goroutine a moment to exit.
	time.Sleep(100 * time.Millisecond)
	consumer.Stop()
}
