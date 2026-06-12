package main

import (
	"net/http"

	reportshandler "axis-flow-back/internal/reports/handler"

	"github.com/go-chi/chi/v5"
)

// registerReportsRoutes mounts all Reports REST routes under /api/v1/reports.
//
// Route matrix:
//
//	GET  /api/v1/reports/evidencias           JWTAuth → h.GetEvidencias
//	GET  /api/v1/reports/asistencias          JWTAuth → h.GetAsistencias
//	GET  /api/v1/reports/graficaevidencia     JWTAuth → h.GetGraficaEvidencia
//	GET  /api/v1/reports/countincidentes      JWTAuth → h.GetCountIncidentes
//	POST /api/v1/reports/geocoding/reverse    JWTAuth → h.ReverseGeocode
//
// Security: all endpoints require a valid JWT; empresa_id is ALWAYS extracted
// from JWT claims — never from query parameters.
func registerReportsRoutes(
	r chi.Router,
	h *reportshandler.ReportsHandler,
	jwtAuth func(http.Handler) http.Handler,
) {
	r.Route("/api/v1/reports", func(r chi.Router) {
		r.Use(jwtAuth)
		r.Get("/evidencias", h.GetEvidencias)
		r.Get("/asistencias", h.GetAsistencias)
		r.Get("/graficaevidencia", h.GetGraficaEvidencia)
		r.Get("/countincidentes", h.GetCountIncidentes)
		r.Post("/geocoding/reverse", h.ReverseGeocode)
	})
}
