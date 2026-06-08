package repository

import (
	"context"
	"fmt"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EvaluacionRepository defines persistence for EvaluacionServicio records.
type EvaluacionRepository interface {
	Create(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error
	ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error)
}

// PgxEvaluacionRepository is a pgx/v5 implementation of EvaluacionRepository.
type PgxEvaluacionRepository struct {
	pool *pgxpool.Pool
}

// NewPgxEvaluacionRepository returns a new PgxEvaluacionRepository.
func NewPgxEvaluacionRepository(pool *pgxpool.Pool) *PgxEvaluacionRepository {
	return &PgxEvaluacionRepository{pool: pool}
}

func (r *PgxEvaluacionRepository) Create(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_evaluacionservicio
		 (cliente_id, puntuacion, comentarios, de_usuario_id, fecha)
		 SELECT $1,$2,$3,$4,$5
		 FROM clientes.clientes_cliente c
		 WHERE c.id=$1 AND c.empresa_id=$6
		 RETURNING id, created_at, updated_at`,
		e.ClienteID, e.Puntuacion, e.Comentarios, e.DeUsuarioID, e.Fecha, empresaID).
		Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("evaluacionRepository.Create: %w", err)
	}
	return nil
}

func (r *PgxEvaluacionRepository) ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, e.cliente_id, e.puntuacion, e.comentarios, e.de_usuario_id, e.fecha, e.created_at, e.updated_at
		 FROM clientes.clientes_evaluacionservicio e
		 JOIN clientes.clientes_cliente c ON c.id = e.cliente_id
		 WHERE e.cliente_id=$1 AND c.empresa_id=$2
		 ORDER BY e.fecha DESC`,
		clienteID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("evaluacionRepository.ListByCliente: %w", err)
	}
	defer rows.Close()

	var out []clientes.EvaluacionServicio
	for rows.Next() {
		var e clientes.EvaluacionServicio
		if err := rows.Scan(&e.ID, &e.ClienteID, &e.Puntuacion, &e.Comentarios,
			&e.DeUsuarioID, &e.Fecha, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("evaluacionRepository.ListByCliente scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
