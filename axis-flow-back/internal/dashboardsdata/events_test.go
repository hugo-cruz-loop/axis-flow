package dashboardsdata_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"axis-flow-back/internal/dashboardsdata"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEventsDB struct {
	clientMap map[string]uuid.UUID
}

func (m *mockEventsDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	idStr := ""
	if len(args) > 0 {
		if u, ok := args[0].(uuid.UUID); ok {
			idStr = u.String()
		}
	}
	clientID, ok := m.clientMap[idStr]
	if !ok {
		return &mockRow{err: pgx.ErrNoRows}
	}
	return &mockRow{vals: []any{clientID}}
}

func TestEventsConsumer_CacheEviction(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cache := dashboardsdata.NewCacheClient(rdb)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientID := uuid.New()
	locationID := uuid.New()
	ticketID := uuid.New()

	dbMock := &mockEventsDB{
		clientMap: map[string]uuid.UUID{
			locationID.String(): clientID,
			ticketID.String():   clientID,
		},
	}

	consumer := dashboardsdata.NewEventsConsumer(rdb, cache, dbMock)
	consumer.Start(ctx)

	// Set cache keys
	servicesKey := cache.FormatKey("cliente", clientID.String(), "servicios-localidad")
	require.NoError(t, rdb.Set(ctx, servicesKey, "some-data", 0).Err())

	ticketsKey := cache.FormatKey("cliente", clientID.String(), "atencion-seguimiento")
	require.NoError(t, rdb.Set(ctx, ticketsKey, "some-data", 0).Err())

	absencesKey := cache.FormatKey("rh", "123", "absentismo")
	require.NoError(t, rdb.Set(ctx, absencesKey, "some-data", 0).Err())

	// 1. Evict via asignacion:events (AsignacionModificada)
	payload := map[string]any{
		"LocationID": locationID,
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	_, err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "asignacion:events",
		ID:     "*",
		Values: map[string]any{
			"event":   "AsignacionModificada",
			"payload": string(payloadBytes),
		},
	}).Result()
	require.NoError(t, err)

	// 2. Evict via atencion:ticket_servicio_actualizado
	_, err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "atencion:ticket_servicio_actualizado",
		ID:     "*",
		Values: map[string]any{
			"ticket_id": ticketID.String(),
		},
	}).Result()
	require.NoError(t, err)

	// 3. Evict via empleados:inasistencia_registrada
	_, err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "empleados:inasistencia_registrada",
		ID:     "*",
		Values: map[string]any{
			"empresa_id": "123",
		},
	}).Result()
	require.NoError(t, err)

	// Wait for consumer processing
	time.Sleep(200 * time.Millisecond)

	// Verify evictions
	val, err := rdb.Get(ctx, servicesKey).Result()
	assert.Equal(t, redis.Nil, err, "servicesKey should be evicted; got val: %s", val)

	val, err = rdb.Get(ctx, ticketsKey).Result()
	assert.Equal(t, redis.Nil, err, "ticketsKey should be evicted; got val: %s", val)

	val, err = rdb.Get(ctx, absencesKey).Result()
	assert.Equal(t, redis.Nil, err, "absencesKey should be evicted; got val: %s", val)
}

func TestEventsConsumer_CacheEviction_DBFailure(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cache := dashboardsdata.NewCacheClient(rdb)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientID := uuid.New()
	locationID := uuid.New() // Not mapped in dbMock

	dbMock := &mockEventsDB{
		clientMap: map[string]uuid.UUID{},
	}

	consumer := dashboardsdata.NewEventsConsumer(rdb, cache, dbMock)
	consumer.Start(ctx)

	// Set cache key
	servicesKey := cache.FormatKey("cliente", clientID.String(), "servicios-localidad")
	require.NoError(t, rdb.Set(ctx, servicesKey, "some-data", 0).Err())

	// 1. Send AsignacionModificada with unmapped locationID
	payload := map[string]any{
		"LocationID": locationID,
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	_, err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "asignacion:events",
		ID:     "*",
		Values: map[string]any{
			"event":   "AsignacionModificada",
			"payload": string(payloadBytes),
		},
	}).Result()
	require.NoError(t, err)

	// Wait for consumer processing
	time.Sleep(200 * time.Millisecond)

	// Verify the cache key is NOT evicted
	val, err := rdb.Get(ctx, servicesKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "some-data", val)
}
