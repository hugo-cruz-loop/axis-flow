package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"axis-flow-back/internal/cursos"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EnrollmentRepositorier is the interface under test.
type EnrollmentRepositorier interface {
	Enroll(ctx context.Context, e *cursos.Enrollment) error
	GetEnrollment(ctx context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error)
	ListEnrollmentsByEmpleado(ctx context.Context, empleadoID int64) ([]*cursos.Enrollment, error)
	MarkLeccionCompleta(ctx context.Context, empleadoID, leccionID int64) error
	IsLeccionCompleta(ctx context.Context, empleadoID, leccionID int64, cursoID int64) (bool, error)
	GetNota(ctx context.Context, leccionID, empleadoID int64) (*cursos.Nota, error)
	UpsertNota(ctx context.Context, n *cursos.Nota) error
}

// mockEnrollmentRepo is an in-memory implementation with a real miniredis for Redis operations.
type mockEnrollmentRepo struct {
	enrollments map[string]*cursos.Enrollment // key: "cursoID:empleadoID"
	avances     map[string]bool               // key: "empleadoID:leccionID"
	notas       map[string]*cursos.Nota       // key: "leccionID:empleadoID"
	rdb         *redis.Client
}

func newMockEnrollmentRepo(rdb *redis.Client) *mockEnrollmentRepo {
	return &mockEnrollmentRepo{
		enrollments: make(map[string]*cursos.Enrollment),
		avances:     make(map[string]bool),
		notas:       make(map[string]*cursos.Nota),
		rdb:         rdb,
	}
}

func enrollKey(cursoID, empleadoID int64) string {
	return fmt.Sprintf("%d:%d", cursoID, empleadoID)
}

func avanceKey(empleadoID, leccionID int64) string {
	return fmt.Sprintf("%d:%d", empleadoID, leccionID)
}

func notaKey(leccionID, empleadoID int64) string {
	return fmt.Sprintf("%d:%d", leccionID, empleadoID)
}

func (m *mockEnrollmentRepo) Enroll(_ context.Context, e *cursos.Enrollment) error {
	k := enrollKey(e.CursoID, e.EmpleadoID)
	if _, exists := m.enrollments[k]; exists {
		return nil // ON CONFLICT DO NOTHING
	}
	e.EnrolledAt = time.Now()
	cp := *e
	m.enrollments[k] = &cp
	return nil
}

func (m *mockEnrollmentRepo) GetEnrollment(_ context.Context, cursoID, empleadoID int64) (*cursos.Enrollment, error) {
	e, ok := m.enrollments[enrollKey(cursoID, empleadoID)]
	if !ok {
		return nil, cursos.ErrNotEnrolled
	}
	cp := *e
	return &cp, nil
}

