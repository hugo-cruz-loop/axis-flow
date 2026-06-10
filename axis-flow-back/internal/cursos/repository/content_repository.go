package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/cursos"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ContentRepository defines persistence for curso content hierarchy.
type ContentRepository interface {
	CreateCurso(ctx context.Context, c *cursos.Curso) error
	GetCurso(ctx context.Context, id int64) (*cursos.Curso, error)
	ListCursos(ctx context.Context, empresaID uuid.UUID, includePrivate bool) ([]*cursos.Curso, error)
	UpdateCurso(ctx context.Context, c *cursos.Curso) error
	SoftDeleteCurso(ctx context.Context, id int64, empresaID uuid.UUID) error
	CreateUnidad(ctx context.Context, u *cursos.Unidad) error
	GetUnidadesByCurso(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)
	UpdateUnidad(ctx context.Context, u *cursos.Unidad) error
	DeleteUnidad(ctx context.Context, id int64) error
	CreateLeccion(ctx context.Context, l *cursos.Leccion) error
	GetLeccionesByUnidad(ctx context.Context, unidadID int64) ([]*cursos.Leccion, error)
	UpdateLeccion(ctx context.Context, l *cursos.Leccion) error
	DeleteLeccion(ctx context.Context, id int64) error
	GetCursoContenido(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)
}

// PgxContentRepository implements ContentRepository using pgx and Redis.
type PgxContentRepository struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewPgxContentRepository constructs a PgxContentRepository.
func NewPgxContentRepository(pool *pgxpool.Pool, rdb *redis.Client) *PgxContentRepository {
	return &PgxContentRepository{pool: pool, rdb: rdb}
}

func (r *PgxContentRepository) CreateCurso(ctx context.Context, c *cursos.Curso) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cursos.cursos_curso
		 (titulo, descripcion, empresa_id, modulo_id, categoria_id, estatus, imagen_url, duracion_minutos, prerequisito_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, created_at`,
		c.Titulo, c.Descripcion, c.EmpresaID, c.ModuloID, c.CategoriaID,
		c.Estatus, c.ImagenURL, c.DuracionMinutos, c.PrerequisitoID,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return mapPgError(err, "PgxContentRepository.CreateCurso")
	}
	return nil
}

func (r *PgxContentRepository) GetCurso(ctx context.Context, id int64) (*cursos.Curso, error) {
	var c cursos.Curso
	err := r.pool.QueryRow(ctx,
		`SELECT id, titulo, descripcion, empresa_id, modulo_id, categoria_id,
		        estatus, imagen_url, duracion_minutos, prerequisito_id, created_at, deleted_at
		 FROM cursos.cursos_curso
		 WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.Titulo, &c.Descripcion, &c.EmpresaID, &c.ModuloID, &c.CategoriaID,
		&c.Estatus, &c.ImagenURL, &c.DuracionMinutos, &c.PrerequisitoID, &c.CreatedAt, &c.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, cursos.ErrCursoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxContentRepository.GetCurso: %w", err)
	}
	if c.DeletedAt != nil {
		return nil, cursos.ErrCursoDeleted
	}
	return &c, nil
}

