package events_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/atencionseguimiento/events"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQuejaServicer captures SuspendQuejasByEmpleado calls.
type mockQuejaServicer struct {
	called    []int64
	returnErr error
}

func (m *mockQuejaServicer) SuspendQuejasByEmpleado(_ context.Context, empleadoID int64) error {
	m.called = append(m.called, empleadoID)
	return m.returnErr
}

// mockTicketServicer captures CloseTicketsByCliente calls.
type mockTicketServicer struct {
	called    []uuid.UUID
	returnErr error
}

func (m *mockTicketServicer) CloseTicketsByCliente(_ context.Context, clienteID uuid.UUID) error {
	m.called = append(m.called, clienteID)
	return m.returnErr
}

func TestEmpleadoDeBajaConsumer_ProcessesEvent(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := &mockQuejaServicer{}
	consumer := events.NewEmpleadoDeBajaConsumer(rdb, svc)

	ctx := context.Background()
	err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "empleados:empleado_de_baja",
		ID:     "*",
		Values: map[string]any{"empleado_id": "42"},
	}).Err()
	require.NoError(t, err)

	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	consumer.Start(runCtx)

	assert.Eventually(t, func() bool {
		return len(svc.called) == 1
	}, 2*time.Second, 50*time.Millisecond, "expected SuspendQuejasByEmpleado to be called once")

	assert.Equal(t, int64(42), svc.called[0])
	consumer.Stop()
}

func TestEmpleadoDeBajaConsumer_StopsOnContextCancel(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := &mockQuejaServicer{}
	consumer := events.NewEmpleadoDeBajaConsumer(rdb, svc)

	ctx, cancel := context.WithCancel(context.Background())
	consumer.Start(ctx)
	cancel()
	time.Sleep(100 * time.Millisecond)
	consumer.Stop()
}

func TestClienteInactivoConsumer_ProcessesEvent(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := &mockTicketServicer{}
	consumer := events.NewClienteInactivoConsumer(rdb, svc)

	ctx := context.Background()
	clienteID := uuid.New()
	err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "clientes:cliente_inactivo",
		ID:     "*",
		Values: map[string]any{"cliente_id": clienteID.String()},
	}).Err()
	require.NoError(t, err)

	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	consumer.Start(runCtx)

	assert.Eventually(t, func() bool {
		return len(svc.called) == 1
	}, 2*time.Second, 50*time.Millisecond, "expected CloseTicketsByCliente to be called once")

	assert.Equal(t, clienteID, svc.called[0])
	consumer.Stop()
}

func TestClienteInactivoConsumer_StopsOnContextCancel(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := &mockTicketServicer{}
	consumer := events.NewClienteInactivoConsumer(rdb, svc)

	ctx, cancel := context.WithCancel(context.Background())
	consumer.Start(ctx)
	cancel()
	time.Sleep(100 * time.Millisecond)
	consumer.Stop()
}
