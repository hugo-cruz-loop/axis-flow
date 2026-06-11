package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"axis-flow-back/internal/formularios"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// PgxPreguntaRepository — PostgreSQL implementation of PreguntaRepository.
// ---------------------------------------------------------------------------

// PgxPreguntaRepository is a PostgreSQL-backed PreguntaRepository.
type PgxPreguntaRepository struct {
	db formulariosDB
}

// NewPgxPreguntaRepository creates a PostgreSQL pregunta repository.
func NewPgxPreguntaRepository(pool *pgxpool.Pool) *PgxPreguntaRepository {
	return &PgxPreguntaRepository{db: pool}
}

// Create inserts a new formularios_pregunta row. The SQL CHECK on tipo_pregunta
// enforces the closed set {1, 2, 3, 5, 8, 11} at the DB level — we do not
// duplicate that check here.
func (r *PgxPreguntaRepository) Create(ctx context.Context, p *formularios.Pregunta) error {
	const q = `
		INSERT INTO formularios.formularios_pregunta
		    (id, formulario_id, orden, tipo_pregunta, texto_pregunta,
		     obligatoria, respuesta_predefinida, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`
	var rawResp any
	if len(p.RespuestaPredefinida) > 0 {
		rawResp = []byte(p.RespuestaPredefinida)
	}
	_, err := r.db.Exec(ctx, q,
		p.ID, p.FormularioID, p.Orden, p.TipoPregunta, p.TextoPregunta,
		p.Obligatoria, rawResp,
	)
	if mapped := mapPgError(err); mapped != nil {
		if errors.Is(mapped, formularios.ErrInvalidInput) || errors.Is(mapped, formularios.ErrConflict) || errors.Is(mapped, formularios.ErrNotFound) {
			return mapped
		}
		return fmt.Errorf("pregunta_repository.Create: %w", err)
	}
	return nil
}

// ListByFormulario returns preguntas for a given formulario, ordered by
// `orden` ASC. Returns an empty slice for unknown formularios. IDOR is
// enforced at the service layer (the parent formulario must be fetched
// with its empresaID first; this matches the 09 pattern where
// `ticketRepo.GetTicketsByCliente` scopes by both empresa AND cliente but
// `quejaRepo.CreateRespuesta` does not).
func (r *PgxPreguntaRepository) ListByFormulario(ctx context.Context, formularioID uuid.UUID) ([]*formularios.Pregunta, error) {
	const q = `
		SELECT id, formulario_id, orden, tipo_pregunta, texto_pregunta,
		       obligatoria, COALESCE(respuesta_predefinida, '{}'::jsonb), created_at, updated_at
		FROM formularios.formularios_pregunta
		WHERE formulario_id = $1
		ORDER BY orden ASC`
	rows, err := r.db.Query(ctx, q, formularioID)
	if err != nil {
		return nil, fmt.Errorf("pregunta_repository.ListByFormulario query: %w", err)
	}
	defer rows.Close()

	var out []*formularios.Pregunta
	for rows.Next() {
		p, scanErr := scanPregunta(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("pregunta_repository.ListByFormulario scan: %w", scanErr)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pregunta_repository.ListByFormulario rows: %w", err)
	}
	if out == nil {
		return []*formularios.Pregunta{}, nil
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// scanners
// ---------------------------------------------------------------------------

type preguntaRowScanner interface {
	Scan(dest ...any) error
}

func scanPregunta(row preguntaRowScanner) (*formularios.Pregunta, error) {
	p := &formularios.Pregunta{}
	var rawResp []byte
	err := row.Scan(
		&p.ID, &p.FormularioID, &p.Orden, &p.TipoPregunta, &p.TextoPregunta,
		&p.Obligatoria, &rawResp, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, formularios.ErrNotFound
		}
		return nil, err
	}
	if len(rawResp) > 0 {
		p.RespuestaPredefinida = append(p.RespuestaPredefinida, rawResp...)
	}
	return p, nil
}

// ---------------------------------------------------------------------------
// InMemPreguntaRepository — goroutine-safe, in-memory adapter.
//
// PR-2 (Repositories) — task 2.2.
//
// Mirrors the SQL CHECK constraint on formularios.formularios_pregunta.tipo_pregunta
// (allowed: 1, 2, 3, 5, 8, 11) so the constraint is testable without a live DB.
// The pgx adapter (above) relies on the same CHECK at the DB level.
// ---------------------------------------------------------------------------

// InMemPreguntaRepository is a goroutine-safe, in-memory PreguntaRepository
// for unit tests.
type InMemPreguntaRepository struct {
	mu        sync.RWMutex
	preguntas map[uuid.UUID]*formularios.Pregunta
}

// NewInMemPreguntaRepository creates an empty in-memory pregunta repository.
func NewInMemPreguntaRepository() *InMemPreguntaRepository {
	return &InMemPreguntaRepository{preguntas: make(map[uuid.UUID]*formularios.Pregunta)}
}

// validTipoPregunta mirrors the SQL CHECK constraint
// chk_formularios_pregunta_tipo CHECK (tipo_pregunta IN (1, 2, 3, 5, 8, 11)).
func validTipoPregunta(t int) bool {
	switch t {
	case formularios.TipoPreguntaTexto, formularios.TipoPreguntaCheckbox, formularios.TipoPreguntaRating,
		formularios.TipoPreguntaMatriz, formularios.TipoPreguntaFoto, formularios.TipoPreguntaFirma:
		return true
	}
	return false
}

// Create inserts a new Pregunta. If ID is uuid.Nil, a fresh ID is generated.
// Returns formularios.ErrInvalidInput if tipo_pregunta is outside the allowed set.
func (r *InMemPreguntaRepository) Create(_ context.Context, p *formularios.Pregunta) error {
	if !validTipoPregunta(p.TipoPregunta) {
		return formularios.ErrInvalidInput
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	cp := *p
	r.preguntas[p.ID] = &cp
	return nil
}

// ListByFormulario returns the preguntas for a given formulario, ordered by
// `orden` ASC. Returns an empty slice for unknown formularios.
func (r *InMemPreguntaRepository) ListByFormulario(_ context.Context, formularioID uuid.UUID) ([]*formularios.Pregunta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*formularios.Pregunta
	for _, p := range r.preguntas {
		if p.FormularioID != formularioID {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Orden < out[j].Orden
	})
	if out == nil {
		return []*formularios.Pregunta{}, nil
	}
	return out, nil
}
