// Package service — RespuestaService: field answer capture.
//
// PR-3 (Services) — task 3.3. Gherkin 2 step "el empleado responde el
// checklist": the service persists, publishes FormularioRespondido, and
// invalidates the cached respuestas list for the check-in. The
// FormularioRespondido payload is intentionally minimal (respuesta_id,
// evento_iniciado_id, pregunta_id, empresa_id) — full answer content
// stays in the DB and is fetched by the Reports consumer via
// ListByIniciado; broadcasting every response value would inflate the
// event envelope for no operational gain.
package service

import (
	"context"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/events"

	"github.com/google/uuid"
)

// RespuestaService defines the use-case operations for capturing and
// listing field answers.
type RespuestaService interface {
	// SubmitRespuesta validates the respuesta, persists it, publishes
	// FormularioRespondido, and invalidates the cached respuestas list
	// for the check-in. The geo range check is enforced in the
	// repository (mirrors the SQL CHECK).
	SubmitRespuesta(ctx context.Context, r *formularios.Respuesta, empresaID uuid.UUID) (*formularios.Respuesta, error)

	// GetRespuestasByIniciado returns the respuestas captured for a
	// check-in, ordered by created_at ASC. Pure delegation; IDOR is
	// enforced at the handler layer (the repository has no tenant column
	// on respuestas; see Deviation #2 in apply-progress).
	GetRespuestasByIniciado(ctx context.Context, iniciadoID, empresaID uuid.UUID) ([]*formularios.Respuesta, error)
}

// respuestaService is the concrete implementation.
type respuestaService struct {
	repo  formularios.RespuestaRepository
	pub   events.EventPublisher
	cache CacheInvalidator
}

// NewRespuestaService constructs a RespuestaService.
func NewRespuestaService(
	repo formularios.RespuestaRepository,
	pub events.EventPublisher,
	cache CacheInvalidator,
) RespuestaService {
	return &respuestaService{repo: repo, pub: pub, cache: cache}
}

// ---------------------------------------------------------------------------
// SubmitRespuesta
// ---------------------------------------------------------------------------

func (s *respuestaService) SubmitRespuesta(
	ctx context.Context,
	r *formularios.Respuesta,
	empresaID uuid.UUID,
) (*formularios.Respuesta, error) {
	if err := validateRespuesta(r); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}

	postWriteHook(ctx, s.pub, func(c context.Context) error {
		return s.cache.UnlinkRespuestasByIniciado(c, r.EventoIniciadoID)
	}, events.StreamFormularioRespondido, map[string]any{
		"respuesta_id":       r.ID,
		"evento_iniciado_id": r.EventoIniciadoID,
		"pregunta_id":        r.PreguntaID,
		"empresa_id":         empresaID,
	})
	return r, nil
}

// validateRespuesta enforces the structural invariants the SQL FK
// guarantees at the DB level: evento_iniciado_id and pregunta_id must be
// present. The geo range and the FKs themselves are checked in the
// repository (it mirrors the SQL CHECKs).
func validateRespuesta(r *formularios.Respuesta) error {
	if r == nil {
		return formularios.ErrInvalidInput
	}
	if r.EventoIniciadoID == uuid.Nil {
		return formularios.ErrInvalidInput
	}
	if r.PreguntaID == uuid.Nil {
		return formularios.ErrInvalidInput
	}
	return nil
}

// ---------------------------------------------------------------------------
// GetRespuestasByIniciado
// ---------------------------------------------------------------------------

func (s *respuestaService) GetRespuestasByIniciado(
	ctx context.Context,
	iniciadoID, empresaID uuid.UUID,
) ([]*formularios.Respuesta, error) {
	// empresaID is accepted in the signature for symmetry with the
	// handler contract and for the future IDOR-via-parent-chain work
	// (PR-8). The repository currently has no tenant column, so this
	// method delegates without enforcement. See Deviation #2.
	_ = empresaID
	return s.repo.ListByIniciado(ctx, iniciadoID)
}
