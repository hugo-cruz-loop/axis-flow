package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"axis-flow-back/internal/bolsatrabajo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TrabajoRepository defines persistence operations for job postings.
type TrabajoRepository interface {
	Create(ctx context.Context, t *bolsatrabajo.Trabajo) error
	GetActiveJobs(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	GetByEmpresa(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	GetRecent(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	SwitchEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error)
}

// PgxTrabajoRepository implements TrabajoRepository using pgx pool.
type PgxTrabajoRepository struct {
	pool *pgxpool.Pool
}

// NewPgxTrabajoRepository constructs a PgxTrabajoRepository.
func NewPgxTrabajoRepository(pool *pgxpool.Pool) *PgxTrabajoRepository {
	return &PgxTrabajoRepository{pool: pool}
}

// Compile-time interface check.
var _ TrabajoRepository = (*PgxTrabajoRepository)(nil)

func (r *PgxTrabajoRepository) Create(ctx context.Context, t *bolsatrabajo.Trabajo) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO bolsa_trabajo.trabajos
		   (id, empresa_id, titulo, descripcion, requisitos, fecha_caducar, estatus_vacante)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING created_at, updated_at`,
		t.ID, t.EmpresaID, t.Titulo, t.Descripcion, t.Requisitos, t.FechaCaducar, t.EstatusVacante,
	).Scan(&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return mapError(err, "PgxTrabajoRepository.Create")
	}
	return nil
}

func (r *PgxTrabajoRepository) GetActiveJobs(
	ctx context.Context,
	search string,
	empresaID *uuid.UUID,
	page, pageSize int,
) ([]*bolsatrabajo.Trabajo, int, error) {
	args := []any{bolsatrabajo.VacanteActivo, time.Now()}
	conds := []string{"estatus_vacante = $1", "fecha_caducar >= $2"}

	if search != "" {
		args = append(args, "%"+search+"%")
		idx := len(args)
		conds = append(conds, fmt.Sprintf("(titulo ILIKE $%d OR descripcion ILIKE $%d)", idx, idx))
	}
	if empresaID != nil {
		args = append(args, *empresaID)
		conds = append(conds, fmt.Sprintf("empresa_id = $%d", len(args)))
	}

	where := "WHERE " + strings.Join(conds, " AND ")
	base := "FROM bolsa_trabajo.trabajos " + where

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+base, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("PgxTrabajoRepository.GetActiveJobs count: %w", err)
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	limitClause := fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.pool.Query(ctx,
		"SELECT id, empresa_id, titulo, descripcion, requisitos, fecha_caducar, estatus_vacante, created_at, updated_at "+base+limitClause,
		args...)
	if err != nil {
		return nil, 0, fmt.Errorf("PgxTrabajoRepository.GetActiveJobs: %w", err)
	}
	defer rows.Close()

	return scanTrabajos(rows)
}

func (r *PgxTrabajoRepository) GetByEmpresa(
	ctx context.Context,
	empresaID uuid.UUID,
	filter, page, pageSize int,
) ([]*bolsatrabajo.Trabajo, int, error) {
	args := []any{empresaID}
	conds := []string{"empresa_id = $1"}

	switch filter {
	case 1:
		conds = append(conds, "estatus_vacante = $2")
		args = append(args, bolsatrabajo.VacanteActivo)
	case 2:
		conds = append(conds, "estatus_vacante = $2")
		args = append(args, bolsatrabajo.VacantePausa)
	}

	where := "WHERE " + strings.Join(conds, " AND ")
	base := "FROM bolsa_trabajo.trabajos " + where

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+base, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("PgxTrabajoRepository.GetByEmpresa count: %w", err)
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	limitClause := fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.pool.Query(ctx,
		"SELECT id, empresa_id, titulo, descripcion, requisitos, fecha_caducar, estatus_vacante, created_at, updated_at "+base+limitClause,
		args...)
	if err != nil {
		return nil, 0, fmt.Errorf("PgxTrabajoRepository.GetByEmpresa: %w", err)
	}
	defer rows.Close()

	return scanTrabajos(rows)
}

func (r *PgxTrabajoRepository) GetRecent(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	base := "FROM bolsa_trabajo.trabajos WHERE created_at >= $1"

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+base, cutoff).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("PgxTrabajoRepository.GetRecent count: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx,
		"SELECT id, empresa_id, titulo, descripcion, requisitos, fecha_caducar, estatus_vacante, created_at, updated_at "+
			base+" ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		cutoff, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("PgxTrabajoRepository.GetRecent: %w", err)
	}
	defer rows.Close()

	return scanTrabajos(rows)
}

func (r *PgxTrabajoRepository) SwitchEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error) {
	// Check existence first to distinguish not-found from forbidden.
	var ownerID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT empresa_id FROM bolsa_trabajo.trabajos WHERE id = $1`, id,
	).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, bolsatrabajo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxTrabajoRepository.SwitchEstatus fetch: %w", err)
	}
	if ownerID != empresaID {
		return nil, bolsatrabajo.ErrForbidden
	}

	var t bolsatrabajo.Trabajo
	err = r.pool.QueryRow(ctx,
		`UPDATE bolsa_trabajo.trabajos
		 SET estatus_vacante=$1, updated_at=now()
		 WHERE id=$2
		 RETURNING id, empresa_id, titulo, descripcion, requisitos, fecha_caducar, estatus_vacante, created_at, updated_at`,
		newEstatus, id,
	).Scan(&t.ID, &t.EmpresaID, &t.Titulo, &t.Descripcion, &t.Requisitos, &t.FechaCaducar,
		&t.EstatusVacante, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("PgxTrabajoRepository.SwitchEstatus update: %w", err)
	}
	return &t, nil
}

// scanTrabajos scans rows into a slice of Trabajo pointers.
func scanTrabajos(rows pgx.Rows) ([]*bolsatrabajo.Trabajo, int, error) {
	var out []*bolsatrabajo.Trabajo
	for rows.Next() {
		var t bolsatrabajo.Trabajo
		if err := rows.Scan(&t.ID, &t.EmpresaID, &t.Titulo, &t.Descripcion, &t.Requisitos,
			&t.FechaCaducar, &t.EstatusVacante, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanTrabajos: %w", err)
		}
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("scanTrabajos rows: %w", err)
	}
	return out, len(out), nil
}

