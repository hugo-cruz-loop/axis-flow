package repository

import (
	"context"
	"fmt"

	empleados "axis-flow-back/internal/empleados"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AsistenciaRepository persists immutable attendance records. It intentionally has no Delete method.
type AsistenciaRepository struct{ db dbConn }

func NewAsistenciaRepository(db dbConn) *AsistenciaRepository { return &AsistenciaRepository{db: db} }
func NewPgxAsistenciaRepository(pool *pgxpool.Pool) *AsistenciaRepository {
	return NewAsistenciaRepository(pgxDB{pool: pool})
}

func (r *AsistenciaRepository) Create(ctx context.Context, a *empleados.Asistencia, empresaID int64) error {
	a.EstatusObservacionEntrada = empleados.ObservacionPendiente
	err := r.db.QueryRow(ctx,
		`INSERT INTO empleados.empleados_asistencias
            (empleado_id, geolocalizacion, estatus_rango, foto_entrada_url, estatus_observacion_entrada, tipo_registro, hora_entrada)
         SELECT $1,$2,$3,$4,$5,$6,$7::time
         WHERE EXISTS (SELECT 1 FROM empleados.empleados_empleado e WHERE e.num_empleado=$1 AND e.empresa_id=$8)
         RETURNING id, estatus_observacion_entrada, fecha, created_at, updated_at`,
		a.EmpleadoID, a.Geolocalizacion, a.EstatusRango, a.FotoEntradaURL, a.EstatusObservacionEntrada, a.TipoRegistro, a.HoraEntrada, empresaID).
		Scan(&a.ID, &a.EstatusObservacionEntrada, &a.Fecha, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("asistenciaRepository.Create: %w", err)
	}
	return nil
}

func (r *AsistenciaRepository) GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.Asistencia, error) {
	rows, err := r.db.Query(ctx, asistenciaSelect()+` WHERE a.empleado_id=$1 AND e.empresa_id=$2 ORDER BY a.fecha DESC, a.hora_entrada DESC`, empleadoID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("asistenciaRepository.GetByEmpleado: %w", err)
	}
	defer rows.Close()
	return scanAsistenciaRows(rows)
}

func (r *AsistenciaRepository) GetByEmpresa(ctx context.Context, empresaID int64) ([]empleados.Asistencia, error) {
	rows, err := r.db.Query(ctx, asistenciaSelect()+` WHERE e.empresa_id=$1 ORDER BY a.fecha DESC, a.hora_entrada DESC`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("asistenciaRepository.GetByEmpresa: %w", err)
	}
	defer rows.Close()
	return scanAsistenciaRows(rows)
}

func asistenciaSelect() string {
	return `SELECT a.id, a.empleado_id, a.geolocalizacion, a.estatus_rango, a.foto_entrada_url,
                   a.estatus_observacion_entrada, a.similitud_facial, a.tipo_registro, a.fecha,
                   a.hora_entrada::text, a.hora_salida::text, a.created_at, a.updated_at
            FROM empleados.empleados_asistencias a
            JOIN empleados.empleados_empleado e ON e.num_empleado=a.empleado_id`
}

func scanAsistenciaRows(rows pgx.Rows) ([]empleados.Asistencia, error) {
	out := []empleados.Asistencia{}
	for rows.Next() {
		var a empleados.Asistencia
		if err := rows.Scan(&a.ID, &a.EmpleadoID, &a.Geolocalizacion, &a.EstatusRango, &a.FotoEntradaURL, &a.EstatusObservacionEntrada, &a.SimilitudFacial, &a.TipoRegistro, &a.Fecha, &a.HoraEntrada, &a.HoraSalida, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("asistenciaRepository scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("asistenciaRepository rows: %w", err)
	}
	return out, nil
}
