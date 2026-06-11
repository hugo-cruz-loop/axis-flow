package dashboardsdata

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// dbHelper defines the minimum DB interface needed by events consumer to resolve client IDs.
type dbHelper interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// EventsConsumer subscribes to Redis Streams and evicts cached dashboard keys upon change events.
type EventsConsumer struct {
	rdb    *redis.Client
	cache  *CacheClient
	db     dbHelper
	stopCh chan struct{}
}

// NewEventsConsumer creates a new EventsConsumer instance.
func NewEventsConsumer(rdb *redis.Client, cache *CacheClient, db dbHelper) *EventsConsumer {
	return &EventsConsumer{
		rdb:    rdb,
		cache:  cache,
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start launches the background goroutines to consume event streams.
func (c *EventsConsumer) Start(ctx context.Context) {
	c.startAsignacionLoop(ctx)
	c.startAtencionLoop(ctx)
	c.startEmpleadosLoop(ctx)
}

// Stop signals all consumer loops to exit cleanly.
func (c *EventsConsumer) Stop() {
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
}

func (c *EventsConsumer) startAsignacionLoop(ctx context.Context) {
	go func() {
		lastID := "0"
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			default:
			}

			entries, err := c.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"asignacion:events", lastID},
				Count:   10,
				Block:   500 * time.Millisecond,
			}).Result()
			if err != nil {
				if err == redis.Nil || err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				slog.Error("EventsConsumer (asignacion): XRead error", slog.String("error", err.Error()))
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					lastID = msg.ID

					event, _ := msg.Values["event"].(string)
					if event != "AsignacionModificada" {
						continue
					}

					payloadStr, _ := msg.Values["payload"].(string)
					var payload struct {
						LocationID uuid.UUID `json:"LocationID"`
					}
					if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
						slog.Warn("EventsConsumer (asignacion): invalid payload JSON", slog.String("error", err.Error()))
						continue
					}

					if payload.LocationID == uuid.Nil {
						continue
					}

					var clientID uuid.UUID
					err = c.db.QueryRow(ctx, "SELECT cliente_id FROM clientes.clientes_localidad WHERE id = $1", payload.LocationID).Scan(&clientID)
					if err != nil {
						slog.Error("EventsConsumer (asignacion): failed to resolve client ID", slog.String("location_id", payload.LocationID.String()), slog.String("error", err.Error()))
						continue
					}

					key := c.cache.FormatKey("cliente", clientID.String(), "servicios-localidad")
					if err := c.cache.Delete(ctx, key); err != nil {
						slog.Error("EventsConsumer (asignacion): failed to evict cache", slog.String("key", key), slog.String("error", err.Error()))
					} else {
						slog.Info("EventsConsumer (asignacion): cache evicted successfully", slog.String("key", key))
					}
				}
			}
		}
	}()
}

func (c *EventsConsumer) startAtencionLoop(ctx context.Context) {
	go func() {
		lastID := "0"
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			default:
			}

			entries, err := c.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"atencion:ticket_servicio_actualizado", lastID},
				Count:   10,
				Block:   500 * time.Millisecond,
			}).Result()
			if err != nil {
				if err == redis.Nil || err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				slog.Error("EventsConsumer (atencion): XRead error", slog.String("error", err.Error()))
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					lastID = msg.ID

					ticketIDStr, _ := msg.Values["ticket_id"].(string)
					ticketID, err := uuid.Parse(ticketIDStr)
					if err != nil {
						slog.Warn("EventsConsumer (atencion): invalid ticket_id", slog.String("value", ticketIDStr))
						continue
					}

					var clientID uuid.UUID
					err = c.db.QueryRow(ctx, "SELECT cliente_id FROM atencion_seguimiento.tickets_servicio WHERE id = $1", ticketID).Scan(&clientID)
					if err != nil {
						slog.Error("EventsConsumer (atencion): failed to resolve client ID", slog.String("ticket_id", ticketID.String()), slog.String("error", err.Error()))
						continue
					}

					key := c.cache.FormatKey("cliente", clientID.String(), "atencion-seguimiento")
					if err := c.cache.Delete(ctx, key); err != nil {
						slog.Error("EventsConsumer (atencion): failed to evict cache", slog.String("key", key), slog.String("error", err.Error()))
					} else {
						slog.Info("EventsConsumer (atencion): cache evicted successfully", slog.String("key", key))
					}
				}
			}
		}
	}()
}

func (c *EventsConsumer) startEmpleadosLoop(ctx context.Context) {
	go func() {
		lastID := "0"
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			default:
			}

			entries, err := c.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"empleados:inasistencia_registrada", lastID},
				Count:   10,
				Block:   500 * time.Millisecond,
			}).Result()
			if err != nil {
				if err == redis.Nil || err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				slog.Error("EventsConsumer (empleados): XRead error", slog.String("error", err.Error()))
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					lastID = msg.ID

					empresaIDStr, _ := msg.Values["empresa_id"].(string)
					if empresaIDStr == "" {
						slog.Warn("EventsConsumer (empleados): missing empresa_id")
						continue
					}

					key := c.cache.FormatKey("rh", empresaIDStr, "absentismo")
					if err := c.cache.Delete(ctx, key); err != nil {
						slog.Error("EventsConsumer (empleados): failed to evict cache", slog.String("key", key), slog.String("error", err.Error()))
					} else {
						slog.Info("EventsConsumer (empleados): cache evicted successfully", slog.String("key", key))
					}
				}
			}
		}
	}()
}
