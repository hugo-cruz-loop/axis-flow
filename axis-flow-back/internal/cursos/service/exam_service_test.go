package service_test

import (
	"context"
	"errors"
	"testing"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/service"
)

// --- mock ExamRepository ---

type mockExamRepo struct {
	getExamen                   func(ctx context.Context, cursoID int64) (*cursos.Examen, error)
	getExamenByID               func(ctx context.Context, id int64) (*cursos.Examen, error)
	getExamenWithPreguntas      func(ctx context.Context, examenID int64) (*cursos.Examen, error)
	getExamenWithPreguntasAdmin func(ctx context.Context, examenID int64) (*cursos.Examen, error)
	getResultados               func(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error)
	countIntentos               func(ctx context.Context, examenID, empleadoID int64) (int, error)
	saveResultado               func(ctx context.Context, r *cursos.ResultadoExamen) error
}

func (m *mockExamRepo) GetExamen(ctx context.Context, cursoID int64) (*cursos.Examen, error) {
	if m.getExamen != nil {
		return m.getExamen(ctx, cursoID)
	}
	return nil, cursos.ErrCursoNotFound
}
func (m *mockExamRepo) GetExamenByID(ctx context.Context, id int64) (*cursos.Examen, error) {
	if m.getExamenByID != nil {
		return m.getExamenByID(ctx, id)
	}
	return nil, cursos.ErrCursoNotFound
}
func (m *mockExamRepo) GetExamenWithPreguntas(ctx context.Context, examenID int64) (*cursos.Examen, error) {
	if m.getExamenWithPreguntas != nil {
		return m.getExamenWithPreguntas(ctx, examenID)
	}
	return nil, nil
}
func (m *mockExamRepo) GetExamenWithPreguntasAdmin(ctx context.Context, examenID int64) (*cursos.Examen, error) {
	if m.getExamenWithPreguntasAdmin != nil {
		return m.getExamenWithPreguntasAdmin(ctx, examenID)
	}
	return nil, nil
}
func (m *mockExamRepo) GetResultados(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error) {
	if m.getResultados != nil {
		return m.getResultados(ctx, examenID, empleadoID)
	}
	return nil, nil
}
func (m *mockExamRepo) CountIntentos(ctx context.Context, examenID, empleadoID int64) (int, error) {
	if m.countIntentos != nil {
		return m.countIntentos(ctx, examenID, empleadoID)
	}
	return 0, nil
}
func (m *mockExamRepo) SaveResultado(ctx context.Context, r *cursos.ResultadoExamen) error {
	if m.saveResultado != nil {
		return m.saveResultado(ctx, r)
	}
	return nil
}

// --- helpers ---

func buildExamenWithPreguntas(numIntentos int16, noteMin float64, opts []bool) *cursos.Examen {
	ex := &cursos.Examen{
		ID:          1,
		CursoID:     10,
		Titulo:      "Test Exam",
		NoteMin:     noteMin,
		NumIntentos: numIntentos,
	}
	_ = opts // preguntas/opciones attached via ExamenPregunta slice — not on Examen struct directly
	return ex
}

// buildFullExam returns an Examen whose Preguntas are already populated inline
// for grading use (admin path). We reuse the same domain types with added opciones.
func buildFullExamAdmin(numIntentos int16, noteMin float64, answers map[int64]bool) *cursos.Examen {
	ex := &cursos.Examen{
		ID:          1,
		CursoID:     10,
		Titulo:      "Test Exam",
		NoteMin:     noteMin,
		NumIntentos: numIntentos,
		Preguntas:   make([]*cursos.ExamenPreguntaConOpciones, 0, len(answers)),
	}
	var i int64 = 1
	for opcionID, correct := range answers {
		pregunta := &cursos.ExamenPreguntaConOpciones{
			ExamenPregunta: cursos.ExamenPregunta{
				ID:       i,
				ExamenID: 1,
			},
			Opciones: []*cursos.ExamenOpcion{
				{ID: opcionID, PreguntaID: i, Texto: "opt", EsCorrecta: correct},
			},
		}
		ex.Preguntas = append(ex.Preguntas, pregunta)
		i++
	}
	return ex
}

// --- Tests ---

func TestResolverExamen_ExceededAttempts(t *testing.T) {
	examRepo := &mockExamRepo{
		getExamenByID: func(_ context.Context, id int64) (*cursos.Examen, error) {
			ex := buildExamenWithPreguntas(2, 70, nil)
			return ex, nil
		},
		countIntentos: func(_ context.Context, examenID, empleadoID int64) (int, error) {
			return 2, nil // already at max
		},
	}

	svc := service.NewExamService(examRepo)
	_, err := svc.ResolverExamen(context.Background(), 99, 1, map[int64]int64{})
	if !errors.Is(err, cursos.ErrExamExceededAttempts) {
		t.Fatalf("expected ErrExamExceededAttempts, got %v", err)
	}
}

