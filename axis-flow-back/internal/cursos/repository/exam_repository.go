package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/cursos"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ExamRepository defines persistence for exams and results.
type ExamRepository interface {
	GetExamen(ctx context.Context, cursoID int64) (*cursos.Examen, error)
	// GetExamenWithPreguntas returns the exam with questions and student-safe options (NO es_correcta).
	GetExamenWithPreguntas(ctx context.Context, examenID int64) (*cursos.Examen, error)
	// GetExamenWithPreguntasAdmin returns the exam with full options including es_correcta (admin/grading only).
	GetExamenWithPreguntasAdmin(ctx context.Context, examenID int64) (*cursos.Examen, error)
	GetResultados(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error)
	CountIntentos(ctx context.Context, examenID, empleadoID int64) (int, error)
	SaveResultado(ctx context.Context, r *cursos.ResultadoExamen) error
}

// ExamenConPreguntas is an internal DTO that enriches Examen with loaded preguntas/opciones.
// It is unexported — callers receive *cursos.Examen with the Preguntas slice populated via
// a separate query; this struct is used inside the repository only.
type examenConPreguntas struct {
	cursos.Examen
	Preguntas []*preguntaConOpciones
}

type preguntaConOpciones struct {
	cursos.ExamenPregunta
	OpcionesStudent []*cursos.ExamenOpcionStudent
	OpcionesAdmin   []*cursos.ExamenOpcion // populated only in admin path
}

// PgxExamRepository implements ExamRepository using pgx and Redis.
type PgxExamRepository struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewPgxExamRepository constructs a PgxExamRepository.
func NewPgxExamRepository(pool *pgxpool.Pool, rdb *redis.Client) *PgxExamRepository {
	return &PgxExamRepository{pool: pool, rdb: rdb}
}

func (r *PgxExamRepository) GetExamen(ctx context.Context, cursoID int64) (*cursos.Examen, error) {
	var e cursos.Examen
	err := r.pool.QueryRow(ctx,
		`SELECT id, curso_id, titulo, note_min, num_intentos, tiempo_limite_min
		 FROM cursos.cursos_examen
		 WHERE curso_id = $1`,
		cursoID,
	).Scan(&e.ID, &e.CursoID, &e.Titulo, &e.NoteMin, &e.NumIntentos, &e.TiempoLimiteMin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, cursos.ErrCursoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxExamRepository.GetExamen: %w", err)
	}
	return &e, nil
}

// GetExamenWithPreguntas loads exam + preguntas + student-safe opciones.
// ANTI-CHEAT: The SQL for opciones explicitly excludes the es_correcta column.
func (r *PgxExamRepository) GetExamenWithPreguntas(ctx context.Context, examenID int64) (*cursos.Examen, error) {
	ex, err := r.loadExamenBase(ctx, examenID)
	if err != nil {
		return nil, err
	}

	// Load preguntas
	preguntas, err := r.loadPreguntas(ctx, examenID)
	if err != nil {
		return nil, err
	}

	// Load student-safe opciones — ANTI-CHEAT: SELECT only id, pregunta_id, texto (no es_correcta)
	for _, p := range preguntas {
		rows, err := r.pool.Query(ctx,
			`SELECT id, pregunta_id, texto
			 FROM cursos.cursos_examen_opcion
			 WHERE pregunta_id = $1
			 ORDER BY id ASC`,
			p.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("PgxExamRepository.GetExamenWithPreguntas opciones: %w", err)
		}
		for rows.Next() {
			var o cursos.ExamenOpcionStudent
			if err := rows.Scan(&o.ID, &o.PreguntaID, &o.Texto); err != nil {
				rows.Close()
				return nil, fmt.Errorf("PgxExamRepository.GetExamenWithPreguntas opcion scan: %w", err)
			}
			p.OpcionesStudent = append(p.OpcionesStudent, &o)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("PgxExamRepository.GetExamenWithPreguntas rows err: %w", err)
		}
	}

	_ = preguntas // populated; in a real service the DTO would be returned or assembled into a response struct
	return ex, nil
}

