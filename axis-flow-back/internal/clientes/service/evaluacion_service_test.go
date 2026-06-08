package service_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/clientes"
	"axis-flow-back/internal/clientes/service"

	"github.com/google/uuid"
)

// ---- Mock EvaluacionRepository ----------------------------------------------

type mockEvaluacionRepo struct {
	createFn func(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error
	listFn   func(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error)
}

func (m *mockEvaluacionRepo) Create(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error {
	return m.createFn(ctx, e, empresaID)
}
func (m *mockEvaluacionRepo) ListByCliente(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error) {
	return m.listFn(ctx, clienteID, empresaID)
}

// ---- Tests ------------------------------------------------------------------

func TestEvaluacionService_CreateEvaluacion_HappyPath(t *testing.T) {
	ctx := context.Background()
	clienteID := uuid.New()
	empresaID := int64(1)

	repo := &mockEvaluacionRepo{
		createFn: func(_ context.Context, e *clientes.EvaluacionServicio, _ int64) error {
			e.ID = uuid.New()
			e.CreatedAt = time.Now()
			e.UpdatedAt = time.Now()
			return nil
		},
	}

	svc := service.NewEvaluacionService(repo)
	e := &clientes.EvaluacionServicio{
		ClienteID:   clienteID,
		Puntuacion:  5,
		DeUsuarioID: uuid.New(),
		Fecha:       time.Now(),
	}

	err := svc.CreateEvaluacion(ctx, e, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.ID == uuid.Nil {
		t.Error("expected ID to be assigned")
	}
}

func TestEvaluacionService_ListEvaluaciones_HappyPath(t *testing.T) {
	ctx := context.Background()
	clienteID := uuid.New()
	empresaID := int64(1)

	expected := []clientes.EvaluacionServicio{
		{ID: uuid.New(), ClienteID: clienteID, Puntuacion: 4},
		{ID: uuid.New(), ClienteID: clienteID, Puntuacion: 3},
	}

	repo := &mockEvaluacionRepo{
		listFn: func(_ context.Context, _ uuid.UUID, _ int64) ([]clientes.EvaluacionServicio, error) {
			return expected, nil
		},
	}

	svc := service.NewEvaluacionService(repo)
	got, err := svc.ListEvaluaciones(ctx, clienteID, empresaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}
