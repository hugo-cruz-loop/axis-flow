package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/service"

	"github.com/google/uuid"
)

// --- mock CatalogRepository ---

type mockCatalogRepo struct {
	createCategoria func(ctx context.Context, c *cursos.Categoria) error
	listCategorias  func(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error)
	updateCategoria func(ctx context.Context, c *cursos.Categoria) error
	deleteCategoria func(ctx context.Context, id int64, empresaID uuid.UUID) error
	createModulo    func(ctx context.Context, m *cursos.Modulo) error
	listModulos     func(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error)
}

func (m *mockCatalogRepo) CreateCategoria(ctx context.Context, c *cursos.Categoria) error {
	if m.createCategoria != nil {
		return m.createCategoria(ctx, c)
	}
	return nil
}
func (m *mockCatalogRepo) ListCategorias(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error) {
	if m.listCategorias != nil {
		return m.listCategorias(ctx, empresaID)
	}
	return nil, nil
}
func (m *mockCatalogRepo) UpdateCategoria(ctx context.Context, c *cursos.Categoria) error {
	if m.updateCategoria != nil {
		return m.updateCategoria(ctx, c)
	}
	return nil
}
func (m *mockCatalogRepo) DeleteCategoria(ctx context.Context, id int64, empresaID uuid.UUID) error {
	if m.deleteCategoria != nil {
		return m.deleteCategoria(ctx, id, empresaID)
	}
	return nil
}
func (m *mockCatalogRepo) CreateModulo(ctx context.Context, mod *cursos.Modulo) error {
	if m.createModulo != nil {
		return m.createModulo(ctx, mod)
	}
	return nil
}
func (m *mockCatalogRepo) ListModulos(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error) {
	if m.listModulos != nil {
		return m.listModulos(ctx, empresaID)
	}
	return nil, nil
}

// --- mock ContentRepository ---

type mockContentRepo struct {
	createCurso       func(ctx context.Context, c *cursos.Curso) error
	getCurso          func(ctx context.Context, id int64) (*cursos.Curso, error)
	listCursos        func(ctx context.Context, empresaID uuid.UUID, includePrivate bool) ([]*cursos.Curso, error)
	updateCurso       func(ctx context.Context, c *cursos.Curso) error
	softDeleteCurso   func(ctx context.Context, id int64, empresaID uuid.UUID) error
	createUnidad      func(ctx context.Context, u *cursos.Unidad) error
	getUnidades       func(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)
	updateUnidad      func(ctx context.Context, u *cursos.Unidad) error
	deleteUnidad      func(ctx context.Context, id int64) error
	createLeccion     func(ctx context.Context, l *cursos.Leccion) error
	getLecciones      func(ctx context.Context, unidadID int64) ([]*cursos.Leccion, error)
	updateLeccion     func(ctx context.Context, l *cursos.Leccion) error
	deleteLeccion     func(ctx context.Context, id int64) error
	getCursoContenido func(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)
}

