package service_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Inline mock — QuejaRepository
// ---------------------------------------------------------------------------

type mockQuejaRepo struct {
	queja     *atencionseguimiento.SolicitudQueja
	respuestas []*atencionseguimiento.RespuestaQueja
	createErr  error
	getErr     error

	createdRespuesta *atencionseguimiento.RespuestaQueja
	suspended        []int64
}

func (m *mockQuejaRepo) Create(_ context.Context, s *atencionseguimiento.SolicitudQueja) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.queja = s
	return nil
}

func (m *mockQuejaRepo) GetByID(_ context.Context, _ uuid.UUID, _ int64) (*atencionseguimiento.SolicitudQueja, error) {
	return m.queja, m.getErr
}

func (m *mockQuejaRepo) ListByEmpresa(_ context.Context, _ uuid.UUID, _ *int, _, _ int) ([]*atencionseguimiento.SolicitudQueja, int, error) {
	return nil, 0, nil
}

func (m *mockQuejaRepo) CreateRespuesta(_ context.Context, r *atencionseguimiento.RespuestaQueja) error {
	m.createdRespuesta = r
	return nil
}

func (m *mockQuejaRepo) GetRespuestas(_ context.Context, _ uuid.UUID, _, _ int) ([]*atencionseguimiento.RespuestaQueja, int, error) {
	return m.respuestas, len(m.respuestas), nil
}

func (m *mockQuejaRepo) SuspendQuejasByEmpleado(_ context.Context, empleadoID int64) error {
	m.suspended = append(m.suspended, empleadoID)
	return nil
}

// ---------------------------------------------------------------------------
// Inline mock — EventPublisher (spy)
// ---------------------------------------------------------------------------

type spyPublisher struct {
	stream  string
	payload map[string]any
	called  bool
}

func (s *spyPublisher) Publish(_ context.Context, stream string, payload map[string]any) error {
	s.called = true
	s.stream = stream
	s.payload = payload
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestQuejaService_CreateQueja_PublishesAndReturnsBoth(t *testing.T) {
	repo := &mockQuejaRepo{}
	pub := &spyPublisher{}
	svc := service.NewQuejaService(repo, pub)

	empresaID := uuid.New()
	q := &atencionseguimiento.SolicitudQueja{
		EmpresaID:   empresaID,
		TipoQuejaID: uuid.New(),
		Titulo:      "Test queja",
		Descripcion: "Descripcion",
	}

	createdQ, msg, err := svc.CreateQueja(context.Background(), q, 42, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdQ == nil {
		t.Fatal("expected queja, got nil")
	}
	if msg == nil {
		t.Fatal("expected mensaje_inicial, got nil")
	}
	if !pub.called {
		t.Fatal("expected event to be published")
	}
	if pub.stream != "atencion:queja_registrada" {
		t.Fatalf("wrong stream: %s", pub.stream)
	}
}

func TestQuejaService_GetQueja_EmpleadoForbiddenOnMismatch(t *testing.T) {
	empresaID := uuid.New()
	ownerID := int64(10)
	repo := &mockQuejaRepo{
		queja: &atencionseguimiento.SolicitudQueja{
			ID:         uuid.New(),
			EmpresaID:  empresaID,
			EmpleadoID: ownerID,
		},
	}
	pub := &spyPublisher{}
	svc := service.NewQuejaService(repo, pub)

	_, err := svc.GetQueja(context.Background(), uuid.New(), 99, "Empleado")
	if err == nil {
		t.Fatal("expected ErrForbidden, got nil")
	}
	if err != atencionseguimiento.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestQuejaService_CreateMensaje_EmpleadoForbiddenOnOtherQueja(t *testing.T) {
	empresaID := uuid.New()
	ownerID := int64(10)
	quejaID := uuid.New()
	repo := &mockQuejaRepo{
		queja: &atencionseguimiento.SolicitudQueja{
			ID:         quejaID,
			EmpresaID:  empresaID,
			EmpleadoID: ownerID,
		},
	}
	pub := &spyPublisher{}
	svc := service.NewQuejaService(repo, pub)

	msg := &atencionseguimiento.RespuestaQueja{Mensaje: "hola"}
	_, err := svc.CreateMensaje(context.Background(), quejaID, msg, 99, "Empleado")
	if err == nil {
		t.Fatal("expected ErrForbidden, got nil")
	}
	if err != atencionseguimiento.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}
