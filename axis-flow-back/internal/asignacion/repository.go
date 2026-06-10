package asignacion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type assignmentDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type assignmentTxDB interface {
	assignmentDB
	Begin(ctx context.Context) (pgx.Tx, error)
}

// PgxAssignmentRepository is a PostgreSQL implementation of AssignmentRepository.
type PgxAssignmentRepository struct {
	db assignmentDB
}

// NewPgxAssignmentRepository creates a PostgreSQL assignment repository.
func NewPgxAssignmentRepository(pool *pgxpool.Pool) *PgxAssignmentRepository {
	return &PgxAssignmentRepository{db: pool}
}

// Create inserts an assignment.
func (r *PgxAssignmentRepository) Create(ctx context.Context, assignment *Assignment) error {
	if err := insertAssignment(ctx, r.db, assignment); err != nil {
		return fmt.Errorf("assignment_repository.Create: %w", err)
	}
	return nil
}

// ReplaceCurrentForEmployee atomically clears the current assignment flag and inserts the new assignment.
func (r *PgxAssignmentRepository) ReplaceCurrentForEmployee(ctx context.Context, assignment *Assignment, deactivatedAt time.Time) error {
	txDB, ok := r.db.(assignmentTxDB)
	if !ok {
		return fmt.Errorf("assignment_repository.ReplaceCurrentForEmployee: transaction support unavailable")
	}

	tx, err := txDB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("assignment_repository.ReplaceCurrentForEmployee begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := execDeactivateCurrentForEmployee(ctx, tx, assignment.EmployeeID, deactivatedAt); err != nil {
		return fmt.Errorf("assignment_repository.ReplaceCurrentForEmployee deactivate: %w", err)
	}
	if err := insertAssignment(ctx, tx, assignment); err != nil {
		return fmt.Errorf("assignment_repository.ReplaceCurrentForEmployee create: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("assignment_repository.ReplaceCurrentForEmployee commit: %w", err)
	}
	committed = true
	return nil
}

func insertAssignment(ctx context.Context, db assignmentDB, assignment *Assignment) error {
	const query = `
		INSERT INTO asignacion.asignacion_asignacion (
			id, empresa_id, empleado_id, localidad_id, servicio_id, turno_id,
			estatus, ultima_asignacion, fecha_inicio, fecha_fin, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	_, err := db.Exec(ctx, query,
		assignment.ID,
		assignment.CompanyID,
		assignment.EmployeeID,
		assignment.LocationID,
		assignment.ServiceID,
		assignment.ShiftID,
		int(assignment.Status),
		assignment.IsCurrent,
		assignment.StartDate,
		assignment.EndDate,
		assignment.CreatedAt,
		assignment.UpdatedAt,
	)
	return err
}

// FindByID loads an assignment by ID.
func (r *PgxAssignmentRepository) FindByID(ctx context.Context, id uuid.UUID) (*Assignment, error) {
	const query = `
		SELECT id, empresa_id, empleado_id, localidad_id, servicio_id, turno_id,
		       estatus, ultima_asignacion, fecha_inicio, fecha_fin, created_at, updated_at
		FROM asignacion.asignacion_asignacion
		WHERE id = $1`
	assignment, err := scanAssignment(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("assignment_repository.FindByID: %w", err)
	}
	return assignment, nil
}

// FindCurrentByEmployee loads the current active assignment for an employee.
func (r *PgxAssignmentRepository) FindCurrentByEmployee(ctx context.Context, employeeID uuid.UUID) (*Assignment, error) {
	const query = `
		SELECT id, empresa_id, empleado_id, localidad_id, servicio_id, turno_id,
		       estatus, ultima_asignacion, fecha_inicio, fecha_fin, created_at, updated_at
		FROM asignacion.asignacion_asignacion
		WHERE empleado_id = $1 AND ultima_asignacion = TRUE
		ORDER BY updated_at DESC
		LIMIT 1`
	assignment, err := scanAssignment(r.db.QueryRow(ctx, query, employeeID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("assignment_repository.FindCurrentByEmployee: %w", err)
	}
	return assignment, nil
}

// ListByClient returns assignments for a client-compatible identifier.
func (r *PgxAssignmentRepository) ListByClient(ctx context.Context, clientID uuid.UUID, filter AssignmentListFilter) ([]Assignment, error) {
	filter.Pagination = filter.Pagination.Normalize()
	query := `
		SELECT id, empresa_id, empleado_id, localidad_id, servicio_id, turno_id,
		       estatus, ultima_asignacion, fecha_inicio, fecha_fin, created_at, updated_at
		FROM asignacion.asignacion_asignacion
		WHERE empresa_id = $1`
	args := []any{clientID}
	if filter.OnlyCurrent != nil {
		query += fmt.Sprintf(" AND ultima_asignacion = $%d", len(args)+1)
		args = append(args, *filter.OnlyCurrent)
	}
	query += fmt.Sprintf(" ORDER BY fecha_inicio DESC, created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, filter.Pagination.Limit, filter.Pagination.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("assignment_repository.ListByClient: %w", err)
	}
	defer rows.Close()
	return scanAssignments(rows)
}

// CountByClient returns total assignments for a client-compatible identifier.
func (r *PgxAssignmentRepository) CountByClient(ctx context.Context, clientID uuid.UUID, filter AssignmentListFilter) (int, error) {
	query := `SELECT COUNT(*) FROM asignacion.asignacion_asignacion WHERE empresa_id = $1`
	args := []any{clientID}
	if filter.OnlyCurrent != nil {
		query += fmt.Sprintf(" AND ultima_asignacion = $%d", len(args)+1)
		args = append(args, *filter.OnlyCurrent)
	}
	var total int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("assignment_repository.CountByClient: %w", err)
	}
	return total, nil
}

// ListActiveBySupervisor returns a clear error until a real supervisor-to-assignment scope relation exists.
func (r *PgxAssignmentRepository) ListActiveBySupervisor(ctx context.Context, supervisorID uuid.UUID, filter Pagination) ([]Assignment, error) {
	return nil, ErrAssignmentSupervisorScopeNotImplemented
}

// CountActiveBySupervisor returns a clear error until a real supervisor-to-assignment scope relation exists.
func (r *PgxAssignmentRepository) CountActiveBySupervisor(ctx context.Context, supervisorID uuid.UUID) (int, error) {
	return 0, ErrAssignmentSupervisorScopeNotImplemented
}

// DeactivateCurrentForEmployee clears the current flag from active assignments for an employee.
func (r *PgxAssignmentRepository) DeactivateCurrentForEmployee(ctx context.Context, employeeID uuid.UUID, deactivatedAt time.Time) error {
	if err := execDeactivateCurrentForEmployee(ctx, r.db, employeeID, deactivatedAt); err != nil {
		return fmt.Errorf("assignment_repository.DeactivateCurrentForEmployee: %w", err)
	}
	return nil
}

func execDeactivateCurrentForEmployee(ctx context.Context, db assignmentDB, employeeID uuid.UUID, deactivatedAt time.Time) error {
	const query = `
		UPDATE asignacion.asignacion_asignacion
		SET ultima_asignacion = FALSE, updated_at = $2
		WHERE empleado_id = $1 AND ultima_asignacion = TRUE`
	_, err := db.Exec(ctx, query, employeeID, deactivatedAt)
	return err
}

// Update persists mutable assignment fields.
func (r *PgxAssignmentRepository) Update(ctx context.Context, assignment *Assignment) error {
	const query = `
		UPDATE asignacion.asignacion_asignacion
		SET localidad_id = $2, turno_id = $3, estatus = $4, fecha_fin = $5, updated_at = $6
		WHERE id = $1`
	tag, err := r.db.Exec(ctx, query,
		assignment.ID,
		assignment.LocationID,
		assignment.ShiftID,
		int(assignment.Status),
		assignment.EndDate,
		assignment.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("assignment_repository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}

func scanAssignment(row pgx.Row) (*Assignment, error) {
	var assignment Assignment
	var status int
	if err := row.Scan(
		&assignment.ID,
		&assignment.CompanyID,
		&assignment.EmployeeID,
		&assignment.LocationID,
		&assignment.ServiceID,
		&assignment.ShiftID,
		&status,
		&assignment.IsCurrent,
		&assignment.StartDate,
		&assignment.EndDate,
		&assignment.CreatedAt,
		&assignment.UpdatedAt,
	); err != nil {
		return nil, err
	}
	assignment.Status = AssignmentStatus(status)
	return &assignment, nil
}

func scanAssignments(rows pgx.Rows) ([]Assignment, error) {
	assignments := []Assignment{}
	for rows.Next() {
		assignment, err := scanAssignment(rows)
		if err != nil {
			return nil, fmt.Errorf("assignment_repository scan: %w", err)
		}
		assignments = append(assignments, *assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("assignment_repository rows: %w", err)
	}
	return assignments, nil
}

// RedisAssignmentCacheInvalidator invalidates cached assignment lookups.
type RedisAssignmentCacheInvalidator struct {
	client redis.Cmdable
}

// NewRedisAssignmentCacheInvalidator creates a Redis-backed cache invalidator.
func NewRedisAssignmentCacheInvalidator(client redis.Cmdable) *RedisAssignmentCacheInvalidator {
	return &RedisAssignmentCacheInvalidator{client: client}
}

// InvalidateEmployeeAssignments deletes assignment cache keys for an employee.
func (c *RedisAssignmentCacheInvalidator) InvalidateEmployeeAssignments(ctx context.Context, employeeID uuid.UUID) error {
	if c == nil || c.client == nil {
		return nil
	}
	keys := []string{
		"asignacion:empleado:" + employeeID.String() + ":active",
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("assignment_cache.InvalidateEmployeeAssignments: %w", err)
	}
	return nil
}

// PgxAssignedActivityRepository implements AssignedActivityRepository over PostgreSQL.
type PgxAssignedActivityRepository struct {
	db assignmentDB
}

// NewPgxAssignedActivityRepository creates a PostgreSQL assigned activity repository.
func NewPgxAssignedActivityRepository(pool *pgxpool.Pool) *PgxAssignedActivityRepository {
	return &PgxAssignedActivityRepository{db: pool}
}

// CreateBatch inserts a batch of assigned activities.
func (r *PgxAssignedActivityRepository) CreateBatch(ctx context.Context, activities []AssignedActivity) error {
	for i := range activities {
		if err := insertAssignedActivity(ctx, r.db, &activities[i]); err != nil {
			return fmt.Errorf("activity_repository.CreateBatch[%d]: %w", i, err)
		}
	}
	return nil
}

func insertAssignedActivity(ctx context.Context, db assignmentDB, a *AssignedActivity) error {
	const query = `
		INSERT INTO asignacion.asignacion_asignaactividad (
			id, asignacion_id, actividad_id, descripcion, frecuencia, orden, estatus,
			comentarios, evidencia_1, evidencia_2, evidencia_3,
			ubicacion_carga_lat, ubicacion_carga_lon, fecha_ejecucion, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`
	lat, lon, execAt := nullableActivityExecFields(a)
	_, err := db.Exec(ctx, query,
		a.ID, a.AssignmentID, a.ActivityID, a.Description, a.Frequency, a.Order, int(a.Status),
		nullString(a.Comments), nullString(a.Evidence1), nullString(a.Evidence2), nullString(a.Evidence3),
		lat, lon, execAt, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

// ListByAssignment returns activities for an assignment, optionally filtered by status.
func (r *PgxAssignedActivityRepository) ListByAssignment(ctx context.Context, assignmentID uuid.UUID, status *ActivityStatus) ([]AssignedActivity, error) {
	const base = `
		SELECT id, asignacion_id, actividad_id, descripcion, frecuencia, orden, estatus,
		       comentarios, evidencia_1, evidencia_2, evidencia_3,
		       ubicacion_carga_lat, ubicacion_carga_lon, fecha_ejecucion, created_at, updated_at
		FROM asignacion.asignacion_asignaactividad
		WHERE asignacion_id = $1`
	var (
		rows pgx.Rows
		err  error
	)
	if status != nil {
		rows, err = r.db.Query(ctx, base+" AND estatus = $2 ORDER BY orden ASC, id ASC", assignmentID, int(*status))
	} else {
		rows, err = r.db.Query(ctx, base+" ORDER BY orden ASC, id ASC", assignmentID)
	}
	if err != nil {
		return nil, fmt.Errorf("activity_repository.ListByAssignment: %w", err)
	}
	defer rows.Close()
	out := []AssignedActivity{}
	for rows.Next() {
		activity, scanErr := scanAssignedActivity(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("activity_repository.ListByAssignment scan: %w", scanErr)
		}
		out = append(out, *activity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("activity_repository.ListByAssignment rows: %w", err)
	}
	return out, nil
}

// UpdateEvidence persists the three evidence URLs, GPS coordinates, comment and completion timestamp, and marks the activity as completed.
func (r *PgxAssignedActivityRepository) UpdateEvidence(ctx context.Context, activityID uuid.UUID, evidence ActivityEvidenceUpdate) (*AssignedActivity, error) {
	const query = `
		UPDATE asignacion.asignacion_asignaactividad
		SET estatus = $2,
		    comentarios = $3,
		    evidencia_1 = $4,
		    evidencia_2 = $5,
		    evidencia_3 = $6,
		    ubicacion_carga_lat = $7,
		    ubicacion_carga_lon = $8,
		    fecha_ejecucion = $9,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, asignacion_id, actividad_id, descripcion, frecuencia, orden, estatus,
		          comentarios, evidencia_1, evidencia_2, evidencia_3,
		          ubicacion_carga_lat, ubicacion_carga_lon, fecha_ejecucion, created_at, updated_at`
	urls := evidence.EvidenceURLs
	var e1, e2, e3 any
	switch len(urls) {
	case 0:
	case 1:
		e1 = urls[0]
	default:
		e1 = urls[0]
		e2 = urls[1]
		if len(urls) > 2 {
			e3 = urls[2]
		}
	}
	row := r.db.QueryRow(ctx, query,
		activityID,
		int(ActivityStatusCompleted),
		nullString(evidence.Comment),
		e1, e2, e3,
		evidence.Latitude, evidence.Longitude,
		evidence.CompletedAt,
	)
	activity, err := scanAssignedActivity(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrActivityNotFound
		}
		return nil, fmt.Errorf("activity_repository.UpdateEvidence: %w", err)
	}
	return activity, nil
}

// BatchUpdateStatus applies status changes to a list of activities in a single statement batch.
func (r *PgxAssignedActivityRepository) BatchUpdateStatus(ctx context.Context, updates []ActivityStatusUpdate) ([]AssignedActivity, error) {
	txDB, ok := r.db.(assignmentTxDB)
	if !ok {
		return nil, fmt.Errorf("activity_repository.BatchUpdateStatus: transaction support unavailable")
	}
	tx, err := txDB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("activity_repository.BatchUpdateStatus begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	out := make([]AssignedActivity, 0, len(updates))
	for _, update := range updates {
		const query = `
			UPDATE asignacion.asignacion_asignaactividad
			SET estatus = $2,
			    comentarios = $3,
			    updated_at = $4
			WHERE id = $1
			RETURNING id, asignacion_id, actividad_id, descripcion, frecuencia, orden, estatus,
			          comentarios, evidencia_1, evidencia_2, evidencia_3,
			          ubicacion_carga_lat, ubicacion_carga_lon, fecha_ejecucion, created_at, updated_at`
		row := tx.QueryRow(ctx, query, update.ActivityID, int(update.Status), nullString(update.Comment), update.UpdatedAt)
		activity, scanErr := scanAssignedActivity(row)
		if scanErr != nil {
			if errors.Is(scanErr, pgx.ErrNoRows) {
				return nil, fmt.Errorf("%w: %s", ErrActivityNotFound, update.ActivityID)
			}
			return nil, fmt.Errorf("activity_repository.BatchUpdateStatus[%s]: %w", update.ActivityID, scanErr)
		}
		out = append(out, *activity)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("activity_repository.BatchUpdateStatus commit: %w", err)
	}
	committed = true
	return out, nil
}

func scanAssignedActivity(row pgx.Row) (*AssignedActivity, error) {
	var a AssignedActivity
	var status int
	var comments, e1, e2, e3 *string
	var lat, lon *float64
	var execAt *time.Time
	if err := row.Scan(
		&a.ID, &a.AssignmentID, &a.ActivityID, &a.Description, &a.Frequency, &a.Order, &status,
		&comments, &e1, &e2, &e3,
		&lat, &lon, &execAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, err
	}
	a.Status = ActivityStatus(status)
	if comments != nil {
		a.Comments = *comments
	}
	if e1 != nil {
		a.Evidence1 = *e1
	}
	if e2 != nil {
		a.Evidence2 = *e2
	}
	if e3 != nil {
		a.Evidence3 = *e3
	}
	a.UploadLatitude = lat
	a.UploadLongitude = lon
	a.ExecutionDate = execAt
	return &a, nil
}

func nullableActivityExecFields(a *AssignedActivity) (lat, lon any, execAt any) {
	if a.UploadLatitude != nil {
		lat = *a.UploadLatitude
	}
	if a.UploadLongitude != nil {
		lon = *a.UploadLongitude
	}
	if a.ExecutionDate != nil {
		execAt = *a.ExecutionDate
	}
	return
}

// PgxAssignedToolRepository implements AssignedToolRepository over PostgreSQL.
type PgxAssignedToolRepository struct {
	db assignmentDB
}

// NewPgxAssignedToolRepository creates a PostgreSQL assigned tool repository.
func NewPgxAssignedToolRepository(pool *pgxpool.Pool) *PgxAssignedToolRepository {
	return &PgxAssignedToolRepository{db: pool}
}

// CreateBatch inserts a batch of assigned tools.
func (r *PgxAssignedToolRepository) CreateBatch(ctx context.Context, tools []AssignedTool) error {
	for i := range tools {
		if err := insertAssignedTool(ctx, r.db, &tools[i]); err != nil {
			return fmt.Errorf("tool_repository.CreateBatch[%d]: %w", i, err)
		}
	}
	return nil
}

func insertAssignedTool(ctx context.Context, db assignmentDB, t *AssignedTool) error {
	const query = `
		INSERT INTO asignacion.asignacion_asignaherramienta (
			id, asignacion_id, herramienta_id, nombre, cantidad, especificaciones,
			estatus_entrega, fecha_entrega, fecha_devolucion, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	deliveredAt, returnedAt := nullableToolDeliveryFields(t)
	_, err := db.Exec(ctx, query,
		t.ID, t.AssignmentID, t.ToolID, t.Name, t.Quantity, nullString(t.Specifications),
		int(t.DeliveryStatus), deliveredAt, returnedAt, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func nullableToolDeliveryFields(t *AssignedTool) (deliveredAt, returnedAt any) {
	if t.DeliveredAt != nil {
		deliveredAt = *t.DeliveredAt
	}
	if t.ReturnedAt != nil {
		returnedAt = *t.ReturnedAt
	}
	return
}

// ListBySupervisor returns tools for active assignments under a supervisor scope.
// The current schema lacks a canonical supervisor relationship, so empresa_id from the assignment is used as the supervisor proxy.
func (r *PgxAssignedToolRepository) ListBySupervisor(ctx context.Context, supervisorID uuid.UUID, status *ToolDeliveryStatus) ([]AssignedTool, error) {
	const base = `
		SELECT t.id, t.asignacion_id, t.herramienta_id, t.nombre, t.cantidad, t.especificaciones,
		       t.estatus_entrega, t.fecha_entrega, t.fecha_devolucion, t.created_at, t.updated_at
		FROM asignacion.asignacion_asignaherramienta t
		JOIN asignacion.asignacion_asignacion a ON a.id = t.asignacion_id
		WHERE a.empresa_id = $1
		  AND a.ultima_asignacion = TRUE`
	var (
		rows pgx.Rows
		err  error
	)
	if status != nil {
		rows, err = r.db.Query(ctx, base+" AND t.estatus_entrega = $2 ORDER BY t.created_at DESC, t.id ASC", supervisorID, int(*status))
	} else {
		rows, err = r.db.Query(ctx, base+" ORDER BY t.created_at DESC, t.id ASC", supervisorID)
	}
	if err != nil {
		return nil, fmt.Errorf("tool_repository.ListBySupervisor: %w", err)
	}
	defer rows.Close()
	out := []AssignedTool{}
	for rows.Next() {
		tool, scanErr := scanAssignedTool(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("tool_repository.ListBySupervisor scan: %w", scanErr)
		}
		out = append(out, *tool)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tool_repository.ListBySupervisor rows: %w", err)
	}
	return out, nil
}

func scanAssignedTool(row pgx.Row) (*AssignedTool, error) {
	var t AssignedTool
	var status int
	var specs *string
	var deliveredAt, returnedAt *time.Time
	if err := row.Scan(
		&t.ID, &t.AssignmentID, &t.ToolID, &t.Name, &t.Quantity, &specs,
		&status, &deliveredAt, &returnedAt, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if specs != nil {
		t.Specifications = *specs
	}
	t.DeliveryStatus = ToolDeliveryStatus(status)
	t.DeliveredAt = deliveredAt
	t.ReturnedAt = returnedAt
	return &t, nil
}

// PgxEmployeeEvaluationRepository implements EmployeeEvaluationRepository over PostgreSQL.
type PgxEmployeeEvaluationRepository struct {
	db assignmentDB
}

// NewPgxEmployeeEvaluationRepository creates a PostgreSQL employee evaluation repository.
func NewPgxEmployeeEvaluationRepository(pool *pgxpool.Pool) *PgxEmployeeEvaluationRepository {
	return &PgxEmployeeEvaluationRepository{db: pool}
}

// Create inserts an employee evaluation row.
func (r *PgxEmployeeEvaluationRepository) Create(ctx context.Context, evaluation *EmployeeEvaluation) error {
	const query = `
		INSERT INTO asignacion.asignacion_evaluacionempleado (
			id, asignacion_id, empleado_id, evaluador_id, calificacion,
			cumple_actividades, comentarios, fecha_evaluacion, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.db.Exec(ctx, query,
		evaluation.ID,
		evaluation.AssignmentID,
		evaluation.EmployeeID,
		evaluation.EvaluatorID,
		evaluation.Rating,
		evaluation.ActivitiesCompliant,
		nullString(evaluation.Comments),
		evaluation.EvaluationDate,
		evaluation.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("evaluation_repository.Create: %w", err)
	}
	return nil
}

// ListByEmployee returns paginated evaluations for an employee, ordered by most recent first.
func (r *PgxEmployeeEvaluationRepository) ListByEmployee(ctx context.Context, employeeID uuid.UUID, filter Pagination) ([]EmployeeEvaluation, error) {
	const query = `
		SELECT id, asignacion_id, empleado_id, evaluador_id, calificacion,
		       cumple_actividades, comentarios, fecha_evaluacion, created_at
		FROM asignacion.asignacion_evaluacionempleado
		WHERE empleado_id = $1
		ORDER BY fecha_evaluacion DESC, created_at DESC, id ASC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, employeeID, filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("evaluation_repository.ListByEmployee: %w", err)
	}
	defer rows.Close()
	out := []EmployeeEvaluation{}
	for rows.Next() {
		evaluation, scanErr := scanEmployeeEvaluation(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("evaluation_repository.ListByEmployee scan: %w", scanErr)
		}
		out = append(out, *evaluation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("evaluation_repository.ListByEmployee rows: %w", err)
	}
	return out, nil
}

func scanEmployeeEvaluation(row pgx.Row) (*EmployeeEvaluation, error) {
	var e EmployeeEvaluation
	var comments *string
	if err := row.Scan(
		&e.ID, &e.AssignmentID, &e.EmployeeID, &e.EvaluatorID, &e.Rating,
		&e.ActivitiesCompliant, &comments, &e.EvaluationDate, &e.CreatedAt,
	); err != nil {
		return nil, err
	}
	if comments != nil {
		e.Comments = *comments
	}
	return &e, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
