package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/handler"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockEvaluacionService struct {
	createFn           func(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
	getByPostulacionFn func(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
}

func (m *mockEvaluacionService) Create(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	return m.createFn(ctx, e, empresaID)
}
func (m *mockEvaluacionService) GetByPostulacion(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
	return m.getByPostulacionFn(ctx, postulacionID, empresaID)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildEvaluacionRouter(svc handler.EvaluacionServicer) http.Handler {
	r := chi.NewRouter()
	h := handler.NewEvaluacionHandler(svc)
	r.Post("/bolsa-trabajo/evaluacion", h.Create)
	r.Get("/bolsa-trabajo/evaluacion/by-postulacion/{id}", h.GetByPostulacion)
	return r
}

// ── tests ─────────────────────────────────────────────────────────────────────

// valid → 201
func TestEvaluacion_Valid(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	result := &bolsatrabajo.Evaluacion{
		ID:            uuid.New(),
		PostulacionID: uuid.New(),
		Puntualidad:   4,
		Cortesia:      5,
		SoftSkills:    3,
		EvaluatorID:   userID,
	}
	svc := &mockEvaluacionService{
		createFn: func(_ context.Context, _ *bolsatrabajo.Evaluacion, _ uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
			return result, nil
		},
	}
	router := buildEvaluacionRouter(svc)

	body, _ := json.Marshal(map[string]any{
		"postulacion_id": uuid.New().String(),
		"puntualidad":    4,
		"cortesia":       5,
		"soft_skills":    3,
		"comentarios":    "good candidate",
	})
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/evaluacion", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKeyTenantID, tenantID.String())
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}
}

// duplicate → 409
func TestEvaluacion_Duplicate(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	svc := &mockEvaluacionService{
		createFn: func(_ context.Context, _ *bolsatrabajo.Evaluacion, _ uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
			return nil, bolsatrabajo.ErrConflict
		},
	}
	router := buildEvaluacionRouter(svc)

	body, _ := json.Marshal(map[string]any{
		"postulacion_id": uuid.New().String(),
		"puntualidad":    4,
		"cortesia":       4,
		"soft_skills":    4,
	})
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/evaluacion", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKeyTenantID, tenantID.String())
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d — body: %s", w.Code, w.Body.String())
	}
}

// score out of range → 422
func TestEvaluacion_ScoreOutOfRange(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	svc := &mockEvaluacionService{
		createFn: func(_ context.Context, _ *bolsatrabajo.Evaluacion, _ uuid.UUID) (*bolsatrabajo.Evaluacion, error) {
			return nil, bolsatrabajo.ErrInvalidFile // reuse ErrInvalidFile as "validation error"
		},
	}
	router := buildEvaluacionRouter(svc)

	body, _ := json.Marshal(map[string]any{
		"postulacion_id": uuid.New().String(),
		"puntualidad":    10, // out of 1-5
		"cortesia":       4,
		"soft_skills":    3,
	})
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/evaluacion", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKeyTenantID, tenantID.String())
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d — body: %s", w.Code, w.Body.String())
	}
}
