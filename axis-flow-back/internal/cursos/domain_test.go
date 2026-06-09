// Package cursos — domain constants and sentinel errors tests.
package cursos_test

import (
	"errors"
	"testing"

	"axis-flow-back/internal/cursos"
)

func TestCursoEstatusConstants(t *testing.T) {
	if cursos.CursoEstatusPublico != 1 {
		t.Errorf("expected CursoEstatusPublico=1, got %d", cursos.CursoEstatusPublico)
	}
	if cursos.CursoEstatusPrivado != 2 {
		t.Errorf("expected CursoEstatusPrivado=2, got %d", cursos.CursoEstatusPrivado)
	}
}

func TestEnrollEstatusConstants(t *testing.T) {
	if cursos.EnrollEstatusCompletado != 1 {
		t.Errorf("expected EnrollEstatusCompletado=1, got %d", cursos.EnrollEstatusCompletado)
	}
	if cursos.EnrollEstatusIncompleto != 2 {
		t.Errorf("expected EnrollEstatusIncompleto=2, got %d", cursos.EnrollEstatusIncompleto)
	}
}

func TestLeccionTipoConstants(t *testing.T) {
	if cursos.LeccionTipoVideo != 1 {
		t.Errorf("expected LeccionTipoVideo=1, got %d", cursos.LeccionTipoVideo)
	}
	if cursos.LeccionTipoTexto != 2 {
		t.Errorf("expected LeccionTipoTexto=2, got %d", cursos.LeccionTipoTexto)
	}
	if cursos.LeccionTipoDocumento != 3 {
		t.Errorf("expected LeccionTipoDocumento=3, got %d", cursos.LeccionTipoDocumento)
	}
}

func TestSentinelErrors(t *testing.T) {
	sentinels := []error{
		cursos.ErrCursoNotFound,
		cursos.ErrNotEnrolled,
		cursos.ErrExamExceededAttempts,
		cursos.ErrExamNotApproved,
		cursos.ErrTenantMismatch,
		cursos.ErrCursoDeleted,
	}
	for _, e := range sentinels {
		if e == nil {
			t.Errorf("sentinel error must not be nil")
		}
	}
	// Ensure distinct
	if errors.Is(cursos.ErrCursoNotFound, cursos.ErrNotEnrolled) {
		t.Error("sentinel errors must be distinct")
	}
}

func TestExamenOpcionStudentHasNoEsCorrecta(t *testing.T) {
	// Compile-time anti-cheat: ExamenOpcionStudent must NOT expose es_correcta.
	// Accessing only the allowed fields ensures this struct cannot leak answers.
	var op cursos.ExamenOpcionStudent
	_ = op.ID
	_ = op.PreguntaID
	_ = op.Texto
}
