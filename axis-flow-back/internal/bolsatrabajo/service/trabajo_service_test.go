package service_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/events"
	"axis-flow-back/internal/bolsatrabajo/service"

	"github.com/google/uuid"
)

// --- mock TrabajoRepository ---

type mockTrabajoRepo struct {
	createFn          func(ctx context.Context, t *bolsatrabajo.Trabajo) error
	getActiveJobsFn   func(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	getByEmpresaFn    func(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	getRecentFn       func(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error)
	switchEstatusFn   func(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error)
	closeAllByEmpresa []uuid.UUID // captures empresaID calls
}

func (m *mockTrabajoRepo) Create(ctx context.Context, t *bolsatrabajo.Trabajo) error {
	return m.createFn(ctx, t)
}
func (m *mockTrabajoRepo) GetActiveJobs(ctx context.Context, search string, empresaID *uuid.UUID, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return m.getActiveJobsFn(ctx, search, empresaID, page, pageSize)
}
func (m *mockTrabajoRepo) GetByEmpresa(ctx context.Context, empresaID uuid.UUID, filter, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return m.getByEmpresaFn(ctx, empresaID, filter, page, pageSize)
}
func (m *mockTrabajoRepo) GetRecent(ctx context.Context, page, pageSize int) ([]*bolsatrabajo.Trabajo, int, error) {
	return m.getRecentFn(ctx, page, pageSize)
}
func (m *mockTrabajoRepo) SwitchEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error) {
	return m.switchEstatusFn(ctx, id, empresaID, newEstatus)
}

// --- mock EventPublisher ---

type mockPublisher struct {
	events []publishedEvent
}

type publishedEvent struct {
	stream  string
	payload map[string]any
}

func (m *mockPublisher) Publish(_ context.Context, stream string, payload map[string]any) error {
	m.events = append(m.events, publishedEvent{stream: stream, payload: payload})
	return nil
}

// --- tests ---

func TestTrabajoService_Create_PastDate_Rejected(t *testing.T) {
	t.Parallel()
	repo := &mockTrabajoRepo{}
	pub := &mockPublisher{}
	svc := service.NewTrabajoService(repo, pub)

	past := time.Now().Add(-24 * time.Hour)
	empresaID := uuid.New()
	trabajo := &bolsatrabajo.Trabajo{Titulo: "Dev", FechaCaducar: past}

	_, err := svc.Create(context.Background(), trabajo, empresaID)
	if err == nil {
		t.Fatal("expected error for past FechaCaducar, got nil")
	}
}

func TestTrabajoService_Create_Valid_PublishesEvent(t *testing.T) {
	t.Parallel()
	empresaID := uuid.New()
	repo := &mockTrabajoRepo{
		createFn: func(_ context.Context, t *bolsatrabajo.Trabajo) error { return nil },
	}
	pub := &mockPublisher{}
	svc := service.NewTrabajoService(repo, pub)

	future := time.Now().Add(48 * time.Hour)
	trabajo := &bolsatrabajo.Trabajo{Titulo: "Dev", FechaCaducar: future}

	result, err := svc.Create(context.Background(), trabajo, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EmpresaID != empresaID {
		t.Errorf("expected empresa_id %v, got %v", empresaID, result.EmpresaID)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].stream != events.StreamVacanteCreada {
		t.Errorf("expected stream %s, got %s", events.StreamVacanteCreada, pub.events[0].stream)
	}
}

func TestTrabajoService_SwitchEstatus_InvalidStatus(t *testing.T) {
	t.Parallel()
	repo := &mockTrabajoRepo{}
	pub := &mockPublisher{}
	svc := service.NewTrabajoService(repo, pub)

	_, err := svc.SwitchEstatus(context.Background(), uuid.New(), uuid.New(), 99)
	if err == nil {
		t.Fatal("expected error for invalid estatus 99")
	}
}

func TestTrabajoService_SwitchEstatus_Valid(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	empresaID := uuid.New()
	expected := &bolsatrabajo.Trabajo{ID: id, EmpresaID: empresaID, EstatusVacante: 2}
	repo := &mockTrabajoRepo{
		switchEstatusFn: func(_ context.Context, gotID, gotEmpresa uuid.UUID, newEstatus int) (*bolsatrabajo.Trabajo, error) {
			return expected, nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewTrabajoService(repo, pub)

	result, err := svc.SwitchEstatus(context.Background(), id, empresaID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != id {
		t.Errorf("expected id %v, got %v", id, result.ID)
	}
}
