// Package jobs holds the concrete business scheduled jobs the
// Scheduler service runs on a cron cadence.
//
// This file defines the ShiftRepository contract the
// `notificaciones_en_tiempo_real` job (PR 3B-i + this PR — 5B-ii)
// uses to read upcoming shifts and active mobile devices, and to
// deactivate devices whose FCM token returns a permanent failure.
//
// The interface is intentionally small: it contains ONLY the
// methods the job's Run flow needs, so the test suite can mock
// the contract directly without dragging the *pgxpool.Pool type
// into the test binary. The pgx implementation lives in the
// sibling file `shift_repository_pg.go`.
//
// Design notes
// ============
//  1. The two read methods (UpcomingShiftsInWindow and
//     ActiveDevicesForEmpleado) are kept separate rather than
//     collapsed into a single JOIN. The original SQL joined
//     asignacion_asignacion with empleados_user_devices in one
//     round-trip; the refactor splits the query so the job can
//     iterate shifts first and then ask for the per-empleado
//     device list. The split costs one extra round-trip per
//     empleado per tick — well within the 5-minute cron
//     cadence — and lets tests stub the two surfaces
//     independently. The production data shape is unchanged
//     (the same (shift, device) tuples the old JOIN produced
//     are produced by the two-method sequence).
//  2. The DeactivateDevice method takes a `time.Time`
//     explicitly so the test can drive the UPDATE without
//     mocking `time.Now`. The production caller passes
//     `time.Now().UTC()`.
//  3. The interface lives in this package (not in a sibling
//     "repository" package) because the job consumes it. A
//     future cross-service consumer would justify promoting the
//     type to its own package; that is not the case today.
package jobs

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// ShiftRepository — public contract.
// ---------------------------------------------------------------------------

// ShiftRepository is the data-access surface the
// notificaciones_en_tiempo_real job needs. The methods are
// concurrency-safe: the underlying pgx pool is, and the
// interface is the only place the job talks to PostgreSQL.
type ShiftRepository interface {
	// UpcomingShiftsInWindow returns every assignment whose
	// hora_inicio falls in the half-open window (from, to] and
	// whose estatus is "active" (1). The result is sorted by
	// (hora_inicio, id) so the caller can iterate in
	// chronological order; the deterministic ordering also
	// keeps the dedup cache keys hit in the same order across
	// runs.
	UpcomingShiftsInWindow(ctx context.Context, from, to time.Time) ([]UpcomingShift, error)

	// ActiveDevicesForEmpleado returns the active device rows
	// (is_active = TRUE) registered for the given empleado.
	// The list is sorted by id for deterministic iteration
	// order in the job's inner loop.
	ActiveDevicesForEmpleado(ctx context.Context, empleadoID int64) ([]Device, error)

	// DeactivateDevice flips the device row to is_active =
	// FALSE and stamps `now` on updated_at. The id is the
	// UUID primary key. The WHERE clause is idempotent: a
	// device that has already been deactivated is left
	// untouched and the UPDATE returns 0 rows. Errors are
	// returned unwrapped so the job's %w wrapping preserves
	// the underlying pgx error class.
	DeactivateDevice(ctx context.Context, id uuid.UUID, now time.Time) error
}

// ---------------------------------------------------------------------------
// Value types — small, allocation-free structs returned by the
// repository. Field names match the underlying column names where
// possible so the SQL scan code stays one-to-one with the SELECT
// list.
// ---------------------------------------------------------------------------

// UpcomingShift is the (shift, employee) tuple produced by
// UpcomingShiftsInWindow. The shift id is the UUID primary key
// of asignacion_asignacion; the empleado id is the FK to
// empleados_empleado. HoraInicio is included for the dedup
// logging path; TipoEvento is one of "entrada", "comida",
// "salida".
type UpcomingShift struct {
	ShiftID    uuid.UUID
	EmpleadoID int64
	TipoEvento string
	HoraInicio time.Time
}

// Device is the (device, fcm_token) tuple produced by
// ActiveDevicesForEmpleado. The id is the UUID primary key of
// empleados_user_devices. FCMToken is the (potentially long)
// device push token registered in the empleados service. The
// IsActive flag is the live read of the column — callers that
// only want active devices still get it because the WHERE
// clause already filters; the field is exposed so future
// consumers (e.g. an admin audit job) can iterate the full
// device list without changing the interface.
type Device struct {
	ID       uuid.UUID
	FCMToken string
	IsActive bool
}
