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

// LocalidadRepository defines persistence for Localidad (client branches/sites).
type LocalidadRepository interface {
	Create(ctx context.Context, l *clientes.Localidad) error
	FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error)
	ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error)
	Update(ctx context.Context, l *clientes.Localidad) error
	Delete(ctx context.Context, id uuid.UUID, empresaID int64) error
}

// PgxLocalidadRepository is a pgx/v5 implementation of LocalidadRepository.
type PgxLocalidadRepository struct {
	pool *pgxpool.Pool
}

// NewPgxLocalidadRepository returns a new PgxLocalidadRepository.
func NewPgxLocalidadRepository(pool *pgxpool.Pool) *PgxLocalidadRepository {
	return &PgxLocalidadRepository{pool: pool}
}

func (r *PgxLocalidadRepository) Create(ctx context.Context, l *clientes.Localidad) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_localidad
		 (cliente_id, nombre, direccion, supervisor_id, tipo_localidad_id, latitud, longitud)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, created_at, updated_at`,
		l.ClienteID, l.Nombre, l.Direccion, l.SupervisorID,
		l.TipoLocalidadID, l.Latitud, l.Longitud).
		Scan(&l.ID, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		if isFKViolation(err) {
			return fmt.Errorf("localidadRepository.Create: tipo_localidad_id invalid: %w", err)
		}
		return fmt.Errorf("localidadRepository.Create: %w", err)
	}
	return nil
}

func (r *PgxLocalidadRepository) FindByID(ctx context.Context, id uuid.UUID, empresaID int64) (*clientes.Localidad, error) {
	var l clientes.Localidad
	err := r.pool.QueryRow(ctx,
		`SELECT l.id, l.cliente_id, l.nombre, l.direccion, l.supervisor_id,
		        l.tipo_localidad_id, l.latitud, l.longitud, l.created_at, l.updated_at
		 FROM clientes.clientes_localidad l
		 JOIN clientes.clientes_cliente c ON c.id = l.cliente_id
		 WHERE l.id=$1 AND c.empresa_id=$2`,
		id, empresaID).
		Scan(&l.ID, &l.ClienteID, &l.Nombre, &l.Direccion, &l.SupervisorID,
			&l.TipoLocalidadID, &l.Latitud, &l.Longitud, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, clientes.ErrLocalidadNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("localidadRepository.FindByID: %w", err)
	}
	return &l, nil
}

func (r *PgxLocalidadRepository) ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.Localidad, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT l.id, l.cliente_id, l.nombre, l.direccion, l.supervisor_id,
		        l.tipo_localidad_id, l.latitud, l.longitud, l.created_at, l.updated_at
		 FROM clientes.clientes_localidad l
		 JOIN clientes.clientes_cliente c ON c.id = l.cliente_id
		 WHERE l.cliente_id=$1 AND c.empresa_id=$2
		 ORDER BY l.nombre ASC`,
		clienteID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("localidadRepository.ListByCliente: %w", err)
	}
	defer rows.Close()

	var out []clientes.Localidad
	for rows.Next() {
		var l clientes.Localidad
		if err := rows.Scan(&l.ID, &l.ClienteID, &l.Nombre, &l.Direccion, &l.SupervisorID,
			&l.TipoLocalidadID, &l.Latitud, &l.Longitud, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("localidadRepository.ListByCliente scan: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *PgxLocalidadRepository) Update(ctx context.Context, l *clientes.Localidad) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_localidad
		 SET nombre=$1, direccion=$2, supervisor_id=$3, tipo_localidad_id=$4, latitud=$5, longitud=$6, updated_at=NOW()
		 WHERE id=$7 AND cliente_id=$8`,
		l.Nombre, l.Direccion, l.SupervisorID, l.TipoLocalidadID,
		l.Latitud, l.Longitud, l.ID, l.ClienteID)
	if err != nil {
		if isFKViolation(err) {
			return fmt.Errorf("localidadRepository.Update: tipo_localidad_id invalid: %w", err)
		}
		return fmt.Errorf("localidadRepository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrLocalidadNotFound
	}
	return nil
}

func (r *PgxLocalidadRepository) Delete(ctx context.Context, id uuid.UUID, empresaID int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM clientes.clientes_localidad l
		 USING clientes.clientes_cliente c
		 WHERE l.id=$1 AND l.cliente_id = c.id AND c.empresa_id=$2`,
		id, empresaID)
	if err != nil {
		return fmt.Errorf("localidadRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrLocalidadNotFound
	}
	return nil
}
