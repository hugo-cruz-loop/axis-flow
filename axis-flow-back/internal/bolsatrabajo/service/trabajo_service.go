// Package service implements the business logic for the bolsatrabajo module.
package service

import (
	"context"
	"fmt"
	"time"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/events"
	"axis-flow-back/internal/bolsatrabajo/repository"

	"github.com/google/uuid"
)

// TrabajoService defines business operations for job postings.
type TrabajoService interface {
	Create(ctx context.Context, t *bolsatrabajo.Trabajo, empresaID uuid.UUID) (*bolsatrabajo.Trabajo, error)
	GetActiveJobs(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	GetByEmpresa(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	GetRecent(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	SwitchEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error)
	CloseAllByEmpresa(ctx context.Context, empresaID uuid.UUID) error
}

// trabajoService is the production implementation of TrabajoService.
type trabajoService struct {
	repo repository.TrabajoRepository
	pub  events.EventPublisher
}

// NewTrabajoService constructs a trabajoService.
func NewTrabajoService(repo repository.TrabajoRepository, pub events.EventPublisher) TrabajoService {
	return &trabajoService{repo: repo, pub: pub}
}

// Compile-time interface check.
var _ TrabajoService = (*trabajoService)(nil)

func (s *trabajoService) Create(ctx context.Context, t *bolsatrabajo.Trabajo, empresaID uuid.UUID) (*bolsatrabajo.Trabajo, error) {
	if !t.FechaCaducar.After(time.Now()) {
		return nil, fmt.Errorf("trabajo: FechaCaducar must be a future date: %w", bolsatrabajo.ErrNotFound)
	}
	t.EmpresaID = empresaID
	if t.EstatusVacante == 0 {
		t.EstatusVacante = bolsatrabajo.VacanteActivo
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	_ = s.pub.Publish(ctx, "VacanteCreada", map[string]any{
		"trabajo_id": t.ID,
		"empresa_id": empresaID,
	})
	return t, nil
}

func (s *trabajoService) GetActiveJobs(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return s.repo.GetActiveJobs(ctx, search, empresaID, page, pageSize)
}

func (s *trabajoService) GetByEmpresa(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return s.repo.GetByEmpresa(ctx, empresaID, filter, page, pageSize)
}

func (s *trabajoService) GetRecent(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return s.repo.GetRecent(ctx, page, pageSize)
}

func (s *trabajoService) SwitchEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error) {
	if newEstatus < 1 || newEstatus > 3 {
		return nil, fmt.Errorf("trabajo: newEstatus %d is not valid (must be 1, 2 or 3): %w", newEstatus, bolsatrabajo.ErrNotFound)
	}
	return s.repo.SwitchEstatus(ctx, id, empresaID, newEstatus)
}

// CloseAllByEmpresa sets estatus=3 (VacanteFin) for every active vacante of the empresa.
// This is intended to be triggered by an EmpresaDeBaja event.
func (s *trabajoService) CloseAllByEmpresa(ctx context.Context, empresaID uuid.UUID) error {
	page := 1
	const batchSize = 100
	for {
		trabajos, total, err := s.repo.GetByEmpresa(ctx, empresaID, bolsatrabajo.VacanteActivo, page, batchSize)
		if err != nil {
			return fmt.Errorf("CloseAllByEmpresa: fetch page %d: %w", page, err)
		}
		for _, t := range trabajos {
			if _, err := s.repo.SwitchEstatus(ctx, t.ID, empresaID, bolsatrabajo.VacanteFin); err != nil {
				return fmt.Errorf("CloseAllByEmpresa: close %v: %w", t.ID, err)
			}
		}
		if len(trabajos) < batchSize || page*batchSize >= total {
			break
		}
		page++
	}
	return nil
}
