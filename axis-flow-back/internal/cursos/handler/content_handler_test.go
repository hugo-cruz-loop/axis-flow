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
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockContentService struct {
	createCursoFn       func(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error
	listCursosFn        func(ctx context.Context, tenantID uuid.UUID, isAdmin bool) ([]*cursos.Curso, error)
	getCursoFn          func(ctx context.Context, id int64, tenantID uuid.UUID) (*cursos.Curso, error)
	getCursoContenidoFn func(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)
	updateCursoFn       func(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error
	deleteCursoFn       func(ctx context.Context, id int64, empresaID uuid.UUID) error
	createUnidadFn      func(ctx context.Context, u *cursos.Unidad) error
	updateUnidadFn      func(ctx context.Context, u *cursos.Unidad) error
	deleteUnidadFn      func(ctx context.Context, id int64) error
	createLeccionFn     func(ctx context.Context, l *cursos.Leccion) error
	updateLeccionFn     func(ctx context.Context, l *cursos.Leccion) error
	deleteLeccionFn     func(ctx context.Context, id int64) error
}

func (m *mockContentService) CreateCurso(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error {
	return m.createCursoFn(ctx, c, empresaID)
}
func (m *mockContentService) ListCursos(ctx context.Context, tenantID uuid.UUID, isAdmin bool) ([]*cursos.Curso, error) {
	return m.listCursosFn(ctx, tenantID, isAdmin)
}
func (m *mockContentService) GetCurso(ctx context.Context, id int64, tenantID uuid.UUID) (*cursos.Curso, error) {
	return m.getCursoFn(ctx, id, tenantID)
}
func (m *mockContentService) GetCursoContenido(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error) {
	return m.getCursoContenidoFn(ctx, cursoID)
}
func (m *mockContentService) UpdateCurso(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error {
	return m.updateCursoFn(ctx, c, empresaID)
}
func (m *mockContentService) DeleteCurso(ctx context.Context, id int64, empresaID uuid.UUID) error {
	return m.deleteCursoFn(ctx, id, empresaID)
}
func (m *mockContentService) CreateUnidad(ctx context.Context, u *cursos.Unidad) error {
	return m.createUnidadFn(ctx, u)
}
func (m *mockContentService) UpdateUnidad(ctx context.Context, u *cursos.Unidad) error {
	return m.updateUnidadFn(ctx, u)
}
func (m *mockContentService) DeleteUnidad(ctx context.Context, id int64) error {
	return m.deleteUnidadFn(ctx, id)
}
func (m *mockContentService) CreateLeccion(ctx context.Context, l *cursos.Leccion) error {
	return m.createLeccionFn(ctx, l)
}
func (m *mockContentService) UpdateLeccion(ctx context.Context, l *cursos.Leccion) error {
	return m.updateLeccionFn(ctx, l)
}
func (m *mockContentService) DeleteLeccion(ctx context.Context, id int64) error {
	return m.deleteLeccionFn(ctx, id)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func withRoleCtx(r *http.Request, role string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ContextKeyRole, role)
	return r.WithContext(ctx)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestContentHandler_ListCursos_AdminIncludesPrivate(t *testing.T) {
	tenantID := uuid.New()
	var capturedIsAdmin bool

	svc := &mockContentService{
		listCursosFn: func(ctx context.Context, tid uuid.UUID, isAdmin bool) ([]*cursos.Curso, error) {
			capturedIsAdmin = isAdmin
			return []*cursos.Curso{{ID: 1, Estatus: cursos.CursoEstatusPrivado}}, nil
		},
	}
	h := handler.NewContentHandler(svc)
	router := chi.NewRouter()
	router.Get("/cursos", h.ListCursos)

	req := httptest.NewRequest(http.MethodGet, "/cursos?include_private=true", nil)
	req = withTenantCtx(req, tenantID)
	req = withRoleCtx(req, "admin")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !capturedIsAdmin {
		t.Fatal("expected isAdmin=true for admin role with include_private")
	}
}

func TestContentHandler_DeleteCurso_SoftDelete_204(t *testing.T) {
	tenantID := uuid.New()
	called := false
	svc := &mockContentService{
		deleteCursoFn: func(ctx context.Context, id int64, empresaID uuid.UUID) error {
			called = true
			return nil
		},
	}
	h := handler.NewContentHandler(svc)
	router := chi.NewRouter()
	router.Delete("/curso/{id}", h.DeleteCurso)

	req := httptest.NewRequest(http.MethodDelete, "/curso/42", nil)
	req = withTenantCtx(req, tenantID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Fatal("expected DeleteCurso to be called")
	}
}

func TestContentHandler_CreateCurso_MissingBody_400(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockContentService{}
	h := handler.NewContentHandler(svc)
	router := chi.NewRouter()
	router.Post("/curso", h.CreateCurso)

	req := httptest.NewRequest(http.MethodPost, "/curso", bytes.NewReader([]byte("not-json")))
	req = withTenantCtx(req, tenantID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestContentHandler_CreateCurso_NoTenant_401(t *testing.T) {
	svc := &mockContentService{}
	h := handler.NewContentHandler(svc)
	router := chi.NewRouter()
	router.Post("/curso", h.CreateCurso)

	body, _ := json.Marshal(map[string]any{"titulo": "X", "categoria_id": 1})
	req := httptest.NewRequest(http.MethodPost, "/curso", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}
