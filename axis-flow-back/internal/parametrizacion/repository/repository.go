package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/parametrizacion"
	paramservice "axis-flow-back/internal/parametrizacion/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SistemaUpsertSQL = `
INSERT INTO parametrizacion.parametrizacion_sistema (clave_parametro, valor, descripcion)
VALUES ($1, $2, $3)
ON CONFLICT (clave_parametro) DO UPDATE
SET valor = EXCLUDED.valor,
    descripcion = COALESCE(EXCLUDED.descripcion, parametrizacion.parametrizacion_sistema.descripcion),
    updated_at = CURRENT_TIMESTAMP
RETURNING created_at, updated_at`

const EvaluacionServicioUpsertSQL = `
INSERT INTO parametrizacion.parametrizacion_evaluacion_servicio (empresa_id, servicio_id, periodicidad_id, activa)
VALUES ($1, $2, $3, $4)
ON CONFLICT (empresa_id, servicio_id) DO UPDATE
SET periodicidad_id = EXCLUDED.periodicidad_id,
    activa = EXCLUDED.activa,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, created_at, updated_at`

const EvaluacionPersonalUpsertSQL = `
INSERT INTO parametrizacion.parametrizacion_evaluacion_personal (empresa_id, periodicidad_id, activa)
VALUES ($1, $2, $3)
ON CONFLICT (empresa_id) DO UPDATE
SET periodicidad_id = EXCLUDED.periodicidad_id,
    activa = EXCLUDED.activa,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, created_at, updated_at`

const DiasInactivosUmbralUpsertSQL = `
INSERT INTO parametrizacion.parametrizacion_dias_inactivos_umbral (empresa_id, umbral_dias)
VALUES ($1, $2)
ON CONFLICT (empresa_id) DO UPDATE
SET umbral_dias = EXCLUDED.umbral_dias,
    updated_at = CURRENT_TIMESTAMP
