package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/bolsatrabajo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EvaluacionRepository defines persistence operations for post-interview evaluations.
type EvaluacionRepository interface {
	Create(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) error
	GetByPostulacion(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
}

// PgxEvaluacionRepository implements EvaluacionRepository using pgx pool.
type PgxEvaluacionRepository struct {
	pool *pgxpool.Pool
}

// NewPgxEvaluacionRepository constructs a PgxEvaluacionRepository.
func NewPgxEvaluacionRepository(pool *pgxpool.Pool) *PgxEvaluacionRepository {
	return &PgxEvaluacionRepository{pool: pool}
}

// Compile-time interface check.
var _ EvaluacionRepository = (*PgxEvaluacionRepository)(nil)

func (r *PgxEvaluacionRepository) Create(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) error {
	// Tenant check: verify the postulacion belongs to a trabajo owned by empresaID.
	var allowed bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1
		   FROM bolsa_trabajo.postulaciones p
		   JOIN bolsa_trabajo.trabajos t ON p.trabajo_id = t.id
		   WHERE p.id=$1 AND t.empresa_id=$2
		 )`,
		e.PostulacionID, empresaID,
	).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("PgxEvaluacionRepository.Create tenant check: %w", err)
	}
	if !allowed {
		return bolsatrabajo.ErrForbidden
	}

	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	err = r.pool.QueryRow(ctx,
		`INSERT INTO bolsa_trabajo.evaluaciones
		   (id, postulacion_id, puntualidad, cortesia, soft_skills, comentarios, evaluator_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING created_at`,
		e.ID, e.PostulacionID, e.Puntualidad, e.Cortesia, e.SoftSkills, e.Comentarios, e.EvaluatorID,
	).Scan(&e.CreatedAt)
	if err != nil {
		return mapError(err, "PgxEvaluacionRepository.Create")
	}
	return nil
}

func (r *PgxEvaluacionRepository) GetByPostulacion(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	// Triple-join tenant check: evaluacion → postulacion → trabajo → empresa_id.
	var ev bolsatrabajo.Evaluacion
	err := r.pool.QueryRow(ctx,
		`SELECT e.id, e.postulacion_id, e.puntualidad, e.cortesia, e.soft_skills, e.comentarios, e.evaluator_id, e.created_at
		 FROM bolsa_trabajo.evaluaciones e
		 JOIN bolsa_trabajo.postulaciones p ON e.postulacion_id = p.id
		 JOIN bolsa_trabajo.trabajos t ON p.trabajo_id = t.id
		 WHERE p.id=$1 AND t.empresa_id=$2`,
		postulacionID, empresaID,
	).Scan(&ev.ID, &ev.PostulacionID, &ev.Puntualidad, &ev.Cortesia, &ev.SoftSkills,
		&ev.Comentarios, &ev.EvaluatorID, &ev.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, bolsatrabajo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxEvaluacionRepository.GetByPostulacion: %w", err)
	}
	return &ev, nil
}
