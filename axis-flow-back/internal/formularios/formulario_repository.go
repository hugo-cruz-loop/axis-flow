package formularios

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// InMemFormularioRepository — goroutine-safe, in-memory adapter.
//
// PR-2 (Repositories) — task 2.1. Mirrors the InMem* pattern from
// internal/atencionseguimiento/repository/incidencia_repository.go so the pgx
// adapter can be swapped in later without changing service code.
// ---------------------------------------------------------------------------

// InMemFormularioRepository is a goroutine-safe, in-memory FormularioRepository
// for unit tests. It enforces the same IDOR rules as the pgx adapter: any
// read that does not match the calling empresa_id returns ErrNotFound (never
// ErrForbidden) to avoid leaking row existence.
type InMemFormularioRepository struct {
	mu          sync.RWMutex
	formularios map[uuid.UUID]*Formulario
}

// NewInMemFormularioRepository creates an empty in-memory formulario repository.
func NewInMemFormularioRepository() *InMemFormularioRepository {
	return &InMemFormularioRepository{formularios: make(map[uuid.UUID]*Formulario)}
}

// Create inserts a new Formulario. If ID is uuid.Nil, a fresh ID is generated.
// CreatedAt / UpdatedAt are stamped if zero.
func (r *InMemFormularioRepository) Create(_ context.Context, f *Formulario) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	now := time.Now().UTC()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	if f.UpdatedAt.IsZero() {
		f.UpdatedAt = now
	}
	cp := *f
	r.formularios[f.ID] = &cp
	return nil
}

// GetByID returns the Formulario if it exists AND belongs to empresaID.
// Any mismatch (missing row or wrong tenant) returns ErrNotFound to avoid
// leaking the existence of a foreign-empresa row.
func (r *InMemFormularioRepository) GetByID(_ context.Context, id, empresaID uuid.UUID) (*Formulario, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.formularios[id]
	if !ok || f.EmpresaID != empresaID {
		return nil, ErrNotFound
	}
	cp := *f
	return &cp, nil
}

// ListByEmpresa returns paginated formularios for an empresa, optionally
// filtered by activo. Ordered by created_at DESC.
func (r *InMemFormularioRepository) ListByEmpresa(_ context.Context, empresaID uuid.UUID, activo *bool, page, pageSize int) ([]*Formulario, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*Formulario
	for _, f := range r.formularios {
		if f.EmpresaID != empresaID {
			continue
		}
		if activo != nil && f.Activo != *activo {
			continue
		}
		cp := *f
		filtered = append(filtered, &cp)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	total := len(filtered)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	if offset >= total {
		return []*Formulario{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}
