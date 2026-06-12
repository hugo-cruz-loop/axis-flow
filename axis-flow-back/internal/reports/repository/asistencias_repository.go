package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"axis-flow-back/internal/reports"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// AsistenciasRepo interface
// ---------------------------------------------------------------------------

// AsistenciasRepo fetches paginated attendance records.
type AsistenciasRepo interface {
	// List returns attendance records for empresaID with optional filters.
	// All positional parameters are safe; empresa_id is ALWAYS applied first.
	List(ctx context.Context, empresaID string, employeeID *string, status *string, dateFrom *string, dateTo *string, page, limit int) ([]reports.AsistenciaRecord, int, error)
}

// ---------------------------------------------------------------------------
// pgAsistenciasRepo — PostgreSQL implementation
// ---------------------------------------------------------------------------

type pgAsistenciasRepo struct {
	db *pgxpool.Pool
}

// NewPgAsistenciasRepo creates a PostgreSQL-backed AsistenciasRepo.
func NewPgAsistenciasRepo(pool *pgxpool.Pool) AsistenciasRepo {
	return &pgAsistenciasRepo{db: pool}
}

func (r *pgAsistenciasRepo) List(ctx context.Context, empresaID string, employeeID *string, status *string, dateFrom *string, dateTo *string, page, limit int) ([]reports.AsistenciaRecord, int, error) {
	args := []any{empresaID}
	argN := 1

	var where strings.Builder
	where.WriteString("WHERE emp.empresa_id = $1")

	if employeeID != nil {
		argN++
		where.WriteString(fmt.Sprintf(" AND a.empleado_id::text = $%d", argN))
		args = append(args, *employeeID)
	}
	if status != nil {
		argN++
		where.WriteString(fmt.Sprintf(" AND a.estatus = $%d", argN))
		args = append(args, *status)
	}
	if dateFrom != nil {
		argN++
		where.WriteString(fmt.Sprintf(" AND a.clock_in::date >= $%d", argN))
		args = append(args, *dateFrom)
	}
	if dateTo != nil {
		argN++
		where.WriteString(fmt.Sprintf(" AND a.clock_in::date <= $%d", argN))
		args = append(args, *dateTo)
	}

	countSQL := `
		SELECT COUNT(*)
		FROM empleados.empleados_asistencias a
		JOIN empleados.empleados_empleado emp ON emp.id = a.empleado_id
		` + where.String()

	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("asistencias_repository.List count: %w", err)
	}
	if total == 0 {
		return []reports.AsistenciaRecord{}, 0, nil
	}

	offset := (page - 1) * limit
	argN++
	limitArg := argN
	argN++
	offsetArg := argN
	args = append(args, limit, offset)

	selectSQL := fmt.Sprintf(`
		SELECT
		    a.id::text,
		    emp.nombre || ' ' || emp.apellido AS empleado_nombre,
		    emp.codigo AS empleado_codigo,
		    a.clock_in::text,
		    COALESCE(a.clock_out::text, ''),
		    a.estatus,
		    COALESCE(a.retraso_minutos, 0),
		    COALESCE(a.latitud, 0),
		    COALESCE(a.longitud, 0)
		FROM empleados.empleados_asistencias a
		JOIN empleados.empleados_empleado emp ON emp.id = a.empleado_id
		%s
		ORDER BY a.clock_in DESC
		LIMIT $%d OFFSET $%d`, where.String(), limitArg, offsetArg)

	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("asistencias_repository.List query: %w", err)
	}
	defer rows.Close()

	var out []reports.AsistenciaRecord
	for rows.Next() {
		var rec reports.AsistenciaRecord
		if scanErr := rows.Scan(
			&rec.ID, &rec.EmpleadoNombre, &rec.EmpleadoCodigo,
			&rec.ClockIn, &rec.ClockOut, &rec.Status,
			&rec.DelayMinutes, &rec.Latitud, &rec.Longitud,
		); scanErr != nil {
			return nil, 0, fmt.Errorf("asistencias_repository.List scan: %w", scanErr)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("asistencias_repository.List rows: %w", err)
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// InMemAsistenciasRepo — in-memory stub for unit tests
// ---------------------------------------------------------------------------

type inMemAsistenciaEntry struct {
	rec        reports.AsistenciaRecord
	empresaID  string
	employeeID string
	fecha      string // date string for range filtering (YYYY-MM-DD)
}

// InMemAsistenciasRepo is a goroutine-safe in-memory AsistenciasRepo for tests.
type InMemAsistenciasRepo struct {
	mu      sync.RWMutex
	entries []inMemAsistenciaEntry
}

// NewInMemAsistenciasRepo creates an empty in-memory AsistenciasRepo.
func NewInMemAsistenciasRepo() *InMemAsistenciasRepo {
	return &InMemAsistenciasRepo{}
}

// Seed adds an AsistenciaRecord for the given empresaID.
func (r *InMemAsistenciasRepo) Seed(rec reports.AsistenciaRecord, empresaID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, inMemAsistenciaEntry{rec: rec, empresaID: empresaID})
	return nil
}

// SeedWithEmployee adds a record associated with a specific employee ID.
func (r *InMemAsistenciasRepo) SeedWithEmployee(rec reports.AsistenciaRecord, empresaID, employeeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, inMemAsistenciaEntry{rec: rec, empresaID: empresaID, employeeID: employeeID})
	return nil
}

// SeedWithDate adds a record with a specific date string for date-range tests.
func (r *InMemAsistenciasRepo) SeedWithDate(rec reports.AsistenciaRecord, empresaID, fecha string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, inMemAsistenciaEntry{rec: rec, empresaID: empresaID, fecha: fecha})
	return nil
}

func (r *InMemAsistenciasRepo) List(_ context.Context, empresaID string, employeeID *string, status *string, dateFrom *string, dateTo *string, page, limit int) ([]reports.AsistenciaRecord, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []reports.AsistenciaRecord
	for _, e := range r.entries {
		if e.empresaID != empresaID {
			continue
		}
		if employeeID != nil && e.employeeID != *employeeID {
			continue
		}
		if status != nil && e.rec.Status != *status {
			continue
		}
		if dateFrom != nil && e.fecha < *dateFrom {
			continue
		}
		if dateTo != nil && e.fecha > *dateTo {
			continue
		}
		filtered = append(filtered, e.rec)
	}

	total := len(filtered)
	offset := (page - 1) * limit
	if offset >= total {
		return []reports.AsistenciaRecord{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}
