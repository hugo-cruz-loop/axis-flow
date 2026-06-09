// Package bolsatrabajo defines domain types, constants, and sentinel errors
// for the BolsaDeTrabajo (job board) module.
package bolsatrabajo

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// EstatusVacante constants for bolsa_trabajo.trabajos.estatus_vacante.
const (
	VacanteActivo = 1 // active — visible to applicants
	VacantePausa  = 2 // paused — not visible
	VacanteFin    = 3 // closed — no more applications
)

// EstatusPostulacion constants for bolsa_trabajo.postulaciones.estatus.
const (
	PostulacionPendiente  = 1 // received, awaiting review
	PostulacionEntrevista = 2 // invited for interview
	PostulacionOferta     = 3 // offer extended
	PostulacionContratado = 4 // hired
	PostulacionRechazado  = 5 // rejected
)

// Trabajo represents a job posting created by an empresa.
type Trabajo struct {
	ID             uuid.UUID
	EmpresaID      uuid.UUID
	Titulo         string
	Descripcion    string
	Requisitos     []string  // stored as jsonb array
	FechaCaducar   time.Time
	EstatusVacante int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Postulacion represents a candidate's application to a Trabajo.
type Postulacion struct {
	ID             uuid.UUID
	TrabajoID      uuid.UUID
	NombreCompleto string
	Email          string
	Telefono       string
	CvURL          string
	Estatus        int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Evaluacion holds the post-interview scorecard for a Postulacion.
type Evaluacion struct {
	ID            uuid.UUID
	PostulacionID uuid.UUID
	Puntualidad   int
	Cortesia      int
	SoftSkills    int
	Comentarios   string
	EvaluatorID   uuid.UUID
	CreatedAt     time.Time
}

// PipelineStats aggregates application counts per stage for a Trabajo.
type PipelineStats struct {
	TrabajoID uuid.UUID
	Stats     []StageStat
}

// StageStat holds the count of applications at a specific pipeline stage.
type StageStat struct {
	Stage     int
	StageName string
	Count     int
}

// Sentinel errors for the bolsatrabajo domain.
var (
	ErrNotFound     = errors.New("bolsatrabajo: not found")
	ErrForbidden    = errors.New("bolsatrabajo: forbidden")
	ErrInvalidFile  = errors.New("bolsatrabajo: invalid file type or size")
	ErrCaptchaFail  = errors.New("bolsatrabajo: captcha validation failed")
	ErrConflict     = errors.New("bolsatrabajo: conflict")
	ErrInvalidInput = errors.New("bolsatrabajo: invalid input")
)