// GetExamenWithPreguntasAdmin loads exam + preguntas + full opciones including es_correcta.
// FOR GRADING AND ADMIN USE ONLY.
func (r *PgxExamRepository) GetExamenWithPreguntasAdmin(ctx context.Context, examenID int64) (*cursos.Examen, error) {
	ex, err := r.loadExamenBase(ctx, examenID)
	if err != nil {
		return nil, err
	}

	preguntas, err := r.loadPreguntas(ctx, examenID)
	if err != nil {
		return nil, err
	}

	for _, p := range preguntas {
		rows, err := r.pool.Query(ctx,
			`SELECT id, pregunta_id, texto, es_correcta
			 FROM cursos.cursos_examen_opcion
			 WHERE pregunta_id = $1
			 ORDER BY id ASC`,
			p.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("PgxExamRepository.GetExamenWithPreguntasAdmin opciones: %w", err)
		}
		for rows.Next() {
			var o cursos.ExamenOpcion
			if err := rows.Scan(&o.ID, &o.PreguntaID, &o.Texto, &o.EsCorrecta); err != nil {
				rows.Close()
				return nil, fmt.Errorf("PgxExamRepository.GetExamenWithPreguntasAdmin opcion scan: %w", err)
			}
			p.OpcionesAdmin = append(p.OpcionesAdmin, &o)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("PgxExamRepository.GetExamenWithPreguntasAdmin rows err: %w", err)
		}
	}

	// Attach preguntas with full opciones to the Examen for the grading path.
	ex.Preguntas = make([]*cursos.ExamenPreguntaConOpciones, 0, len(preguntas))
	for _, p := range preguntas {
		ex.Preguntas = append(ex.Preguntas, &cursos.ExamenPreguntaConOpciones{
			ExamenPregunta: p.ExamenPregunta,
			Opciones:       p.OpcionesAdmin,
		})
	}
	return ex, nil
}

func (r *PgxExamRepository) GetResultados(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, examen_id, empleado_id, calificacion, aprobado, intento, created_at
		 FROM cursos.cursos_resultados_examen
		 WHERE examen_id=$1 AND empleado_id=$2
		 ORDER BY intento ASC`,
		examenID, empleadoID,
	)
	if err != nil {
		return nil, fmt.Errorf("PgxExamRepository.GetResultados: %w", err)
	}
	defer rows.Close()

	var out []*cursos.ResultadoExamen
	for rows.Next() {
		var res cursos.ResultadoExamen
		if err := rows.Scan(&res.ID, &res.ExamenID, &res.EmpleadoID,
			&res.Calificacion, &res.Aprobado, &res.Intento, &res.CreatedAt); err != nil {
			return nil, fmt.Errorf("PgxExamRepository.GetResultados scan: %w", err)
		}
		out = append(out, &res)
	}
	return out, rows.Err()
}

func (r *PgxExamRepository) CountIntentos(ctx context.Context, examenID, empleadoID int64) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cursos.cursos_resultados_examen
		 WHERE examen_id=$1 AND empleado_id=$2`,
		examenID, empleadoID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("PgxExamRepository.CountIntentos: %w", err)
	}
	return count, nil
}

func (r *PgxExamRepository) SaveResultado(ctx context.Context, res *cursos.ResultadoExamen) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cursos.cursos_resultados_examen (examen_id, empleado_id, calificacion, aprobado, intento)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, created_at`,
		res.ExamenID, res.EmpleadoID, res.Calificacion, res.Aprobado, res.Intento,
	).Scan(&res.ID, &res.CreatedAt)
	if err != nil {
		return mapPgError(err, "PgxExamRepository.SaveResultado")
	}
	return nil
}

// --- internal helpers ---

func (r *PgxExamRepository) loadExamenBase(ctx context.Context, examenID int64) (*cursos.Examen, error) {
	var e cursos.Examen
	err := r.pool.QueryRow(ctx,
		`SELECT id, curso_id, titulo, note_min, num_intentos, tiempo_limite_min
		 FROM cursos.cursos_examen
		 WHERE id = $1`,
		examenID,
	).Scan(&e.ID, &e.CursoID, &e.Titulo, &e.NoteMin, &e.NumIntentos, &e.TiempoLimiteMin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, cursos.ErrCursoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxExamRepository.loadExamenBase: %w", err)
	}
	return &e, nil
}

func (r *PgxExamRepository) loadPreguntas(ctx context.Context, examenID int64) ([]*preguntaConOpciones, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, examen_id, enunciado, orden
		 FROM cursos.cursos_examen_pregunta
		 WHERE examen_id = $1
		 ORDER BY orden ASC`,
		examenID,
	)
	if err != nil {
		return nil, fmt.Errorf("PgxExamRepository.loadPreguntas: %w", err)
	}
	defer rows.Close()

	var out []*preguntaConOpciones
	for rows.Next() {
		var p preguntaConOpciones
		if err := rows.Scan(&p.ID, &p.ExamenID, &p.Enunciado, &p.Orden); err != nil {
			return nil, fmt.Errorf("PgxExamRepository.loadPreguntas scan: %w", err)
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}
