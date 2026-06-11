package dashboardsdata

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers dashboards BFF endpoints on the provided chi router, protecting them with jwtAuth.
func RegisterRoutes(r chi.Router, h *HTTPHandler, jwtAuth func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(jwtAuth)
		r.Route("/api/v1/dashboards", func(r chi.Router) {
			r.Get("/admin-empresa/{idEmp}/evaluaciones-clientes", h.GetEvaluacionesClientes)
			r.Get("/admin-empresa/{idEmp}/empleados/count", h.GetEmpleadosCount)
			r.Get("/admin-empresa/{idEmp}/actividades/count", h.GetActividadesCount)
			r.Get("/admin-empresa/{idEmp}/ausencias/count", h.GetAusenciasCount)
			r.Get("/cliente/{idCliente}/servicios-localidad", h.GetServiciosLocalidad)
			r.Get("/cliente/{idCliente}/atencion-seguimiento/status", h.GetAtencionSeguimientoStatus)
			r.Get("/rh/{idEmp}/bolsa-trabajo/vacantes-activas", h.GetBolsaTrabajoVacantesActivas)
			r.Get("/rh/{idEmp}/empleados/absentismo", h.GetEmpleadosAbsentismo)
		})
	})
}
