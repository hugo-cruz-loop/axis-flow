package service

import (
	"context"
	"time"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/repository"
)

// ExamService defines business operations for exam resolution and result retrieval.
type ExamService interface {
	// GetExamenForStudent loads questions WITHOUT es_correcta (anti-cheat).
	GetExamenForStudent(ctx context.Context, cursoID int64) (*cursos.Examen, error)
	// ResolverExamen grades the exam for a given employee.
	ResolverExamen(ctx context.Context, empleadoID, examenID int64, respuestas map[int64]int64) (*cursos.ResultadoExamen, error)
	// GetResultados returns all attempt results for an employee on a given exam.
	GetResultados(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error)
}

type pgxExamService struct {
	exam repository.ExamRepository
}

// NewExamService constructs an ExamService.
func NewExamService(exam repository.ExamRepository) ExamService {
	return &pgxExamService{exam: exam}
}

func (s *pgxExamService) GetExamenForStudent(ctx context.Context, cursoID int64) (*cursos.Examen, error) {
	ex, err := s.exam.GetExamen(ctx, cursoID)
	if err != nil {
		return nil, err
	}
	// Load student-safe questions (no es_correcta).
	return s.exam.GetExamenWithPreguntas(ctx, ex.ID)
}

// ResolverExamen grades the employee's answers and persists the result.
//
// Algorithm:
//  1. Load exam metadata to check attempt limit.
//  2. Count existing attempts → if >= NumIntentos → ErrExamExceededAttempts.
//  3. Load full exam with es_correcta for grading.
//  4. For each pregunta, check if the chosen opcion ID has es_correcta=true.
//  5. calificacion = (correctas / total_preguntas) * 100.
//  6. aprobado = calificacion >= examen.NoteMin.
//  7. Save and return ResultadoExamen.
func (s *pgxExamService) ResolverExamen(ctx context.Context, empleadoID, examenID int64, respuestas map[int64]int64) (*cursos.ResultadoExamen, error) {
	// 1. Get exam metadata by exam ID (not curso_id).
	ex, err := s.exam.GetExamenByID(ctx, examenID)
	if err != nil {
		return nil, err
	}

	// 2. Count existing attempts.
	intentos, err := s.exam.CountIntentos(ctx, ex.ID, empleadoID)
	if err != nil {
		return nil, err
	}
	if intentos >= int(ex.NumIntentos) {
		return nil, cursos.ErrExamExceededAttempts
	}

	// 3. Load full exam with es_correcta (admin path).
	fullEx, err := s.exam.GetExamenWithPreguntasAdmin(ctx, ex.ID)
	if err != nil {
		return nil, err
	}

	// 4–5. Grade.
	var correctas int
	total := len(fullEx.Preguntas)
	for _, pregunta := range fullEx.Preguntas {
		chosenOpcionID, ok := respuestas[pregunta.ID]
		if !ok {
			continue
		}
		for _, opcion := range pregunta.Opciones {
			if opcion.ID == chosenOpcionID && opcion.EsCorrecta {
				correctas++
				break
			}
		}
	}

	var calificacion float64
	if total > 0 {
		calificacion = float64(correctas) / float64(total) * 100
	}

	// 6. Determine aprobado.
	aprobado := calificacion >= ex.NoteMin

	// 7. Save result.
	resultado := &cursos.ResultadoExamen{
		ExamenID:     ex.ID,
		EmpleadoID:   empleadoID,
		Calificacion: &calificacion,
		Aprobado:     &aprobado,
		Intento:      intentos + 1,
		CreatedAt:    time.Now(),
	}
	if err := s.exam.SaveResultado(ctx, resultado); err != nil {
		return nil, err
	}
	return resultado, nil
}

func (s *pgxExamService) GetResultados(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error) {
	return s.exam.GetResultados(ctx, examenID, empleadoID)
}
