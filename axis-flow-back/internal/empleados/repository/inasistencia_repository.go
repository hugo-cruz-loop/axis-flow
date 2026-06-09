package repository

import (
	"context"
	"fmt"

	empleados "axis-flow-back/internal/empleados"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InasistenciaRepository persists absences and incidence records.
type InasistenciaRepository struct{ db dbConn }

func NewInasistenciaRepository(db dbConn) *InasistenciaRepository {
	return &InasistenciaRepository{db: db}
}
func NewPgxInasistenciaRepository(pool *pgxpool.Pool) *InasistenciaRepository {
	return NewInasistenciaRepository(pgxDB{pool: pool})
}

func (r *InasistenciaRepository) Create(ctx context.Context, i *empleados.Inasistencia, empresaID int64) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO empleados.empleados_inasistencia
            (empleado_id, tipo_incidencia, fecha_inicio, fecha_fin, justificante_url, aprobado, observaciones)
         SELECT $1,$2,$3,$4,$5,false,$6
         WHERE EXISTS (SELECT 1 FROM empleados.empleados_empleado e WHERE e.num_empleado=$1 AND e.empresa_id=$7)
         RETURNING id, aprobado, created_at, updated_at`,
		i.EmpleadoID, i.TipoIncidencia, i.FechaInicio, i.FechaFin, i.JustificanteURL, i.Observaciones, empresaID).
		Scan(&i.ID, &i.Aprobado, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inasistenciaRepository.Create: %w", err)
	}
	return nil
}

func (r *InasistenciaRepository) GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.Inasistencia, error) {
	rows, err := r.db.Query(ctx, inasistenciaSelect()+` WHERE i.empleado_id=$1 AND e.empresa_id=$2 ORDER BY i.fecha_inicio DESC`, empleadoID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("inasistenciaRepository.GetByEmpleado: %w", err)
	}
	defer rows.Close()
	return scanInasistenciaRows(rows)
}

func (r *InasistenciaRepository) UpdateStatus(ctx context.Context, id uuid.UUID, empresaID int64, aprobado bool, observaciones *string, resolvedBy uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE empleados.empleados_inasistencia i
         SET aprobado=$1, observaciones=$2, resolved_by=$3, resolved_at=NOW(), updated_at=NOW()
         FROM empleados.empleados_empleado e
         WHERE i.empleado_id=e.num_empleado AND i.id=$4 AND e.empresa_id=$5`,
		aprobado, observaciones, resolvedBy, id, empresaID)
	if err != nil {
		return fmt.Errorf("inasistenciaRepository.UpdateStatus: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return empleados.ErrEmpleadoNotFound
	}
	return nil
}

func inasistenciaSelect() string {
	return `SELECT i.id, i.empleado_id, i.tipo_incidencia, i.fecha_inicio, i.fecha_fin, i.justificante_url,
                   i.aprobado, i.observaciones, i.resolved_by, i.resolved_at, i.created_at, i.updated_at
            FROM empleados.empleados_inasistencia i
            JOIN empleados.empleados_empleado e ON e.num_empleado=i.empleado_id`
}

func scanInasistenciaRows(rows pgx.Rows) ([]empleados.Inasistencia, error) {
	out := []empleados.Inasistencia{}
	for rows.Next() {
		var i empleados.Inasistencia
		if err := rows.Scan(&i.ID, &i.EmpleadoID, &i.TipoIncidencia, &i.FechaInicio, &i.FechaFin, &i.JustificanteURL, &i.Aprobado, &i.Observaciones, &i.ResolvedBy, &i.ResolvedAt, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, fmt.Errorf("inasistenciaRepository scan: %w", err)
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("inasistenciaRepository rows: %w", err)
	}
	return out, nil
}
