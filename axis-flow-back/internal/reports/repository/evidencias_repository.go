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
// EvidenciasRepo interface
// ---------------------------------------------------------------------------

// EvidenciasRepo fetches paginated activity evidence records.
type EvidenciasRepo interface {
	// List returns evidence records for empresaID, optionally filtered by
	// clienteID and/or fecha (YYYY-MM-DD). Returns (items, total, error).
	// empresa_id is ALWAYS applied as a positional parameter for tenant isolation.
	List(ctx context.Context, empresaID string, clienteID *string, fecha *string, page, limit int) ([]reports.Evidencia, int, error)
}

// ---------------------------------------------------------------------------
// pgEvidenciasRepo — PostgreSQL implementation
// ---------------------------------------------------------------------------

type pgEvidenciasRepo struct {
	db       *pgxpool.Pool
	s3Domain string
}

// NewPgEvidenciasRepo creates a PostgreSQL-backed EvidenciasRepo.
// s3Domain is prepended to every photo path (e.g. "https://cdn.example.com").
func NewPgEvidenciasRepo(pool *pgxpool.Pool, s3Domain string) EvidenciasRepo {
	return &pgEvidenciasRepo{db: pool, s3Domain: s3Domain}
}

// List executes a COUNT then a paginated SELECT for evidence records.
// Query is built dynamically with a strings.Builder — only AND clauses are
// appended; user values ALWAYS go into the args slice, never into SQL text.
func (r *pgEvidenciasRepo) List(ctx context.Context, empresaID string, clienteID *string, fecha *string, page, limit int) ([]reports.Evidencia, int, error) {
	args := []any{empresaID}
	argN := 1 // $1 = empresa_id

	var where strings.Builder
	where.WriteString("WHERE a.empresa_id = $1")

	if clienteID != nil {
		argN++
		where.WriteString(fmt.Sprintf(" AND a.cliente_id = $%d", argN))
		args = append(args, *clienteID)
	}
	if fecha != nil {
		argN++
		where.WriteString(fmt.Sprintf(" AND a.fecha_ejecucion::date = $%d", argN))
		args = append(args, *fecha)
	}

	// COUNT query.
	countSQL := "SELECT COUNT(*) FROM asignacion.asignacion_asignacion a " + where.String()
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("evidencias_repository.List count: %w", err)
	}
	if total == 0 {
		return []reports.Evidencia{}, 0, nil
	}

	// Pagination args — s3Domain is a config param, added as positional $N.
	offset := (page - 1) * limit
	argN++
	s3DomainArg := argN
	argN++
	limitArg := argN
	argN++
	offsetArg := argN
	args = append(args, r.s3Domain, limit, offset)

	selectSQL := fmt.Sprintf(`
		SELECT
		    a.id::text AS asignacion_id,
		    act.id::text AS actividad_id,
		    e.nombre || ' ' || e.apellido AS empleado_nombre,
		    act.descripcion AS actividad_descripcion,
		    a.fecha_ejecucion::text,
		    a.latitud,
		    a.longitud,
		    COALESCE(
		        json_agg($%d || f.ruta ORDER BY f.id) FILTER (WHERE f.ruta IS NOT NULL),
		        '[]'::json
		    ) AS evidencias
		FROM asignacion.asignacion_asignacion a
		JOIN asignacion.asignacion_asignaactividad aa ON aa.asignacion_id = a.id
		JOIN asignacion.asignacion_actividad act ON act.id = aa.actividad_id
		LEFT JOIN asignacion.asignacion_foto f ON f.asignacion_id = a.id
		JOIN empleados.empleados_empleado e ON e.id = a.empleado_id
		%s
		GROUP BY a.id, act.id, e.nombre, e.apellido, act.descripcion, a.fecha_ejecucion, a.latitud, a.longitud
		ORDER BY a.fecha_ejecucion DESC
		LIMIT $%d OFFSET $%d`, s3DomainArg, where.String(), limitArg, offsetArg)

	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("evidencias_repository.List query: %w", err)
	}
	defer rows.Close()

	var out []reports.Evidencia
	for rows.Next() {
		var ev reports.Evidencia
		var evidenciasJSON string
		if scanErr := rows.Scan(
			&ev.AsignacionID, &ev.ActividadID, &ev.EmpleadoNombre,
			&ev.ActividadDescripcion, &ev.FechaEjecucion,
			&ev.Latitud, &ev.Longitud, &evidenciasJSON,
		); scanErr != nil {
			return nil, 0, fmt.Errorf("evidencias_repository.List scan: %w", scanErr)
		}
		// JSON array from Postgres is kept as raw string; service layer parses it.
		ev.Evidencias = []string{evidenciasJSON}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("evidencias_repository.List rows: %w", err)
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// InMemEvidenciasRepo — in-memory stub for unit tests
// ---------------------------------------------------------------------------

type inMemEvidenciaEntry struct {
	ev        reports.Evidencia
	empresaID string
	clienteID string
	fecha     string
}

// InMemEvidenciasRepo is a goroutine-safe in-memory EvidenciasRepo for tests.
type InMemEvidenciasRepo struct {
	mu      sync.RWMutex
	entries []inMemEvidenciaEntry
}

// NewInMemEvidenciasRepo creates an empty in-memory EvidenciasRepo.
func NewInMemEvidenciasRepo() *InMemEvidenciasRepo {
	return &InMemEvidenciasRepo{}
}

// Seed adds an Evidencia associated with the given empresaID.
func (r *InMemEvidenciasRepo) Seed(ev reports.Evidencia, empresaID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, inMemEvidenciaEntry{ev: ev, empresaID: empresaID})
	return nil
}

// SeedWithCliente adds an Evidencia with both empresaID and clienteID.
func (r *InMemEvidenciasRepo) SeedWithCliente(ev reports.Evidencia, empresaID, clienteID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, inMemEvidenciaEntry{ev: ev, empresaID: empresaID, clienteID: clienteID})
	return nil
}

// SeedWithFecha adds an Evidencia with empresaID and a specific date string.
func (r *InMemEvidenciasRepo) SeedWithFecha(ev reports.Evidencia, empresaID, fecha string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, inMemEvidenciaEntry{ev: ev, empresaID: empresaID, fecha: fecha})
	return nil
}

func (r *InMemEvidenciasRepo) List(_ context.Context, empresaID string, clienteID *string, fecha *string, page, limit int) ([]reports.Evidencia, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []reports.Evidencia
	for _, e := range r.entries {
		if e.empresaID != empresaID {
			continue
		}
		if clienteID != nil && e.clienteID != *clienteID {
			continue
		}
		if fecha != nil && e.fecha != *fecha {
			continue
		}
		filtered = append(filtered, e.ev)
	}

	total := len(filtered)
	offset := (page - 1) * limit
	if offset >= total {
		return []reports.Evidencia{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}
