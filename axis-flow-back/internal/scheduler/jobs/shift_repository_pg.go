// Package jobs — pgx implementation of the ShiftRepository
// contract. The file lives alongside `shift_repository.go` so
// the interface and its production implementation can be
// reviewed together.
//
// All SQL is built with positional parameters ($1, $2, …) per
// the spec's "Seguridad > SQL Injection Prevention (Hardening)"
// section. No string interpolation, no fmt.Sprintf in WHERE
// clauses.
//
// The file imports *pgxpool.Pool directly. The interface
// abstraction lives one level up in `shift_repository.go`; the
// pgx pool is wrapped at the wiring layer
// (cmd/server/scheduler_wiring.go) and the resulting
// *pgShiftRepository is injected into the job's constructor.
package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgShiftRepository is the production ShiftRepository, backed
// by a *pgxpool.Pool. The struct is intentionally small — the
// wiring layer in main.go constructs it once at startup and
// passes the value to the job's constructor.
type pgShiftRepository struct {
	pool *pgxpool.Pool
}

// NewPgShiftRepository returns a ShiftRepository wired to the
// supplied pool. The pool is captured by reference; the
// returned repository shares the pool's lifecycle (closing
// the pool is the wiring layer's responsibility).
func NewPgShiftRepository(pool *pgxpool.Pool) ShiftRepository {
	return &pgShiftRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// SQL — prepared statements (positional parameters only).
// ---------------------------------------------------------------------------

// upcomingShiftsQuery is the canonical "shifts in the next 15
// minutes" query. The shape matches the in-package assumption
// documented in `notificaciones_en_tiempo_real.go`:
//
//	asignacion.asignacion_asignacion(
//	    id            UUID PRIMARY KEY,
//	    empleado_id   BIGINT NOT NULL,
//	    estatus       INT    NOT NULL,
//	    hora_inicio   TIMESTAMPTZ NOT NULL,
//	    tipo_evento   VARCHAR NOT NULL
//	)
//
// The WHERE clause is the strict 15-minute window from the
// spec's Gherkin scenario 1 ("ventana estricta de 15 minutos").
// ORDER BY hora_inicio, id is deterministic so dedup-cache
// iteration order is stable across runs.
const upcomingShiftsQuery = `
SELECT
    a.id,
    a.empleado_id,
    a.tipo_evento,
    a.hora_inicio
FROM asignacion.asignacion_asignacion AS a
WHERE a.estatus = 1
  AND a.hora_inicio > $1
  AND a.hora_inicio <= $2
ORDER BY a.hora_inicio, a.id
`

// activeDevicesForEmpleadoQuery returns the active device rows
// for a single empleado. The shape matches the documented
// schema for empleados.empleados_user_devices. The result is
// sorted by id for deterministic iteration order in the
// per-device loop.
const activeDevicesForEmpleadoQuery = `
SELECT
    d.id,
    d.fcm_token,
    d.is_active
FROM empleados.empleados_user_devices AS d
WHERE d.empleado_id = $1
  AND d.is_active = TRUE
ORDER BY d.id
`

// deactivateDeviceQuery is the prepared statement the job runs
// when an FCM Send returns ErrFCMPermanentFailure. The
// idempotency guard `AND is_active = TRUE` is intentional —
// the same WHERE clause is on the active-devices SELECT, so
// the device is left untouched if it was already deactivated
// by a concurrent call (defence in depth: the per-job
// distributed lock held by the cron runner is the primary
// guard).
const deactivateDeviceQuery = `
UPDATE empleados.empleados_user_devices
SET is_active = FALSE,
    updated_at = $2
WHERE id = $1
  AND is_active = TRUE
`

// ---------------------------------------------------------------------------
// Method implementations.
// ---------------------------------------------------------------------------

// UpcomingShiftsInWindow returns every assignment whose
// hora_inicio falls in the half-open window (from, to] and
// whose estatus is "active" (1). The (from, to] window is the
// strict 15-minute window documented in Gherkin scenario 1.
func (r *pgShiftRepository) UpcomingShiftsInWindow(ctx context.Context, from, to time.Time) ([]UpcomingShift, error) {
	rows, err := r.pool.Query(ctx, upcomingShiftsQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("ShiftRepository.UpcomingShiftsInWindow: %w", err)
	}
	defer rows.Close()

	out := make([]UpcomingShift, 0, 16)
	for rows.Next() {
		var s UpcomingShift
		if scanErr := rows.Scan(&s.ShiftID, &s.EmpleadoID, &s.TipoEvento, &s.HoraInicio); scanErr != nil {
			return nil, fmt.Errorf("ShiftRepository.UpcomingShiftsInWindow scan: %w", scanErr)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ShiftRepository.UpcomingShiftsInWindow rows: %w", err)
	}
	return out, nil
}

// ActiveDevicesForEmpleado returns the active device rows
// (is_active = TRUE) for the supplied empleado id. An empty
// result is a valid "no devices" answer, not an error — the
// caller iterates the result and falls through to the next
// shift on empty.
func (r *pgShiftRepository) ActiveDevicesForEmpleado(ctx context.Context, empleadoID int64) ([]Device, error) {
	rows, err := r.pool.Query(ctx, activeDevicesForEmpleadoQuery, empleadoID)
	if err != nil {
		return nil, fmt.Errorf("ShiftRepository.ActiveDevicesForEmpleado: %w", err)
	}
	defer rows.Close()

	out := make([]Device, 0, 2)
	for rows.Next() {
		var d Device
		if scanErr := rows.Scan(&d.ID, &d.FCMToken, &d.IsActive); scanErr != nil {
			return nil, fmt.Errorf("ShiftRepository.ActiveDevicesForEmpleado scan: %w", scanErr)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ShiftRepository.ActiveDevicesForEmpleado rows: %w", err)
	}
	return out, nil
}

// DeactivateDevice flips the device row to is_active = FALSE.
// The function is idempotent: a row whose is_active is already
// FALSE is left untouched and the UPDATE returns 0 rows. The
// caller treats the 0-row result as a no-op and does not
// surface an error.
func (r *pgShiftRepository) DeactivateDevice(ctx context.Context, id uuid.UUID, now time.Time) error {
	_, err := r.pool.Exec(ctx, deactivateDeviceQuery, id, now)
	if err != nil {
		return fmt.Errorf("ShiftRepository.DeactivateDevice: %w", err)
	}
	return nil
}
