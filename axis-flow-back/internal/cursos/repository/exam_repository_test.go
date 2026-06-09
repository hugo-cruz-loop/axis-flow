package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"axis-flow-back/internal/cursos"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExamRepositorier is the interface under test.
type ExamRepositorier interface {
	GetExamen(ctx context.Context, cursoID int64) (*cursos.Examen, error)
	GetExamenWithPreguntas(ctx context.Context, examenID int64) (*cursos.Examen, error)
	GetExamenWithPreguntasAdmin(ctx context.Context, examenID int64) (*cursos.Examen, error)
	GetResultados(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error)
	CountIntentos(ctx context.Context, examenID, empleadoID int64) (int, error)
	SaveResultado(ctx context.Context, r *cursos.ResultadoExamen) error
}

// ExamenWithOpciones augments Examen with preguntas for test inspection.
type ExamenWithOpciones struct {
	cursos.Examen
	Preguntas []ExamenPreguntaWithOpciones `json:"preguntas"`
}

type ExamenPreguntaWithOpciones struct {
	cursos.ExamenPregunta
	// Student view: use ExamenOpcionStudent (no es_correcta)
	OpcionesStudent []cursos.ExamenOpcionStudent `json:"opciones"`
	// Admin view: use ExamenOpcion (has es_correcta)
	OpcionesAdmin []cursos.ExamenOpcion `json:"opciones_admin,omitempty"`
}

// mockExamRepo is an in-memory implementation for unit testing.
type mockExamRepo struct {
	examenes   map[int64]*ExamenWithOpciones // keyed by examen.ID
	byCursoID  map[int64]int64               // cursoID → examenID
	resultados map[int64][]*cursos.ResultadoExamen
	nextExID   int64
	nextResID  int64
}

func newMockExamRepo() *mockExamRepo {
	return &mockExamRepo{
		examenes:   make(map[int64]*ExamenWithOpciones),
		byCursoID:  make(map[int64]int64),
		resultados: make(map[int64][]*cursos.ResultadoExamen),
		nextExID:   1,
		nextResID:  1,
	}
}

// seedExamen is a helper to seed an exam with preguntas and opciones for tests.
func (m *mockExamRepo) seedExamen(ex *ExamenWithOpciones) {
	ex.ID = m.nextExID
	m.nextExID++
	m.examenes[ex.ID] = ex
	m.byCursoID[ex.CursoID] = ex.ID
}

func (m *mockExamRepo) GetExamen(_ context.Context, cursoID int64) (*cursos.Examen, error) {
	exID, ok := m.byCursoID[cursoID]
	if !ok {
		return nil, cursos.ErrCursoNotFound
	}
	e := m.examenes[exID].Examen
	return &e, nil
}

func (m *mockExamRepo) GetExamenWithPreguntas(_ context.Context, examenID int64) (*cursos.Examen, error) {
	// Returns student-safe view: opciones use ExamenOpcionStudent (no es_correcta)
	ex, ok := m.examenes[examenID]
	if !ok {
		return nil, cursos.ErrCursoNotFound
	}
	e := ex.Examen
	return &e, nil
}

// getExamenWithPreguntasStudentView returns the full structure for anti-cheat assertion.
func (m *mockExamRepo) getExamenWithPreguntasStudentView(examenID int64) (*ExamenWithOpciones, error) {
	ex, ok := m.examenes[examenID]
	if !ok {
		return nil, cursos.ErrCursoNotFound
	}
	// Return a copy with only student-safe opciones populated
	result := ExamenWithOpciones{Examen: ex.Examen}
	for _, p := range ex.Preguntas {
		pCopy := ExamenPreguntaWithOpciones{ExamenPregunta: p.ExamenPregunta}
		pCopy.OpcionesStudent = p.OpcionesStudent
		// OpcionesAdmin intentionally not copied
		result.Preguntas = append(result.Preguntas, pCopy)
	}
	return &result, nil
}

func (m *mockExamRepo) GetExamenWithPreguntasAdmin(_ context.Context, examenID int64) (*cursos.Examen, error) {
	e, ok := m.examenes[examenID]
	if !ok {
		return nil, cursos.ErrCursoNotFound
	}
	ex := e.Examen
	return &ex, nil
}

