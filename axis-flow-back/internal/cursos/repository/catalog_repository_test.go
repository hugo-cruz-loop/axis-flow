package repository_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/cursos"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CatalogRepositorier is the interface under test.
type CatalogRepositorier interface {
	CreateCategoria(ctx context.Context, c *cursos.Categoria) error
	ListCategorias(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error)
	UpdateCategoria(ctx context.Context, c *cursos.Categoria) error
	DeleteCategoria(ctx context.Context, id int64, empresaID uuid.UUID) error
	CreateModulo(ctx context.Context, m *cursos.Modulo) error
	ListModulos(ctx context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error)
}

// mockCatalogRepo is an in-memory implementation for unit testing.
type mockCatalogRepo struct {
	categorias map[int64]*cursos.Categoria
	modulos    map[int64]*cursos.Modulo
	nextCatID  int64
	nextModID  int64
}

func newMockCatalogRepo() *mockCatalogRepo {
	return &mockCatalogRepo{
		categorias: make(map[int64]*cursos.Categoria),
		modulos:    make(map[int64]*cursos.Modulo),
		nextCatID:  1,
		nextModID:  1,
	}
}

func (m *mockCatalogRepo) CreateCategoria(_ context.Context, c *cursos.Categoria) error {
	c.ID = m.nextCatID
	m.nextCatID++
	cp := *c
	m.categorias[c.ID] = &cp
	return nil
}

func (m *mockCatalogRepo) ListCategorias(_ context.Context, empresaID uuid.UUID) ([]*cursos.Categoria, error) {
	var out []*cursos.Categoria
	for _, c := range m.categorias {
		if c.EmpresaID == empresaID {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *mockCatalogRepo) UpdateCategoria(_ context.Context, c *cursos.Categoria) error {
	existing, ok := m.categorias[c.ID]
	if !ok || existing.EmpresaID != c.EmpresaID {
		return cursos.ErrCursoNotFound
	}
	cp := *c
	m.categorias[c.ID] = &cp
	return nil
}

func (m *mockCatalogRepo) DeleteCategoria(_ context.Context, id int64, empresaID uuid.UUID) error {
	existing, ok := m.categorias[id]
	if !ok || existing.EmpresaID != empresaID {
		return cursos.ErrCursoNotFound
	}
	delete(m.categorias, id)
	return nil
}

func (m *mockCatalogRepo) CreateModulo(_ context.Context, mod *cursos.Modulo) error {
	mod.ID = m.nextModID
	m.nextModID++
	cp := *mod
	m.modulos[mod.ID] = &cp
	return nil
}

func (m *mockCatalogRepo) ListModulos(_ context.Context, empresaID uuid.UUID) ([]*cursos.Modulo, error) {
	var out []*cursos.Modulo
	for _, mod := range m.modulos {
		if mod.EmpresaID == empresaID {
			cp := *mod
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ---- Tests ----

func TestCatalogRepo_CreateCategoria_AssignsID(t *testing.T) {
	repo := newMockCatalogRepo()
	empresaID := uuid.New()

	cat := &cursos.Categoria{
		Nombre:    "Tecnología",
		EmpresaID: empresaID,
	}
	err := repo.CreateCategoria(context.Background(), cat)
	require.NoError(t, err)
	assert.Greater(t, cat.ID, int64(0))
}

func TestCatalogRepo_ListCategorias_TenantIsolation(t *testing.T) {
	repo := newMockCatalogRepo()
	empresaA := uuid.New()
	empresaB := uuid.New()

	require.NoError(t, repo.CreateCategoria(context.Background(), &cursos.Categoria{Nombre: "A1", EmpresaID: empresaA}))
	require.NoError(t, repo.CreateCategoria(context.Background(), &cursos.Categoria{Nombre: "A2", EmpresaID: empresaA}))
	require.NoError(t, repo.CreateCategoria(context.Background(), &cursos.Categoria{Nombre: "B1", EmpresaID: empresaB}))

	list, err := repo.ListCategorias(context.Background(), empresaA)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	listB, err := repo.ListCategorias(context.Background(), empresaB)
	require.NoError(t, err)
	assert.Len(t, listB, 1)
}

func TestCatalogRepo_UpdateCategoria_WrongTenant_ReturnsNotFound(t *testing.T) {
	repo := newMockCatalogRepo()
	empresaA := uuid.New()
	empresaB := uuid.New()

	cat := &cursos.Categoria{Nombre: "Original", EmpresaID: empresaA}
	require.NoError(t, repo.CreateCategoria(context.Background(), cat))

	err := repo.UpdateCategoria(context.Background(), &cursos.Categoria{
		ID:        cat.ID,
		Nombre:    "Hijack",
		EmpresaID: empresaB, // wrong tenant
	})
	assert.ErrorIs(t, err, cursos.ErrCursoNotFound)
}

func TestCatalogRepo_DeleteCategoria_WrongTenant_ReturnsNotFound(t *testing.T) {
	repo := newMockCatalogRepo()
	empresaA := uuid.New()
	empresaB := uuid.New()

	cat := &cursos.Categoria{Nombre: "ToDelete", EmpresaID: empresaA}
	require.NoError(t, repo.CreateCategoria(context.Background(), cat))

	err := repo.DeleteCategoria(context.Background(), cat.ID, empresaB)
	assert.ErrorIs(t, err, cursos.ErrCursoNotFound)
}

func TestCatalogRepo_DeleteCategoria_HappyPath(t *testing.T) {
	repo := newMockCatalogRepo()
	empresaID := uuid.New()

	cat := &cursos.Categoria{Nombre: "ToDelete", EmpresaID: empresaID}
	require.NoError(t, repo.CreateCategoria(context.Background(), cat))
	require.NoError(t, repo.DeleteCategoria(context.Background(), cat.ID, empresaID))

	list, err := repo.ListCategorias(context.Background(), empresaID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestCatalogRepo_ListModulos_TenantIsolation(t *testing.T) {
	repo := newMockCatalogRepo()
	empresaA := uuid.New()
	empresaB := uuid.New()

	require.NoError(t, repo.CreateModulo(context.Background(), &cursos.Modulo{Nombre: "M1", EmpresaID: empresaA}))
	require.NoError(t, repo.CreateModulo(context.Background(), &cursos.Modulo{Nombre: "M2", EmpresaID: empresaB}))

	listA, err := repo.ListModulos(context.Background(), empresaA)
	require.NoError(t, err)
	assert.Len(t, listA, 1)
	assert.Equal(t, "M1", listA[0].Nombre)

	// Wrong tenant returns empty
	listNone, err := repo.ListModulos(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, listNone)
}
