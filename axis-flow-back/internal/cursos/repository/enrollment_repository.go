package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"axis-flow-back/internal/cursos"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	enrollmentTTL   = 86400 * time.Second  // 24 hours
	avanceLeccionTTL = 604800 * time.Second // 7 days
)

// EnrollmentRepository defines persistence for enrollment and progress tracking.
type EnrollmentRepository interface {
	Enroll(ctx context.Context, e *cursos.Enrollment) error
	GetEnrollment(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error)
	ListEnrollmentsByEmpleado(ctx context.Context, empleadoID int64) ([]*cursos.Enrollment, error)
	MarkLeccionCompleta(ctx context.Context, empleadoID, leccionID int64) error
	IsLeccionCompleta(ctx context.Context, empleadoID, leccionID int64, cursoID int64) (bool, error)
	GetNota(ctx context.Context, leccionID, empleadoID int64) (*cursos.Nota, error)
	UpsertNota(ctx context.Context, n *cursos.Nota) error
}

// PgxEnrollmentRepository implements EnrollmentRepository using pgx and Redis.
type PgxEnrollmentRepository struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewPgxEnrollmentRepository constructs a PgxEnrollmentRepository.
func NewPgxEnrollmentRepository(pool *pgxpool.Pool, rdb *redis.Client) *PgxEnrollmentRepository {
	return &PgxEnrollmentRepository{pool: pool, rdb: rdb}
}

