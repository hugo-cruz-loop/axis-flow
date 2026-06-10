package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"axis-flow-back/internal/atencionseguimiento"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// IncidenciaRepository — interface.
// ---------------------------------------------------------------------------

// IncidenciaRepository defines persistence operations for incidencia supervisor entities.
type IncidenciaRepository interface {
	Create(ctx context.Context, i *atencionseguimiento.IncidenciaSupervisor) error
	ListByEmpresa(ctx context.Context, empresaID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error)
}

// ---------------------------------------------------------------------------
// PgxIncidenciaRepository — PostgreSQL implementation.
// ---------------------------------------------------------------------------

// PgxIncidenciaRepository is a PostgreSQL-backed implementation of IncidenciaRepository.
type PgxIncidenciaRepository struct {
	db quejaDB
}

// NewPgxIncidenciaRepository creates a PostgreSQL incidencia repository.
func NewPgxIncidenciaRepository(pool *pgxpool.Pool) *PgxIncidenciaRepository {
	return &PgxIncidenciaRepository{db: pool}
}

// Create inserts a new IncidenciaSupervisor row.
func (r *PgxIncidenciaRepository) Create(ctx context.Context, i *atencionseguimiento.IncidenciaSupervisor) error {
	const q = `
		INSERT INTO atencion_seguimiento.incidencias_supervisor
		    (id, empresa_id, supervisor_id, empleado_id, localidad_id,
		     tipo_incidencia_id, descripcion, sancion_sugerida, evidencia_url, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())`
	_, err := r.db.Exec(ctx, q,
		i.ID, i.EmpresaID, i.SupervisorID, i.EmpleadoID, i.LocalidadID,
		i.TipoIncidenciaID, i.Descripcion,
		nullStr(i.SancionSugerida), nullStr(i.EvidenciaURL),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return atencionseguimiento.ErrConflict
		}
		return fmt.Errorf("incidencia_repository.Create: %w", err)
	}
	return nil
}

// ListByEmpresa returns paginated incidencias for an empresa, ordered by created_at DESC.
func (r *PgxIncidenciaRepository) ListByEmpresa(ctx context.Context, empresaID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error) {
	offset := (page - 1) * pageSize
	const countQ = `SELECT COUNT(*) FROM atencion_seguimiento.incidencias_supervisor WHERE empresa_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQ, empresaID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("incidencia_repository.ListByEmpresa count: %w", err)
	}
	const q = `
		SELECT id, empresa_id, supervisor_id, empleado_id, localidad_id,
		       tipo_incidencia_id, descripcion,
		       COALESCE(sancion_sugerida,''), COALESCE(evidencia_url,''),
		       created_at, updated_at
		FROM atencion_seguimiento.incidencias_supervisor
		WHERE empresa_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, empresaID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("incidencia_repository.ListByEmpresa query: %w", err)
	}
	defer rows.Close()
	var out []*atencionseguimiento.IncidenciaSupervisor
	for rows.Next() {
		i, scanErr := scanIncidencia(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("incidencia_repository.ListByEmpresa scan: %w", scanErr)
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("incidencia_repository.ListByEmpresa rows: %w", err)
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// scanner
// ---------------------------------------------------------------------------

type incidenciaRowScanner interface {
	Scan(dest ...any) error
}

func scanIncidencia(row incidenciaRowScanner) (*atencionseguimiento.IncidenciaSupervisor, error) {
	i := &atencionseguimiento.IncidenciaSupervisor{}
	err := row.Scan(
		&i.ID, &i.EmpresaID, &i.SupervisorID, &i.EmpleadoID, &i.LocalidadID,
		&i.TipoIncidenciaID, &i.Descripcion,
		&i.SancionSugerida, &i.EvidenciaURL,
		&i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, atencionseguimiento.ErrNotFound
		}
		return nil, err
	}
	return i, nil
}

// ---------------------------------------------------------------------------
// InMemIncidenciaRepository — in-memory implementation for unit tests.
// ---------------------------------------------------------------------------

// InMemIncidenciaRepository is a goroutine-safe, in-memory IncidenciaRepository for unit tests.
type InMemIncidenciaRepository struct {
	mu         sync.RWMutex
	incidencias []*atencionseguimiento.IncidenciaSupervisor
}

// NewInMemIncidenciaRepository creates an empty in-memory incidencia repository.
func NewInMemIncidenciaRepository() *InMemIncidenciaRepository {
	return &InMemIncidenciaRepository{}
}

func (r *InMemIncidenciaRepository) Create(_ context.Context, i *atencionseguimiento.IncidenciaSupervisor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	cp := *i
	r.incidencias = append(r.incidencias, &cp)
	return nil
}

func (r *InMemIncidenciaRepository) ListByEmpresa(_ context.Context, empresaID uuid.UUID, page, pageSize int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*atencionseguimiento.IncidenciaSupervisor
	for _, i := range r.incidencias {
		if i.EmpresaID == empresaID {
			cp := *i
			filtered = append(filtered, &cp)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	total := len(filtered)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []*atencionseguimiento.IncidenciaSupervisor{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}
