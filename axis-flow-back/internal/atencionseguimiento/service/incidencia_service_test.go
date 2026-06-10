package service_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/atencionseguimiento"
	"axis-flow-back/internal/atencionseguimiento/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Inline mock — IncidenciaRepository
// ---------------------------------------------------------------------------

type mockIncidenciaRepo struct {
	created *atencionseguimiento.IncidenciaSupervisor
}

func (m *mockIncidenciaRepo) Create(_ context.Context, i *atencionseguimiento.IncidenciaSupervisor) error {
	m.created = i
	return nil
}

func (m *mockIncidenciaRepo) ListByEmpresa(_ context.Context, _ uuid.UUID, _, _ int) ([]*atencionseguimiento.IncidenciaSupervisor, int, error) {
	return nil, 0, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestIncidenciaService_CreateIncidencia_EmpresaMismatch_Forbidden(t *testing.T) {
	repo := &mockIncidenciaRepo{}
	pub := &spyPublisher{}
	svc := service.NewIncidenciaService(repo, pub)

	supervisorID := uuid.New()
	jwtEmpresaID := uuid.New()
	bodyEmpresaID := uuid.New() // different

	inc := &atencionseguimiento.IncidenciaSupervisor{
		EmpresaID:   bodyEmpresaID,
		Descripcion: "fraude",
	}

	_, err := svc.CreateIncidencia(context.Background(), inc, supervisorID, jwtEmpresaID)
	if err == nil {
		t.Fatal("expected ErrForbidden, got nil")
	}
	if err != atencionseguimiento.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestIncidenciaService_CreateIncidencia_Valid_PublishesEvent(t *testing.T) {
	repo := &mockIncidenciaRepo{}
	pub := &spyPublisher{}
	svc := service.NewIncidenciaService(repo, pub)

	supervisorID := uuid.New()
	empresaID := uuid.New()

	inc := &atencionseguimiento.IncidenciaSupervisor{
		EmpresaID:   empresaID,
		Descripcion: "tardanza",
	}

	created, err := svc.CreateIncidencia(context.Background(), inc, supervisorID, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created == nil {
		t.Fatal("expected incidencia, got nil")
	}
	if !pub.called {
		t.Fatal("expected event to be published")
	}
	if pub.stream != "atencion:incidencia_operativa_registrada" {
		t.Fatalf("wrong stream: %s", pub.stream)
	}
}
