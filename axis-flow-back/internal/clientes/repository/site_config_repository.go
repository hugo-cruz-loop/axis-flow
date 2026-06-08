package repository

import (
	"context"
	"fmt"

	"axis-flow-back/internal/clientes"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SiteConfigRepository defines persistence for site-level configuration entities.
// Tenant isolation requires a 2-hop join:
// clientes_horario/herramienta/actividad → clientes_localidad → clientes_cliente WHERE empresa_id=$N
type SiteConfigRepository interface {
	// Servicios
	AddServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64) error
	RemoveServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64) error
	ListServicios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.ServiciosLocalidad, error)

	// Horarios
	CreateHorario(ctx context.Context, h *clientes.Horario) error
	ListHorarios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Horario, error)
	UpdateHorario(ctx context.Context, h *clientes.Horario) error
	DeleteHorario(ctx context.Context, id uuid.UUID, empresaID int64) error

	// Herramientas
	CreateHerramienta(ctx context.Context, h *clientes.Herramienta) error
	ListHerramientas(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Herramienta, error)
	UpdateHerramienta(ctx context.Context, h *clientes.Herramienta) error
	DeleteHerramienta(ctx context.Context, id uuid.UUID, empresaID int64) error

	// Actividades
	CreateActividad(ctx context.Context, a *clientes.Actividad) error
	ListActividades(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Actividad, error)
	UpdateActividad(ctx context.Context, a *clientes.Actividad) error
	DeleteActividad(ctx context.Context, id uuid.UUID, empresaID int64) error
}

// PgxSiteConfigRepository is a pgx/v5 implementation of SiteConfigRepository.
type PgxSiteConfigRepository struct {
	pool *pgxpool.Pool
}

// NewPgxSiteConfigRepository returns a new PgxSiteConfigRepository.
func NewPgxSiteConfigRepository(pool *pgxpool.Pool) *PgxSiteConfigRepository {
	return &PgxSiteConfigRepository{pool: pool}
}

// ---- Servicios --------------------------------------------------------------

func (r *PgxSiteConfigRepository) AddServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clientes.clientes_serviciolocalidad (localidad_id, servicio_id)
		 VALUES ($1,$2)`,
		localidadID, servicioID)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("siteConfigRepository.AddServicio: servicio already assigned to localidad: %w", err)
		}
		return fmt.Errorf("siteConfigRepository.AddServicio: %w", err)
	}
	return nil
}

func (r *PgxSiteConfigRepository) RemoveServicio(ctx context.Context, localidadID uuid.UUID, servicioID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM clientes.clientes_serviciolocalidad
		 WHERE localidad_id=$1 AND servicio_id=$2`,
		localidadID, servicioID)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.RemoveServicio: %w", err)
	}
	return nil
}

func (r *PgxSiteConfigRepository) ListServicios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.ServiciosLocalidad, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.localidad_id, s.servicio_id, s.status_activo, s.created_at, s.updated_at
		 FROM clientes.clientes_serviciolocalidad s
		 JOIN clientes.clientes_localidad l ON l.id = s.localidad_id
		 JOIN clientes.clientes_cliente c ON c.id = l.cliente_id
		 WHERE s.localidad_id=$1 AND c.empresa_id=$2`,
		localidadID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("siteConfigRepository.ListServicios: %w", err)
	}
	defer rows.Close()

	var out []clientes.ServiciosLocalidad
	for rows.Next() {
		var s clientes.ServiciosLocalidad
		if err := rows.Scan(&s.ID, &s.LocalidadID, &s.ServicioID, &s.StatusActivo, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("siteConfigRepository.ListServicios scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---- Horarios ---------------------------------------------------------------

func (r *PgxSiteConfigRepository) CreateHorario(ctx context.Context, h *clientes.Horario) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_horario
		 (localidad_id, hora_entrada, hora_salida, hora_comida_inicio, hora_comida_fin)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, created_at, updated_at`,
		h.LocalidadID, h.HoraEntrada, h.HoraSalida, h.HoraComidaInicio, h.HoraComidaFin).
		Scan(&h.ID, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.CreateHorario: %w", err)
	}
	return nil
}

func (r *PgxSiteConfigRepository) ListHorarios(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Horario, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT h.id, h.localidad_id, h.hora_entrada, h.hora_salida, h.hora_comida_inicio, h.hora_comida_fin, h.created_at, h.updated_at
		 FROM clientes.clientes_horario h
		 JOIN clientes.clientes_localidad l ON l.id = h.localidad_id
		 JOIN clientes.clientes_cliente c ON c.id = l.cliente_id
		 WHERE h.localidad_id=$1 AND c.empresa_id=$2`,
		localidadID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("siteConfigRepository.ListHorarios: %w", err)
	}
	defer rows.Close()

	var out []clientes.Horario
	for rows.Next() {
		var h clientes.Horario
		if err := rows.Scan(&h.ID, &h.LocalidadID, &h.HoraEntrada, &h.HoraSalida,
			&h.HoraComidaInicio, &h.HoraComidaFin, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("siteConfigRepository.ListHorarios scan: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *PgxSiteConfigRepository) UpdateHorario(ctx context.Context, h *clientes.Horario) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_horario
		 SET hora_entrada=$1, hora_salida=$2, hora_comida_inicio=$3, hora_comida_fin=$4, updated_at=NOW()
		 WHERE id=$5 AND localidad_id=$6`,
		h.HoraEntrada, h.HoraSalida, h.HoraComidaInicio, h.HoraComidaFin, h.ID, h.LocalidadID)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.UpdateHorario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrLocalidadNotFound
	}
	return nil
}

