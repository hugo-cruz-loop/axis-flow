package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/clientes"
	"axis-flow-back/internal/clientes/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stub ──────────────────────────────────────────────────────────────────────

type stubEvaluacionService struct {
	createFn func(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error
	listFn   func(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error)
}

func (s *stubEvaluacionService) CreateEvaluacion(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error {
	return s.createFn(ctx, e, empresaID)
}
func (s *stubEvaluacionService) ListEvaluaciones(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error) {
	return s.listFn(ctx, clienteID, empresaID)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCreateEvaluacion_OutOfRange_400(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubEvaluacionService{
		createFn: func(_ context.Context, _ *clientes.EvaluacionServicio, _ int64) error {
			return nil
		},
	}
	h := handler.NewEvaluacionHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"puntuacion": 6, // out of range
	})
	r := httptest.NewRequest(http.MethodPost, "/cliente/"+clienteID.String()+"/evaluaciones?empresa_id=1",
		bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", clienteID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()
	h.CreateEvaluacion(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListEvaluaciones_Happy_200(t *testing.T) {
	clienteID := uuid.New()
	svc := &stubEvaluacionService{
		listFn: func(_ context.Context, _ uuid.UUID, _ int64) ([]clientes.EvaluacionServicio, error) {
			return []clientes.EvaluacionServicio{
				{ID: uuid.New(), ClienteID: clienteID, Puntuacion: 5},
			}, nil
		},
	}
	h := handler.NewEvaluacionHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/cliente/"+clienteID.String()+"/evaluaciones?empresa_id=1", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", clienteID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()
	h.ListEvaluaciones(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}
