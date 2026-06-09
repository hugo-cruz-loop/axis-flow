package service

import (
	"context"
	"fmt"
	"io"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/events"
	"axis-flow-back/internal/bolsatrabajo/repository"
	"axis-flow-back/internal/bolsatrabajo/storage"
	"axis-flow-back/internal/bolsatrabajo/turnstile"

	"github.com/google/uuid"
)

// PostulacionService defines business operations for job applications.
type PostulacionService interface {
	Apply(ctx context.Context, p *bolsatrabajo.Postulacion, cvReader io.Reader, cvSize int64, turnstileToken, remoteIP string) (*bolsatrabajo.Postulacion, error)
	GetStatsByTrabajo(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error)
	UpdateEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error)
}

// postulacionService is the production implementation of PostulacionService.
type postulacionService struct {
	repo      repository.PostulacionRepository
	turnstile turnstile.TurnstileClient
	storage   storage.BolsaTrabajoStorage
	pub       events.EventPublisher
}

// NewPostulacionService constructs a postulacionService.
func NewPostulacionService(
	repo repository.PostulacionRepository,
	ts turnstile.TurnstileClient,
	st storage.BolsaTrabajoStorage,
	pub events.EventPublisher,
) PostulacionService {
	return &postulacionService{repo: repo, turnstile: ts, storage: st, pub: pub}
}

// Compile-time interface check.
var _ PostulacionService = (*postulacionService)(nil)

// Apply processes a job application:
// 1. Verify Turnstile captcha token.
// 2. Validate CV magic bytes (PDF or DOCX, max 5 MB).
// 3. Generate a UUID key and upload the CV.
// 4. Persist the postulacion.
// 5. Publish PostulacionRecibida event.
func (s *postulacionService) Apply(
	ctx context.Context,
	p *bolsatrabajo.Postulacion,
	cvReader io.Reader,
	cvSize int64,
	turnstileToken, remoteIP string,
) (*bolsatrabajo.Postulacion, error) {
	// Step 1 — captcha verification.
	if err := s.turnstile.Verify(ctx, turnstileToken, remoteIP); err != nil {
		return nil, err
	}

	// Step 2 — MIME validation.
	if err := storage.ValidateMagicBytes(cvReader, cvSize); err != nil {
		return nil, err
	}

	// Step 3 — upload CV.
	key := uuid.New().String() + ".cv"
	url, err := s.storage.UploadCV(ctx, key, cvReader, cvSize, "application/octet-stream")
	if err != nil {
		return nil, fmt.Errorf("postulacion: upload CV: %w", err)
	}
	p.CvURL = url

	// Step 4 — persist.
	if p.Estatus == 0 {
		p.Estatus = bolsatrabajo.PostulacionPendiente
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}

	// Step 5 — publish event.
	_ = s.pub.Publish(ctx, "PostulacionRecibida", map[string]any{
		"postulacion_id": p.ID,
		"trabajo_id":     p.TrabajoID,
	})

	return p, nil
}

func (s *postulacionService) GetStatsByTrabajo(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error) {
	return s.repo.GetStatsByTrabajo(ctx, trabajoID, empresaID)
}

func (s *postulacionService) UpdateEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error) {
	if newEstatus < 1 || newEstatus > 5 {
		return nil, fmt.Errorf("postulacion: newEstatus %d is invalid (must be 1..5): %w", newEstatus, bolsatrabajo.ErrNotFound)
	}

	result, err := s.repo.UpdateEstatus(ctx, id, empresaID, newEstatus)
	if err != nil {
		return nil, err
	}

	_ = s.pub.Publish(ctx, "PostulacionEstatusActualizado", map[string]any{
		"postulacion_id": id,
		"new_estatus":    newEstatus,
	})
	if newEstatus == bolsatrabajo.PostulacionContratado {
		_ = s.pub.Publish(ctx, "CandidatoContratado", map[string]any{
			"postulacion_id": id,
			"empresa_id":     empresaID,
		})
	}

	return result, nil
}
