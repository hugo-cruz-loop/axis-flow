package repository

import (
	"context"
	"fmt"
	"sync"

	"axis-flow-back/internal/reports"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// StatsRepo interface
// ---------------------------------------------------------------------------

// StatsRepo provides aggregated statistical queries for the Reports dashboard.
type StatsRepo interface {
	// GraficaEvidencia returns weekly evidence compliance aggregates for empresaID,
	// optionally scoped to a specific clienteID.
	GraficaEvidencia(ctx context.Context, empresaID string, clienteID *string) ([]reports.GraficaEvidenciaStat, error)
	// CountIncidentes returns incident counts grouped by status from
	// atencion_seguimiento.solicitudes_queja.
	CountIncidentes(ctx context.Context, empresaID string) ([]reports.IncidenteCount, error)
}

// ---------------------------------------------------------------------------
// pgStatsRepo — PostgreSQL implementation
// ---------------------------------------------------------------------------

type pgStatsRepo struct {
	db *pgxpool.Pool
}

// NewPgStatsRepo creates a PostgreSQL-backed StatsRepo.
func NewPgStatsRepo(pool *pgxpool.Pool) StatsRepo {
	return &pgStatsRepo{db: pool}
}

func (r *pgStatsRepo) GraficaEvidencia(ctx context.Context, empresaID string, clienteID *string) ([]reports.GraficaEvidenciaStat, error) {
	args := []any{empresaID}
	argN := 1

	extraWhere := ""
	if clienteID != nil {
		argN++
		extraWhere = fmt.Sprintf(" AND a.cliente_id = $%d", argN)
		args = append(args, *clienteID)
	}

	// Aggregate by ISO week; "compliant" = rows where at least one photo exists.
	selectSQL := fmt.Sprintf(`
		SELECT
		    to_char(date_trunc('week', a.fecha_ejecucion), 'IYYY-"W"IW') AS week,
		    COUNT(DISTINCT a.id)::int AS total,
		    COUNT(DISTINCT a.id) FILTER (WHERE f.id IS NOT NULL)::int AS compliant
		FROM asignacion.asignacion_asignacion a
		LEFT JOIN asignacion.asignacion_foto f ON f.asignacion_id = a.id
		WHERE a.empresa_id = $1 %s
		GROUP BY date_trunc('week', a.fecha_ejecucion)
		ORDER BY date_trunc('week', a.fecha_ejecucion)`, extraWhere)

	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("stats_repository.GraficaEvidencia query: %w", err)
	}
	defer rows.Close()

	var out []reports.GraficaEvidenciaStat
	for rows.Next() {
		var s reports.GraficaEvidenciaStat
		if scanErr := rows.Scan(&s.Week, &s.Total, &s.Compliant); scanErr != nil {
			return nil, fmt.Errorf("stats_repository.GraficaEvidencia scan: %w", scanErr)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stats_repository.GraficaEvidencia rows: %w", err)
	}
	return out, nil
}

func (r *pgStatsRepo) CountIncidentes(ctx context.Context, empresaID string) ([]reports.IncidenteCount, error) {
	const q = `
		SELECT
		    CASE estatus
		        WHEN 1 THEN 'abierto'
		        WHEN 2 THEN 'en_proceso'
		        WHEN 3 THEN 'cerrado'
		        ELSE 'desconocido'
		    END AS status,
		    COUNT(*)::int AS count
		FROM atencion_seguimiento.solicitudes_queja
		WHERE empresa_id = $1
		GROUP BY estatus
		ORDER BY estatus`

	rows, err := r.db.Query(ctx, q, empresaID)
	if err != nil {
		return nil, fmt.Errorf("stats_repository.CountIncidentes query: %w", err)
	}
	defer rows.Close()

	var out []reports.IncidenteCount
	for rows.Next() {
		var c reports.IncidenteCount
		if scanErr := rows.Scan(&c.Status, &c.Count); scanErr != nil {
			return nil, fmt.Errorf("stats_repository.CountIncidentes scan: %w", scanErr)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stats_repository.CountIncidentes rows: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// InMemStatsRepo — in-memory stub for unit tests
// ---------------------------------------------------------------------------

type statsGraficaKey struct {
	empresaID string
	clienteID string // "" means no client filter
}

// InMemStatsRepo is a goroutine-safe in-memory StatsRepo for tests.
type InMemStatsRepo struct {
	mu             sync.RWMutex
	graficaData    map[statsGraficaKey][]reports.GraficaEvidenciaStat
	incidenteData  map[string][]reports.IncidenteCount // keyed by empresaID
}

// NewInMemStatsRepo creates an empty in-memory StatsRepo.
func NewInMemStatsRepo() *InMemStatsRepo {
	return &InMemStatsRepo{
		graficaData:   make(map[statsGraficaKey][]reports.GraficaEvidenciaStat),
		incidenteData: make(map[string][]reports.IncidenteCount),
	}
}

// SeedGraficaEvidencia registers stats for empresaID with no client filter.
func (r *InMemStatsRepo) SeedGraficaEvidencia(empresaID string, stats []reports.GraficaEvidenciaStat) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.graficaData[statsGraficaKey{empresaID: empresaID}] = stats
}

// SeedGraficaEvidenciaWithCliente registers stats for empresaID + clienteID.
func (r *InMemStatsRepo) SeedGraficaEvidenciaWithCliente(empresaID, clienteID string, stats []reports.GraficaEvidenciaStat) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.graficaData[statsGraficaKey{empresaID: empresaID, clienteID: clienteID}] = stats
}

// SeedIncidenteCounts registers incident counts for empresaID.
func (r *InMemStatsRepo) SeedIncidenteCounts(empresaID string, counts []reports.IncidenteCount) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.incidenteData[empresaID] = counts
}

func (r *InMemStatsRepo) GraficaEvidencia(_ context.Context, empresaID string, clienteID *string) ([]reports.GraficaEvidenciaStat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := statsGraficaKey{empresaID: empresaID}
	if clienteID != nil {
		key.clienteID = *clienteID
	}
	data, ok := r.graficaData[key]
	if !ok {
		return []reports.GraficaEvidenciaStat{}, nil
	}
	out := make([]reports.GraficaEvidenciaStat, len(data))
	copy(out, data)
	return out, nil
}

func (r *InMemStatsRepo) CountIncidentes(_ context.Context, empresaID string) ([]reports.IncidenteCount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, ok := r.incidenteData[empresaID]
	if !ok {
		return []reports.IncidenteCount{}, nil
	}
	out := make([]reports.IncidenteCount, len(data))
	copy(out, data)
	return out, nil
}
