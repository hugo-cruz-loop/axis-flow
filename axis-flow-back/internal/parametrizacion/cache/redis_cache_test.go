package cache_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/parametrizacion/cache"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisCacheSetGetAndUnlink(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	c := cache.NewRedisCache(client)
	ctx := context.Background()

	payload := samplePayload{EmpresaID: 12, Label: "config"}
	if err := c.Set(ctx, "parametrizacion:test:12", payload, time.Hour); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	var got samplePayload
	hit, err := c.Get(ctx, "parametrizacion:test:12", &got)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !hit || got != payload {
		t.Fatalf("cache hit mismatch: hit=%v got=%#v want=%#v", hit, got, payload)
	}
	if ttl := server.TTL("parametrizacion:test:12"); ttl <= 0 {
		t.Fatalf("expected positive TTL, got %s", ttl)
	}
	if err := c.Delete(ctx, "parametrizacion:test:12"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	hit, err = c.Get(ctx, "parametrizacion:test:12", &got)
	if err != nil {
		t.Fatalf("Get after delete returned error: %v", err)
	}
	if hit {
		t.Fatal("expected miss after delete")
	}
}

type samplePayload struct {
	EmpresaID int64  `json:"empresa_id"`
	Label     string `json:"label"`
}
