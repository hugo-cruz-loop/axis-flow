package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/bolsatrabajo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostulacionRepository defines persistence operations for job applications.
type PostulacionRepository interface {
	Create(ctx context.Context, p *bolsatrabajo.Postulacion) error
	GetStatsByTrabajo(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error)
	UpdateEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error)
}

// PgxPostulacionRepository implements PostulacionRepository using pgx pool.
type PgxPostulacionRepository struct {
	pool *pgxpool.Pool
}

// NewPgxPostulacionRepository constructs a PgxPostulacionRepository.
func NewPgxPostulacionRepository(pool *pgxpool.Pool) *PgxPostulacionRepository {
	return &PgxPostulacionRepository{pool: pool}
}

// Compile-time interface check.
var _ PostulacionRepository = (*PgxPostulacionRepository)(nil)

func (r *PgxPostulacionRepository) Create(ctx context.Context, p *bolsatrabajo.Postulacion) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO bolsa_trabajo.postulaciones
		   (id, trabajo_id, nombre_completo, email, telefono, cv_url, estatus)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING created_at, updated_at`,
		p.ID, p.TrabajoID, p.NombreCompleto, p.Email, p.Telefono, p.CvURL, p.Estatus,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return mapError(err, "PgxPostulacionRepository.Create")
	}
	return nil
}

// stageNames maps estatus constants to human-readable labels.
var stageNames = map[int]string{
	bolsatrabajo.PostulacionPendiente:  "Pendiente",
	bolsatrabajo.PostulacionEntrevista: "Entrevista",
	bolsatrabajo.PostulacionOferta:     "Oferta",
	bolsatrabajo.PostulacionContratado: "Contratado",
	bolsatrabajo.PostulacionRechazado:  "Rechazado",
}

var allStages = []int{
	bolsatrabajo.PostulacionPendiente,
	bolsatrabajo.PostulacionEntrevista,
	bolsatrabajo.PostulacionOferta,
	bolsatrabajo.PostulacionContratado,
	bolsatrabajo.PostulacionRechazado,
}

func (r *PgxPostulacionRepository) GetStatsByTrabajo(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error) {
	// Tenant check: trabajo must belong to empresaID.
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM bolsa_trabajo.trabajos WHERE id=$1 AND empresa_id=$2)`,
		trabajoID, empresaID,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("PgxPostulacionRepository.GetStatsByTrabajo tenant check: %w", err)
	}
	if !exists {
		return nil, bolsatrabajo.ErrForbidden
	}

	rows, err := r.pool.Query(ctx,
		`SELECT p.estatus, COUNT(*) AS cnt
		 FROM bolsa_trabajo.postulaciones p
		 JOIN bolsa_trabajo.trabajos t ON p.trabajo_id = t.id
		 WHERE t.id=$1 AND t.empresa_id=$2
		 GROUP BY p.estatus`,
		trabajoID, empresaID,
	)
	if err != nil {
		return nil, fmt.Errorf("PgxPostulacionRepository.GetStatsByTrabajo: %w", err)
	}
	defer rows.Close()

	counts := make(map[int]int)
	for rows.Next() {
		var estatus, cnt int
		if err := rows.Scan(&estatus, &cnt); err != nil {
			return nil, fmt.Errorf("PgxPostulacionRepository.GetStatsByTrabajo scan: %w", err)
		}
		counts[estatus] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("PgxPostulacionRepository.GetStatsByTrabajo rows: %w", err)
	}

	stats := make([]bolsatrabajo.StageStat, 0, len(allStages))
	for _, stage := range allStages {
		stats = append(stats, bolsatrabajo.StageStat{
			Stage:     stage,
			StageName: stageNames[stage],
			Count:     counts[stage], // 0 if not in map
		})
	}

	return &bolsatrabajo.PipelineStats{TrabajoID: trabajoID, Stats: stats}, nil
}

func (r *PgxPostulacionRepository) UpdateEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error) {
	// Tenant-isolated UPDATE: joins through trabajos to enforce empresa_id ownership.
	var p bolsatrabajo.Postulacion
	err := r.pool.QueryRow(ctx,
		`UPDATE bolsa_trabajo.postulaciones p
		 SET estatus=$3, updated_at=now()
		 FROM bolsa_trabajo.trabajos t
		 WHERE p.id=$1 AND p.trabajo_id=t.id AND t.empresa_id=$2
		 RETURNING p.id, p.trabajo_id, p.nombre_completo, p.email, p.telefono, p.cv_url, p.estatus, p.created_at, p.updated_at`,
		id, empresaID, newEstatus,
	).Scan(&p.ID, &p.TrabajoID, &p.NombreCompleto, &p.Email, &p.Telefono, &p.CvURL,
		&p.Estatus, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Could be not-found or forbidden — return ErrNotFound (callers may check ownership separately).
		return nil, bolsatrabajo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PgxPostulacionRepository.UpdateEstatus: %w", err)
	}
	return &p, nil
}

