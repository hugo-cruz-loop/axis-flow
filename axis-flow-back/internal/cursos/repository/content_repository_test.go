package repository_test

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/cursos"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ContentRepositorier is the interface under test.
type ContentRepositorier interface {
	CreateCurso(ctx context.Context, c *cursos.Curso) error
	GetCurso(ctx context.Context, id int64) (*cursos.Curso, error)
	ListCursos(ctx context.Context, empresaID uuid.UUID, includePrivate bool) ([]*cursos.Curso, error)
	UpdateCurso(ctx context.Context, c *cursos.Curso) error
	SoftDeleteCurso(ctx context.Context, id int64, empresaID uuid.UUID) error
	CreateUnidad(ctx context.Context, u *cursos.Unidad) error
	GetUnidadesByCurso(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)
	UpdateUnidad(ctx context.Context, u *cursos.Unidad) error
	DeleteUnidad(ctx context.Context, id int64) error
	CreateLeccion(ctx context.Context, l *cursos.Leccion) error
	GetLeccionesByUnidad(ctx context.Context, unidadID int64) ([]*cursos.Leccion, error)
	UpdateLeccion(ctx context.Context, l *cursos.Leccion) error
	DeleteLeccion(ctx context.Context, id int64) error
	GetCursoContenido(ctx context.Context, cursoID int64) ([]*cursos.Unidad, error)
}

// mockContentRepo is an in-memory implementation for unit testing.
type mockContentRepo struct {
	cursos        map[int64]*cursos.Curso
	unidades      map[int64]*cursos.Unidad
	lecciones     map[int64]*cursos.Leccion
	nextCursoID   int64
	nextUnidadID  int64
	nextLeccionID int64
}

func newMockContentRepo() *mockContentRepo {
	return &mockContentRepo{
		cursos:        make(map[int64]*cursos.Curso),
		unidades:      make(map[int64]*cursos.Unidad),
		lecciones:     make(map[int64]*cursos.Leccion),
		nextCursoID:   1,
		nextUnidadID:  1,
		nextLeccionID: 1,
	}
}

func (m *mockContentRepo) CreateCurso(_ context.Context, c *cursos.Curso) error {
	c.ID = m.nextCursoID
	m.nextCursoID++
	now := time.Now()
	c.CreatedAt = now
	cp := *c
	m.cursos[c.ID] = &cp
	return nil
}

func (m *mockContentRepo) GetCurso(_ context.Context, id int64) (*cursos.Curso, error) {
	c, ok := m.cursos[id]
	if !ok {
		return nil, cursos.ErrCursoNotFound
	}
	if c.DeletedAt != nil {
		return nil, cursos.ErrCursoDeleted
	}
	cp := *c
	return &cp, nil
}

func (m *mockContentRepo) ListCursos(_ context.Context, empresaID uuid.UUID, includePrivate bool) ([]*cursos.Curso, error) {
	var out []*cursos.Curso
	for _, c := range m.cursos {
		if c.EmpresaID != empresaID {
			continue
		}
		if c.DeletedAt != nil {
			continue
		}
		if !includePrivate && c.Estatus != int16(cursos.CursoEstatusPublico) {
			continue
		}
		cp := *c
		out = append(out, &cp)
	}
	return out, nil
}

func (m *mockContentRepo) UpdateCurso(_ context.Context, c *cursos.Curso) error {
	existing, ok := m.cursos[c.ID]
	if !ok {
		return cursos.ErrCursoNotFound
	}
	if existing.EmpresaID != c.EmpresaID {
		return cursos.ErrTenantMismatch
	}
	cp := *c
	m.cursos[c.ID] = &cp
	return nil
}

func (m *mockContentRepo) SoftDeleteCurso(_ context.Context, id int64, empresaID uuid.UUID) error {
	c, ok := m.cursos[id]
	if !ok {
		return cursos.ErrCursoNotFound
	}
	if c.EmpresaID != empresaID {
		return cursos.ErrTenantMismatch
	}
	now := time.Now()
	c.DeletedAt = &now
	return nil
}

func (m *mockContentRepo) CreateUnidad(_ context.Context, u *cursos.Unidad) error {
	u.ID = m.nextUnidadID
	m.nextUnidadID++
	cp := *u
	m.unidades[u.ID] = &cp
	return nil
}

