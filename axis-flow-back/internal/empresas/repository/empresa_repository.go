package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"axis-flow-back/internal/empresas"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxEmpresaRepository implements empresa persistence using pgx directly.
type PgxEmpresaRepository struct{ pool *pgxpool.Pool }

func NewPgxEmpresaRepository(pool *pgxpool.Pool) *PgxEmpresaRepository {
	return &PgxEmpresaRepository{pool: pool}
}

func (r *PgxEmpresaRepository) Create(ctx context.Context, e *empresas.Empresa) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO empresas.empresas_empresa (nombre, direccion, telefono, representante_id, plan_id, status)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, created_at, updated_at`,
		e.Nombre, e.Direccion, e.Telefono, e.RepresentanteID, e.PlanID, e.Status).
		Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if isFKViolation(err) {
			return empresas.ErrEmpresaHasDependents
		}
		return fmt.Errorf("PgxEmpresaRepository.Create: %w", err)
	}
	return nil
}

func (r *PgxEmpresaRepository) FindByID(ctx context.Context, id int64) (*empresas.Empresa, error) {
	var e empresas.Empresa
	err := r.pool.QueryRow(ctx,
		`SELECT id, nombre, direccion, telefono, representante_id, plan_id, status, vigencia, created_at, updated_at
		 FROM empresas.empresas_empresa WHERE id=$1`, id).
		Scan(&e.ID, &e.Nombre, &e.Direccion, &e.Telefono, &e.RepresentanteID,
			&e.PlanID, &e.Status, &e.Vigencia, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empresas.ErrEmpresaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxEmpresaRepository.FindByID: %w", err)
	}
	return &e, nil
}

func (r *PgxEmpresaRepository) Update(ctx context.Context, e *empresas.Empresa) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE empresas.empresas_empresa
		 SET nombre=$1, direccion=$2, telefono=$3, updated_at=NOW()
		 WHERE id=$4`,
		e.Nombre, e.Direccion, e.Telefono, e.ID)
	if err != nil {
		return fmt.Errorf("PgxEmpresaRepository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return empresas.ErrEmpresaNotFound
	}
	return nil
}

func (r *PgxEmpresaRepository) UpdateStatus(ctx context.Context, id int64, status empresas.EmpresaStatus, vigencia *time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE empresas.empresas_empresa
		 SET status=$1, vigencia=$2, updated_at=NOW()
		 WHERE id=$3`,
		status, vigencia, id)
	if err != nil {
		return fmt.Errorf("PgxEmpresaRepository.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return empresas.ErrEmpresaNotFound
	}
	return nil
}

// Delete removes an empresa and all sub-resources in a single transaction.
// Cascade order: servicios → apoderados → datosfiscales → pagos → empresa.
func (r *PgxEmpresaRepository) Delete(ctx context.Context, id int64) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("PgxEmpresaRepository.Delete begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	steps := []string{
		`DELETE FROM empresas.empresas_servicio WHERE empresa_id=$1`,
		`DELETE FROM empresas.empresas_apoderado WHERE empresa_id=$1`,
		`DELETE FROM empresas.empresas_datosfiscales WHERE empresa_id=$1`,
		`DELETE FROM empresas.empresas_pagos WHERE empresa_id=$1`,
		`DELETE FROM empresas.empresas_empresa WHERE id=$1`,
	}
	for _, q := range steps {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return fmt.Errorf("PgxEmpresaRepository.Delete: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("PgxEmpresaRepository.Delete commit: %w", err)
	}
	return nil
}

func (r *PgxEmpresaRepository) ListAll(ctx context.Context) ([]empresas.Empresa, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, nombre, direccion, telefono, representante_id, plan_id, status, vigencia, created_at, updated_at
		 FROM empresas.empresas_empresa ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("PgxEmpresaRepository.ListAll: %w", err)
	}
	defer rows.Close()

	var out []empresas.Empresa
	for rows.Next() {
		var e empresas.Empresa
		if err := rows.Scan(&e.ID, &e.Nombre, &e.Direccion, &e.Telefono, &e.RepresentanteID,
			&e.PlanID, &e.Status, &e.Vigencia, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("PgxEmpresaRepository.ListAll scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
