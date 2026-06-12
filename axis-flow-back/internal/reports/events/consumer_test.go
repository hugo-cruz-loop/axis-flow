package events_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"axis-flow-back/internal/reports"
	"axis-flow-back/internal/reports/events"
	"axis-flow-back/internal/reports/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Stub implementations for consumer dependencies.
// ---------------------------------------------------------------------------

type stubGeoRepo struct {
	deleteCalled []string
}

func (s *stubGeoRepo) GetByCoords(_ context.Context, _, _ float64) (*reports.DireccionCache, error) {
	return nil, reports.ErrNotFound
}
func (s *stubGeoRepo) Store(_ context.Context, _ reports.DireccionCache) error { return nil }
func (s *stubGeoRepo) DeleteByTenant(_ context.Context, empresaID string) error {
	s.deleteCalled = append(s.deleteCalled, empresaID)
	return nil
}

// Ensure stubGeoRepo satisfies the repository.GeocodingRepo interface at compile time.
var _ repository.GeocodingRepo = (*stubGeoRepo)(nil)

type stubGeoCache struct {
	deletePatternCalled []string
}

func (s *stubGeoCache) Get(_ context.Context, _, _ float64) (string, bool) { return "", false }
func (s *stubGeoCache) Set(_ context.Context, _, _ float64, _ string, _ time.Duration) error {
	return nil
}
func (s *stubGeoCache) DeleteByPattern(_ context.Context, pattern string) error {
	s.deletePatternCalled = append(s.deletePatternCalled, pattern)
	return nil
}

// Ensure stubGeoCache satisfies the repository.GeocodingCache interface at compile time.
var _ repository.GeocodingCache = (*stubGeoCache)(nil)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestEmpresaDeBajaConsumer_ProcessesEvent(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	geoRepo := &stubGeoRepo{}
	geoCache := &stubGeoCache{}
	pub := &events.NoopPublisher{}

	// Capture log output to verify warnings.
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	consumer := events.NewEmpresaDeBajaConsumer(rdb, geoRepo, geoCache, pub, logger)

	// Run the consumer briefly.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go consumer.Start(ctx)

	// Give the consumer a moment to enter its XREAD BLOCK call.
	time.Sleep(100 * time.Millisecond)

	// Publish the EmpresaDeBaja event AFTER the consumer is listening.
	payload, _ := json.Marshal(map[string]string{"empresa_id": "empresa-42"})
	rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: events.StreamEmpresaDeBaja,
		Values: map[string]any{"payload": string(payload)},
	})

	// Poll until the event is processed.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(geoCache.deletePatternCalled) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	consumer.Stop()

	if len(geoRepo.deleteCalled) == 0 {
		t.Error("expected GeocodingRepo.DeleteByTenant to be called")
	} else if geoRepo.deleteCalled[0] != "empresa-42" {
		t.Errorf("wrong empresa_id: %q", geoRepo.deleteCalled[0])
	}

	if len(geoCache.deletePatternCalled) == 0 {
		t.Error("expected GeocodingCache.DeleteByPattern to be called")
	} else if geoCache.deletePatternCalled[0] != "reports:geocoding:*" {
		t.Errorf("wrong pattern: %q", geoCache.deletePatternCalled[0])
	}

	// Verify a warn-level log was emitted before purge.
	if !bytes.Contains(logBuf.Bytes(), []byte("geocoding cache")) {
		t.Errorf("expected warning log about geocoding cache purge, got:\n%s", logBuf.String())
	}
}
