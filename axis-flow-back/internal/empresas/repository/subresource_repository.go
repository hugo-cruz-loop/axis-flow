package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/empresas"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---- PagoRepository ----

type PgxPagoRepository struct{ pool *pgxpool.Pool }

func NewPgxPagoRepository(pool *pgxpool.Pool) *PgxPagoRepository {
	return &PgxPagoRepository{pool: pool}
}

func (r *PgxPagoRepository) Create(ctx context.Context, p *empresas.Pago) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO empresas.empresas_pagos (empresa_id, token_pago, stripe_session_id, estatus_pago, monto)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, created_at, updated_at`,
		p.EmpresaID, p.TokenPago, p.StripeSessionID, p.EstatusPago, p.Monto).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return empresas.ErrDuplicateTokenPago
		}
		if isFKViolation(err) {
			return empresas.ErrEmpresaHasDependents
		}
		return fmt.Errorf("PgxPagoRepository.Create: %w", err)
	}
	return nil
}

func (r *PgxPagoRepository) FindByToken(ctx context.Context, token string) (*empresas.Pago, error) {
	var p empresas.Pago
	err := r.pool.QueryRow(ctx,
		`SELECT id, empresa_id, token_pago, stripe_session_id, estatus_pago, monto, created_at, updated_at
		 FROM empresas.empresas_pagos WHERE token_pago=$1`, token).
		Scan(&p.ID, &p.EmpresaID, &p.TokenPago, &p.StripeSessionID, &p.EstatusPago, &p.Monto, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empresas.ErrEmpresaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxPagoRepository.FindByToken: %w", err)
	}
	return &p, nil
}

func (r *PgxPagoRepository) FindByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Pago, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, empresa_id, token_pago, stripe_session_id, estatus_pago, monto, created_at, updated_at
		 FROM empresas.empresas_pagos WHERE empresa_id=$1 ORDER BY id ASC`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("PgxPagoRepository.FindByEmpresaID: %w", err)
	}
	defer rows.Close()

	var out []empresas.Pago
	for rows.Next() {
		var p empresas.Pago
		if err := rows.Scan(&p.ID, &p.EmpresaID, &p.TokenPago, &p.StripeSessionID, &p.EstatusPago, &p.Monto, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("PgxPagoRepository.FindByEmpresaID scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PgxPagoRepository) UpdateStatus(ctx context.Context, id int64, status empresas.PagoStatus, stripeSessionID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE empresas.empresas_pagos
		 SET estatus_pago=$1, stripe_session_id=$2, updated_at=NOW()
		 WHERE id=$3`,
		status, stripeSessionID, id)
	if err != nil {
		return fmt.Errorf("PgxPagoRepository.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return empresas.ErrEmpresaNotFound
	}
	return nil
}

// ---- DatosFiscalesRepository ----

type PgxDatosFiscalesRepository struct{ pool *pgxpool.Pool }

func NewPgxDatosFiscalesRepository(pool *pgxpool.Pool) *PgxDatosFiscalesRepository {
	return &PgxDatosFiscalesRepository{pool: pool}
}

func (r *PgxDatosFiscalesRepository) Create(ctx context.Context, d *empresas.DatosFiscales) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO empresas.empresas_datosfiscales (empresa_id, rfc, razon_social, logo_url, imss_patronal, repse)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, created_at, updated_at`,
		d.EmpresaID, d.RFC, d.RazonSocial, d.LogoURL, d.IMSSPatronal, d.REPSE).
		Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return empresas.ErrDuplicateRFC
		}
		if isFKViolation(err) {
			return empresas.ErrEmpresaHasDependents
		}
		return fmt.Errorf("PgxDatosFiscalesRepository.Create: %w", err)
	}
	return nil
}

func (r *PgxDatosFiscalesRepository) FindByEmpresaID(ctx context.Context, empresaID int64) (*empresas.DatosFiscales, error) {
	var d empresas.DatosFiscales
	err := r.pool.QueryRow(ctx,
		`SELECT id, empresa_id, rfc, razon_social, logo_url, imss_patronal, repse, created_at, updated_at
		 FROM empresas.empresas_datosfiscales WHERE empresa_id=$1`, empresaID).
		Scan(&d.ID, &d.EmpresaID, &d.RFC, &d.RazonSocial, &d.LogoURL, &d.IMSSPatronal, &d.REPSE, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empresas.ErrEmpresaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxDatosFiscalesRepository.FindByEmpresaID: %w", err)
	}
	return &d, nil
}

func (r *PgxDatosFiscalesRepository) Update(ctx context.Context, d *empresas.DatosFiscales) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE empresas.empresas_datosfiscales
		 SET razon_social=$1, logo_url=$2, imss_patronal=$3, repse=$4, updated_at=NOW()
		 WHERE id=$5`,
		d.RazonSocial, d.LogoURL, d.IMSSPatronal, d.REPSE, d.ID)
	if err != nil {
		return fmt.Errorf("PgxDatosFiscalesRepository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return empresas.ErrEmpresaNotFound
	}
	return nil
}

// ---- ApoderadoRepository ----

type PgxApoderadoRepository struct{ pool *pgxpool.Pool }

func NewPgxApoderadoRepository(pool *pgxpool.Pool) *PgxApoderadoRepository {
	return &PgxApoderadoRepository{pool: pool}
}

func (r *PgxApoderadoRepository) Create(ctx context.Context, a *empresas.Apoderado) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO empresas.empresas_apoderado (empresa_id, nombre, curp, rfc, email, telefono)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, created_at, updated_at`,
		a.EmpresaID, a.Nombre, a.CURP, a.RFC, a.Email, a.Telefono).
		Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if isFKViolation(err) {
			return empresas.ErrEmpresaHasDependents
		}
		return fmt.Errorf("PgxApoderadoRepository.Create: %w", err)
	}
	return nil
}

