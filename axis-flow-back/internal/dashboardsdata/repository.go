package dashboardsdata

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// dbConn defines the interface required to query the database.
// This allows pgxpool.Pool and transactions to be used, and enables easy mocking.
type dbConn interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgxRepository implements multi-schema operational query reads.
type PgxRepository struct {
	db dbConn
}

// NewPgxRepository constructs a new PgxRepository.
func NewPgxRepository(db dbConn) *PgxRepository {
	return &PgxRepository{db: db}
}

// GetCompanyTenantID retrieves the tenant UUID associated with the company representative user.
func (r *PgxRepository) GetCompanyTenantID(ctx context.Context, companyID int64) (uuid.UUID, error) {
	var tenantID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT u.tenant_id 
		FROM empresas.empresas_empresa e
		JOIN users.identity_users u ON u.id = e.representante_id
		WHERE e.id = $1`, companyID).Scan(&tenantID)
	return tenantID, err
}

// GetCompanyIDByTenantID resolves a company ID (bigint) using its tenant UUID.
func (r *PgxRepository) GetCompanyIDByTenantID(ctx context.Context, tenantID uuid.UUID, companyID *int64) error {
	return r.db.QueryRow(ctx, `
		SELECT e.id
		FROM empresas.empresas_empresa e
		JOIN users.identity_users u ON u.id = e.representante_id
		WHERE u.tenant_id = $1`, tenantID).Scan(companyID)
}

// GetClientCompanyID retrieves the company ID (bigint) that owns the given client.
func (r *PgxRepository) GetClientCompanyID(ctx context.Context, clientID uuid.UUID, companyID *int64) error {
	return r.db.QueryRow(ctx, `
		SELECT empresa_id 
		FROM clientes.clientes_cliente 
		WHERE id = $1`, clientID).Scan(companyID)
}

// GetEvaluacionesClientes reads average ratings and counts grouped by client for a company.
func (r *PgxRepository) GetEvaluacionesClientes(ctx context.Context, companyID int64, clientIDFilter *uuid.UUID) ([]ClientEvaluation, error) {
	query := `
		SELECT 
			c.id AS cliente_id,
			c.nombre_comercial AS cliente_nombre,
			COALESCE(AVG(e.calificacion)::float8, 0.0) AS promedio_puntuacion,
			COUNT(e.id)::int AS total_evaluaciones
		FROM clientes.clientes_cliente c
		LEFT JOIN asignacion.asignacion_asignacion a ON a.empresa_id = c.id
		LEFT JOIN asignacion.asignacion_evaluacionempleado e ON e.asignacion_id = a.id
		WHERE c.empresa_id = $1`

	args := []any{companyID}
	if clientIDFilter != nil {
		query += " AND c.id = $2"
		args = append(args, *clientIDFilter)
	}

	query += " GROUP BY c.id, c.nombre_comercial ORDER BY c.nombre_comercial"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evaluations []ClientEvaluation
	for rows.Next() {
		var eval ClientEvaluation
		if err := rows.Scan(&eval.ClientID, &eval.ClientNombre, &eval.PromedioPuntuacion, &eval.TotalEvaluaciones); err != nil {
			return nil, err
		}
		evaluations = append(evaluations, eval)
	}
	return evaluations, rows.Err()
}

// GetEmpleadosCount returns total count of active employees in a company.
func (r *PgxRepository) GetEmpleadosCount(ctx context.Context, companyID int64) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int 
		FROM empleados.empleados_empleado 
		WHERE empresa_id = $1 AND status = 1`, companyID).Scan(&count)
	return count, err
}

