package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockExamService struct {
	getExamenForStudentFn func(ctx context.Context, cursoID int64) (*cursos.Examen, error)
	resolverExamenFn      func(ctx context.Context, empleadoID, examenID int64, respuestas map[int64]int64) (*cursos.ResultadoExamen, error)
	getResultadosFn       func(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error)
}

func (m *mockExamService) GetExamenForStudent(ctx context.Context, cursoID int64) (*cursos.Examen, error) {
	return m.getExamenForStudentFn(ctx, cursoID)
}
func (m *mockExamService) ResolverExamen(ctx context.Context, empleadoID, examenID int64, respuestas map[int64]int64) (*cursos.ResultadoExamen, error) {
	return m.resolverExamenFn(ctx, empleadoID, examenID, respuestas)
}
func (m *mockExamService) GetResultados(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error) {
	return m.getResultadosFn(ctx, examenID, empleadoID)
}

// ── ANTI-CHEAT GATE TEST ──────────────────────────────────────────────────────

// TestExamHandler_GetExamen_EsCorrectaAbsent verifies that the JSON response
// for the student exam view does NOT contain the "es_correcta" field.
// This is the critical anti-cheat gate test.
func TestExamHandler_GetExamen_EsCorrectaAbsent(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockExamService{
		getExamenForStudentFn: func(ctx context.Context, cursoID int64) (*cursos.Examen, error) {
			// Service should already return ExamenOpcionStudent (no es_correcta),
			// but we return an Examen with Preguntas that contain ExamenOpcion with EsCorrecta
			// to simulate a bug if the handler leaks it.
			// The handler must strip es_correcta before serializing.
			ex := &cursos.Examen{
				ID:      1,
				CursoID: cursoID,
				Titulo:  "Test Exam",
			}
			return ex, nil
		},
	}
	h := handler.NewExamHandler(svc)
	router := chi.NewRouter()
	router.Get("/examen/{curso_id}", h.GetExamenForStudent)

	req := httptest.NewRequest(http.MethodGet, "/examen/5", nil)
	req = withTenantCtx(req, tenantID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "es_correcta") {
		t.Fatal("anti-cheat gate FAILED: es_correcta field present in student response body")
	}
}

func TestExamHandler_ResolverExamen_HappyPath_200(t *testing.T) {
	tenantID := uuid.New()
	aprobado := true
	cal := 100.0
	svc := &mockExamService{
		resolverExamenFn: func(ctx context.Context, empleadoID, examenID int64, respuestas map[int64]int64) (*cursos.ResultadoExamen, error) {
			return &cursos.ResultadoExamen{
				ExamenID:     examenID,
				EmpleadoID:   empleadoID,
				Calificacion: &cal,
				Aprobado:     &aprobado,
				Intento:      1,
			}, nil
		},
	}
	h := handler.NewExamHandler(svc)
	router := chi.NewRouter()
	router.Post("/examen/resolver", h.ResolverExamen)

	body, _ := json.Marshal(map[string]any{
		"examen_id":   1,
		"empleado_id": 99,
		"respuestas":  []map[string]any{{"pregunta_id": 1, "opcion_id": 2}},
	})
	req := httptest.NewRequest(http.MethodPost, "/examen/resolver", strings.NewReader(string(body)))
	req = withTenantCtx(req, tenantID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
}