RETURNING created_at, updated_at`

const DiasInactivosDeleteSQL = `DELETE FROM parametrizacion.parametrizacion_dias_inactivos WHERE id=$1 AND empresa_id=$2`

type queryer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// PgxUnitOfWork opens one PostgreSQL transaction for application-service write boundaries.
type PgxUnitOfWork struct {
	pool *pgxpool.Pool
}

func NewPgxUnitOfWork(pool *pgxpool.Pool) *PgxUnitOfWork { return &PgxUnitOfWork{pool: pool} }

func (u *PgxUnitOfWork) WithinTx(ctx context.Context, fn func(context.Context, paramservice.Repositories) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("parametrizacion tx begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repos := NewServiceRepositories(tx)
	if err := fn(ctx, repos); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("parametrizacion tx commit: %w", err)
	}
	return nil
}

// Repositories groups transaction-bound repository adapters.
type Repositories struct {
	Sistema            parametrizacion.SistemaRepository
	EvaluacionServicio parametrizacion.EvaluacionServicioRepository
	EvaluacionPersonal parametrizacion.EvaluacionPersonalRepository
	DiasInactivos      parametrizacion.DiasInactivosRepository
}

func NewServiceRepositories(q queryer) paramservice.Repositories {
	repos := NewRepositories(q)
	return paramservice.Repositories{
		Sistema:            repos.Sistema,
		EvaluacionServicio: repos.EvaluacionServicio,
		EvaluacionPersonal: repos.EvaluacionPersonal,
		DiasInactivos:      repos.DiasInactivos,
	}
}

func NewRepositories(q queryer) Repositories {
	return Repositories{
		Sistema:            NewSistemaRepository(q),
		EvaluacionServicio: NewEvaluacionServicioRepository(q),
		EvaluacionPersonal: NewEvaluacionPersonalRepository(q),
		DiasInactivos:      NewDiasInactivosRepository(q),
	}
}

type SistemaRepository struct{ q queryer }

func NewSistemaRepository(q queryer) *SistemaRepository { return &SistemaRepository{q: q} }
func (r *SistemaRepository) Upsert(ctx context.Context, p *parametrizacion.SistemaParametro) error {
	if err := r.q.QueryRow(ctx, SistemaUpsertSQL, p.ClaveParametro, p.Valor, nullableString(p.Descripcion)).Scan(&p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("sistema upsert: %w", err)
	}
	return nil
}
func (r *SistemaRepository) GetByClave(ctx context.Context, clave string) (*parametrizacion.SistemaParametro, error) {
	const q = `SELECT clave_parametro, valor, COALESCE(descripcion,''), created_at, updated_at FROM parametrizacion.parametrizacion_sistema WHERE clave_parametro=$1`
	var p parametrizacion.SistemaParametro
	if err := r.q.QueryRow(ctx, q, clave).Scan(&p.ClaveParametro, &p.Valor, &p.Descripcion, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, mapNotFound("sistema get", err)
	}
	return &p, nil
}
func (r *SistemaRepository) List(ctx context.Context) ([]*parametrizacion.SistemaParametro, error) {
	rows, err := r.q.Query(ctx, `SELECT clave_parametro, valor, COALESCE(descripcion,''), created_at, updated_at FROM parametrizacion.parametrizacion_sistema ORDER BY clave_parametro`)
	if err != nil {
		return nil, fmt.Errorf("sistema list: %w", err)
	}
	defer rows.Close()
	var out []*parametrizacion.SistemaParametro
	for rows.Next() {
		var p parametrizacion.SistemaParametro
		if err := rows.Scan(&p.ClaveParametro, &p.Valor, &p.Descripcion, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("sistema list scan: %w", err)
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

type EvaluacionServicioRepository struct{ q queryer }

func NewEvaluacionServicioRepository(q queryer) *EvaluacionServicioRepository {
	return &EvaluacionServicioRepository{q: q}
}
func (r *EvaluacionServicioRepository) Upsert(ctx context.Context, c *parametrizacion.EvaluacionServicio) error {
	return r.q.QueryRow(ctx, EvaluacionServicioUpsertSQL, c.EmpresaID, c.ServicioID, c.PeriodicidadID, c.Activa).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}
func (r *EvaluacionServicioRepository) GetByEmpresaServicio(ctx context.Context, empresaID, servicioID int64) (*parametrizacion.EvaluacionServicio, error) {
	const q = `SELECT id, empresa_id, servicio_id, periodicidad_id, activa, created_at, updated_at FROM parametrizacion.parametrizacion_evaluacion_servicio WHERE empresa_id=$1 AND servicio_id=$2`
	var c parametrizacion.EvaluacionServicio
	if err := r.q.QueryRow(ctx, q, empresaID, servicioID).Scan(&c.ID, &c.EmpresaID, &c.ServicioID, &c.PeriodicidadID, &c.Activa, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, mapNotFound("evaluacion servicio get", err)
	}
	return &c, nil
}
func (r *EvaluacionServicioRepository) ListByEmpresa(ctx context.Context, empresaID int64) ([]*parametrizacion.EvaluacionServicio, error) {
	rows, err := r.q.Query(ctx, `SELECT id, empresa_id, servicio_id, periodicidad_id, activa, created_at, updated_at FROM parametrizacion.parametrizacion_evaluacion_servicio WHERE empresa_id=$1 ORDER BY servicio_id`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("evaluacion servicio list: %w", err)
	}
	defer rows.Close()
	var out []*parametrizacion.EvaluacionServicio
	for rows.Next() {
		var c parametrizacion.EvaluacionServicio
		if err := rows.Scan(&c.ID, &c.EmpresaID, &c.ServicioID, &c.PeriodicidadID, &c.Activa, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("evaluacion servicio scan: %w", err)
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

type EvaluacionPersonalRepository struct{ q queryer }

func NewEvaluacionPersonalRepository(q queryer) *EvaluacionPersonalRepository {
	return &EvaluacionPersonalRepository{q: q}
}
func (r *EvaluacionPersonalRepository) Upsert(ctx context.Context, c *parametrizacion.EvaluacionPersonal) error {
	return r.q.QueryRow(ctx, EvaluacionPersonalUpsertSQL, c.EmpresaID, c.PeriodicidadID, c.Activa).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}
func (r *EvaluacionPersonalRepository) GetByEmpresa(ctx context.Context, empresaID int64) (*parametrizacion.EvaluacionPersonal, error) {
	const q = `SELECT id, empresa_id, periodicidad_id, activa, created_at, updated_at FROM parametrizacion.parametrizacion_evaluacion_personal WHERE empresa_id=$1`
	var c parametrizacion.EvaluacionPersonal
	if err := r.q.QueryRow(ctx, q, empresaID).Scan(&c.ID, &c.EmpresaID, &c.PeriodicidadID, &c.Activa, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, mapNotFound("evaluacion personal get", err)
	}
	return &c, nil
}

type DiasInactivosRepository struct{ q queryer }

func NewDiasInactivosRepository(q queryer) *DiasInactivosRepository {
	return &DiasInactivosRepository{q: q}
}
func (r *DiasInactivosRepository) AddDia(ctx context.Context, d *parametrizacion.DiaInactivo) error {
	const q = `INSERT INTO parametrizacion.parametrizacion_dias_inactivos (empresa_id, fecha, descripcion) VALUES ($1,$2,$3) ON CONFLICT (empresa_id, fecha) DO UPDATE SET descripcion=EXCLUDED.descripcion, updated_at=CURRENT_TIMESTAMP RETURNING id, created_at, updated_at`
	return r.q.QueryRow(ctx, q, d.EmpresaID, d.Fecha, d.Descripcion).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}
func (r *DiasInactivosRepository) DeleteDia(ctx context.Context, id, empresaID int64) error {
	tag, err := r.q.Exec(ctx, DiasInactivosDeleteSQL, id, empresaID)
	if err != nil {
		return fmt.Errorf("dias delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return parametrizacion.ErrNotFound
	}
	return nil
}
func (r *DiasInactivosRepository) ListByEmpresaYear(ctx context.Context, empresaID int64, year int) ([]*parametrizacion.DiaInactivo, error) {
	rows, err := r.q.Query(ctx, `SELECT id, empresa_id, fecha, descripcion, created_at, updated_at FROM parametrizacion.parametrizacion_dias_inactivos WHERE empresa_id=$1 AND EXTRACT(YEAR FROM fecha)=$2 ORDER BY fecha`, empresaID, year)
	if err != nil {
		return nil, fmt.Errorf("dias list: %w", err)
	}
	defer rows.Close()
	var out []*parametrizacion.DiaInactivo
	for rows.Next() {
		var d parametrizacion.DiaInactivo
		if err := rows.Scan(&d.ID, &d.EmpresaID, &d.Fecha, &d.Descripcion, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("dias scan: %w", err)
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}
func (r *DiasInactivosRepository) UpsertUmbral(ctx context.Context, u *parametrizacion.DiasInactivosUmbral) error {
	return r.q.QueryRow(ctx, DiasInactivosUmbralUpsertSQL, u.EmpresaID, u.UmbralDias).Scan(&u.CreatedAt, &u.UpdatedAt)
}
func (r *DiasInactivosRepository) GetUmbral(ctx context.Context, empresaID int64) (*parametrizacion.DiasInactivosUmbral, error) {
	const q = `SELECT empresa_id, umbral_dias, created_at, updated_at FROM parametrizacion.parametrizacion_dias_inactivos_umbral WHERE empresa_id=$1`
	var u parametrizacion.DiasInactivosUmbral
	if err := r.q.QueryRow(ctx, q, empresaID).Scan(&u.EmpresaID, &u.UmbralDias, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &parametrizacion.DiasInactivosUmbral{EmpresaID: empresaID, UmbralDias: 5}, nil
		}
		return nil, fmt.Errorf("umbral get: %w", err)
	}
	return &u, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func mapNotFound(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return parametrizacion.ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}
