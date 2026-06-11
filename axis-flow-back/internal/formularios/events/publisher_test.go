// Package events_test — publisher tests for PR-5 (5.1).
//
// Mirrors the atencionseguimiento publisher_test pattern. Covers:
//   - Each of the 4 formularios stream constants (XADD lands in
//     the right stream)
//   - Payload field round-trip (XRANGE returns the published fields)
//   - Empty-payload handling (XADD with no fields would fail on
//     real Redis; miniredis matches the contract — the publisher
//     sends a synthetic marker field when the payload is empty)
//   - Redis client error propagation
//   - MaxLen cap is applied (XADD MAXLEN ~ N)
package events_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/formularios/events"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPublisherFixture(t *testing.T) (*miniredis.Miniredis, *redis.Client, *events.RedisStreamPublisher) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb, events.NewRedisStreamPublisher(rdb)
}

// streamLength is a tiny local helper to keep the test bodies
// focused on the assertion. rdb is reused from the fixture.
func streamLength(t *testing.T, rdb *redis.Client, stream string) int64 {
	t.Helper()
	length, err := rdb.XLen(context.Background(), stream).Result()
	require.NoError(t, err)
	return length
}

// TestPublisher_XAddToFormularioCreadoStream asserts that publishing
// to the StreamFormularioCreado constant lands the entry in
// formularios:formulario_creado.
func TestPublisher_XAddToFormularioCreadoStream(t *testing.T) {
	mr, rdb, pub := newPublisherFixture(t)

	require.NoError(t, pub.Publish(context.Background(), events.StreamFormularioCreado, map[string]any{
		"formulario_id": "f-1",
		"empresa_id":    "e-1",
		"nombre":        "Control Higienico",
	}))

	assert.True(t, mr.Exists("formularios:formulario_creado"), "stream must exist after Publish")
	assert.Equal(t, int64(1), streamLength(t, rdb, events.StreamFormularioCreado))
}

// TestPublisher_XAddToEventoIniciadoStream mirrors the first test for
// the EventoIniciado stream (Gherkin 2 check-in step).
func TestPublisher_XAddToEventoIniciadoStream(t *testing.T) {
	mr, _, pub := newPublisherFixture(t)

	require.NoError(t, pub.Publish(context.Background(), events.StreamEventoIniciado, map[string]any{
		"evento_id":          "ev-1",
		"evento_iniciado_id": "ei-1",
		"empresa_id":         "e-1",
		"empleado_id":        int64(42),
	}))

	assert.True(t, mr.Exists("formularios:evento_iniciado"))
}

// TestPublisher_XAddToFormularioRespondidoStream covers the response
// step (Gherkin 2 respuesta).
func TestPublisher_XAddToFormularioRespondidoStream(t *testing.T) {
	mr, _, pub := newPublisherFixture(t)

	require.NoError(t, pub.Publish(context.Background(), events.StreamFormularioRespondido, map[string]any{
		"respuesta_id":       "r-1",
		"evento_iniciado_id": "ei-1",
		"pregunta_id":        "p-1",
		"empresa_id":         "e-1",
	}))

	assert.True(t, mr.Exists("formularios:formulario_respondido"))
}

// TestPublisher_XAddToReporteGeneradoStream covers the PDF report
// event (Gherkin 3).
func TestPublisher_XAddToReporteGeneradoStream(t *testing.T) {
	mr, _, pub := newPublisherFixture(t)

	require.NoError(t, pub.Publish(context.Background(), events.StreamReporteGenerado, map[string]any{
		"reporte_url":        "https://s3.example.com/reports/x.pdf",
		"evento_iniciado_id": "ei-1",
		"empresa_id":         "e-1",
	}))

	assert.True(t, mr.Exists("formularios:reporte_generado"))
}

// TestPublisher_FieldsAreRoundTrippedViaXRANGE asserts that the
// payload field-value pairs survive the XADD round-trip — XRANGE
// returns the same fields. The publisher coerces values to strings
// (Redis stream fields are always strings); the test asserts that
// the string coercion is consistent for strings, numbers, and UUIDs.
func TestPublisher_FieldsAreRoundTrippedViaXRANGE(t *testing.T) {
	_, rdb, pub := newPublisherFixture(t)

	stream := events.StreamFormularioCreado
	require.NoError(t, pub.Publish(context.Background(), stream, map[string]any{
		"formulario_id": "f-42",
		"empresa_id":    "e-99",
		"nombre":        "Control Higienico",
		"empleado_id":   int64(7),
	}))

	entries, err := rdb.XRange(context.Background(), stream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "f-42", entries[0].Values["formulario_id"])
	assert.Equal(t, "e-99", entries[0].Values["empresa_id"])
	assert.Equal(t, "Control Higienico", entries[0].Values["nombre"])
	// Numeric values are stringified; the spec consumers parse on
	// their end. We assert the value is the string form of the int.
	assert.Equal(t, "7", entries[0].Values["empleado_id"])
}

// TestPublisher_EmptyPayloadStillPublishes asserts that the
// publisher handles an empty payload without failing the XADD
// (XADD with zero fields would otherwise fail). The publisher
// inserts a synthetic "empty" marker so the entry round-trips.
func TestPublisher_EmptyPayloadStillPublishes(t *testing.T) {
	mr, rdb, pub := newPublisherFixture(t)

	require.NoError(t, pub.Publish(context.Background(), events.StreamFormularioCreado, map[string]any{}))

	assert.Equal(t, int64(1), streamLength(t, rdb, events.StreamFormularioCreado), "empty payload must still produce an XADD entry")
	assert.True(t, mr.Exists("formularios:formulario_creado"))
}

// TestPublisher_RedisErrorReturned asserts that a closed miniredis
// client surfaces an error from Publish (the call site swallows it
// via postWriteHook, but the test pins the failure mode for the
// publisher).
func TestPublisher_RedisErrorReturned(t *testing.T) {
	mr, rdb, pub := newPublisherFixture(t)
	mr.Close()

	err := pub.Publish(context.Background(), events.StreamFormularioCreado, map[string]any{
		"formulario_id": "f-1",
	})
	assert.Error(t, err, "Publish must return the underlying Redis error when the client is closed")
	_ = rdb
}

// TestPublisher_MaxLenCapIsApplied asserts that XADD uses the
// MAXLEN ~ N trim so the stream never grows past the cap (the
// spec's at-least-once delivery with bounded retention — the
// 09 module uses the same pattern with MaxLen=10000).
func TestPublisher_MaxLenCapIsApplied(t *testing.T) {
	mr, rdb, _ := newPublisherFixture(t)
	// Use the WithMaxLen constructor to force a small cap (the
	// production default is 10000 — way larger than this test
	// publishes, so the trim would not be exercised).
	stream := "formularios:formulario_creado_capped"
	pub := events.NewRedisStreamPublisherWithMaxLen(rdb, 5)

	// Publish 12 messages with MaxLen=5; miniredis trims exactly
	// to MaxLen (no Approx slack) — assert the length is bounded
	// by the cap, not the publish count.
	for i := 0; i < 12; i++ {
		require.NoError(t, pub.Publish(context.Background(), stream, map[string]any{"i": i}))
	}
	length, err := rdb.XLen(context.Background(), stream).Result()
	require.NoError(t, err)
	assert.LessOrEqual(t, length, int64(5), "XADD MAXLEN ~ N must keep the stream bounded to MaxLen")
	_ = mr
}