func (r *PgxContentRepository) ListCursos(ctx context.Context, empresaID uuid.UUID, includePrivate bool) ([]*cursos.Curso, error) {
	query := `SELECT id, titulo, descripcion, empresa_id, modulo_id, categoria_id,
	                 estatus, imagen_url, duracion_minutos, prerequisito_id, created_at, deleted_at
	          FROM cursos.cursos_curso
	          WHERE empresa_id = $1 AND deleted_at IS NULL`
	if !includePrivate {
		query += fmt.Sprintf(" AND estatus = %d", cursos.CursoEstatusPublico)
	}
	query += " ORDER BY id ASC"

	rows, err := r.pool.Query(ctx, query, empresaID)
	if err != nil {
		return nil, fmt.Errorf("PgxContentRepository.ListCursos: %w", err)
	}
	defer rows.Close()

	var out []*cursos.Curso
	for rows.Next() {
		var c cursos.Curso
		if err := rows.Scan(&c.ID, &c.Titulo, &c.Descripcion, &c.EmpresaID, &c.ModuloID, &c.CategoriaID,
			&c.Estatus, &c.ImagenURL, &c.DuracionMinutos, &c.PrerequisitoID, &c.CreatedAt, &c.DeletedAt); err != nil {
			return nil, fmt.Errorf("PgxContentRepository.ListCursos scan: %w", err)
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *PgxContentRepository) UpdateCurso(ctx context.Context, c *cursos.Curso) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE cursos.cursos_curso
		 SET titulo=$1, descripcion=$2, modulo_id=$3, categoria_id=$4,
		     estatus=$5, imagen_url=$6, duracion_minutos=$7, prerequisito_id=$8
		 WHERE id=$9 AND empresa_id=$10 AND deleted_at IS NULL`,
		c.Titulo, c.Descripcion, c.ModuloID, c.CategoriaID,
		c.Estatus, c.ImagenURL, c.DuracionMinutos, c.PrerequisitoID,
		c.ID, c.EmpresaID,
	)
	if err != nil {
		return mapPgError(err, "PgxContentRepository.UpdateCurso")
	}
	if tag.RowsAffected() == 0 {
		return cursos.ErrCursoNotFound
	}
	return nil
}

func (r *PgxContentRepository) SoftDeleteCurso(ctx context.Context, id int64, empresaID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE cursos.cursos_curso
		 SET deleted_at = NOW()
		 WHERE id=$1 AND empresa_id=$2 AND deleted_at IS NULL`,
		id, empresaID,
	)
	if err != nil {
		return fmt.Errorf("PgxContentRepository.SoftDeleteCurso: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return cursos.ErrCursoNotFound
	}
	return nil
}

func (r *PgxContentRepository) CreateUnidad(ctx context.Context, u *cursos.Unidad) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cursos.cursos_unidad (curso_id, titulo, orden)
		 VALUES ($1,$2,$3)
		 RETURNING id`,
		u.CursoID, u.Titulo, u.Orden,
	).Scan(&u.ID)
	if err != nil {
		return mapPgError(err, "PgxContentRepository.CreateUnidad")
	}
	return nil
}

func (r *PgxContentRepository) GetUnidadesByCurso(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, curso_id, titulo, orden
		 FROM cursos.cursos_unidad
		 WHERE curso_id = $1
		 ORDER BY orden ASC`,
		cursoID,
	)
	if err != nil {
		return nil, fmt.Errorf("PgxContentRepository.GetUnidadesByCurso: %w", err)
	}
	defer rows.Close()

	var out []*cursos.Unidad
	for rows.Next() {
		var u cursos.Unidad
		if err := rows.Scan(&u.ID, &u.CursoID, &u.Titulo, &u.Orden); err != nil {
			return nil, fmt.Errorf("PgxContentRepository.GetUnidadesByCurso scan: %w", err)
		}
		out = append(out, &u)
	}
	return out, rows.Err()
}

func (r *PgxContentRepository) UpdateUnidad(ctx context.Context, u *cursos.Unidad) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE cursos.cursos_unidad SET titulo=$1, orden=$2 WHERE id=$3`,
		u.Titulo, u.Orden, u.ID,
	)
	if err != nil {
		return fmt.Errorf("PgxContentRepository.UpdateUnidad: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return cursos.ErrCursoNotFound
	}
	return nil
}

func (r *PgxContentRepository) DeleteUnidad(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM cursos.cursos_unidad WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("PgxContentRepository.DeleteUnidad: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return cursos.ErrCursoNotFound
	}
	return nil
}

func (r *PgxContentRepository) CreateLeccion(ctx context.Context, l *cursos.Leccion) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cursos.cursos_leccion (unidad_id, titulo, tipo, contenido_url, orden)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id`,
		l.UnidadID, l.Titulo, l.Tipo, l.ContenidoURL, l.Orden,
	).Scan(&l.ID)
	if err != nil {
		return mapPgError(err, "PgxContentRepository.CreateLeccion")
	}
	return nil
}

func (r *PgxContentRepository) GetLeccionesByUnidad(ctx context.Context, unidadID int64) ([]*cursos.Leccion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, unidad_id, titulo, tipo, contenido_url, orden
		 FROM cursos.cursos_leccion
		 WHERE unidad_id = $1
		 ORDER BY orden ASC`,
		unidadID,
	)
	if err != nil {
		return nil, fmt.Errorf("PgxContentRepository.GetLeccionesByUnidad: %w", err)
	}
	defer rows.Close()

	var out []*cursos.Leccion
	for rows.Next() {
		var l cursos.Leccion
		if err := rows.Scan(&l.ID, &l.UnidadID, &l.Titulo, &l.Tipo, &l.ContenidoURL, &l.Orden); err != nil {
			return nil, fmt.Errorf("PgxContentRepository.GetLeccionesByUnidad scan: %w", err)
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

func (r *PgxContentRepository) UpdateLeccion(ctx context.Context, l *cursos.Leccion) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE cursos.cursos_leccion
		 SET titulo=$1, tipo=$2, contenido_url=$3, orden=$4
		 WHERE id=$5`,
		l.Titulo, l.Tipo, l.ContenidoURL, l.Orden, l.ID,
	)
	if err != nil {
		return fmt.Errorf("PgxContentRepository.UpdateLeccion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return cursos.ErrCursoNotFound
	}
	return nil
}

func (r *PgxContentRepository) DeleteLeccion(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM cursos.cursos_leccion WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("PgxContentRepository.DeleteLeccion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return cursos.ErrCursoNotFound
	}
	return nil
}

// GetCursoContenido eagerly loads all unidades and their lecciones for a curso.
func (r *PgxContentRepository) GetCursoContenido(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error) {
	unidades, err := r.GetUnidadesByCurso(ctx, cursoID)
	if err != nil {
		return nil, err
	}
	return unidades, nil
}
