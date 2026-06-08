package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SatelliteRepository defines persistence for the 1:1 satellite records of a Cliente.
// RFC is stored and returned as an opaque encrypted string; the repo never encrypts/decrypts.
type SatelliteRepository interface {
	// Factura
	CreateFactura(ctx context.Context, f *clientes.Factura) error
	GetFactura(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Factura, error)
	UpdateFactura(ctx context.Context, f *clientes.Factura) error

	// Presupuesto
	CreatePresupuesto(ctx context.Context, p *clientes.Presupuesto) error
	GetPresupuesto(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error)
	UpdatePresupuesto(ctx context.Context, p *clientes.Presupuesto) error

	// CalendarioLaboral
	CreateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral) error
	GetCalendario(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error)
	UpdateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral) error
}

// PgxSatelliteRepository is a pgx/v5 implementation of SatelliteRepository.
type PgxSatelliteRepository struct {
	pool *pgxpool.Pool
}

// NewPgxSatelliteRepository returns a new PgxSatelliteRepository.
func NewPgxSatelliteRepository(pool *pgxpool.Pool) *PgxSatelliteRepository {
	return &PgxSatelliteRepository{pool: pool}
}

// ---- Factura ----------------------------------------------------------------

func (r *PgxSatelliteRepository) CreateFactura(ctx context.Context, f *clientes.Factura) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_factura (cliente_id, rfc, razon_social, domicilio_fiscal)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, created_at, updated_at`,
		f.ClienteID, f.RFC, f.RazonSocial, f.DomicilioFiscal).
		Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return clientes.ErrFacturaExists
		}
		return fmt.Errorf("satelliteRepository.CreateFactura: %w", err)
	}
	return nil
}

func (r *PgxSatelliteRepository) GetFactura(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Factura, error) {
	var f clientes.Factura
	err := r.pool.QueryRow(ctx,
		`SELECT f.id, f.cliente_id, f.rfc, f.razon_social, f.domicilio_fiscal, f.created_at, f.updated_at
		 FROM clientes.clientes_factura f
		 JOIN clientes.clientes_cliente c ON c.id = f.cliente_id
		 WHERE f.cliente_id=$1 AND c.empresa_id=$2`,
		clienteID, empresaID).
		Scan(&f.ID, &f.ClienteID, &f.RFC, &f.RazonSocial, &f.DomicilioFiscal, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, clientes.ErrClienteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("satelliteRepository.GetFactura: %w", err)
	}
	return &f, nil
}

func (r *PgxSatelliteRepository) UpdateFactura(ctx context.Context, f *clientes.Factura) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_factura
		 SET rfc=$1, razon_social=$2, domicilio_fiscal=$3, updated_at=NOW()
		 WHERE id=$4 AND cliente_id=$5`,
		f.RFC, f.RazonSocial, f.DomicilioFiscal, f.ID, f.ClienteID)
	if err != nil {
		return fmt.Errorf("satelliteRepository.UpdateFactura: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrClienteNotFound
	}
	return nil
}

// ---- Presupuesto ------------------------------------------------------------

func (r *PgxSatelliteRepository) CreatePresupuesto(ctx context.Context, p *clientes.Presupuesto) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_presupuesto (cliente_id, personal_requerido, material_estimado, costo_mensual)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, created_at, updated_at`,
		p.ClienteID, p.PersonalRequerido, p.MaterialEstimado, p.CostoMensual).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return clientes.ErrPresupuestoExists
		}
		return fmt.Errorf("satelliteRepository.CreatePresupuesto: %w", err)
	}
	return nil
}

func (r *PgxSatelliteRepository) GetPresupuesto(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.Presupuesto, error) {
	var p clientes.Presupuesto
	err := r.pool.QueryRow(ctx,
		`SELECT p.id, p.cliente_id, p.personal_requerido, p.material_estimado, p.costo_mensual, p.created_at, p.updated_at
		 FROM clientes.clientes_presupuesto p
		 JOIN clientes.clientes_cliente c ON c.id = p.cliente_id
		 WHERE p.cliente_id=$1 AND c.empresa_id=$2`,
		clienteID, empresaID).
		Scan(&p.ID, &p.ClienteID, &p.PersonalRequerido, &p.MaterialEstimado, &p.CostoMensual, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, clientes.ErrClienteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("satelliteRepository.GetPresupuesto: %w", err)
	}
	return &p, nil
}

func (r *PgxSatelliteRepository) UpdatePresupuesto(ctx context.Context, p *clientes.Presupuesto) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_presupuesto
		 SET personal_requerido=$1, material_estimado=$2, costo_mensual=$3, updated_at=NOW()
		 WHERE id=$4 AND cliente_id=$5`,
		p.PersonalRequerido, p.MaterialEstimado, p.CostoMensual, p.ID, p.ClienteID)
	if err != nil {
		return fmt.Errorf("satelliteRepository.UpdatePresupuesto: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrClienteNotFound
	}
	return nil
}