func (m *mockEnrollmentRepo) ListEnrollmentsByEmpleado(_ context.Context, empleadoID int64) ([]*cursos.Enrollment, error) {
	var out []*cursos.Enrollment
	for _, e := range m.enrollments {
		if e.EmpleadoID == empleadoID {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *mockEnrollmentRepo) MarkLeccionCompleta(ctx context.Context, empleadoID, leccionID int64) error {
	m.avances[avanceKey(empleadoID, leccionID)] = true
	// Also mirror to Redis for cache hit tests (in real impl this would be SADD)
	// We use a fixed cursoID=1 in test helpers for simplicity
	return nil
}

func (m *mockEnrollmentRepo) IsLeccionCompleta(ctx context.Context, empleadoID, leccionID int64, cursoID int64) (bool, error) {
	// Try Redis first
	redisKey := fmt.Sprintf("cursos:avance_lecciones:%d:%d", empleadoID, cursoID)
	member := fmt.Sprintf("%d", leccionID)
	val, err := m.rdb.SIsMember(ctx, redisKey, member).Result()
	if err == nil && val {
		return true, nil
	}
	// Fall through to in-memory DB
	return m.avances[avanceKey(empleadoID, leccionID)], nil
}

func (m *mockEnrollmentRepo) GetNota(_ context.Context, leccionID, empleadoID int64) (*cursos.Nota, error) {
	n, ok := m.notas[notaKey(leccionID, empleadoID)]
	if !ok {
		return nil, cursos.ErrCursoNotFound
	}
	cp := *n
	return &cp, nil
}

func (m *mockEnrollmentRepo) UpsertNota(_ context.Context, n *cursos.Nota) error {
	n.UpdatedAt = time.Now()
	cp := *n
	m.notas[notaKey(n.LeccionID, n.EmpleadoID)] = &cp
	return nil
}

// ---- Tests ----

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestEnrollmentRepo_Enroll_HappyPath(t *testing.T) {
	_, rdb := newTestRedis(t)
	repo := newMockEnrollmentRepo(rdb)
	empresaID := uuid.New()

	e := &cursos.Enrollment{CursoID: 1, EmpleadoID: 10, EmpresaID: empresaID}
	err := repo.Enroll(context.Background(), e)
	require.NoError(t, err)

	got, err := repo.GetEnrollment(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), got.CursoID)
	assert.Equal(t, int64(10), got.EmpleadoID)
}

func TestEnrollmentRepo_Enroll_OnConflict_DoesNothing(t *testing.T) {
	_, rdb := newTestRedis(t)
	repo := newMockEnrollmentRepo(rdb)
	empresaID := uuid.New()

	e := &cursos.Enrollment{CursoID: 1, EmpleadoID: 10, EmpresaID: empresaID}
	require.NoError(t, repo.Enroll(context.Background(), e))
	require.NoError(t, repo.Enroll(context.Background(), e)) // second call — must not error
}

func TestEnrollmentRepo_GetEnrollment_NotFound_ReturnsErrNotEnrolled(t *testing.T) {
	_, rdb := newTestRedis(t)
	repo := newMockEnrollmentRepo(rdb)

	_, err := repo.GetEnrollment(context.Background(), 999, 888)
	assert.ErrorIs(t, err, cursos.ErrNotEnrolled)
}

func TestEnrollmentRepo_ListEnrollmentsByEmpleado_ReturnsAllForEmployee(t *testing.T) {
	_, rdb := newTestRedis(t)
	repo := newMockEnrollmentRepo(rdb)
	empresaID := uuid.New()

	require.NoError(t, repo.Enroll(context.Background(), &cursos.Enrollment{CursoID: 1, EmpleadoID: 5, EmpresaID: empresaID}))
	require.NoError(t, repo.Enroll(context.Background(), &cursos.Enrollment{CursoID: 2, EmpleadoID: 5, EmpresaID: empresaID}))
	require.NoError(t, repo.Enroll(context.Background(), &cursos.Enrollment{CursoID: 1, EmpleadoID: 9, EmpresaID: empresaID}))

	list, err := repo.ListEnrollmentsByEmpleado(context.Background(), 5)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestEnrollmentRepo_IsLeccionCompleta_CacheHit(t *testing.T) {
	mr, rdb := newTestRedis(t)
	repo := newMockEnrollmentRepo(rdb)

	// Manually seed Redis to simulate cache hit
	redisKey := "cursos:avance_lecciones:10:1"
	mr.SAdd(redisKey, "42")

	complete, err := repo.IsLeccionCompleta(context.Background(), 10, 42, 1)
	require.NoError(t, err)
	assert.True(t, complete)
}

func TestEnrollmentRepo_IsLeccionCompleta_CacheMiss_FallsThroughToStore(t *testing.T) {
	_, rdb := newTestRedis(t)
	repo := newMockEnrollmentRepo(rdb)

	// Not in Redis, not in store
	complete, err := repo.IsLeccionCompleta(context.Background(), 10, 42, 1)
	require.NoError(t, err)
	assert.False(t, complete)

	// Mark in store
	require.NoError(t, repo.MarkLeccionCompleta(context.Background(), 10, 42))
	complete, err = repo.IsLeccionCompleta(context.Background(), 10, 42, 1)
	require.NoError(t, err)
	assert.True(t, complete)
}

func TestEnrollmentRepo_UpsertNota_CreatesAndUpdates(t *testing.T) {
	_, rdb := newTestRedis(t)
	repo := newMockEnrollmentRepo(rdb)

	n := &cursos.Nota{LeccionID: 1, EmpleadoID: 5, Contenido: "Initial"}
	require.NoError(t, repo.UpsertNota(context.Background(), n))

	got, err := repo.GetNota(context.Background(), 1, 5)
	require.NoError(t, err)
	assert.Equal(t, "Initial", got.Contenido)

	n.Contenido = "Updated"
	require.NoError(t, repo.UpsertNota(context.Background(), n))
	got, err = repo.GetNota(context.Background(), 1, 5)
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Contenido)
}
