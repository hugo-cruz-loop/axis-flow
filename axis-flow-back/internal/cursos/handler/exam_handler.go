package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"axis-flow-back/internal/cursos"
)

// ExamServicer is the interface ExamHandler depends on.
type ExamServicer interface {
	GetExamenForStudent(ctx context.Context, cursoID int64) (*cursos.Examen, error)
	ResolverExamen(ctx context.Context, empleadoID, examenID int64, respuestas map[int64]int64) (*cursos.ResultadoExamen, error)
	GetResultados(ctx context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error)
}

// ExamHandler handles HTTP requests for exam operations.
type ExamHandler struct {
	svc ExamServicer
}

// NewExamHandler constructs an ExamHandler.
func NewExamHandler(svc ExamServicer) *ExamHandler {
	return &ExamHandler{svc: svc}
}

// studentExamen is the safe API view of an Examen — Preguntas field uses
// ExamenOpcionStudent (no es_correcta). This prevents answer leakage.
type studentPreguntas struct {
	ID        int64                  `json:"id"`
	ExamenID  int64                  `json:"examen_id"`
	Enunciado string                 `json:"enunciado"`
	Orden     int                    `json:"orden"`
	Opciones  []*cursos.ExamenOpcionStudent `json:"opciones,omitempty"`
}

type studentExamenView struct {
	ID              int64               `json:"id"`
	CursoID         int64               `json:"curso_id"`
	Titulo          string              `json:"titulo"`
	NoteMin         float64             `json:"note_min"`
	NumIntentos     int16               `json:"num_intentos"`
	TiempoLimiteMin *int                `json:"tiempo_limite_min,omitempty"`
	Preguntas       []*studentPreguntas `json:"preguntas,omitempty"`
}

// toStudentView converts an Examen to the student-safe view (no es_correcta).
func toStudentView(ex *cursos.Examen) *studentExamenView {
	v := &studentExamenView{
		ID:              ex.ID,
		CursoID:         ex.CursoID,
		Titulo:          ex.Titulo,
		NoteMin:         ex.NoteMin,
		NumIntentos:     ex.NumIntentos,
		TiempoLimiteMin: ex.TiempoLimiteMin,
	}
	for _, p := range ex.Preguntas {
		sp := &studentPreguntas{
			ID:        p.ID,
			ExamenID:  p.ExamenID,
			Enunciado: p.Enunciado,
			Orden:     p.Orden,
		}
		for _, o := range p.Opciones {
			sp.Opciones = append(sp.Opciones, &cursos.ExamenOpcionStudent{
				ID:         o.ID,
				PreguntaID: o.PreguntaID,
				Texto:      o.Texto,
			})
		}
		v.Preguntas = append(v.Preguntas, sp)
	}
	return v
}

// GetExamenForStudent handles GET /examen/{curso_id}
// ANTI-CHEAT: response uses ExamenOpcionStudent (no es_correcta).
func (h *ExamHandler) GetExamenForStudent(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	cursoID, ok := parsePathInt64(w, r, "curso_id")
	if !ok {
		return
	}

	ex, err := h.svc.GetExamenForStudent(r.Context(), cursoID)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": toStudentView(ex)})
}

// ResolverExamen handles POST /examen/resolver
// Body: {examen_id, empleado_id, respuestas: [{pregunta_id, opcion_id}]}
func (h *ExamHandler) ResolverExamen(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}

	var body struct {
		ExamenID   int64 `json:"examen_id"`
		EmpleadoID int64 `json:"empleado_id"`
		Respuestas []struct {
			PreguntaID int64 `json:"pregunta_id"`
			OpcionID   int64 `json:"opcion_id"`
		} `json:"respuestas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	if body.ExamenID <= 0 || body.EmpleadoID <= 0 {
		writeError(w, "BAD_REQUEST", "examen_id and empleado_id are required", http.StatusBadRequest)
		return
	}

	respuestasMap := make(map[int64]int64, len(body.Respuestas))
	for _, resp := range body.Respuestas {
		respuestasMap[resp.PreguntaID] = resp.OpcionID
	}

	resultado, err := h.svc.ResolverExamen(r.Context(), body.EmpleadoID, body.ExamenID, respuestasMap)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resultado})
}

// GetResultados handles GET /resultados-examen?examen_id=&empleado_id=
func (h *ExamHandler) GetResultados(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	examenID, ok := parseQueryInt64(w, r, "examen_id")
	if !ok {
		return
	}
	empleadoID, ok := parseQueryInt64(w, r, "empleado_id")
	if !ok {
		return
	}

	list, err := h.svc.GetResultados(r.Context(), examenID, empleadoID)
	if err != nil {
		writeCursosError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}
