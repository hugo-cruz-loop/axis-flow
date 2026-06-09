// Package handler provides HTTP handlers for the cursos module.
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

type mockCatalogService struct {
	createCategoriaFn func(ctx context.Context, c *cursos.Categoria, empresaID uuid.UUID) error
	listCategoriasFn  func(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error)
	updateCategoriaFn func(ctx context.Context, c *cursos.Categoria) error
	deleteCategoriaFn func(ctx context.Context, id int64, empresaID uuid.UUID) error
	createModuloFn    func(ctx context.Context, m *cursos.Modulo, empresaID uuid.UUID) error
	listModulosFn     func(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error)
}

func (m *mockCatalogService) CreateCategoria(ctx context.Context, c *cursos.Categoria, empresaID uuid.UUID) error {
	return m.createCategoriaFn(ctx, c, empresaID)
}
func (m *mockCatalogService) ListCategorias(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error) {
	return m.listCategoriasFn(ctx, empresaID)
}
func (m *mockCatalogService) UpdateCategoria(ctx context.Context, c *cursos.Categoria) error {
	return m.updateCategoriaFn(ctx, c)
}
func (m *mockCatalogService) DeleteCategoria(ctx context.Context, id int64, empresaID uuid.UUID) error {
	return m.deleteCategoriaFn(ctx, id, empresaID)
}
func (m *mockCatalogService) CreateModulo(ctx context.Context, mo *cursos.Modulo, empresaID uuid.UUID) error {
	return m.createModuloFn(ctx, mo, empresaID)
}
func (m *mockCatalogService) ListModulos(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error) {
	return m.listModulosFn(ctx, empresaID)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func withTenantCtx(r *http.Request, tenantID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ContextKeyTenantID, tenantID.String())
	return r.WithContext(ctx)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCatalogHandler_CreateCategoria_HappyPath(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockCatalogService{
		createCategoriaFn: func(ctx context.Context, c *cursos.Categoria, empresaID uuid.UUID) error {
			c.ID = 1
			return nil
		},
	}
	h := handler.NewCatalogHandler(svc)

	router := chi.NewRouter()
	router.Post("/categoria", h.CreateCategoria)

	body := map[string]string{"nombre": "Backend", "descripcion": "Go courses"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/categoria", bytes.NewReader(b))
	req = withTenantCtx(req, tenantID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d: %s", w.Code, w.Body.String())
	}
}

func TestCatalogHandler_CreateCategoria_TenantMissing_401(t *testing.T) {
	svc := &mockCatalogService{}
	h := handler.NewCatalogHandler(svc)

	router := chi.NewRouter()
	router.Post("/categoria", h.CreateCategoria)

	body := map[string]string{"nombre": "X"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/categoria", bytes.NewReader(b))
	// No tenant context injected
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}
