package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	identity "axis-flow-back/internal/domain"
	empleados "axis-flow-back/internal/empleados"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateEmpleadoParams contains identity and business data needed to create an employee atomically.
type CreateEmpleadoParams struct {
	UsuarioID       uuid.UUID
	TenantID        uuid.UUID
	Email           string
	PasswordHash    string
	IDEmpleado      string
	EmpresaID       int64
	Nombre          string
	ApellidoPaterno string
	ApellidoMaterno string
	CreatedBy       uuid.UUID
}

// EmpleadoRepository persists empleados aggregates.
type EmpleadoRepository struct{ db dbTransactor }

// NewEmpleadoRepository creates an EmpleadoRepository with a testable DB adapter.
func NewEmpleadoRepository(db dbTransactor) *EmpleadoRepository { return &EmpleadoRepository{db: db} }

// NewPgxEmpleadoRepository creates an EmpleadoRepository backed by pgxpool.
func NewPgxEmpleadoRepository(pool *pgxpool.Pool) *EmpleadoRepository {
	return NewEmpleadoRepository(pgxDB{pool: pool})
}

// Create inserts users.identity_users, empleados.empleados_empleado, and the EMPLEADO role in one transaction.
func (r *EmpleadoRepository) Create(ctx context.Context, p CreateEmpleadoParams) (*empleados.Empleado, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("empleadoRepository.Create begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if p.UsuarioID == uuid.Nil {
		p.UsuarioID = uuid.New()
	}
	createdBy := any(nil)
	if p.CreatedBy != uuid.Nil {
		createdBy = p.CreatedBy
	}
	if strings.TrimSpace(p.PasswordHash) == "" {
		p.PasswordHash = "PENDING_ACTIVATION"
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO users.identity_users
            (id, tenant_id, email, password_hash, first_name, last_name, status, failed_login_attempts, created_at, updated_at, created_by)
         VALUES ($1,$2,$3,$4,$5,$6,$7,0,NOW(),NOW(),$8)`,
		p.UsuarioID, p.TenantID, p.Email, p.PasswordHash, p.Nombre, p.ApellidoPaterno, string(identity.StatusPendingActivation), createdBy)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, empleados.ErrEmpleadoAlreadyExists
		}
		return nil, fmt.Errorf("empleadoRepository.Create identity user: %w", err)
	}

	e := &empleados.Empleado{UsuarioID: p.UsuarioID, IDEmpleado: p.IDEmpleado, EmpresaID: p.EmpresaID, Nombre: p.Nombre, ApellidoPaterno: p.ApellidoPaterno, ApellidoMaterno: p.ApellidoMaterno}
	err = tx.QueryRow(ctx,
		`INSERT INTO empleados.empleados_empleado
            (id_empleado, usuario_id, empresa_id, nombre, apellido_paterno, apellido_materno, status)
         VALUES ($1,$2,$3,$4,$5,$6,$7)
         RETURNING num_empleado, status, created_at, updated_at`,
		p.IDEmpleado, p.UsuarioID, p.EmpresaID, p.Nombre, p.ApellidoPaterno, p.ApellidoMaterno, empleados.EmpleadoStatusIncompleto).
		Scan(&e.NumEmpleado, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, empleados.ErrDuplicateIdEmpleado
		}
		return nil, fmt.Errorf("empleadoRepository.Create empleado: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO users.identity_user_roles (user_id, role_id, assigned_at)
         SELECT $1, id, NOW()
         FROM users.identity_roles
         WHERE code = $2`,
		p.UsuarioID, "EMPLEADO")
	if err != nil {
		return nil, fmt.Errorf("empleadoRepository.Create role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("empleadoRepository.Create commit: %w", err)
	}
	committed = true
	return e, nil
}

// GetByID returns an employee by primary key scoped to empresa_id.
func (r *EmpleadoRepository) GetByID(ctx context.Context, numEmpleado, empresaID int64) (*empleados.Empleado, error) {
	return scanEmpleado(r.db.QueryRow(ctx,
		`SELECT num_empleado, id_empleado, usuario_id, empresa_id, nombre, apellido_paterno,
                COALESCE(apellido_materno,''), status, created_at, updated_at
         FROM empleados.empleados_empleado
         WHERE num_empleado=$1 AND empresa_id=$2`, numEmpleado, empresaID))
}

// GetByEmpresa returns employees for one empresa.
func (r *EmpleadoRepository) GetByEmpresa(ctx context.Context, empresaID int64) ([]empleados.Empleado, error) {
	rows, err := r.db.Query(ctx,
		`SELECT num_empleado, id_empleado, usuario_id, empresa_id, nombre, apellido_paterno,
                COALESCE(apellido_materno,''), status, created_at, updated_at
         FROM empleados.empleados_empleado
         WHERE empresa_id=$1
         ORDER BY num_empleado ASC`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("empleadoRepository.GetByEmpresa: %w", err)
	}
	defer rows.Close()
	return scanEmpleadoRows(rows)
}

// GetByUserID returns an employee by linked identity user scoped to empresa_id.
func (r *EmpleadoRepository) GetByUserID(ctx context.Context, userID uuid.UUID, empresaID int64) (*empleados.Empleado, error) {
	return scanEmpleado(r.db.QueryRow(ctx,
		`SELECT num_empleado, id_empleado, usuario_id, empresa_id, nombre, apellido_paterno,
                COALESCE(apellido_materno,''), status, created_at, updated_at
         FROM empleados.empleados_empleado
         WHERE usuario_id=$1 AND empresa_id=$2`, userID, empresaID))
}

// GetNumEmpleadoByUserID returns the num_empleado of the first
// empleado linked to the given identity user. PR-5 (5.0) seam: the
// auth service uses this to enrich the access token with the JWT
// empleado_id claim.
//
// Returns:
//
//	- (numEmpleado > 0, nil) when an empleado is linked.
//	- (0, nil)            when no empleado is linked (the auth
//	                      flow is NOT blocked — see auth_service.EmpleadoLookup docs).
//	- (0, err)            on genuine lookup failures (DB down, etc.).
//
// The lookup is by usuario_id alone (no empresa filter). A user is
// expected to be linked to at most one empleado; if multiple rows
// exist the lowest num_empleado wins (deterministic). The
// production schema enforces the link at create-time via
// CreateEmpleadoParams.
func (r *EmpleadoRepository) GetNumEmpleadoByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var numEmpleado int64
	err := r.db.QueryRow(ctx,
		`SELECT num_empleado
         FROM empleados.empleados_empleado
         WHERE usuario_id=$1
         ORDER BY num_empleado ASC
         LIMIT 1`, userID).Scan(&numEmpleado)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("empleadoRepository.GetNumEmpleadoByUserID: %w", err)
	}
	return numEmpleado, nil
}

