// Package service provides business logic for the cursos domain.
package service

import (
	"context"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/repository"

	"github.com/google/uuid"
)

// CursoService defines business operations for course catalog and content management.
type CursoService interface {
	CreateCategoria(ctx context.Context, c *cursos.Categoria, empresaID uuid.UUID) error
	ListCategorias(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error)
	UpdateCategoria(ctx context.Context, c *cursos.Categoria) error
	DeleteCategoria(ctx context.Context, id int64, empresaID uuid.UUID) error

	CreateCurso(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error
	GetCurso(ctx context.Context, id int64, tenantID uuid.UUID) (*cursos.Curso, error)
	ListCursos(ctx context.Context, tenantID uuid.UUID, isAdmin bool) ([]*cursos.Curso, error)
	UpdateCurso(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error
	DeleteCurso(ctx context.Context, id int64, empresaID uuid.UUID) error

	GetCursoContenido(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)

	CreateModulo(ctx context.Context, m *cursos.Modulo, empresaID uuid.UUID) error
	ListModulos(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error)

	CreateUnidad(ctx context.Context, u *cursos.Unidad) error
	UpdateUnidad(ctx context.Context, u *cursos.Unidad) error
	DeleteUnidad(ctx context.Context, id int64) error

	CreateLeccion(ctx context.Context, l *cursos.Leccion) error
	UpdateLeccion(ctx context.Context, l *cursos.Leccion) error
	DeleteLeccion(ctx context.Context, id int64) error
}

// pgxCursoService implements CursoService using repository interfaces.
type pgxCursoService struct {
	catalog repository.CatalogRepository
	content repository.ContentRepository
}

// NewCursoService constructs a CursoService.
func NewCursoService(catalog repository.CatalogRepository, content repository.ContentRepository) CursoService {
	return &pgxCursoService{catalog: catalog, content: content}
}

func (s *pgxCursoService) CreateCategoria(ctx context.Context, c *cursos.Categoria, empresaID uuid.UUID) error {
	c.EmpresaID = empresaID
	return s.catalog.CreateCategoria(ctx, c)
}

func (s *pgxCursoService) ListCategorias(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error) {
	return s.catalog.ListCategorias(ctx, empresaID)
}

func (s *pgxCursoService) UpdateCategoria(ctx context.Context, c *cursos.Categoria) error {
	return s.catalog.UpdateCategoria(ctx, c)
}

func (s *pgxCursoService) DeleteCategoria(ctx context.Context, id int64, empresaID uuid.UUID) error {
	return s.catalog.DeleteCategoria(ctx, id, empresaID)
}

func (s *pgxCursoService) CreateCurso(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error {
	c.EmpresaID = empresaID
	return s.content.CreateCurso(ctx, c)
}

// GetCurso returns the course. If the course is private (estatus=2), the caller's tenantID
// must match the course's EmpresaID, otherwise ErrTenantMismatch is returned.
func (s *pgxCursoService) GetCurso(ctx context.Context, id int64, tenantID uuid.UUID) (*cursos.Curso, error) {
	c, err := s.content.GetCurso(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.Estatus == cursos.CursoEstatusPrivado && c.EmpresaID != tenantID {
		return nil, cursos.ErrTenantMismatch
	}
	return c, nil
}

// ListCursos returns courses visible to the caller. When isAdmin is true, private
// courses owned by the tenant are included; otherwise only public courses are returned.
func (s *pgxCursoService) ListCursos(ctx context.Context, tenantID uuid.UUID, isAdmin bool) ([]*cursos.Curso, error) {
	return s.content.ListCursos(ctx, tenantID, isAdmin)
}

func (s *pgxCursoService) UpdateCurso(ctx context.Context, c *cursos.Curso, empresaID uuid.UUID) error {
	c.EmpresaID = empresaID
	return s.content.UpdateCurso(ctx, c)
}

func (s *pgxCursoService) DeleteCurso(ctx context.Context, id int64, empresaID uuid.UUID) error {
	return s.content.SoftDeleteCurso(ctx, id, empresaID)
}

func (s *pgxCursoService) GetCursoContenido(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error) {
	return s.content.GetCursoContenido(ctx, cursoID)
}

func (s *pgxCursoService) CreateModulo(ctx context.Context, m *cursos.Modulo, empresaID uuid.UUID) error {
	m.EmpresaID = empresaID
	return s.catalog.CreateModulo(ctx, m)
}

func (s *pgxCursoService) ListModulos(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error) {
	return s.catalog.ListModulos(ctx, empresaID)
}

func (s *pgxCursoService) CreateUnidad(ctx context.Context, u *cursos.Unidad) error {
	return s.content.CreateUnidad(ctx, u)
}

func (s *pgxCursoService) UpdateUnidad(ctx context.Context, u *cursos.Unidad) error {
	return s.content.UpdateUnidad(ctx, u)
}

func (s *pgxCursoService) DeleteUnidad(ctx context.Context, id int64) error {
	return s.content.DeleteUnidad(ctx, id)
}

func (s *pgxCursoService) CreateLeccion(ctx context.Context, l *cursos.Leccion) error {
	return s.content.CreateLeccion(ctx, l)
}

func (s *pgxCursoService) UpdateLeccion(ctx context.Context, l *cursos.Leccion) error {
	return s.content.UpdateLeccion(ctx, l)
}

func (s *pgxCursoService) DeleteLeccion(ctx context.Context, id int64) error {
	return s.content.DeleteLeccion(ctx, id)
}
