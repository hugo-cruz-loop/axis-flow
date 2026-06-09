package service_test

import (
	"context"
	"errors"
	"testing"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/events"
	"axis-flow-back/internal/bolsatrabajo/service"

	"github.com/google/uuid"
)

// --- mock EvaluacionRepository ---

type mockEvaluacionRepo struct {
	createFn          func(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) error
	getByPostulacionFn func(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
}

func (m *mockEvaluacionRepo) Create(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) error {
	return m.createFn(ctx, e, empresaID)
}
func (m *mockEvaluacionRepo) GetByPostulacion(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	return m.getByPostulacionFn(ctx, postulacionID, empresaID)
}

// --- tests ---

func TestEvaluacionService_Create_ScoreOutOfRange(t *testing.T) {
	t.Parallel()
	svc := service.NewEvaluacionService(&mockEvaluacionRepo{}, &mockPublisher{})

	ev := &bolsatrabajo.Evaluacion{
		Puntualidad: 6, // invalid — must be 1..5
		Cortesia:    3,
		SoftSkills:  3,
	}
	_, err := svc.Create(context.Background(), ev, uuid.New())
	if err == nil {
		t.Fatal("expected error for out-of-range score, got nil")
	}
}

func TestEvaluacionService_Create_Valid_PublishesEvent(t *testing.T) {
	t.Parallel()
	empresaID := uuid.New()
	repo := &mockEvaluacionRepo{
		createFn: func(_ context.Context, _ *bolsatrabajo.Evaluacion, _ uuid.UUID) error { return nil },
	}
	pub := &mockPublisher{}
	svc := service.NewEvaluacionService(repo, pub)

	ev := &bolsatrabajo.Evaluacion{
		PostulacionID: uuid.New(),
		Puntualidad:   4,
		Cortesia:      5,
		SoftSkills:    3,
		EvaluatorID:   uuid.New(),
	}
	result, err := svc.Create(context.Background(), ev, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].stream != events.StreamPostulacionEvaluada {
		t.Errorf("expected stream %s, got %s", events.StreamPostulacionEvaluada, pub.events[0].stream)
	}
}

func TestEvaluacionService_Create_ZeroScore_Rejected(t *testing.T) {
	t.Parallel()
	svc := service.NewEvaluacionService(&mockEvaluacionRepo{}, &mockPublisher{})
	ev := &bolsatrabajo.Evaluacion{Puntualidad: 0, Cortesia: 3, SoftSkills: 3}
	_, err := svc.Create(context.Background(), ev, uuid.New())
	if err == nil {
		t.Fatal("expected error for score 0")
	}
}

func TestEvaluacionService_GetByPostulacion_Forbidden(t *testing.T) {
	t.Parallel()
	repo := &mockEvaluacionRepo{
		getByPostulacionFn: func(_ context.Context, _, _ uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
			return nil, bolsatrabajo.ErrForbidden
		},
	}
	svc := service.NewEvaluacionService(repo, &mockPublisher{})
	_, err := svc.GetByPostulacion(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, bolsatrabajo.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}
