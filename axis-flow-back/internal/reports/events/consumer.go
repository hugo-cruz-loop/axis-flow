package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"axis-flow-back/internal/reports/repository"

	"github.com/redis/go-redis/v9"
)

const (
	consumerMaxRetries = 5
	consumerBlock      = 500 * time.Millisecond
	consumerCount      = 10
)

// EmpresaDeBajaConsumer reads from the StreamEmpresaDeBaja Redis Stream and
// evicts geocoding cache entries whenever a company is deactivated.
type EmpresaDeBajaConsumer struct {
	rdb      redis.UniversalClient
	geoRepo  repository.GeocodingRepo
	geoCache repository.GeocodingCache
	pub      EventPublisher
	logger   *slog.Logger
	cancel   context.CancelFunc
}

// NewEmpresaDeBajaConsumer creates an EmpresaDeBajaConsumer.
func NewEmpresaDeBajaConsumer(
	rdb redis.UniversalClient,
	geoRepo repository.GeocodingRepo,
	geoCache repository.GeocodingCache,
	pub EventPublisher,
	logger *slog.Logger,
) *EmpresaDeBajaConsumer {
	return &EmpresaDeBajaConsumer{
		rdb:      rdb,
		geoRepo:  geoRepo,
		geoCache: geoCache,
		pub:      pub,
		logger:   logger,
	}
}

// Start launches the consumer loop in a goroutine. The loop reads from
// StreamEmpresaDeBaja using XREAD BLOCK 500ms and calls processMessage for
// each entry. It stops when ctx is cancelled.
func (c *EmpresaDeBajaConsumer) Start(ctx context.Context) {
	innerCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	lastID := "$"

	for {
		select {
		case <-innerCtx.Done():
			return
		default:
		}

		results, err := c.rdb.XRead(innerCtx, &redis.XReadArgs{
			Streams: []string{StreamEmpresaDeBaja, lastID},
			Count:   consumerCount,
			Block:   consumerBlock,
		}).Result()

		if err != nil {
			// redis.Nil means BLOCK timeout — normal, keep looping.
			if err == redis.Nil {
				continue
			}
			// Context cancelled.
			if innerCtx.Err() != nil {
				return
			}
			c.logger.Error("consumer: XREAD error", "stream", StreamEmpresaDeBaja, "error", err)
			continue
		}

		for _, stream := range results {
			for _, msg := range stream.Messages {
				c.processMessageWithRetry(innerCtx, msg)
				lastID = msg.ID
			}
		}
	}
}

// Stop cancels the consumer loop.
func (c *EmpresaDeBajaConsumer) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// processMessageWithRetry retries up to consumerMaxRetries times with
// exponential backoff. After exhaustion it logs the error (DLQ stub).
func (c *EmpresaDeBajaConsumer) processMessageWithRetry(ctx context.Context, msg redis.XMessage) {
	var lastErr error
	for attempt := 0; attempt < consumerMaxRetries; attempt++ {
		if err := c.processMessage(ctx, msg); err != nil {
			lastErr = err
			wait := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
			time.Sleep(wait)
			continue
		}
		return
	}
	// DLQ stub — log and drop.
	c.logger.Error("consumer: max retries exhausted; dropping message",
		"stream", StreamEmpresaDeBaja,
		"msg_id", msg.ID,
		"error", lastErr,
	)
}

// processMessage handles a single EmpresaDeBaja stream entry.
func (c *EmpresaDeBajaConsumer) processMessage(ctx context.Context, msg redis.XMessage) error {
	rawPayload, ok := msg.Values["payload"].(string)
	if !ok {
		c.logger.Warn("consumer: missing payload field", "msg_id", msg.ID)
		return nil // malformed — drop without retry
	}

	var evt struct {
		EmpresaID string `json:"empresa_id"`
	}
	if err := json.Unmarshal([]byte(rawPayload), &evt); err != nil {
		c.logger.Warn("consumer: cannot parse payload", "msg_id", msg.ID, "error", err)
		return nil // malformed — drop without retry
	}

	// 1. Delete tenant geocoding entries from the DB (currently a no-op, logged by repo).
	if err := c.geoRepo.DeleteByTenant(ctx, evt.EmpresaID); err != nil {
		c.logger.Error("consumer: geoRepo.DeleteByTenant failed", "empresa_id", evt.EmpresaID, "error", err)
		return err
	}

	// 2. Purge the entire geocoding Redis cache (cross-tenant — intentional).
	c.logger.Warn("consumer: purging entire geocoding cache",
		"trigger", "empresa_de_baja",
		"empresa_id", evt.EmpresaID,
	)
	if err := c.geoCache.DeleteByPattern(ctx, "reports:geocoding:*"); err != nil {
		c.logger.Error("consumer: geoCache.DeleteByPattern failed", "error", err)
		return err
	}

	// 3. Publish eviction event so downstream can react.
	_ = c.pub.Publish(ctx, StreamGeocodingCacheEvicted, map[string]string{
		"empresa_id": evt.EmpresaID,
	})

	return nil
}
