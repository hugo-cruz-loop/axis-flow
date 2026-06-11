package events_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"axis-flow-back/internal/parametrizacion"
	"axis-flow-back/internal/parametrizacion/events"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisPublisherWritesEventEnvelopeToStream(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	publisher := events.NewRedisPublisher(client, events.RedisPublisherConfig{StreamKey: "parametrizacion:events:test", MaxLen: 100, OperationTimeout: time.Second})

	event := parametrizacion.DomainEvent{
		Name:       parametrizacion.EventSystemSettingUpdated,
		OccurredAt: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		Payload: map[string]any{
			"clave_parametro": "WEBSOCKET_CHANNEL_NAME",
			"valor":           "axis_flow",
			"updated_by":      "00000000-0000-0000-0000-000000000012",
		},
	}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	entries, err := client.XRange(context.Background(), "parametrizacion:events:test", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d stream entries, want 1", len(entries))
	}
	if entries[0].Values["event"] != parametrizacion.EventSystemSettingUpdated {
		t.Fatalf("event field mismatch: %#v", entries[0].Values)
	}
	payloadRaw, ok := entries[0].Values["payload"].(string)
	if !ok {
		t.Fatalf("payload must be a JSON string: %#v", entries[0].Values)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["clave_parametro"] != "WEBSOCKET_CHANNEL_NAME" || payload["valor"] != "axis_flow" {
		t.Fatalf("payload mismatch: %#v", payload)
	}
}
