package main

import (
	"context"
	"io"

	"axis-flow-back/internal/bolsatrabajo"

	"github.com/google/uuid"
)

// ── stubTrabajoSvc ────────────────────────────────────────────────────────────

type stubTrabajoSvc struct{}

func (s *stubTrabajoSvc) Create(_ context.Context, t *bolsatrabajo.Trabajo, _ uuid.UUID) (*bolsatrabajo.Trabajo, error) {
	return t, nil
}
func (s *stubTrabajoSvc) GetActiveJobs(_ context.Context, _ string, _ *uuid.UUID, _, _ int) ([]*bolsatrabajo.Trabajo, int, error) {
	return nil, 0, nil
}
func (s *stubTrabajoSvc) GetByEmpresa(_ context.Context, _ uuid.UUID, _, _, _ int) ([]*bolsatrabajo.Trabajo, int, error) {
	return nil, 0, nil
}
func (s *stubTrabajoSvc) GetRecent(_ context.Context, _, _ int) ([]*bolsatrabajo.Trabajo, int, error) {
	return nil, 0, nil
}
func (s *stubTrabajoSvc) SwitchEstatus(_ context.Context, _, _ uuid.UUID, _ int) (*bolsatrabajo.Trabajo, error) {
	return &bolsatrabajo.Trabajo{}, nil
}
func (s *stubTrabajoSvc) CloseAllByEmpresa(_ context.Context, _ uuid.UUID) error { return nil }

// ── stubPostulacionSvc ────────────────────────────────────────────────────────

type stubPostulacionSvc struct{}

func (s *stubPostulacionSvc) Apply(_ context.Context, p *bolsatrabajo.Postulacion, _ io.Reader, _ int64, _, _ string) (*bolsatrabajo.Postulacion, error) {
	return p, nil
}
func (s *stubPostulacionSvc) GetStatsByTrabajo(_ context.Context, _, _ uuid.UUID) (*bolsatrabajo.PipelineStats, error) {
	return &bolsatrabajo.PipelineStats{}, nil
}
func (s *stubPostulacionSvc) UpdateEstatus(_ context.Context, _, _ uuid.UUID, _ int) (*bolsatrabajo.Postulacion, error) {
	return &bolsatrabajo.Postulacion{}, nil
}

// ── stubEvaluacionSvc ─────────────────────────────────────────────────────────

type stubEvaluacionSvc struct{}

func (s *stubEvaluacionSvc) Create(_ context.Context, e *bolsatrabajo.Evaluacion, _ uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	return e, nil
}
func (s *stubEvaluacionSvc) GetByPostulacion(_ context.Context, _, _ uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	return &bolsatrabajo.Evaluacion{}, nil
}
