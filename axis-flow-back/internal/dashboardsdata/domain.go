package dashboardsdata

import (
	"github.com/google/uuid"
)

// AdminEmpresaEvaluacionesResponse represents the response for evaluations query.
type AdminEmpresaEvaluacionesResponse struct {
	Data AdminEmpresaEvaluacionesData `json:"data"`
}

// AdminEmpresaEvaluacionesData holds evaluations grouped by client.
type AdminEmpresaEvaluacionesData struct {
	EmpresaID    int64              `json:"empresa_id"`
	Evaluaciones []ClientEvaluation `json:"evaluaciones"`
}

// ClientEvaluation represents the average score and count for a client.
type ClientEvaluation struct {
	ClientID           uuid.UUID `json:"cliente_id"`
	ClientNombre       string    `json:"cliente_nombre"`
	PromedioPuntuacion float64   `json:"promedio_puntuacion"`
	TotalEvaluaciones  int       `json:"total_evaluaciones"`
}

// AdminEmpresaEmpleadosCountResponse represents the response for active employees count query.
type AdminEmpresaEmpleadosCountResponse struct {
	Data AdminEmpresaEmpleadosCountData `json:"data"`
}

// AdminEmpresaEmpleadosCountData holds total count of active employees.
type AdminEmpresaEmpleadosCountData struct {
	EmpresaID      int64 `json:"empresa_id"`
	TotalEmpleados int   `json:"total_empleados"`
}

// AdminEmpresaActividadesCountResponse represents the response for completed activities count query.
type AdminEmpresaActividadesCountResponse struct {
	Data AdminEmpresaActividadesCountData `json:"data"`
}

// AdminEmpresaActividadesCountData holds total count of completed activities.
type AdminEmpresaActividadesCountData struct {
	EmpresaID                   int64  `json:"empresa_id"`
	TotalActividadesFinalizadas int    `json:"total_actividades_finalizadas"`
	Periodo                     string `json:"periodo"`
}

// AdminEmpresaAusenciasCountResponse represents the response for historical absences count query.
type AdminEmpresaAusenciasCountResponse struct {
	Data AdminEmpresaAusenciasCountData `json:"data"`
}

// AdminEmpresaAusenciasCountData holds total count of absences.
type AdminEmpresaAusenciasCountData struct {
	EmpresaID      int64 `json:"empresa_id"`
	TotalAusencias int   `json:"total_ausencias"`
	Year           *int  `json:"year,omitempty"`
}

// ClienteServiciosLocalidadResponse represents the response for services by locality query.
type ClienteServiciosLocalidadResponse struct {
	Data ClienteServiciosLocalidadData `json:"data"`
}

// ClienteServiciosLocalidadData holds localities and their active services.
type ClienteServiciosLocalidadData struct {
	ClientID    uuid.UUID           `json:"cliente_id"`
	Localidades []LocalidadServicio `json:"localidades"`
}

// LocalidadServicio represents a client location and its associated services.
type LocalidadServicio struct {
	LocalidadID     uuid.UUID  `json:"localidad_id"`
	LocalidadNombre string     `json:"localidad_nombre"`
	Servicios       []Servicio `json:"servicios"`
}

// Servicio represents service detail with count of assigned employees.
type Servicio struct {
	ServicioID         int64  `json:"servicio_id"`
	ServicioNombre     string `json:"servicio_nombre"`
	EmpleadosAsignados int    `json:"empleados_asignados"`
}

// ClienteAtencionSeguimientoStatusResponse represents the response for ticket status breakdown.
type ClienteAtencionSeguimientoStatusResponse struct {
	Data ClienteAtencionSeguimientoStatusData `json:"data"`
}

// ClienteAtencionSeguimientoStatusData holds ticket counts breakdown.
type ClienteAtencionSeguimientoStatusData struct {
	ClientID     uuid.UUID        `json:"cliente_id"`
	Tickets      TicketBreakdown  `json:"tickets"`
	TotalTickets int              `json:"total_tickets"`
}

// TicketBreakdown represents ticket counts grouped by status.
type TicketBreakdown struct {
	Pendiente  int `json:"pendiente"`
	EnProceso  int `json:"en_proceso"`
	Finalizado int `json:"finalizado"`
}

// RHTotalTrabajosActivosResponse represents the response for active vacancies query.
type RHTotalTrabajosActivosResponse struct {
	Data RHTotalTrabajosActivosData `json:"data"`
}

// RHTotalTrabajosActivosData holds active vacancies.
type RHTotalTrabajosActivosData struct {
	EmpresaID            int64     `json:"empresa_id"`
	TotalVacantesActivas int       `json:"total_vacantes_activas"`
	Vacantes             []Vacante `json:"vacantes"`
}

// Vacante represents a vacancy listing.
type Vacante struct {
	VacanteID        string `json:"vacante_id"`
	Titulo           string `json:"titulo"`
	Departamento     string `json:"departamento"`
	FechaPublicacion string `json:"fecha_publicacion"`
}

// RHEmpleadosAbsentismoResponse represents the response for absenteeism rate.
type RHEmpleadosAbsentismoResponse struct {
	Data RHEmpleadosAbsentismoData `json:"data"`
}

// RHEmpleadosAbsentismoData holds absenteeism statistics.
type RHEmpleadosAbsentismoData struct {
	EmpresaID             int64   `json:"empresa_id"`
	TasaAbsentismo        float64 `json:"tasa_absentismo"`
	DiasLaborablesTotales int     `json:"dias_laborables_totales"`
	TotalInasistencias    int     `json:"total_inasistencias"`
	EmpleadosAfectados    int     `json:"empleados_afectados"`
}