func (m *mockContentRepo) CreateCurso(ctx context.Context, c *cursos.Curso) error {
	if m.createCurso != nil {
		return m.createCurso(ctx, c)
	}
	return nil
}
func (m *mockContentRepo) GetCurso(ctx context.Context, id int64) (*cursos.Curso, error) {
	if m.getCurso != nil {
		return m.getCurso(ctx, id)
	}
	return nil, cursos.ErrCursoNotFound
}
func (m *mockContentRepo) ListCursos(ctx context.Context, empresaID uuid.UUID, includePrivate bool) ([]*cursos.Curso, error) {
	if m.listCursos != nil {
		return m.listCursos(ctx, empresaID, includePrivate)
	}
	return nil, nil
}
func (m *mockContentRepo) UpdateCurso(ctx context.Context, c *cursos.Curso) error {
	if m.updateCurso != nil {
		return m.updateCurso(ctx, c)
	}
	return nil
}
func (m *mockContentRepo) SoftDeleteCurso(ctx context.Context, id int64, empresaID uuid.UUID) error {
	if m.softDeleteCurso != nil {
		return m.softDeleteCurso(ctx, id, empresaID)
	}
	return nil
}
func (m *mockContentRepo) CreateUnidad(ctx context.Context, u *cursos.Unidad) error {
	if m.createUnidad != nil {
		return m.createUnidad(ctx, u)
	}
	return nil
}
func (m *mockContentRepo) GetUnidadesByCurso(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error) {
	if m.getUnidades != nil {
		return m.getUnidades(ctx, cursoID)
	}
	return nil, nil
}
func (m *mockContentRepo) UpdateUnidad(ctx context.Context, u *cursos.Unidad) error {
	if m.updateUnidad != nil {
		return m.updateUnidad(ctx, u)
	}
	return nil
}
func (m *mockContentRepo) DeleteUnidad(ctx context.Context, id int64) error {
	if m.deleteUnidad != nil {
		return m.deleteUnidad(ctx, id)
	}
	return nil
}
func (m *mockContentRepo) CreateLeccion(ctx context.Context, l *cursos.Leccion) error {
	if m.createLeccion != nil {
		return m.createLeccion(ctx, l)
	}
	return nil
}
func (m *mockContentRepo) GetLeccionesByUnidad(ctx context.Context, unidadID int64) ([]*cursos.Leccion, error) {
	if m.getLecciones != nil {
		return m.getLecciones(ctx, unidadID)
	}
	return nil, nil
}
func (m *mockContentRepo) UpdateLeccion(ctx context.Context, l *cursos.Leccion) error {
	if m.updateLeccion != nil {
		return m.updateLeccion(ctx, l)
	}
	return nil
}
func (m *mockContentRepo) DeleteLeccion(ctx context.Context, id int64) error {
	if m.deleteLeccion != nil {
		return m.deleteLeccion(ctx, id)
	}
	return nil
}
func (m *mockContentRepo) GetCursoContenido(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error) {
	if m.getCursoContenido != nil {
		return m.getCursoContenido(ctx, cursoID)
	}
	return nil, nil
}

// --- Tests ---

func TestGetCurso_PrivateWrongTenant_ErrTenantMismatch(t *testing.T) {
	ownerID := uuid.New()
	callerID := uuid.New()

	now := time.Now()
	contentRepo := &mockContentRepo{
		getCurso: func(_ context.Context, id int64) (*cursos.Curso, error) {
			return &cursos.Curso{
				ID:        id,
				Titulo:    "Secure Course",
				EmpresaID: ownerID,
				Estatus:   cursos.CursoEstatusPrivado,
				CreatedAt: now,
			}, nil
		},
	}

	svc := service.NewCursoService(&mockCatalogRepo{}, contentRepo)
	_, err := svc.GetCurso(context.Background(), 1, callerID)
	if !errors.Is(err, cursos.ErrTenantMismatch) {
		t.Fatalf("expected ErrTenantMismatch, got %v", err)
	}
}

func TestGetCurso_PrivateCorrectTenant_OK(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now()

	contentRepo := &mockContentRepo{
		getCurso: func(_ context.Context, id int64) (*cursos.Curso, error) {
			return &cursos.Curso{
				ID:        id,
				Titulo:    "Private Course",
				EmpresaID: tenantID,
				Estatus:   cursos.CursoEstatusPrivado,
				CreatedAt: now,
			}, nil
		},
	}

	svc := service.NewCursoService(&mockCatalogRepo{}, contentRepo)
	c, err := svc.GetCurso(context.Background(), 1, tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != 1 {
		t.Fatalf("expected curso id 1, got %d", c.ID)
	}
}

func TestGetCurso_Public_AnyTenant_OK(t *testing.T) {
	ownerID := uuid.New()
	callerID := uuid.New()
	now := time.Now()

	contentRepo := &mockContentRepo{
		getCurso: func(_ context.Context, id int64) (*cursos.Curso, error) {
			return &cursos.Curso{
				ID:        id,
				EmpresaID: ownerID,
				Estatus:   cursos.CursoEstatusPublico,
				CreatedAt: now,
			}, nil
		},
	}

	svc := service.NewCursoService(&mockCatalogRepo{}, contentRepo)
	c, err := svc.GetCurso(context.Background(), 1, callerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected curso, got nil")
	}
}

func TestListCursos_AdminIncludesPrivate(t *testing.T) {
	tenantID := uuid.New()
	called := false

	contentRepo := &mockContentRepo{
		listCursos: func(_ context.Context, empresaID uuid.UUID, includePrivate bool) ([]*cursos.Curso, error) {
			called = true
			if !includePrivate {
				t.Error("expected includePrivate=true for admin")
			}
			return []*cursos.Curso{}, nil
		},
	}

	svc := service.NewCursoService(&mockCatalogRepo{}, contentRepo)
	_, err := svc.ListCursos(context.Background(), tenantID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected listCursos to be called")
	}
}
