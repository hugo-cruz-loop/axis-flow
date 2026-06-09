// Package middleware provides HTTP middleware for the cursos module.
package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/cursos"
	cursosmw "axis-flow-back/internal/cursos/middleware"

	appmiddleware "axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
)

// ── mock enrollment repo ──────────────────────────────────────────────────────

type mockEnrollRepo struct {
	getEnrollmentFn func(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error)
}

func (m *mockEnrollRepo) GetEnrollment(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error) {
	return m.getEnrollmentFn(ctx, cursoID, empleadoID)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestIsEnrolled_Enrolled_CallsNext(t *testing.T) {
	repo := &mockEnrollRepo{
		getEnrollmentFn: func(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error) {
			return &cursos.Enrollment{ID: 1, CursoID: cursoID, EmpleadoID: empleadoID}, nil
		},
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	router := chi.NewRouter()
	router.With(cursosmw.IsEnrolled(repo)).Get("/curso/{curso_id}/content", next)

	req := httptest.NewRequest(http.MethodGet, "/curso/5/content", nil)
	// Inject empleado_id via context key
	ctx := context.WithValue(req.Context(), appmiddleware.ContextKeyEmpleadoID, int64(99))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestIsEnrolled_QueryParamCursoID_Enrolled_CallsNext(t *testing.T) {
	repo := &mockEnrollRepo{
		getEnrollmentFn: func(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error) {
			if cursoID != 5 {
				return nil, cursos.ErrNotEnrolled
			}
			return &cursos.Enrollment{ID: 1, CursoID: cursoID, EmpleadoID: empleadoID}, nil
		},
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	router := chi.NewRouter()
	// Route without {curso_id} URL param — uses ?curso_id= query param instead.
	router.With(cursosmw.IsEnrolled(repo)).Post("/examen/resolver", next)

	req := httptest.NewRequest(http.MethodPost, "/examen/resolver?curso_id=5", nil)
	ctx := context.WithValue(req.Context(), appmiddleware.ContextKeyEmpleadoID, int64(99))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestIsEnrolled_NotEnrolled_403(t *testing.T) {
	repo := &mockEnrollRepo{
		getEnrollmentFn: func(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error) {
			return nil, cursos.ErrNotEnrolled
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router := chi.NewRouter()
	router.With(cursosmw.IsEnrolled(repo)).Get("/curso/{curso_id}/content", next)

	req := httptest.NewRequest(http.MethodGet, "/curso/5/content", nil)
	ctx := context.WithValue(req.Context(), appmiddleware.ContextKeyEmpleadoID, int64(99))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}
}
