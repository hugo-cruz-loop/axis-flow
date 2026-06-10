package formularios

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// InMemPreguntaRepository — goroutine-safe, in-memory adapter.
//
// PR-2 (Repositories) — task 2.2.
//
// Mirrors the SQL CHECK constraint on formularios.formularios_pregunta.tipo_pregunta
// (allowed: 1, 2, 3, 5, 8, 11) so the constraint is testable without a live DB.
// The pgx adapter will rely on the same CHECK at the DB level.
// ---------------------------------------------------------------------------

// InMemPreguntaRepository is a goroutine-safe, in-memory PreguntaRepository
// for unit tests.
type InMemPreguntaRepository struct {
	mu        sync.RWMutex
	preguntas map[uuid.UUID]*Pregunta
}

// NewInMemPreguntaRepository creates an empty in-memory pregunta repository.
func NewInMemPreguntaRepository() *InMemPreguntaRepository {
	return &InMemPreguntaRepository{preguntas: make(map[uuid.UUID]*Pregunta)}
}

// validTipoPregunta mirrors the SQL CHECK constraint
// chk_formularios_pregunta_tipo CHECK (tipo_pregunta IN (1, 2, 3, 5, 8, 11)).
func validTipoPregunta(t int) bool {
	switch t {
	case TipoPreguntaTexto, TipoPreguntaCheckbox, TipoPreguntaRating,
		TipoPreguntaMatriz, TipoPreguntaFoto, TipoPreguntaFirma:
		return true
	}
	return false
}

// Create inserts a new Pregunta. If ID is uuid.Nil, a fresh ID is generated.
// Returns ErrInvalidInput if tipo_pregunta is outside the allowed set.
func (r *InMemPreguntaRepository) Create(_ context.Context, p *Pregunta) error {
	if !validTipoPregunta(p.TipoPregunta) {
		return ErrInvalidInput
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
func (r *InMemPreguntaRepository) ListByFormulario(_ context.Context, formularioID uuid.UUID) ([]*Pregunta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*Pregunta
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
		return []*Pregunta{}, nil
	}
	return out, nil
}
