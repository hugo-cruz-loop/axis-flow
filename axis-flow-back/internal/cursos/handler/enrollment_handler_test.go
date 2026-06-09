package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockEnrollmentService struct {
	enrollFn                  func(ctx context.Context, e *cursos.Enrollment) error
	listEnrollmentsByEmpleadoFn func(ctx context.Context, empleadoID int64) ([]*cursos.Enrollment, error)
	markLeccionCompletaFn     func(ctx context.Context, empleadoID, leccionID int64) error
	getNotaFn                 func(ctx context.Context, leccionID, empleadoID int64) (*cursos.Nota, error)
	upsertNotaFn              func(ctx context.Context, n *cursos.Nota) error
}

func (m *mockEnrollmentService) Enroll(ctx context.Context, e *cursos.Enrollment) error {
	return m.enrollFn(ctx, e)
}
func (m *mockEnrollmentService) ListEnrollmentsByEmpleado(ctx context.Context, empleadoID int64) ([]*cursos.Enrollment, error) {
	return m.listEnrollmentsByEmpleadoFn(ctx, empleadoID)
}
func (m *mockEnrollmentService) MarkLeccionCompleta(ctx context.Context, empleadoID, leccionID int64) error {
	return m.markLeccionCompletaFn(ctx, empleadoID, leccionID)
}
func (m *mockEnrollmentService) GetNota(ctx context.Context, leccionID, empleadoID int64) (*cursos.Nota, error) {
	return m.getNotaFn(ctx, leccionID, empleadoID)
}
func (m *mockEnrollmentService) UpsertNota(ctx context.Context, n *cursos.Nota) error {
	return m.upsertNotaFn(ctx, n)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestEnrollmentHandler_MarkLeccionCompleta_HappyPath_200(t *testing.T) {
	tenantID := uuid.New()
	called := false
	svc := &mockEnrollmentService{
		markLeccionCompletaFn: func(ctx context.Context, empleadoID, leccionID int64) error {
			called = true
			return nil
		},
	}
	h := handler.NewEnrollmentHandler(svc)
	router := chi.NewRouter()
	router.Post("/avance-leccion", h.MarkLeccionCompleta)

	body, _ := json.Marshal(map[string]any{
		"empleado_id": 99,
		"leccion_id":  10,
		"curso_id":    5,
	})
	req := httptest.NewRequest(http.MethodPost, "/avance-leccion", bytes.NewReader(body))
	req = withTenantCtx(req, tenantID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Fatal("expected MarkLeccionCompleta to be called")
	}
}

func TestEnrollmentHandler_Enroll_NoTenant_401(t *testing.T) {
	svc := &mockEnrollmentService{}
	h := handler.NewEnrollmentHandler(svc)
	router := chi.NewRouter()
	router.Post("/enroll", h.Enroll)

	body, _ := json.Marshal(map[string]any{"curso_id": 1, "empleado_id": 1})
	req := httptest.NewRequest(http.MethodPost, "/enroll", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}
