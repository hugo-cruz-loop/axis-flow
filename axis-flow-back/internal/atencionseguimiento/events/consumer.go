package events

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	streamEmpleadoDeBaja  = "empleados:empleado_de_baja"
	streamClienteInactivo = "clientes:cliente_inactivo"
)

// QuejaServicer is the minimal interface the EmpleadoDeBajaConsumer depends on.
type QuejaServicer interface {
	SuspendQuejasByEmpleado(ctx context.Context, empleadoID int64) error
}

// TicketServicer is the minimal interface the ClienteInactivoConsumer depends on.
type TicketServicer interface {
	CloseTicketsByCliente(ctx context.Context, clienteID uuid.UUID) error
}

// EmpleadoDeBajaConsumer reads "empleados:empleado_de_baja" and calls
// QuejaServicer.SuspendQuejasByEmpleado for each event.
type EmpleadoDeBajaConsumer struct {
	rdb    *redis.Client
	svc    QuejaServicer
	stopCh chan struct{}
}

// NewEmpleadoDeBajaConsumer constructs an EmpleadoDeBajaConsumer.
func NewEmpleadoDeBajaConsumer(rdb *redis.Client, svc QuejaServicer) *EmpleadoDeBajaConsumer {
	return &EmpleadoDeBajaConsumer{
		rdb:    rdb,
		svc:    svc,
		stopCh: make(chan struct{}),
	}
}

// Start launches the consumer loop in a goroutine.
func (c *EmpleadoDeBajaConsumer) Start(ctx context.Context) {
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
				Streams: []string{streamEmpleadoDeBaja, lastID},
				Count:   10,
				Block:   500 * time.Millisecond,
			}).Result()
			if err != nil {
				if err == redis.Nil || err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				slog.Error("EmpleadoDeBajaConsumer: XRead error", slog.String("error", err.Error()))
				continue
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					lastID = msg.ID
					empleadoIDStr, _ := msg.Values["empleado_id"].(string)
					empleadoID, err := strconv.ParseInt(empleadoIDStr, 10, 64)
					if err != nil {
						slog.Warn("EmpleadoDeBajaConsumer: invalid empleado_id", slog.String("value", empleadoIDStr))
						continue
					}
					if err := c.svc.SuspendQuejasByEmpleado(ctx, empleadoID); err != nil {
						slog.Error("EmpleadoDeBajaConsumer: SuspendQuejasByEmpleado failed",
							slog.String("empleado_id", empleadoIDStr),
							slog.String("error", err.Error()),
						)
					}
				}
			}
		}
	}()
}

// Stop signals the consumer goroutine to exit.
func (c *EmpleadoDeBajaConsumer) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

// ClienteInactivoConsumer reads "clientes:cliente_inactivo" and calls
// TicketServicer.CloseTicketsByCliente for each event.
type ClienteInactivoConsumer struct {
	rdb    *redis.Client
	svc    TicketServicer
	stopCh chan struct{}
}

// NewClienteInactivoConsumer constructs a ClienteInactivoConsumer.
func NewClienteInactivoConsumer(rdb *redis.Client, svc TicketServicer) *ClienteInactivoConsumer {
	return &ClienteInactivoConsumer{
		rdb:    rdb,
		svc:    svc,
		stopCh: make(chan struct{}),
	}
}

// Start launches the consumer loop in a goroutine.
func (c *ClienteInactivoConsumer) Start(ctx context.Context) {
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
				Streams: []string{streamClienteInactivo, lastID},
				Count:   10,
				Block:   500 * time.Millisecond,
			}).Result()
			if err != nil {
				if err == redis.Nil || err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				slog.Error("ClienteInactivoConsumer: XRead error", slog.String("error", err.Error()))
				continue
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					lastID = msg.ID
					clienteIDStr, _ := msg.Values["cliente_id"].(string)
					clienteID, err := uuid.Parse(clienteIDStr)
					if err != nil {
						slog.Warn("ClienteInactivoConsumer: invalid cliente_id", slog.String("value", clienteIDStr))
						continue
					}
					if err := c.svc.CloseTicketsByCliente(ctx, clienteID); err != nil {
						slog.Error("ClienteInactivoConsumer: CloseTicketsByCliente failed",
							slog.String("cliente_id", clienteIDStr),
							slog.String("error", err.Error()),
						)
					}
				}
			}
		}
	}()
}

// Stop signals the consumer goroutine to exit.
func (c *ClienteInactivoConsumer) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}
