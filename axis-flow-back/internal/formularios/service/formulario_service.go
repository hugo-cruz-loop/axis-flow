// Package service — FormularioService: header CRUD + pregunta add.
//
// PR-3 (Services) — task 3.1. Covers Gherkin 1 (Creación de plantilla de
// formulario por un Administrador): a Formulario header is validated,
// persisted, the FormularioCreado event is published, and the cached list
// for that empresa is invalidated. AddPregunta enforces IDOR by looking up
// the parent formulario under the caller's tenant.
package service

import (
	"context"
	"strings"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/events"

	"github.com/google/uuid"
)

// FormularioService defines the use-case operations exposed to the HTTP
// handler layer for Formulario headers and their child Pregunta rows.
type FormularioService interface {
	// CreateFormulario validates the form, persists it, publishes the
	// FormularioCreado event, and invalidates the cached formularios
	// list for the tenant. Returns the persisted entity (with a fresh
	// ID if the caller did not supply one) or a formularios sentinel
	// error.
	CreateFormulario(ctx context.Context, f *formularios.Formulario, empresaID uuid.UUID) (*formularios.Formulario, error)

	// GetFormulariosByEmpresa returns the paginated list of formularios
	// for an empresa, optionally filtered by activo. Pure delegation to
	// the repository — the cache is not invalidated on reads.
	GetFormulariosByEmpresa(ctx context.Context, empresaID uuid.UUID, activo *bool, page, pageSize int) ([]*formularios.Formulario, int, error)

	// AddPregunta appends a question to an existing formulario. IDOR is
	// enforced by verifying the parent formulario belongs to the caller's
	// empresa. Does not publish an event (the spec publishes only on the
	// header creation) and does not invalidate any cached list (no list
	// is keyed by pregunta in PR-1's domain).
	AddPregunta(ctx context.Context, p *formularios.Pregunta, empresaID uuid.UUID) (*formularios.Pregunta, error)
}

// formularioService is the concrete implementation. It is intentionally
// unexported; callers obtain an instance via NewFormularioService.
type formularioService struct {
	formRepo   formularios.FormularioRepository
	pregRepo   formularios.PreguntaRepository
	pub        events.EventPublisher
	cache      CacheInvalidator
}

// NewFormularioService constructs a FormularioService.
//
// All four dependencies are required; pass a NoopPublisher and a no-op
// CacheInvalidator if your environment does not publish events or cache
// list entries (the service still persists the form).
func NewFormularioService(
	formRepo formularios.FormularioRepository,
	pregRepo formularios.PreguntaRepository,
	pub events.EventPublisher,
	cache CacheInvalidator,
) FormularioService {
	return &formularioService{
		formRepo: formRepo,
		pregRepo: pregRepo,
		pub:      pub,
		cache:    cache,
	}
}

// ---------------------------------------------------------------------------
// CreateFormulario
// ---------------------------------------------------------------------------

// maxFormularioNombre mirrors the openapi schema maxLength on /formulario
// POST body nombre. Keeping the cap in one constant lets a future spec
// bump update both the openapi and the service in lockstep.
const maxFormularioNombre = 150

func (s *formularioService) CreateFormulario(
	ctx context.Context,
	f *formularios.Formulario,
	empresaID uuid.UUID,
) (*formularios.Formulario, error) {
	// Validate input. The error messages are intentionally generic — the
	// openapi contract does not require the offending value in the
	// response, and echoing the input would leak user-supplied data
	// (PII) into error envelopes.
	if err := validateFormulario(f); err != nil {
		return nil, err
	}
	// Tenant check: body EmpresaID must match the caller's JWT-empresaID.
	if f.EmpresaID != empresaID {
		return nil, formularios.ErrForbidden
	}

	if err := s.formRepo.Create(ctx, f); err != nil {
		return nil, err
	}

	// Fire-and-forget post-write: publish + cache invalidation. The
	// helper swallows publisher and cache errors; the spec's outbox
	// pattern (PR-5) is the source of truth for delivery.
	postWriteHook(ctx, s.pub, func(c context.Context) error {
		return s.cache.UnlinkFormulariosByEmpresa(c, empresaID)
	}, events.StreamFormularioCreado, map[string]any{
		"formulario_id": f.ID,
		"empresa_id":    empresaID,
		"nombre":        f.Nombre,
	})

	return f, nil
}

// validateFormulario returns ErrInvalidInput if the form is structurally
// invalid. The error message is intentionally generic to avoid PII leaks.
func validateFormulario(f *formularios.Formulario) error {
	if f == nil {
		return formularios.ErrInvalidInput
	}
	if strings.TrimSpace(f.Nombre) == "" {
		return formularios.ErrInvalidInput
	}
	if len(f.Nombre) > maxFormularioNombre {
		return formularios.ErrInvalidInput
	}
	return nil
}

// ---------------------------------------------------------------------------
// GetFormulariosByEmpresa
// ---------------------------------------------------------------------------

func (s *formularioService) GetFormulariosByEmpresa(
	ctx context.Context,
	empresaID uuid.UUID,
	activo *bool,
	page, pageSize int,
) ([]*formularios.Formulario, int, error) {
	return s.formRepo.ListByEmpresa(ctx, empresaID, activo, page, pageSize)
}

// ---------------------------------------------------------------------------
// AddPregunta
// ---------------------------------------------------------------------------

func (s *formularioService) AddPregunta(
	ctx context.Context,
	p *formularios.Pregunta,
	empresaID uuid.UUID,
) (*formularios.Pregunta, error) {
	if err := validatePregunta(p); err != nil {
		return nil, err
	}
	// IDOR: confirm the parent formulario belongs to the caller's
	// empresa. The InMem/Pgx FormularioRepository.GetByID returns
	// formularios.ErrNotFound for foreign-tenant rows so the call does
	// not leak the existence of a foreign-empresa row.
	if _, err := s.formRepo.GetByID(ctx, p.FormularioID, empresaID); err != nil {
		return nil, err
	}

	if err := s.pregRepo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// validatePregunta returns ErrInvalidInput for missing texto_pregunta or
// non-positive orden. The SQL CHECK on tipo_pregunta is enforced in the
// repository (it mirrors the DB-level constraint).
func validatePregunta(p *formularios.Pregunta) error {
	if p == nil {
		return formularios.ErrInvalidInput
	}
	if p.Orden < 1 {
		return formularios.ErrInvalidInput
	}
	if strings.TrimSpace(p.TextoPregunta) == "" {
		return formularios.ErrInvalidInput
	}
	return nil
}
