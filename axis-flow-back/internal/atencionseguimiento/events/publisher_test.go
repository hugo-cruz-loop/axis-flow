package events_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/atencionseguimiento/events"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisStreamPublisher_Publish(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	pub := events.NewRedisStreamPublisher(rdb)
	ctx := context.Background()

	t.Run("creates entry in correct stream", func(t *testing.T) {
		payload := map[string]any{"queja_id": "abc-123", "empresa_id": "eid-1"}
		err := pub.Publish(ctx, events.StreamQuejaRegistrada, payload)
		require.NoError(t, err)

		length, err := rdb.XLen(ctx, events.StreamQuejaRegistrada).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), length)
	})

	t.Run("each Publish appends a new entry", func(t *testing.T) {
		stream := events.StreamTicketServicioCreado
		for i := 0; i < 3; i++ {
			err := pub.Publish(ctx, stream, map[string]any{"n": i})
			require.NoError(t, err)
		}
		length, err := rdb.XLen(ctx, stream).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(3), length)
	})

	t.Run("payload fields are stored in the stream entry", func(t *testing.T) {
		stream := events.StreamIncidenciaOperativaRegistrada
		payload := map[string]any{"incidencia_id": "inc-999"}
		err := pub.Publish(ctx, stream, payload)
		require.NoError(t, err)

		entries, err := rdb.XRange(ctx, stream, "-", "+").Result()
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "inc-999", entries[0].Values["incidencia_id"])
	})
}
