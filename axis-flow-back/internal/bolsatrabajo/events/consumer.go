package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const streamEmpresaDeBaja = "empresas:empresa_de_baja"

// TrabajoServicer is the minimal interface the consumer depends on.
// It is defined here to avoid an import cycle with the service package.
type TrabajoServicer interface {
	CloseAllByEmpresa(ctx context.Context, empresaID uuid.UUID) error
}

// EmpresaDeBajaConsumer reads from the Redis Stream "empresas:empresa_de_baja"
// and calls TrabajoServicer.CloseAllByEmpresa for each event.
type EmpresaDeBajaConsumer struct {
	rdb     *redis.Client
	service TrabajoServicer
	stopCh  chan struct{}
}

// NewEmpresaDeBajaConsumer constructs an EmpresaDeBajaConsumer.
func NewEmpresaDeBajaConsumer(rdb *redis.Client, svc TrabajoServicer) *EmpresaDeBajaConsumer {
	return &EmpresaDeBajaConsumer{
		rdb:     rdb,
		service: svc,
		stopCh:  make(chan struct{}),
	}
}

// Start launches the consumer loop in a goroutine.
// The loop exits when ctx is cancelled or Stop is called.
func (c *EmpresaDeBajaConsumer) Start(ctx context.Context) {
	go func() {
		lastID := "0" // read from the beginning on first start
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			default:
			}

			entries, err := c.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{streamEmpresaDeBaja, lastID},
				Count:   10,
				Block:   500 * time.Millisecond,
			}).Result()
			if err != nil {
				if err == redis.Nil || err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				slog.Error("EmpresaDeBajaConsumer: XRead error", slog.String("error", err.Error()))
				continue
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					lastID = msg.ID
					empresaIDStr, _ := msg.Values["empresa_id"].(string)
					empresaID, err := uuid.Parse(empresaIDStr)
					if err != nil {
						slog.Warn("EmpresaDeBajaConsumer: invalid empresa_id", slog.String("value", empresaIDStr))
						continue
					}
					if err := c.service.CloseAllByEmpresa(ctx, empresaID); err != nil {
						slog.Error("EmpresaDeBajaConsumer: CloseAllByEmpresa failed",
							slog.String("empresa_id", empresaIDStr),
							slog.String("error", err.Error()),
						)
					}
				}
			}
		}
	}()
}

// Stop signals the consumer goroutine to exit.
func (c *EmpresaDeBajaConsumer) Stop() {
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
}
