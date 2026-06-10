// Package repository provides pgx-backed and in-memory implementations of the
// formularios repository interfaces, plus a Redis cache invalidator.
//
// Mirrors the layout of internal/atencionseguimiento/repository/ (PR-2 of
// 09_AtencionSeguimiento_Service_Spec): the same file holds both a
// PgxXxxRepository (production) and an InMemXxxRepository (unit tests),
// satisfying the formularios port interfaces declared in
// internal/formularios/domain.go.
package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"axis-flow-back/internal/formularios"

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
// read that does not match the calling empresa_id returns formularios.ErrNotFound
// (never formularios.ErrForbidden) to avoid leaking row existence.
type InMemFormularioRepository struct {
	mu          sync.RWMutex
	formularios map[uuid.UUID]*formularios.Formulario
}

// NewInMemFormularioRepository creates an empty in-memory formulario repository.
func NewInMemFormularioRepository() *InMemFormularioRepository {
	return &InMemFormularioRepository{formularios: make(map[uuid.UUID]*formularios.Formulario)}
}

// Create inserts a new Formulario. If ID is uuid.Nil, a fresh ID is generated.
// CreatedAt / UpdatedAt are stamped if zero.
func (r *InMemFormularioRepository) Create(_ context.Context, f *formularios.Formulario) error {
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
// Any mismatch (missing row or wrong tenant) returns formularios.ErrNotFound to
// avoid leaking the existence of a foreign-empresa row.
func (r *InMemFormularioRepository) GetByID(_ context.Context, id, empresaID uuid.UUID) (*formularios.Formulario, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.formularios[id]
	if !ok || f.EmpresaID != empresaID {
		return nil, formularios.ErrNotFound
	}
	cp := *f
	return &cp, nil
}

// ListByEmpresa returns paginated formularios for an empresa, optionally
// filtered by activo. Ordered by created_at DESC.
func (r *InMemFormularioRepository) ListByEmpresa(_ context.Context, empresaID uuid.UUID, activo *bool, page, pageSize int) ([]*formularios.Formulario, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*formularios.Formulario
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
		return []*formularios.Formulario{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}