func TestResolverExamen_AllCorrect_Aprobado(t *testing.T) {
	// Build a deterministic exam with 3 preguntas.
	// preguntaID=1 → correct opcionID=10
	// preguntaID=2 → correct opcionID=20
	// preguntaID=3 → correct opcionID=30
	fullExam := &cursos.Examen{
		ID: 1, CursoID: 10, NumIntentos: 3, NoteMin: 70,
		Preguntas: []*cursos.ExamenPreguntaConOpciones{
			{
				ExamenPregunta: cursos.ExamenPregunta{ID: 1, ExamenID: 1},
				Opciones: []*cursos.ExamenOpcion{{ID: 10, PreguntaID: 1, EsCorrecta: true}},
			},
			{
				ExamenPregunta: cursos.ExamenPregunta{ID: 2, ExamenID: 1},
				Opciones: []*cursos.ExamenOpcion{{ID: 20, PreguntaID: 2, EsCorrecta: true}},
			},
			{
				ExamenPregunta: cursos.ExamenPregunta{ID: 3, ExamenID: 1},
				Opciones: []*cursos.ExamenOpcion{{ID: 30, PreguntaID: 3, EsCorrecta: true}},
			},
		},
	}

	examRepo := &mockExamRepo{
		getExamenByID: func(_ context.Context, id int64) (*cursos.Examen, error) {
			return &cursos.Examen{
				ID: 1, CursoID: 10, NumIntentos: 3, NoteMin: 70,
			}, nil
		},
		countIntentos: func(_ context.Context, _, _ int64) (int, error) {
			return 0, nil
		},
		getExamenWithPreguntasAdmin: func(_ context.Context, _ int64) (*cursos.Examen, error) {
			return fullExam, nil
		},
		saveResultado: func(_ context.Context, r *cursos.ResultadoExamen) error {
			r.ID = 42
			return nil
		},
	}

	// respuestas: preguntaID → opcionID chosen (all correct options)
	respuestas := map[int64]int64{1: 10, 2: 20, 3: 30}

	svc := service.NewExamService(examRepo)
	result, err := svc.ResolverExamen(context.Background(), 99, 1, respuestas)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Aprobado == nil || !*result.Aprobado {
		t.Fatal("expected aprobado=true")
	}
	if result.Calificacion == nil || *result.Calificacion != 100.0 {
		t.Fatalf("expected calificacion=100, got %v", result.Calificacion)
	}
}

func TestResolverExamen_NoneCorrect_NotAprobado(t *testing.T) {
	// preguntaID=1 → correct opcionID=10; preguntaID=2 → correct opcionID=20
	// Employee will pick opcionID=99 (wrong) for both
	fullExam := &cursos.Examen{
		ID: 1, CursoID: 10, NumIntentos: 3, NoteMin: 70,
		Preguntas: []*cursos.ExamenPreguntaConOpciones{
			{
				ExamenPregunta: cursos.ExamenPregunta{ID: 1, ExamenID: 1},
				Opciones: []*cursos.ExamenOpcion{{ID: 10, PreguntaID: 1, EsCorrecta: true}},
			},
			{
				ExamenPregunta: cursos.ExamenPregunta{ID: 2, ExamenID: 1},
				Opciones: []*cursos.ExamenOpcion{{ID: 20, PreguntaID: 2, EsCorrecta: true}},
			},
		},
	}

	examRepo := &mockExamRepo{
		getExamenByID: func(_ context.Context, id int64) (*cursos.Examen, error) {
			return &cursos.Examen{
				ID: 1, CursoID: 10, NumIntentos: 3, NoteMin: 70,
			}, nil
		},
		countIntentos: func(_ context.Context, _, _ int64) (int, error) { return 0, nil },
		getExamenWithPreguntasAdmin: func(_ context.Context, _ int64) (*cursos.Examen, error) {
			return fullExam, nil
		},
		saveResultado: func(_ context.Context, r *cursos.ResultadoExamen) error { return nil },
	}

	// Employee chose option 99 for each pregunta (wrong — correct ids are 10, 20)
	respuestas := map[int64]int64{1: 99, 2: 99}

	svc := service.NewExamService(examRepo)
	result, err := svc.ResolverExamen(context.Background(), 99, 1, respuestas)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Aprobado == nil || *result.Aprobado {
		t.Fatal("expected aprobado=false")
	}
	if result.Calificacion == nil || *result.Calificacion != 0.0 {
		t.Fatalf("expected calificacion=0, got %v", result.Calificacion)
	}
}