func (r *PgxSiteConfigRepository) DeleteHorario(ctx context.Context, id uuid.UUID, empresaID int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM clientes.clientes_horario h
		 USING clientes.clientes_localidad l, clientes.clientes_cliente c
		 WHERE h.id=$1 AND h.localidad_id = l.id AND l.cliente_id = c.id AND c.empresa_id=$2`,
		id, empresaID)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.DeleteHorario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrLocalidadNotFound
	}
	return nil
}

// ---- Herramientas -----------------------------------------------------------

func (r *PgxSiteConfigRepository) CreateHerramienta(ctx context.Context, h *clientes.Herramienta) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_herramienta (localidad_id, nombre, cantidad, especificaciones)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, created_at, updated_at`,
		h.LocalidadID, h.Nombre, h.Cantidad, h.Especificaciones).
		Scan(&h.ID, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.CreateHerramienta: %w", err)
	}
	return nil
}

func (r *PgxSiteConfigRepository) ListHerramientas(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Herramienta, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT h.id, h.localidad_id, h.nombre, h.cantidad, h.especificaciones, h.created_at, h.updated_at
		 FROM clientes.clientes_herramienta h
		 JOIN clientes.clientes_localidad l ON l.id = h.localidad_id
		 JOIN clientes.clientes_cliente c ON c.id = l.cliente_id
		 WHERE h.localidad_id=$1 AND c.empresa_id=$2`,
		localidadID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("siteConfigRepository.ListHerramientas: %w", err)
	}
	defer rows.Close()

	var out []clientes.Herramienta
	for rows.Next() {
		var h clientes.Herramienta
		if err := rows.Scan(&h.ID, &h.LocalidadID, &h.Nombre, &h.Cantidad,
			&h.Especificaciones, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("siteConfigRepository.ListHerramientas scan: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *PgxSiteConfigRepository) UpdateHerramienta(ctx context.Context, h *clientes.Herramienta) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_herramienta
		 SET nombre=$1, cantidad=$2, especificaciones=$3, updated_at=NOW()
		 WHERE id=$4 AND localidad_id=$5`,
		h.Nombre, h.Cantidad, h.Especificaciones, h.ID, h.LocalidadID)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.UpdateHerramienta: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrLocalidadNotFound
	}
	return nil
}

func (r *PgxSiteConfigRepository) DeleteHerramienta(ctx context.Context, id uuid.UUID, empresaID int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM clientes.clientes_herramienta h
		 USING clientes.clientes_localidad l, clientes.clientes_cliente c
		 WHERE h.id=$1 AND h.localidad_id = l.id AND l.cliente_id = c.id AND c.empresa_id=$2`,
		id, empresaID)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.DeleteHerramienta: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrLocalidadNotFound
	}
	return nil
}

// ---- Actividades ------------------------------------------------------------

func (r *PgxSiteConfigRepository) CreateActividad(ctx context.Context, a *clientes.Actividad) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clientes.clientes_actividad (localidad_id, descripcion, frecuencia, orden)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, created_at, updated_at`,
		a.LocalidadID, a.Descripcion, a.Frecuencia, a.Orden).
		Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.CreateActividad: %w", err)
	}
	return nil
}

func (r *PgxSiteConfigRepository) ListActividades(ctx context.Context, localidadID uuid.UUID, empresaID int64) ([]clientes.Actividad, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.localidad_id, a.descripcion, a.frecuencia, a.orden, a.created_at, a.updated_at
		 FROM clientes.clientes_actividad a
		 JOIN clientes.clientes_localidad l ON l.id = a.localidad_id
		 JOIN clientes.clientes_cliente c ON c.id = l.cliente_id
		 WHERE a.localidad_id=$1 AND c.empresa_id=$2
		 ORDER BY a.orden ASC`,
		localidadID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("siteConfigRepository.ListActividades: %w", err)
	}
	defer rows.Close()

	var out []clientes.Actividad
	for rows.Next() {
		var a clientes.Actividad
		if err := rows.Scan(&a.ID, &a.LocalidadID, &a.Descripcion, &a.Frecuencia,
			&a.Orden, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("siteConfigRepository.ListActividades scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *PgxSiteConfigRepository) UpdateActividad(ctx context.Context, a *clientes.Actividad) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clientes.clientes_actividad
		 SET descripcion=$1, frecuencia=$2, orden=$3, updated_at=NOW()
		 WHERE id=$4 AND localidad_id=$5`,
		a.Descripcion, a.Frecuencia, a.Orden, a.ID, a.LocalidadID)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.UpdateActividad: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrLocalidadNotFound
	}
	return nil
}

func (r *PgxSiteConfigRepository) DeleteActividad(ctx context.Context, id uuid.UUID, empresaID int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM clientes.clientes_actividad a
		 USING clientes.clientes_localidad l, clientes.clientes_cliente c
		 WHERE a.id=$1 AND a.localidad_id = l.id AND l.cliente_id = c.id AND c.empresa_id=$2`,
		id, empresaID)
	if err != nil {
		return fmt.Errorf("siteConfigRepository.DeleteActividad: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return clientes.ErrLocalidadNotFound
	}
	return nil
}