// ---- CalendarioLaboral ------------------------------------------------------

func (r *PgxSatelliteRepository) CreateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral) error {
	semana, err := json.Marshal(cal.SemanaLaboral)
	if err != nil {
		return fmt.Errorf("satelliteRepository.CreateCalendario marshal semana: %w", err)
	}
	dias, err := json.Marshal(cal.DiasInhabiles)
	if err != nil {
		return fmt.Errorf("satelliteRepository.CreateCalendario marshal dias: %w", err)
	}

	err = r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_calendariofaboral (cliente_id, semana_laboral, dias_inhabiles)
		 VALUES ($1,$2,$3)
		 RETURNING created_at, updated_at`,
		cal.ClienteID, semana, dias).
		Scan(&cal.CreatedAt, &cal.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return clientes.ErrCalendarioExists
		}
		return fmt.Errorf("satelliteRepository.CreateCalendario: %w", err)
	}
	return nil
}

func (r *PgxSatelliteRepository) GetCalendario(ctx context.Context, clienteID uuid.UUID, empresaID int64) (*clientes.CalendarioLaboral, error) {
	var cal clientes.CalendarioLaboral
	var semanaRaw, diasRaw []byte

	err := r.pool.QueryRow(ctx,
		`SELECT cl.cliente_id, cl.semana_laboral, cl.dias_inhabiles, cl.created_at, cl.updated_at
		 FROM clientes.clientes_calendariofaboral cl
		 JOIN clientes.clientes_cliente c ON c.id = cl.cliente_id
		 WHERE cl.cliente_id=$1 AND c.empresa_id=$2`,
		clienteID, empresaID).
		Scan(&cal.ClienteID, &semanaRaw, &diasRaw, &cal.CreatedAt, &cal.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, clientes.ErrClienteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("satelliteRepository.GetCalendario: %w", err)
	}

	if err := json.Unmarshal(semanaRaw, &cal.SemanaLaboral); err != nil {
		return nil, fmt.Errorf("satelliteRepository.GetCalendario unmarshal semana: %w", err)
	}
	if err := json.Unmarshal(diasRaw, &cal.DiasInhabiles); err != nil {
		return nil, fmt.Errorf("satelliteRepository.GetCalendario unmarshal dias: %w", err)
	}
	return &cal, nil
}

func (r *PgxSatelliteRepository) UpdateCalendario(ctx context.Context, cal *clientes.CalendarioLaboral) error {
	semana, err := json.Marshal(cal.SemanaLaboral)
	if err != nil {
		return fmt.Errorf("satelliteRepository.UpdateCalendario marshal semana: %w", err)
	}
	dias, err := json.Marshal(cal.DiasInhabiles)
	if err != nil {
		return fmt.Errorf("satelliteRepository.UpdateCalendario marshal dias: %w", err)
	}

	tag, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_calendariofaboral
		 SET semana_laboral=$1, dias_inhabiles=$2, updated_at=NOW()
		 WHERE cliente_id=$3`,
		semana, dias, cal.ClienteID)
	if err != nil {
		return fmt.Errorf("satelliteRepository.UpdateCalendario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrClienteNotFound
	}
	return nil
}
