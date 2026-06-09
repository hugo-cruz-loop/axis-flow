// Package repository provides pgx-backed data access for the cursos domain.
package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/cursos"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// CatalogRepository defines the persistence contract for catalog entities.
type CatalogRepository interface {
	CreateCategoria(ctx context.Context, c *cursos.Categoria) error
	ListCategorias(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error)
	UpdateCategoria(ctx context.Context, c *cursos.Categoria) error
	DeleteCategoria(ctx context.Context, id int64, empresaID uuid.UUID) error
	CreateModulo(ctx context.Context, m *cursos.Modulo) error
	ListModulos(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error)
}

// PgxCatalogRepository implements CatalogRepository using pgx and Redis.
type PgxCatalogRepository struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewPgxCatalogRepository constructs a PgxCatalogRepository.
func NewPgxCatalogRepository(pool *pgxpool.Pool, rdb *redis.Client) *PgxCatalogRepository {
	return &PgxCatalogRepository{pool: pool, rdb: rdb}
}

func (r *PgxCatalogRepository) CreateCategoria(ctx context.Context, c *cursos.Categoria) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cursos.cursos_categoria (nombre, descripcion, empresa_id)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		c.Nombre, c.Descripcion, c.EmpresaID,
	).Scan(&c.ID)
	if err != nil {
		return mapPgError(err, "PgxCatalogRepository.CreateCategoria")
	}
	return nil
}

func (r *PgxCatalogRepository) ListCategorias(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, nombre, descripcion, empresa_id
		 FROM cursos.cursos_categoria
		 WHERE empresa_id = $1
		 ORDER BY id ASC`,
		empresaID,
	)
	if err != nil {
		return nil, fmt.Errorf("PgxCatalogRepository.ListCategorias: %w", err)
	}
	defer rows.Close()

	var out []*cursos.Categoria
	for rows.Next() {
		var c cursos.Categoria
		if err := rows.Scan(&c.ID, &c.Nombre, &c.Descripcion, &c.EmpresaID); err != nil {
			return nil, fmt.Errorf("PgxCatalogRepository.ListCategorias scan: %w", err)
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *PgxCatalogRepository) UpdateCategoria(ctx context.Context, c *cursos.Categoria) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE cursos.cursos_categoria
		 SET nombre=$1, descripcion=$2
		 WHERE id=$3 AND empresa_id=$4`,
		c.Nombre, c.Descripcion, c.ID, c.EmpresaID,
	)
	if err != nil {
		return mapPgError(err, "PgxCatalogRepository.UpdateCategoria")
	}
	if tag.RowsAffected() == 0 {
		return cursos.ErrCursoNotFound
	}
	return nil
}

func (r *PgxCatalogRepository) DeleteCategoria(ctx context.Context, id int64, empresaID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM cursos.cursos_categoria WHERE id=$1 AND empresa_id=$2`,
		id, empresaID,
	)
	if err != nil {
		return mapPgError(err, "PgxCatalogRepository.DeleteCategoria")
	}
	if tag.RowsAffected() == 0 {
		return cursos.ErrCursoNotFound
	}
	return nil
}

func (r *PgxCatalogRepository) CreateModulo(ctx context.Context, m *cursos.Modulo) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cursos.cursos_modulo (nombre, empresa_id)
		 VALUES ($1, $2)
		 RETURNING id`,
		m.Nombre, m.EmpresaID,
	).Scan(&m.ID)
	if err != nil {
		return mapPgError(err, "PgxCatalogRepository.CreateModulo")
	}
	return nil
}

func (r *PgxCatalogRepository) ListModulos(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, nombre, empresa_id
		 FROM cursos.cursos_modulo
		 WHERE empresa_id = $1
		 ORDER BY id ASC`,
		empresaID,
	)
	if err != nil {
		return nil, fmt.Errorf("PgxCatalogRepository.ListModulos: %w", err)
	}
	defer rows.Close()

	var out []*cursos.Modulo
	for rows.Next() {
		var m cursos.Modulo
		if err := rows.Scan(&m.ID, &m.Nombre, &m.EmpresaID); err != nil {
			return nil, fmt.Errorf("PgxCatalogRepository.ListModulos scan: %w", err)
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// mapPgError converts well-known pgconn error codes to domain sentinels.
func mapPgError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%s: unique constraint violation: %w", op, err)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%s: FK violation: %w", op, err)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return cursos.ErrCursoNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}
