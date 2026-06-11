// Package main — formularios_routes.go: HTTP route registration for
// the Formularios module.
//
// PR-4 (REST/HTTP) — task 4.4. Mirrors the structure of
// cmd/server/atencion_routes.go (PR-4 of 09_AtencionSeguimiento_Service_Spec).
//
// Mount prefix: /api/v1/formularios
//
// Route matrix (8 routes total):
//
//	POST   /formulario                                  JWTAuth + RequireRoles("Admin","Manager") → formH.CreateFormulario
//	GET    /formulario/byempresa/{id}                    JWTAuth → formH.GetFormulariosByEmpresa
//	POST   /pregunta                                    JWTAuth + RequireRoles("Admin") → formH.CreatePregunta
//	POST   /evento                                      JWTAuth + RequireRoles("Manager") → evH.CreateEvento
//	POST   /evento_iniciado                             JWTAuth → evH.IniciarEvento
//	GET    /evento/byEmpId/{empId}/{cteId}              JWTAuth → evH.GetEventosByEmpCte
//	POST   /respuesta                                   JWTAuth → respH.SubmitRespuesta
//	GET    /respuesta/reporte/pregunta/{id}/pdf         JWTAuth → respH.GetReportePDF
//
// PR-5 (5.4): the publisher, cache invalidator, pgx pool, and
// PDFRenderer/ReportStorage dependencies are wired via
// cmd/server/formularios_module.go's newFormulariosModule.
//
// PR-6 (6.4): the real PDFRenderer (Gotenberg HTTP), ReportStorage
// (S3 + IAM), and Locker (Redis SET-NX-with-TTL) are wired in
// formularios_module.go. The render pool (Celery equivalent) is
// also wired. No TODO(PR-6) markers remain in the module.
package main

import (
	"net/http"

	formulariosHandler "axis-flow-back/internal/formularios/handler"

	"github.com/go-chi/chi/v5"
)

// registerFormulariosRoutes mounts all formularios routes under
// /api/v1/formularios. The three handler instances are passed in
// directly (the orchestrator's PR-4 brief signature) so the test
// harness can wire stub services without polluting this file with
// test-only deps. main.go (PR-5) will pass the real handlers
// constructed from the production service stack.
func registerFormulariosRoutes(
	r chi.Router,
	jwtAuth func(http.Handler) http.Handler,
	requireRoles func(...string) func(http.Handler) http.Handler,
	formH *formulariosHandler.FormularioHandler,
	evH *formulariosHandler.EventoHandler,
	respH *formulariosHandler.RespuestaHandler,
) {
	r.Route("/api/v1/formularios", func(r chi.Router) {
		r.Use(jwtAuth)

		// ── Formulario ──────────────────────────────────────────
		// Spec says Admin / Manager can build templates. The
		// orchestrator's brief maps the openapi `Admin` to the
		// existing 09 role "Admin" and the openapi `Manager` to
		// "Manager" (a constructor that is anticipated in the
		// future RBAC catalog).
		r.With(requireRoles("Admin", "Manager")).Post("/formulario", formH.CreateFormulario)
		r.Get("/formulario/byempresa/{id}", formH.GetFormulariosByEmpresa)
		r.With(requireRoles("Admin")).Post("/pregunta", formH.CreatePregunta)

		// ── Evento ──────────────────────────────────────────────
		r.With(requireRoles("Manager")).Post("/evento", evH.CreateEvento)
		r.Post("/evento_iniciado", evH.IniciarEvento)
		r.Get("/evento/byEmpId/{empId}/{cteId}", evH.GetEventosByEmpCte)

		// ── Respuesta ──────────────────────────────────────────
		r.Post("/respuesta", respH.SubmitRespuesta)
		r.Get("/respuesta/reporte/pregunta/{id}/pdf", respH.GetReportePDF)
	})
}