func (r *PgxApoderadoRepository) FindByID(ctx context.Context, id int64) (*empresas.Apoderado, error) {
	var a empresas.Apoderado
	err := r.pool.QueryRow(ctx,
		`SELECT id, empresa_id, nombre, curp, rfc, email, telefono, created_at, updated_at
		 FROM empresas.empresas_apoderado WHERE id=$1`, id).
		Scan(&a.ID, &a.EmpresaID, &a.Nombre, &a.CURP, &a.RFC, &a.Email, &a.Telefono, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empresas.ErrEmpresaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxApoderadoRepository.FindByID: %w", err)
	}
	return &a, nil
}

func (r *PgxApoderadoRepository) ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Apoderado, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, empresa_id, nombre, curp, rfc, email, telefono, created_at, updated_at
		 FROM empresas.empresas_apoderado WHERE empresa_id=$1 ORDER BY id ASC`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("PgxApoderadoRepository.ListByEmpresaID: %w", err)
	}
	defer rows.Close()

	var out []empresas.Apoderado
	for rows.Next() {
		var a empresas.Apoderado
		if err := rows.Scan(&a.ID, &a.EmpresaID, &a.Nombre, &a.CURP, &a.RFC, &a.Email, &a.Telefono, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("PgxApoderadoRepository.ListByEmpresaID scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *PgxApoderadoRepository) Update(ctx context.Context, a *empresas.Apoderado) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE empresas.empresas_apoderado
		 SET nombre=$1, curp=$2, rfc=$3, email=$4, telefono=$5, updated_at=NOW()
		 WHERE id=$6`,
		a.Nombre, a.CURP, a.RFC, a.Email, a.Telefono, a.ID)
	if err != nil {
		return fmt.Errorf("PgxApoderadoRepository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return empresas.ErrEmpresaNotFound
	}
	return nil
}

func (r *PgxApoderadoRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM empresas.empresas_apoderado WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("PgxApoderadoRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return empresas.ErrEmpresaNotFound
	}
	return nil
}

// ---- ServicioRepository ----

type PgxServicioRepository struct{ pool *pgxpool.Pool }

func NewPgxServicioRepository(pool *pgxpool.Pool) *PgxServicioRepository {
	return &PgxServicioRepository{pool: pool}
}

func (r *PgxServicioRepository) Create(ctx context.Context, s *empresas.Servicio) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO empresas.empresas_servicio (empresa_id, nombre, descripcion, precio, status_activo)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, created_at, updated_at`,
		s.EmpresaID, s.Nombre, s.Descripcion, s.Precio, s.StatusActivo).
		Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if isFKViolation(err) {
			return empresas.ErrEmpresaHasDependents
		}
		return fmt.Errorf("PgxServicioRepository.Create: %w", err)
	}
	return nil
}

func (r *PgxServicioRepository) FindByID(ctx context.Context, id int64) (*empresas.Servicio, error) {
	var s empresas.Servicio
	err := r.pool.QueryRow(ctx,
		`SELECT id, empresa_id, nombre, descripcion, precio, status_activo, created_at, updated_at
		 FROM empresas.empresas_servicio WHERE id=$1`, id).
		Scan(&s.ID, &s.EmpresaID, &s.Nombre, &s.Descripcion, &s.Precio, &s.StatusActivo, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empresas.ErrEmpresaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxServicioRepository.FindByID: %w", err)
	}
	return &s, nil
}

func (r *PgxServicioRepository) ListByEmpresaID(ctx context.Context, empresaID int64) ([]empresas.Servicio, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, empresa_id, nombre, descripcion, precio, status_activo, created_at, updated_at
		 FROM empresas.empresas_servicio WHERE empresa_id=$1 ORDER BY id ASC`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("PgxServicioRepository.ListByEmpresaID: %w", err)
	}
	defer rows.Close()

	var out []empresas.Servicio
	for rows.Next() {
		var s empresas.Servicio
		if err := rows.Scan(&s.ID, &s.EmpresaID, &s.Nombre, &s.Descripcion, &s.Precio, &s.StatusActivo, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("PgxServicioRepository.ListByEmpresaID scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *PgxServicioRepository) Update(ctx context.Context, s *empresas.Servicio) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE empresas.empresas_servicio
		 SET nombre=$1, descripcion=$2, precio=$3, status_activo=$4, updated_at=NOW()
		 WHERE id=$5`,
		s.Nombre, s.Descripcion, s.Precio, s.StatusActivo, s.ID)
	if err != nil {
		return fmt.Errorf("PgxServicioRepository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return empresas.ErrEmpresaNotFound
	}
	return nil
}

func (r *PgxServicioRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM empresas.empresas_servicio WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("PgxServicioRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return empresas.ErrEmpresaNotFound
	}
	return nil
}
