package events

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Stream name constants for bolsatrabajo domain events.
const (
	StreamVacanteCreada                 = "bolsa_trabajo:vacante_creada"
	StreamPostulacionRecibida           = "bolsa_trabajo:postulacion_recibida"
	StreamPostulacionEstatusActualizado = "bolsa_trabajo:postulacion_estatus_actualizado"
	StreamPostulacionEvaluada           = "bolsa_trabajo:postulacion_evaluada"
	StreamCandidatoContratado           = "bolsa_trabajo:candidato_contratado"
)

// RedisStreamPublisher implements EventPublisher using Redis Streams (XADD).
type RedisStreamPublisher struct {
	rdb *redis.Client
}

// NewRedisStreamPublisher constructs a RedisStreamPublisher.
func NewRedisStreamPublisher(rdb *redis.Client) *RedisStreamPublisher {
	return &RedisStreamPublisher{rdb: rdb}
}

// Publish sends the payload as an XADD to the given stream.
// Payload values are serialised as string key-value pairs.
// Secret values MUST NOT be included in payload.
func (p *RedisStreamPublisher) Publish(ctx context.Context, stream string, payload map[string]any) error {
	values := make(map[string]any, len(payload))
	for k, v := range payload {
		values[k] = fmt.Sprintf("%v", v)
	}
	args := &redis.XAddArgs{
		Stream: stream,
		ID:     "*", // auto-generate ID
		Values: values,
	}
	return p.rdb.XAdd(ctx, args).Err()
}

// Compile-time check.
var _ EventPublisher = (*RedisStreamPublisher)(nil)