// GetActividadesCount returns total count of completed activities since the given timestamp.
func (r *PgxRepository) GetActividadesCount(ctx context.Context, companyID int64, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(aa.id)::int
		FROM asignacion.asignacion_asignaactividad aa
		JOIN asignacion.asignacion_asignacion a ON aa.asignacion_id = a.id
		JOIN clientes.clientes_cliente c ON a.empresa_id = c.id
		WHERE c.empresa_id = $1 AND aa.estatus = 2 AND aa.fecha_ejecucion >= $2`, companyID, since).Scan(&count)
	return count, err
}

// GetAusenciasCount returns total historical absences for a company, optionally filtered by year.
func (r *PgxRepository) GetAusenciasCount(ctx context.Context, companyID int64, year *int) (int, error) {
	query := `
		SELECT COUNT(i.id)::int
		FROM empleados.empleados_inasistencia i
		JOIN empleados.empleados_empleado e ON i.empleado_id = e.num_empleado
		WHERE e.empresa_id = $1`

	args := []any{companyID}
	if year != nil {
		query += " AND EXTRACT(YEAR FROM i.fecha_inicio) = $2"
		args = append(args, *year)
	}

	var count int
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

// GetServiciosLocalidad returns localidad branches and their active services with assigned staff count.
func (r *PgxRepository) GetServiciosLocalidad(ctx context.Context, clientID uuid.UUID) ([]LocalidadServicio, error) {
	rows, err := r.db.Query(ctx, `
		SELECT 
			l.id AS localidad_id,
			l.nombre AS localidad_nombre,
			cs.id AS servicio_id,
			cs.name AS servicio_nombre,
			COUNT(a.id)::int AS empleados_asignados
		FROM clientes.clientes_localidad l
		JOIN clientes.clientes_serviciolocalidad sl ON sl.localidad_id = l.id
		JOIN catalogos.catalog_services cs ON cs.id = sl.servicio_id
		LEFT JOIN asignacion.asignacion_asignacion a ON a.localidad_id = l.id 
			AND a.servicio_id = sl.id 
			AND a.estatus = 1 
			AND a.ultima_asignacion = TRUE
		WHERE l.cliente_id = $1 AND sl.status_activo = TRUE
		GROUP BY l.id, l.nombre, cs.id, cs.name
		ORDER BY l.nombre, cs.name`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	localityMap := make(map[uuid.UUID]*LocalidadServicio)
	var localities []LocalidadServicio

	for rows.Next() {
		var (
			locID      uuid.UUID
			locNombre  string
			servID     int64
			servNombre string
			empAsig    int
		)
		if err := rows.Scan(&locID, &locNombre, &servID, &servNombre, &empAsig); err != nil {
			return nil, err
		}

		loc, ok := localityMap[locID]
		if !ok {
			loc = &LocalidadServicio{
				LocalidadID:     locID,
				LocalidadNombre: locNombre,
				Servicios:       []Servicio{},
			}
			localityMap[locID] = loc
			localities = append(localities, *loc)
		}

		loc.Servicios = append(loc.Servicios, Servicio{
			ServicioID:         servID,
			ServicioNombre:     servNombre,
			EmpleadosAsignados: empAsig,
		})
	}

	// Update the returned slice with populated services
	for i := range localities {
		localities[i] = *localityMap[localities[i].LocalidadID]
	}

	return localities, rows.Err()
}

// GetAtencionSeguimientoStatus returns ticket status counts breakdown.
func (r *PgxRepository) GetAtencionSeguimientoStatus(ctx context.Context, clientID uuid.UUID) (TicketBreakdown, int, error) {
	var (
		tickets TicketBreakdown
		total   int
	)
	err := r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN estatus = 1 THEN 1 ELSE 0 END), 0)::int AS pendiente,
			COALESCE(SUM(CASE WHEN estatus = 2 THEN 1 ELSE 0 END), 0)::int AS en_proceso,
			COALESCE(SUM(CASE WHEN estatus = 3 THEN 1 ELSE 0 END), 0)::int AS finalizado,
			COUNT(id)::int AS total
		FROM atencion_seguimiento.tickets_servicio
		WHERE cliente_id = $1`, clientID).Scan(&tickets.Pendiente, &tickets.EnProceso, &tickets.Finalizado, &total)
	return tickets, total, err
}

// GetBolsaTrabajoVacantesActivas returns a list of active job vacancies for a company.
func (r *PgxRepository) GetBolsaTrabajoVacantesActivas(ctx context.Context, companyTenantID uuid.UUID) ([]Vacante, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, titulo, created_at 
		FROM bolsa_trabajo.trabajos
		WHERE empresa_id = $1 AND estatus_vacante = 1
		ORDER BY created_at DESC`, companyTenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vacancies []Vacante
	for rows.Next() {
		var (
			idStr     string
			titulo    string
			createdAt time.Time
		)
		if err := rows.Scan(&idStr, &titulo, &createdAt); err != nil {
			return nil, err
		}
		vacancies = append(vacancies, Vacante{
			VacanteID:        idStr,
			Titulo:           titulo,
			Departamento:     "Operaciones", // Default fallback as department is not a column in trabajos
			FechaPublicacion: createdAt.Format(time.RFC3339),
		})
	}
	return vacancies, rows.Err()
}