func (m *mockExamRepo) GetResultados(_ context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error) {
	var out []*cursos.ResultadoExamen
	for _, r := range m.resultados[examenID] {
		if r.EmpleadoID == empleadoID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *mockExamRepo) CountIntentos(_ context.Context, examenID, empleadoID int64) (int, error) {
	count := 0
	for _, r := range m.resultados[examenID] {
		if r.EmpleadoID == empleadoID {
			count++
		}
	}
	return count, nil
}

func (m *mockExamRepo) SaveResultado(_ context.Context, r *cursos.ResultadoExamen) error {
	r.ID = m.nextResID
	m.nextResID++
	r.CreatedAt = time.Now()
	cp := *r
	m.resultados[r.ExamenID] = append(m.resultados[r.ExamenID], &cp)
	return nil
}

// ---- Tests ----

func TestExamRepo_GetExamen_HappyPath(t *testing.T) {
	repo := newMockExamRepo()
	repo.seedExamen(&ExamenWithOpciones{
		Examen: cursos.Examen{
			CursoID:     5,
			Titulo:      "Final Exam",
			NoteMin:     60,
			NumIntentos: 3,
		},
	})

	ex, err := repo.GetExamen(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, "Final Exam", ex.Titulo)
	assert.Equal(t, float64(60), ex.NoteMin)
}

func TestExamRepo_GetExamen_NotFound(t *testing.T) {
	repo := newMockExamRepo()
	_, err := repo.GetExamen(context.Background(), 999)
	assert.ErrorIs(t, err, cursos.ErrCursoNotFound)
}

func TestExamRepo_AntiCheat_StudentView_DoesNotExposeEsCorrecta(t *testing.T) {
	repo := newMockExamRepo()
	repo.seedExamen(&ExamenWithOpciones{
		Examen: cursos.Examen{CursoID: 1, Titulo: "Quiz", NoteMin: 70, NumIntentos: 2},
		Preguntas: []ExamenPreguntaWithOpciones{
			{
				ExamenPregunta: cursos.ExamenPregunta{ExamenID: 1, Enunciado: "2+2=?", Orden: 1},
				OpcionesStudent: []cursos.ExamenOpcionStudent{
					{ID: 1, PreguntaID: 1, Texto: "3"},
					{ID: 2, PreguntaID: 1, Texto: "4"},
				},
				OpcionesAdmin: []cursos.ExamenOpcion{
					{ID: 1, PreguntaID: 1, Texto: "3", EsCorrecta: false},
					{ID: 2, PreguntaID: 1, Texto: "4", EsCorrecta: true},
				},
			},
		},
	})

	studentView, err := repo.getExamenWithPreguntasStudentView(1)
	require.NoError(t, err)

	// Marshal to JSON and verify es_correcta is absent from student view
	b, err := json.Marshal(studentView)
	require.NoError(t, err)
	jsonStr := string(b)

	// ExamenOpcionStudent does NOT have es_correcta field
	assert.NotContains(t, jsonStr, "es_correcta",
		"ANTI-CHEAT: es_correcta must NOT appear in the student exam view JSON")

	// Admin view SHOULD have es_correcta
	for _, p := range studentView.Preguntas {
		assert.Empty(t, p.OpcionesAdmin, "student view must not contain admin opciones")
		assert.NotEmpty(t, p.OpcionesStudent, "student view must contain student opciones")
	}
}

func TestExamRepo_CountIntentos_ReturnsCorrectCount(t *testing.T) {
	repo := newMockExamRepo()
	cal := 85.0
	apr := true
	require.NoError(t, repo.SaveResultado(context.Background(), &cursos.ResultadoExamen{ExamenID: 1, EmpleadoID: 10, Calificacion: &cal, Aprobado: &apr, Intento: 1}))
	require.NoError(t, repo.SaveResultado(context.Background(), &cursos.ResultadoExamen{ExamenID: 1, EmpleadoID: 10, Calificacion: &cal, Aprobado: &apr, Intento: 2}))
	require.NoError(t, repo.SaveResultado(context.Background(), &cursos.ResultadoExamen{ExamenID: 1, EmpleadoID: 99, Calificacion: &cal, Aprobado: &apr, Intento: 1}))

	count, err := repo.CountIntentos(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Different employee
	count2, err := repo.CountIntentos(context.Background(), 1, 99)
	require.NoError(t, err)
	assert.Equal(t, 1, count2)
}

func TestExamRepo_SaveResultado_AssignsID(t *testing.T) {
	repo := newMockExamRepo()
	cal := 90.0
	apr := true
	r := &cursos.ResultadoExamen{ExamenID: 1, EmpleadoID: 5, Calificacion: &cal, Aprobado: &apr, Intento: 1}
	err := repo.SaveResultado(context.Background(), r)
	require.NoError(t, err)
	assert.Greater(t, r.ID, int64(0))
	assert.False(t, r.CreatedAt.IsZero())
}

func TestExamRepo_GetResultados_FiltersByEmpleado(t *testing.T) {
	repo := newMockExamRepo()
	cal := 75.0
	apr := false
	require.NoError(t, repo.SaveResultado(context.Background(), &cursos.ResultadoExamen{ExamenID: 2, EmpleadoID: 1, Calificacion: &cal, Aprobado: &apr, Intento: 1}))
	require.NoError(t, repo.SaveResultado(context.Background(), &cursos.ResultadoExamen{ExamenID: 2, EmpleadoID: 2, Calificacion: &cal, Aprobado: &apr, Intento: 1}))

	res, err := repo.GetResultados(context.Background(), 2, 1)
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, int64(1), res[0].EmpleadoID)
}
