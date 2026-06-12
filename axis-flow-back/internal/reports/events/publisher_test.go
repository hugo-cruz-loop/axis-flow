package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"axis-flow-back/internal/reports/events"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisEventPublisher_Publish_XADD(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	pub := events.NewRedisEventPublisher(rdb)

	payload := map[string]string{"empresa_id": "abc-123"}
	if err := pub.Publish(context.Background(), events.StreamGeocodingCacheEvicted, payload); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Verify the stream has exactly one entry.
	msgs, err := rdb.XRange(context.Background(), events.StreamGeocodingCacheEvicted, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRANGE: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 stream entry, got %d", len(msgs))
	}

	// Verify the payload field is present and valid JSON.
	raw, ok := msgs[0].Values["payload"].(string)
	if !ok {
		t.Fatal("expected 'payload' field in stream entry")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["empresa_id"] != "abc-123" {
		t.Errorf("unexpected empresa_id: %q", decoded["empresa_id"])
	}
}

func TestNoopPublisher_Publish_NoError(t *testing.T) {
	var pub events.NoopPublisher
	if err := pub.Publish(context.Background(), events.StreamGeocodingCacheEvicted, nil); err != nil {
		t.Errorf("NoopPublisher should never error, got: %v", err)
	}
}