func (r *PgxEnrollmentRepository) Enroll(ctx context.Context, e *cursos.Enrollment) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cursos.cursos_enroll_curso (curso_id, empleado_id, empresa_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (curso_id, empleado_id) DO NOTHING
		 RETURNING id, avance_porcentaje, estatus, enrolled_at`,
		e.CursoID, e.EmpleadoID, e.EmpresaID,
	).Scan(&e.ID, &e.AvancePorcentaje, &e.Estatus, &e.EnrolledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Conflict — row already existed, silently ignore
		return nil
	}
	if err != nil {
		return mapPgError(err, "PgxEnrollmentRepository.Enroll")
	}
	return nil
}

func (r *PgxEnrollmentRepository) GetEnrollment(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error) {
	var e cursos.Enrollment
	err := r.pool.QueryRow(ctx,
		`SELECT id, curso_id, empleado_id, empresa_id, avance_porcentaje, estatus, enrolled_at, completed_at
		 FROM cursos.cursos_enroll_curso
		 WHERE curso_id=$1 AND empleado_id=$2`,
		cursoID, empleadoID,
	).Scan(&e.ID, &e.CursoID, &e.EmpleadoID, &e.EmpresaID,
		&e.AvancePorcentaje, &e.Estatus, &e.EnrolledAt, &e.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, cursos.ErrNotEnrolled
	}
	if err != nil {
		return nil, fmt.Errorf("PgxEnrollmentRepository.GetEnrollment: %w", err)
	}
	return &e, nil
}

func (r *PgxEnrollmentRepository) ListEnrollmentsByEmpleado(ctx context.Context, empleadoID int64) ([]*cursos.Enrollment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, curso_id, empleado_id, empresa_id, avance_porcentaje, estatus, enrolled_at, completed_at
		 FROM cursos.cursos_enroll_curso
		 WHERE empleado_id=$1
		 ORDER BY enrolled_at DESC`,
		empleadoID,
	)
	if err != nil {
		return nil, fmt.Errorf("PgxEnrollmentRepository.ListEnrollmentsByEmpleado: %w", err)
	}
	defer rows.Close()

	var out []*cursos.Enrollment
	for rows.Next() {
		var e cursos.Enrollment
		if err := rows.Scan(&e.ID, &e.CursoID, &e.EmpleadoID, &e.EmpresaID,
			&e.AvancePorcentaje, &e.Estatus, &e.EnrolledAt, &e.CompletedAt); err != nil {
			return nil, fmt.Errorf("PgxEnrollmentRepository.ListEnrollmentsByEmpleado scan: %w", err)
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// MarkLeccionCompleta records lesson completion and UNLINKs the Redis cache key asynchronously.
func (r *PgxEnrollmentRepository) MarkLeccionCompleta(ctx context.Context, empleadoID, leccionID int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO cursos.cursos_avance_leccion (empleado_id, leccion_id)
		 VALUES ($1, $2)
		 ON CONFLICT (empleado_id, leccion_id) DO NOTHING`,
		empleadoID, leccionID,
	)
	if err != nil {
		return fmt.Errorf("PgxEnrollmentRepository.MarkLeccionCompleta: %w", err)
	}

	// Resolve curso_id for the Redis key
	var cursoID int64
	err = r.pool.QueryRow(ctx,
		`SELECT u.curso_id FROM cursos.cursos_unidad u
		 JOIN cursos.cursos_leccion l ON l.unidad_id = u.id
		 WHERE l.id = $1`,
		leccionID,
	).Scan(&cursoID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("PgxEnrollmentRepository.MarkLeccionCompleta resolve cursoID: %w", err)
	}
	if cursoID > 0 {
		cacheKey := fmt.Sprintf("cursos:avance_lecciones:%d:%d", empleadoID, cursoID)
		// UNLINK is async delete — does not block
		_ = r.rdb.Unlink(ctx, cacheKey)
	}
	return nil
}

// IsLeccionCompleta checks the Redis SET first; falls through to DB on cache miss.
func (r *PgxEnrollmentRepository) IsLeccionCompleta(ctx context.Context, empleadoID, leccionID int64, cursoID int64) (bool, error) {
	cacheKey := fmt.Sprintf("cursos:avance_lecciones:%d:%d", empleadoID, cursoID)
	member := fmt.Sprintf("%d", leccionID)

	val, err := r.rdb.SIsMember(ctx, cacheKey, member).Result()
	if err == nil && val {
		return true, nil
	}

	// DB fallback
	var exists bool
	err = r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM cursos.cursos_avance_leccion WHERE empleado_id=$1 AND leccion_id=$2)`,
		empleadoID, leccionID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("PgxEnrollmentRepository.IsLeccionCompleta: %w", err)
	}

	if exists {
		// Warm the Redis cache — add to SET with TTL
		pipe := r.rdb.Pipeline()
		pipe.SAdd(ctx, cacheKey, member)
		pipe.Expire(ctx, cacheKey, avanceLeccionTTL)
		_, _ = pipe.Exec(ctx)
	}
	return exists, nil
}

func (r *PgxEnrollmentRepository) GetNota(ctx context.Context, leccionID, empleadoID int64) (*cursos.Nota, error) {
	var n cursos.Nota
	err := r.pool.QueryRow(ctx,
		`SELECT id, leccion_id, empleado_id, contenido, updated_at
		 FROM cursos.cursos_notas
		 WHERE leccion_id=$1 AND empleado_id=$2`,
		leccionID, empleadoID,
	).Scan(&n.ID, &n.LeccionID, &n.EmpleadoID, &n.Contenido, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, cursos.ErrCursoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxEnrollmentRepository.GetNota: %w", err)
	}
	return &n, nil
}

// UpsertNota inserts or updates a nota for a given leccion+empleado pair.
func (r *PgxEnrollmentRepository) UpsertNota(ctx context.Context, n *cursos.Nota) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cursos.cursos_notas (leccion_id, empleado_id, contenido, updated_at)
		 VALUES ($1,$2,$3,NOW())
		 ON CONFLICT (leccion_id, empleado_id)
		 DO UPDATE SET contenido=EXCLUDED.contenido, updated_at=NOW()
		 RETURNING id, updated_at`,
		n.LeccionID, n.EmpleadoID, n.Contenido,
	).Scan(&n.ID, &n.UpdatedAt)
	if err != nil {
		return mapPgError(err, "PgxEnrollmentRepository.UpsertNota")
	}
	return nil
}
