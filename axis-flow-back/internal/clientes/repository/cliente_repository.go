// Package repository implements persistence for the clientes module.
package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ClienteRepository defines persistence operations for the Cliente aggregate.
type ClienteRepository interface {
	Create(ctx context.Context, c *clientes.Cliente) error
	FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error)
	ListByEmpresa(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error)
	Update(ctx context.Context, c *clientes.Cliente) error
	Delete(ctx context.Context, id uuid.UUID, empresaID int64) error
	CheckQualityGate(ctx context.Context, clienteID uuid.UUID) (*clientes.QualityGateStatus, error)
	PatchEstatus(ctx context.Context, clienteID uuid.UUID, empresaID int64, estatus int) (httpStatusHint int, gate *clientes.QualityGateStatus, err error)
}

// PgxClienteRepository is a pgx/v5 implementation of ClienteRepository.
type PgxClienteRepository struct {
	pool *pgxpool.Pool
}

// NewPgxClienteRepository returns a new PgxClienteRepository.
func NewPgxClienteRepository(pool *pgxpool.Pool) *PgxClienteRepository {
	return &PgxClienteRepository{pool: pool}
}

func (r *PgxClienteRepository) Create(ctx context.Context, c *clientes.Cliente) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_cliente
		 (empresa_id, representante_id, nombre_comercial, razon_social, fecha_inicio_contrato, estatus)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, created_at, updated_at`,
		c.EmpresaID, c.RepresentanteID, c.NombreComercial, c.RazonSocial,
		c.FechaInicioContrato, c.Estatus).
		Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("clienteRepository.Create: %w", err)
	}
	return nil
}

func (r *PgxClienteRepository) FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	var c clientes.Cliente
	err := r.pool.QueryRow(ctx,
		`SELECT id, empresa_id, representante_id, nombre_comercial, razon_social,
		        fecha_inicio_contrato, estatus, created_at, updated_at
		 FROM clientes.clientes_cliente
		 WHERE id=$1 AND empresa_id=$2`, id, empresaID).
		Scan(&c.ID, &c.EmpresaID, &c.RepresentanteID, &c.NombreComercial, &c.RazonSocial,
			&c.FechaInicioContrato, &c.Estatus, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, clientes.ErrClienteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("clienteRepository.FindByID: %w", err)
	}
	return &c, nil
}

func (r *PgxClienteRepository) FindByUserID(ctx context.Context, userID uuid.UUID, empresaID int64) (*clientes.Cliente, error) {
	var c clientes.Cliente
	err := r.pool.QueryRow(ctx,
		`SELECT id, empresa_id, representante_id, nombre_comercial, razon_social,
		        fecha_inicio_contrato, estatus, created_at, updated_at
		 FROM clientes.clientes_cliente
		 WHERE representante_id=$1 AND empresa_id=$2`, userID, empresaID).
		Scan(&c.ID, &c.EmpresaID, &c.RepresentanteID, &c.NombreComercial, &c.RazonSocial,
			&c.FechaInicioContrato, &c.Estatus, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, clientes.ErrClienteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("clienteRepository.FindByUserID: %w", err)
	}
	return &c, nil
}

func (r *PgxClienteRepository) ListByEmpresa(ctx context.Context, empresaID int64, page, size int) ([]clientes.Cliente, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM clientes.clientes_cliente WHERE empresa_id=$1`, empresaID).
		Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("clienteRepository.ListByEmpresa count: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, empresa_id, representante_id, nombre_comercial, razon_social,
		        fecha_inicio_contrato, estatus, created_at, updated_at
		 FROM clientes.clientes_cliente
		 WHERE empresa_id=$1
		 ORDER BY nombre_comercial ASC
		 LIMIT $2 OFFSET $3`, empresaID, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("clienteRepository.ListByEmpresa query: %w", err)
	}
	defer rows.Close()

	var out []clientes.Cliente
	for rows.Next() {
		var c clientes.Cliente
		if err := rows.Scan(&c.ID, &c.EmpresaID, &c.RepresentanteID, &c.NombreComercial, &c.RazonSocial,
			&c.FechaInicioContrato, &c.Estatus, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("clienteRepository.ListByEmpresa scan: %w", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *PgxClienteRepository) Update(ctx context.Context, c *clientes.Cliente) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_cliente
		 SET representante_id=$1, nombre_comercial=$2, razon_social=$3,
		     fecha_inicio_contrato=$4, estatus=$5, updated_at=NOW()
		 WHERE id=$6 AND empresa_id=$7`,
		c.RepresentanteID, c.NombreComercial, c.RazonSocial,
		c.FechaInicioContrato, c.Estatus, c.ID, c.EmpresaID)
	if err != nil {
		return fmt.Errorf("clienteRepository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrClienteNotFound
	}
	return nil
}

func (r *PgxClienteRepository) Delete(ctx context.Context, id uuid.UUID, empresaID int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM clientes.clientes_cliente WHERE id=$1 AND empresa_id=$2`, id, empresaID)
	if err != nil {
		if isFKViolation(err) {
			return clientes.ErrClienteHasDependents
		}
		return fmt.Errorf("clienteRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrClienteNotFound
	}
	return nil
}

// CheckQualityGate queries whether a cliente has all 3 satellite records.
func (r *PgxClienteRepository) CheckQualityGate(ctx context.Context, clienteID uuid.UUID) (*clientes.QualityGateStatus, error) {
	gate := &clientes.QualityGateStatus{ClienteID: clienteID}

	err := r.pool.QueryRow(ctx,
		`SELECT
		   EXISTS(SELECT 1 FROM clientes.clientes_factura WHERE cliente_id=$1)     AS factura_ok,
		   EXISTS(SELECT 1 FROM clientes.clientes_presupuesto WHERE cliente_id=$1)  AS presupuesto_ok,
		   EXISTS(SELECT 1 FROM clientes.clientes_calendariofaboral WHERE cliente_id=$1) AS calendario_ok`,
		clienteID).
		Scan(&gate.FacturaOK, &gate.PresupuestoOK, &gate.CalendarioOK)
	if err != nil {
		return nil, fmt.Errorf("clienteRepository.CheckQualityGate: %w", err)
	}
	gate.CanActivate = gate.FacturaOK && gate.PresupuestoOK && gate.CalendarioOK
	return gate, nil
}

// PatchEstatus attempts to update estatus. If the DB trigger raises a check violation
// (quality gate not met), it returns (206, gateStatus, nil). On success: (200, nil, nil).
func (r *PgxClienteRepository) PatchEstatus(ctx context.Context, clienteID uuid.UUID, empresaID int64, estatus int) (int, *clientes.QualityGateStatus, error) {
	_, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_cliente SET estatus=$1, updated_at=NOW()
		 WHERE id=$2 AND empresa_id=$3`,
		estatus, clienteID, empresaID)
	if err != nil {
		if isCheckViolation(err) {
			gate, gErr := r.CheckQualityGate(ctx, clienteID)
			if gErr != nil {
				return 0, nil, fmt.Errorf("clienteRepository.PatchEstatus gate check: %w", gErr)
			}
			return 206, gate, nil
		}
		return 0, nil, fmt.Errorf("clienteRepository.PatchEstatus: %w", err)
	}
	return 200, nil, nil
}
