package events_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/bolsatrabajo/events"

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
		payload := map[string]any{"empresa_id": "abc-123", "titulo": "Golang Dev"}
		err := pub.Publish(ctx, events.StreamVacanteCreada, payload)
		require.NoError(t, err)

		// miniredis exposes stream length via XLen
		length, err := rdb.XLen(ctx, events.StreamVacanteCreada).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), length)
	})

	t.Run("each Publish appends a new entry", func(t *testing.T) {
		stream := events.StreamPostulacionRecibida
		for i := 0; i < 3; i++ {
			err := pub.Publish(ctx, stream, map[string]any{"n": i})
			require.NoError(t, err)
		}
		length, err := rdb.XLen(ctx, stream).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(3), length)
	})

	t.Run("payload fields are stored in the stream entry", func(t *testing.T) {
		stream := events.StreamCandidatoContratado
		payload := map[string]any{"candidato_id": "xyz-999"}
		err := pub.Publish(ctx, stream, payload)
		require.NoError(t, err)

		entries, err := rdb.XRange(ctx, stream, "-", "+").Result()
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "xyz-999", entries[0].Values["candidato_id"])
	})
}
