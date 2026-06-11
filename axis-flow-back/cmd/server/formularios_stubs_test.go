// Package main — formularios_stubs_test.go: stub services for the
// formularios routes test harness. Mirrors atencion_stubs_test.go
// (PR-4 of 09_AtencionSeguimiento_Service_Spec). Every stub returns
// a benign no-op value so the route matrix test can assert the
// 401 / 403 / 200 / 201 outcomes without exercising real business
// logic.
package main

import (
	"context"

	"axis-flow-back/internal/formularios"

	"github.com/google/uuid"
)

// ── stubFormularioSvc ────────────────────────────────────────────────────

type stubFormularioSvc struct{}

func (s *stubFormularioSvc) CreateFormulario(_ context.Context, f *formularios.Formulario, _ uuid.UUID) (*formularios.Formulario, error) {
	if f.ID == (uuid.UUID{}) {
		f.ID = uuid.New()
	}
	return f, nil
}
func (s *stubFormularioSvc) GetFormulariosByEmpresa(_ context.Context, _ uuid.UUID, _ *bool, _, _ int) ([]*formularios.Formulario, int, error) {
	return []*formularios.Formulario{}, 0, nil
}
func (s *stubFormularioSvc) AddPregunta(_ context.Context, p *formularios.Pregunta, _ uuid.UUID) (*formularios.Pregunta, error) {
	if p.ID == (uuid.UUID{}) {
		p.ID = uuid.New()
	}
	return p, nil
}

// ── stubEventoSvc ────────────────────────────────────────────────────────

type stubEventoSvc struct{}

func (s *stubEventoSvc) CreateEvento(_ context.Context, e *formularios.Evento, _ []uuid.UUID, _ uuid.UUID) (*formularios.Evento, error) {
	if e.ID == (uuid.UUID{}) {
		e.ID = uuid.New()
	}
	return e, nil
}
func (s *stubEventoSvc) GetEvento(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*formularios.Evento, error) {
	return &formularios.Evento{}, nil
}
func (s *stubEventoSvc) GetEventosByEmpCte(_ context.Context, _, _ uuid.UUID, _ *string, _, _ int) ([]*formularios.Evento, int, error) {
	return []*formularios.Evento{}, 0, nil
}
func (s *stubEventoSvc) IniciarEvento(_ context.Context, ei *formularios.EventoIniciado, _ uuid.UUID) (*formularios.EventoIniciado, error) {
	if ei.ID == (uuid.UUID{}) {
		ei.ID = uuid.New()
	}
	return ei, nil
}
func (s *stubEventoSvc) CancelEvento(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

// ── stubRespuestaSvc ─────────────────────────────────────────────────────

type stubRespuestaSvc struct{}

func (s *stubRespuestaSvc) SubmitRespuesta(_ context.Context, r *formularios.Respuesta, _ uuid.UUID) (*formularios.Respuesta, error) {
	if r.ID == (uuid.UUID{}) {
		r.ID = uuid.New()
	}
	return r, nil
}
func (s *stubRespuestaSvc) GetRespuestasByIniciado(_ context.Context, _ uuid.UUID, _ uuid.UUID) ([]*formularios.Respuesta, error) {
	return []*formularios.Respuesta{}, nil
}

// ── stubPDFSvc ───────────────────────────────────────────────────────────

// stubPDFSvc implements service.PDFService. PR-4 AMEND (FIX 2): the
// interface now returns (bytes, url, error). The route harness never
// inspects the body (it only walks the route matrix) so we return a
// minimal %PDF-1.4 stub.
type stubPDFSvc struct{}

func (s *stubPDFSvc) GenerateReporte(_ context.Context, _, _ uuid.UUID) ([]byte, string, error) {
	return []byte(
		"%PDF-1.4\n" +
			"1 0 obj<</Type/Catalog>>endobj\n" +
			"trailer<</Root 1 0 R>>\n" +
			"%%EOF\n",
	), "https://s3.example.com/reports/stub.pdf", nil
}