// SoftDelete marks the employee as baja and deactivates the linked identity user atomically.
func (r *EmpleadoRepository) SoftDelete(ctx context.Context, numEmpleado, empresaID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("empleadoRepository.SoftDelete begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	var userID uuid.UUID
	err = tx.QueryRow(ctx,
		`UPDATE empleados.empleados_empleado
         SET status=$1, updated_at=NOW()
         WHERE num_empleado=$2 AND empresa_id=$3
         RETURNING usuario_id`, empleados.EmpleadoStatusBaja, numEmpleado, empresaID).
		Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return empleados.ErrEmpleadoNotFound
	}
	if err != nil {
		return fmt.Errorf("empleadoRepository.SoftDelete empleado: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE users.identity_users
         SET status=$1, updated_at=NOW()
         WHERE id=$2`, string(identity.StatusInactive), userID)
	if err != nil {
		return fmt.Errorf("empleadoRepository.SoftDelete user: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return empleados.ErrEmpleadoNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("empleadoRepository.SoftDelete commit: %w", err)
	}
	committed = true
	return nil
}

// Update changes employee profile data scoped to empresa_id.
func (r *EmpleadoRepository) Update(ctx context.Context, e *empleados.Empleado) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE empleados.empleados_empleado
         SET nombre=$1, apellido_paterno=$2, apellido_materno=$3, status=$4, updated_at=NOW()
         WHERE num_empleado=$5 AND empresa_id=$6`,
		e.Nombre, e.ApellidoPaterno, e.ApellidoMaterno, e.Status, e.NumEmpleado, e.EmpresaID)
	if err != nil {
		return fmt.Errorf("empleadoRepository.Update: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return empleados.ErrEmpleadoNotFound
	}
	return nil
}

func scanEmpleado(row pgx.Row) (*empleados.Empleado, error) {
	var e empleados.Empleado
	err := row.Scan(&e.NumEmpleado, &e.IDEmpleado, &e.UsuarioID, &e.EmpresaID, &e.Nombre, &e.ApellidoPaterno, &e.ApellidoMaterno, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empleados.ErrEmpleadoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("empleadoRepository scan: %w", err)
	}
	return &e, nil
}

func scanEmpleadoRows(rows pgx.Rows) ([]empleados.Empleado, error) {
	out := []empleados.Empleado{}
	for rows.Next() {
		e, err := scanEmpleado(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("empleadoRepository rows: %w", err)
	}
	return out, nil
}
