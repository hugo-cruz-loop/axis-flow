// Package cursos defines domain types and sentinel errors for the Cursos module.
package cursos

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// CursoEstatus constants for cursos_curso.estatus.
const (
	CursoEstatusPublico  = 1 // visible to all enrolled employees
	CursoEstatusPrivado  = 2 // tenant-restricted
)

// EnrollEstatus constants for cursos_enroll_curso.estatus.
const (
	EnrollEstatusCompletado  = 1
	EnrollEstatusIncompleto  = 2
)

// LeccionTipo constants for cursos_leccion.tipo.
const (
	LeccionTipoVideo     = 1
	LeccionTipoTexto     = 2
	LeccionTipoDocumento = 3
)

// Sentinel errors for the cursos domain.
var (
	ErrCursoNotFound       = errors.New("curso not found")
	ErrNotEnrolled         = errors.New("employee is not enrolled in this course")
	ErrExamExceededAttempts = errors.New("exam attempt limit exceeded")
	ErrExamNotApproved     = errors.New("exam not approved — certificate not eligible")
	ErrTenantMismatch      = errors.New("tenant mismatch")
	ErrCursoDeleted        = errors.New("curso has been deleted")
)

// Categoria maps to cursos.cursos_categoria.
type Categoria struct {
	ID          int64     `json:"id"`
	Nombre      string    `json:"nombre"`
	Descripcion string    `json:"descripcion,omitempty"`
	EmpresaID   uuid.UUID `json:"empresa_id"`
}

// Modulo maps to cursos.cursos_modulo.
type Modulo struct {
	ID        int64     `json:"id"`
	Nombre    string    `json:"nombre"`
	EmpresaID uuid.UUID `json:"empresa_id"`
}

// Curso is the aggregate root — maps to cursos.cursos_curso.
type Curso struct {
	ID               int64      `json:"id"`
	Titulo           string     `json:"titulo"`
	Descripcion      string     `json:"descripcion,omitempty"`
	EmpresaID        uuid.UUID  `json:"empresa_id"`
	ModuloID         *int64     `json:"modulo_id,omitempty"`
	CategoriaID      int64      `json:"categoria_id"`
	Estatus          int16      `json:"estatus"`
	ImagenURL        string     `json:"imagen_url,omitempty"`
	DuracionMinutos  int        `json:"duracion_minutos"`
	PrerequisitoID   *int64     `json:"prerequisito_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// Unidad maps to cursos.cursos_unidad.
type Unidad struct {
	ID      int64  `json:"id"`
	CursoID int64  `json:"curso_id"`
	Titulo  string `json:"titulo"`
	Orden   int    `json:"orden"`
}

// Leccion maps to cursos.cursos_leccion.
type Leccion struct {
	ID           int64  `json:"id"`
	UnidadID     int64  `json:"unidad_id"`
	Titulo       string `json:"titulo"`
	Tipo         int16  `json:"tipo"`
	ContenidoURL string `json:"contenido_url,omitempty"`
	Orden        int    `json:"orden"`
}

// Nota maps to cursos.cursos_notas.
type Nota struct {
	ID         int64     `json:"id"`
	LeccionID  int64     `json:"leccion_id"`
	EmpleadoID int64     `json:"empleado_id"`
	Contenido  string    `json:"contenido"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Enrollment maps to cursos.cursos_enroll_curso.
type Enrollment struct {
	ID                int64      `json:"id"`
	CursoID           int64      `json:"curso_id"`
	EmpleadoID        int64      `json:"empleado_id"`
	EmpresaID         uuid.UUID  `json:"empresa_id"`
	AvancePorcentaje  float64    `json:"avance_porcentaje"`
	Estatus           int16      `json:"estatus"`
	EnrolledAt        time.Time  `json:"enrolled_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

// AvanceLeccion maps to cursos.cursos_avance_leccion.
type AvanceLeccion struct {
	EmpleadoID   int64     `json:"empleado_id"`
	LeccionID    int64     `json:"leccion_id"`
	CompletadoAt time.Time `json:"completado_at"`
}

// Examen maps to cursos.cursos_examen.
type Examen struct {
	ID              int64    `json:"id"`
	CursoID         int64    `json:"curso_id"`
	Titulo          string   `json:"titulo"`
	NoteMin         float64  `json:"note_min"`
	NumIntentos     int16    `json:"num_intentos"`
	TiempoLimiteMin *int     `json:"tiempo_limite_min,omitempty"`
	// Preguntas is populated by repository load helpers (admin path for grading).
	Preguntas []*ExamenPreguntaConOpciones `json:"preguntas,omitempty"`
}

// ExamenPreguntaConOpciones enriches ExamenPregunta with its admin opciones (es_correcta included).
// Used by the service grading path — never serialised to API responses directly.
type ExamenPreguntaConOpciones struct {
	ExamenPregunta
	Opciones []*ExamenOpcion `json:"opciones,omitempty"`
}

// ExamenPregunta maps to cursos.cursos_examen_pregunta.
type ExamenPregunta struct {
	ID        int64  `json:"id"`
	ExamenID  int64  `json:"examen_id"`
	Enunciado string `json:"enunciado"`
	Orden     int    `json:"orden"`
}

// ExamenOpcion maps to cursos.cursos_examen_opcion — includes es_correcta for internal use only.
type ExamenOpcion struct {
	ID         int64  `json:"id"`
	PreguntaID int64  `json:"pregunta_id"`
	Texto      string `json:"texto"`
	EsCorrecta bool   `json:"es_correcta"`
}

// ExamenOpcionStudent is the public view of an exam option served to employees.
// es_correcta is intentionally absent to prevent answer leakage.
type ExamenOpcionStudent struct {
	ID         int64  `json:"id"`
	PreguntaID int64  `json:"pregunta_id"`
	Texto      string `json:"texto"`
}

// ResultadoExamen maps to cursos.cursos_resultados_examen.
type ResultadoExamen struct {
	ID          int64      `json:"id"`
	ExamenID    int64      `json:"examen_id"`
	EmpleadoID  int64      `json:"empleado_id"`
	Calificacion *float64  `json:"calificacion,omitempty"`
	Aprobado    *bool      `json:"aprobado,omitempty"`
	Intento     int        `json:"intento"`
	CreatedAt   time.Time  `json:"created_at"`
}
