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
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"axis-flow-back/internal/formularios"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Common DB / cache interface shared by all Pgx adapters in this package.
//
// Mirrors 09's `quejaDB` interface in internal/atencionseguimiento/repository/.
// We use a minimal subset (Exec / Query / QueryRow) so tests can substitute a
// fake without dragging in a real pgxpool.Pool.
// ---------------------------------------------------------------------------

type formulariosDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// nullStr returns nil for empty strings so we can map Go zero values to SQL
// NULL consistently. Mirrors 09's helper in
// internal/atencionseguimiento/repository/queja_repository.go.
func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// mapPgError translates pgconn.PgError codes into formularios sentinel
// errors. CHECK (23514) → ErrInvalidInput; UNIQUE (23505) → ErrConflict;
// FK (23503) → ErrNotFound. Any other error is wrapped with a generic
// message and returned as-is for the caller to log.
func mapPgError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23514": // check_violation
			return formularios.ErrInvalidInput
		case "23505": // unique_violation
			return formularios.ErrConflict
		case "23503": // foreign_key_violation
			return formularios.ErrNotFound
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return formularios.ErrNotFound
	}
	return err
}

// ---------------------------------------------------------------------------
// PgxFormularioRepository — PostgreSQL implementation of FormularioRepository.
// ---------------------------------------------------------------------------

// PgxFormularioRepository is a PostgreSQL-backed FormularioRepository.
type PgxFormularioRepository struct {
	db    formulariosDB
	cache *RedisFormulariosCacheInvalidator
}

// NewPgxFormularioRepository creates a PostgreSQL formulario repository.
// Pass a nil redis.Client to disable cache invalidation.
func NewPgxFormularioRepository(pool *pgxpool.Pool, redisClient redis.Cmdable) *PgxFormularioRepository {
	var cache *RedisFormulariosCacheInvalidator
	if redisClient != nil {
		cache = NewRedisFormulariosCacheInvalidator(redisClient)
	}
	return &PgxFormularioRepository{db: pool, cache: cache}
}

// Create inserts a new formularios_formulario row.
func (r *PgxFormularioRepository) Create(ctx context.Context, f *formularios.Formulario) error {
	const q = `
		INSERT INTO formularios.formularios_formulario
		    (id, empresa_id, nombre, descripcion, activo, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())`
	_, err := r.db.Exec(ctx, q,
		f.ID, f.EmpresaID, f.Nombre, nullStr(f.Descripcion), f.Activo,
	)
	if mapped := mapPgError(err); mapped != nil {
		if errors.Is(mapped, formularios.ErrInvalidInput) || errors.Is(mapped, formularios.ErrConflict) {
			return mapped
		}
		return fmt.Errorf("formulario_repository.Create: %w", err)
	}
	return nil
}

// GetByID fetches a formularios_formulario row by primary key, scoped by
// empresa_id for IDOR. Returns formularios.ErrNotFound on miss or wrong tenant.
func (r *PgxFormularioRepository) GetByID(ctx context.Context, id, empresaID uuid.UUID) (*formularios.Formulario, error) {
	const q = `
		SELECT id, empresa_id, nombre, COALESCE(descripcion,''), activo, created_at, updated_at
		FROM formularios.formularios_formulario
		WHERE id = $1 AND empresa_id = $2`
	f, err := scanFormulario(r.db.QueryRow(ctx, q, id, empresaID))
	if err != nil {
		if errors.Is(err, formularios.ErrNotFound) {
			return nil, formularios.ErrNotFound
		}
		return nil, fmt.Errorf("formulario_repository.GetByID: %w", err)
	}
	return f, nil
}

// ListByEmpresa returns paginated formularios for an empresa, optionally
// filtered by activo. Ordered by created_at DESC.
func (r *PgxFormularioRepository) ListByEmpresa(ctx context.Context, empresaID uuid.UUID, activo *bool, page, pageSize int) ([]*formularios.Formulario, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	baseQ := `FROM formularios.formularios_formulario WHERE empresa_id = $1`
	args := []any{empresaID}
	if activo != nil {
		baseQ += fmt.Sprintf(" AND activo = $%d", len(args)+1)
		args = append(args, *activo)
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) "+baseQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("formulario_repository.ListByEmpresa count: %w", err)
	}

	selectQ := "SELECT id, empresa_id, nombre, COALESCE(descripcion,''), activo, created_at, updated_at " +
		baseQ + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("formulario_repository.ListByEmpresa query: %w", err)
	}
	defer rows.Close()

	var out []*formularios.Formulario
	for rows.Next() {
		f, scanErr := scanFormulario(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("formulario_repository.ListByEmpresa scan: %w", scanErr)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("formulario_repository.ListByEmpresa rows: %w", err)
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// scanners
// ---------------------------------------------------------------------------

type formularioRowScanner interface {
	Scan(dest ...any) error
}

func scanFormulario(row formularioRowScanner) (*formularios.Formulario, error) {
	f := &formularios.Formulario{}
	err := row.Scan(
		&f.ID, &f.EmpresaID, &f.Nombre, &f.Descripcion, &f.Activo, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, formularios.ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

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