func (m *mockContentRepo) GetUnidadesByCurso(_ context.Context, cursoID int64) ([]*cursos.Unidad, error) {
	var out []*cursos.Unidad
	for _, u := range m.unidades {
		if u.CursoID == cursoID {
			cp := *u
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *mockContentRepo) UpdateUnidad(_ context.Context, u *cursos.Unidad) error {
	if _, ok := m.unidades[u.ID]; !ok {
		return cursos.ErrCursoNotFound
	}
	cp := *u
	m.unidades[u.ID] = &cp
	return nil
}

func (m *mockContentRepo) DeleteUnidad(_ context.Context, id int64) error {
	if _, ok := m.unidades[id]; !ok {
		return cursos.ErrCursoNotFound
	}
	delete(m.unidades, id)
	return nil
}

func (m *mockContentRepo) CreateLeccion(_ context.Context, l *cursos.Leccion) error {
	l.ID = m.nextLeccionID
	m.nextLeccionID++
	cp := *l
	m.lecciones[l.ID] = &cp
	return nil
}

func (m *mockContentRepo) GetLeccionesByUnidad(_ context.Context, unidadID int64) ([]*cursos.Leccion, error) {
	var out []*cursos.Leccion
	for _, l := range m.lecciones {
		if l.UnidadID == unidadID {
			cp := *l
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *mockContentRepo) UpdateLeccion(_ context.Context, l *cursos.Leccion) error {
	if _, ok := m.lecciones[l.ID]; !ok {
		return cursos.ErrCursoNotFound
	}
	cp := *l
	m.lecciones[l.ID] = &cp
	return nil
}

func (m *mockContentRepo) DeleteLeccion(_ context.Context, id int64) error {
	if _, ok := m.lecciones[id]; !ok {
		return cursos.ErrCursoNotFound
	}
	delete(m.lecciones, id)
	return nil
}

func (m *mockContentRepo) GetCursoContenido(_ context.Context, cursoID int64) ([]*cursos.Unidad, error) {
	return m.GetUnidadesByCurso(context.Background(), cursoID)
}

// ---- Tests ----

func TestContentRepo_GetCurso_ReturnsErrCursoDeleted_WhenSoftDeleted(t *testing.T) {
	repo := newMockContentRepo()
	empresaID := uuid.New()

	c := &cursos.Curso{Titulo: "Deleted", EmpresaID: empresaID, Estatus: int16(cursos.CursoEstatusPublico)}
	require.NoError(t, repo.CreateCurso(context.Background(), c))
	require.NoError(t, repo.SoftDeleteCurso(context.Background(), c.ID, empresaID))

	_, err := repo.GetCurso(context.Background(), c.ID)
	assert.ErrorIs(t, err, cursos.ErrCursoDeleted)
}

func TestContentRepo_GetCurso_ReturnsErrNotFound_WhenMissing(t *testing.T) {
	repo := newMockContentRepo()
	_, err := repo.GetCurso(context.Background(), 9999)
	assert.ErrorIs(t, err, cursos.ErrCursoNotFound)
}

func TestContentRepo_ListCursos_VisibilityFilter_ExcludesPrivateWhenNotIncluded(t *testing.T) {
	repo := newMockContentRepo()
	empresaID := uuid.New()

	public := &cursos.Curso{Titulo: "Public", EmpresaID: empresaID, Estatus: int16(cursos.CursoEstatusPublico)}
	private := &cursos.Curso{Titulo: "Private", EmpresaID: empresaID, Estatus: int16(cursos.CursoEstatusPrivado)}
	require.NoError(t, repo.CreateCurso(context.Background(), public))
	require.NoError(t, repo.CreateCurso(context.Background(), private))

	list, err := repo.ListCursos(context.Background(), empresaID, false)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "Public", list[0].Titulo)
}

func TestContentRepo_ListCursos_IncludePrivate_ReturnsBoth(t *testing.T) {
	repo := newMockContentRepo()
	empresaID := uuid.New()

	require.NoError(t, repo.CreateCurso(context.Background(), &cursos.Curso{Titulo: "Public", EmpresaID: empresaID, Estatus: int16(cursos.CursoEstatusPublico)}))
	require.NoError(t, repo.CreateCurso(context.Background(), &cursos.Curso{Titulo: "Private", EmpresaID: empresaID, Estatus: int16(cursos.CursoEstatusPrivado)}))

	list, err := repo.ListCursos(context.Background(), empresaID, true)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestContentRepo_ListCursos_TenantIsolation(t *testing.T) {
	repo := newMockContentRepo()
	empresaA := uuid.New()
	empresaB := uuid.New()

	require.NoError(t, repo.CreateCurso(context.Background(), &cursos.Curso{Titulo: "A", EmpresaID: empresaA, Estatus: int16(cursos.CursoEstatusPublico)}))
	require.NoError(t, repo.CreateCurso(context.Background(), &cursos.Curso{Titulo: "B", EmpresaID: empresaB, Estatus: int16(cursos.CursoEstatusPublico)}))

	list, err := repo.ListCursos(context.Background(), empresaA, true)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "A", list[0].Titulo)
}

func TestContentRepo_SoftDeleteCurso_WrongTenant_ReturnsMismatch(t *testing.T) {
	repo := newMockContentRepo()
	empresaA := uuid.New()
	empresaB := uuid.New()

	c := &cursos.Curso{Titulo: "X", EmpresaID: empresaA, Estatus: int16(cursos.CursoEstatusPublico)}
	require.NoError(t, repo.CreateCurso(context.Background(), c))

	err := repo.SoftDeleteCurso(context.Background(), c.ID, empresaB)
	assert.ErrorIs(t, err, cursos.ErrTenantMismatch)
}

func TestContentRepo_UnidadAndLeccion_CRUD(t *testing.T) {
	repo := newMockContentRepo()
	empresaID := uuid.New()

	c := &cursos.Curso{Titulo: "C", EmpresaID: empresaID, Estatus: int16(cursos.CursoEstatusPublico)}
	require.NoError(t, repo.CreateCurso(context.Background(), c))

	u := &cursos.Unidad{CursoID: c.ID, Titulo: "Unidad 1", Orden: 1}
	require.NoError(t, repo.CreateUnidad(context.Background(), u))
	assert.Greater(t, u.ID, int64(0))

	l := &cursos.Leccion{UnidadID: u.ID, Titulo: "Intro", Tipo: int16(cursos.LeccionTipoVideo), Orden: 1}
	require.NoError(t, repo.CreateLeccion(context.Background(), l))
	assert.Greater(t, l.ID, int64(0))

	lecciones, err := repo.GetLeccionesByUnidad(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Len(t, lecciones, 1)

	require.NoError(t, repo.DeleteLeccion(context.Background(), l.ID))
	lecciones, err = repo.GetLeccionesByUnidad(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Empty(t, lecciones)
}
